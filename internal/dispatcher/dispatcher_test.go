package dispatcher

import (
	"context"
	"errors"
	"testing"

	"github.com/cesarsousa94/auren-transfer-agent/internal/queue"
	"github.com/cesarsousa94/auren-transfer-agent/internal/worker"
)

func dispatcherJob(t *testing.T, destination string, maxAttempts int) worker.Job {
	t.Helper()
	job, err := worker.NewJob(worker.JobInput{SourceURL: "https://example.com/source.ts", DestinationKey: destination, MaxAttempts: maxAttempts})
	if err != nil {
		t.Fatalf("NewJob returned error: %v", err)
	}
	return job
}

func TestDispatcherRunOnceSummarizesSucceededJobs(t *testing.T) {
	q, err := queue.NewMemoryQueue(4)
	if err != nil {
		t.Fatalf("NewMemoryQueue returned error: %v", err)
	}
	if err := q.Enqueue(context.Background(), dispatcherJob(t, "ok.ts", 1)); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	pool, err := worker.NewPool(worker.PoolOptions{Concurrency: 1, Queue: q, Handler: worker.NoopHandler()})
	if err != nil {
		t.Fatalf("NewPool returned error: %v", err)
	}
	dispatcher, err := New(Options{Pool: pool, RetryQueue: q})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	result, err := dispatcher.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Executed != 1 || result.Succeeded != 1 || result.Failed != 0 || result.Retried != 0 || result.QueueLength != 0 {
		t.Fatalf("unexpected dispatch result: %#v", result)
	}
	result.WorkerResults[0].Job.Metadata = map[string]string{"mutated": "yes"}
	again, _ := dispatcher.RunOnce(context.Background())
	if len(again.WorkerResults) != 1 || again.WorkerResults[0].Executed {
		t.Fatalf("expected defensive previous result and idle second run, got %#v", again)
	}
}

func TestDispatcherRequeuesFailedJobWhenEligible(t *testing.T) {
	q, err := queue.NewMemoryQueue(4)
	if err != nil {
		t.Fatalf("NewMemoryQueue returned error: %v", err)
	}
	if err := q.Enqueue(context.Background(), dispatcherJob(t, "retry.ts", 2)); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	expected := errors.New("temporary failure")
	pool, err := worker.NewPool(worker.PoolOptions{Concurrency: 1, Queue: q, Handler: worker.HandlerFunc(func(context.Context, worker.Job) (worker.HandlerResult, error) {
		return worker.HandlerResult{Status: worker.JobStatusFailed}, expected
	})})
	if err != nil {
		t.Fatalf("NewPool returned error: %v", err)
	}
	dispatcher, err := New(Options{Pool: pool, RetryQueue: q})
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	result, err := dispatcher.RunOnce(context.Background())
	if !errors.Is(err, expected) {
		t.Fatalf("expected worker error, got %v", err)
	}
	if result.Executed != 1 || result.Failed != 1 || result.Retried != 1 || q.Len() != 1 {
		t.Fatalf("expected one retry, result=%#v queued=%d", result, q.Len())
	}
	requeued := q.Snapshot()[0]
	if requeued.Attempt != 1 || requeued.Status != worker.JobStatusQueued || requeued.Metadata["retry_reason"] != RetryReasonEligible {
		t.Fatalf("unexpected requeued job: %#v", requeued)
	}
}

func TestNewRejectsNilPool(t *testing.T) {
	if _, err := New(Options{}); err == nil {
		t.Fatalf("expected nil pool error")
	}
}
