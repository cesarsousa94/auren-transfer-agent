package worker

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

type testQueue struct {
	mu   sync.Mutex
	jobs []Job
}

func newTestQueue(jobs ...Job) *testQueue {
	queue := &testQueue{}
	for _, job := range jobs {
		queued, _ := job.WithStatus(JobStatusQueued, time.Now().UTC())
		queue.jobs = append(queue.jobs, queued)
	}
	return queue
}

func (queue *testQueue) Dequeue(ctx context.Context) (Job, bool, error) {
	if err := ctx.Err(); err != nil {
		return Job{}, false, err
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	if len(queue.jobs) == 0 {
		return Job{}, false, nil
	}
	job := queue.jobs[0]
	queue.jobs = queue.jobs[1:]
	return job.Clone(), true, nil
}

func (queue *testQueue) Len() int {
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return len(queue.jobs)
}

func workerTestJob(t *testing.T, destination string) Job {
	t.Helper()
	job, err := NewJob(JobInput{SourceURL: "https://example.com/video.ts", DestinationKey: destination, MaxAttempts: 2})
	if err != nil {
		t.Fatalf("NewJob returned error: %v", err)
	}
	return job
}

func TestWorkerRunOnceExecutesQueuedJob(t *testing.T) {
	q := newTestQueue(workerTestJob(t, "one.ts"))
	worker, err := NewWorker(WorkerOptions{Queue: q, Handler: NoopHandler()})
	if err != nil {
		t.Fatalf("NewWorker returned error: %v", err)
	}
	result, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if !result.Executed || result.Job.Status != JobStatusSucceeded || result.Job.Attempt != 1 {
		t.Fatalf("unexpected result: %#v", result)
	}
	if result.Job.Metadata["worker_message"] != "noop" {
		t.Fatalf("expected noop metadata, got %#v", result.Job.Metadata)
	}
}

func TestWorkerRunOnceEmptyQueueIsNotError(t *testing.T) {
	worker, err := NewWorker(WorkerOptions{Queue: newTestQueue(), Handler: NoopHandler()})
	if err != nil {
		t.Fatalf("NewWorker returned error: %v", err)
	}
	result, err := worker.RunOnce(context.Background())
	if err != nil || result.Executed {
		t.Fatalf("expected idle result without error, got result=%#v err=%v", result, err)
	}
}

func TestWorkerMarksHandlerErrorAsFailed(t *testing.T) {
	expected := errors.New("handler failed")
	worker, err := NewWorker(WorkerOptions{Queue: newTestQueue(workerTestJob(t, "failed.ts")), Handler: HandlerFunc(func(context.Context, Job) (HandlerResult, error) {
		return HandlerResult{Status: JobStatusSucceeded}, expected
	})})
	if err != nil {
		t.Fatalf("NewWorker returned error: %v", err)
	}
	result, err := worker.RunOnce(context.Background())
	if !errors.Is(err, expected) {
		t.Fatalf("expected handler error, got %v", err)
	}
	if result.Job.Status != JobStatusFailed || result.Error == "" {
		t.Fatalf("expected failed result, got %#v", result)
	}
}

func TestWorkerUsesHandlerMetadataDefensively(t *testing.T) {
	worker, err := NewWorker(WorkerOptions{Queue: newTestQueue(workerTestJob(t, "metadata.ts")), Handler: HandlerFunc(func(context.Context, Job) (HandlerResult, error) {
		return HandlerResult{Status: JobStatusSucceeded, Metadata: map[string]string{"phase": "5.3"}, Now: time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC)}, nil
	})})
	if err != nil {
		t.Fatalf("NewWorker returned error: %v", err)
	}
	result, err := worker.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	if result.Job.Metadata["phase"] != "5.3" || result.EndedAt.IsZero() {
		t.Fatalf("metadata/end time missing: %#v", result)
	}
}
