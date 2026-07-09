package queue

import (
	"context"
	"errors"
	"testing"

	"github.com/auren/auren-transfer-agent/internal/worker"
)

func testJob(t *testing.T, destination string) worker.Job {
	t.Helper()
	job, err := worker.NewJob(worker.JobInput{SourceURL: "https://example.com/source.ts", DestinationKey: destination})
	if err != nil {
		t.Fatalf("NewJob returned error: %v", err)
	}
	return job
}

func TestMemoryQueueEnqueueDequeueFIFO(t *testing.T) {
	q, err := NewMemoryQueue(2)
	if err != nil {
		t.Fatalf("NewMemoryQueue returned error: %v", err)
	}
	first := testJob(t, "first.ts")
	second := testJob(t, "second.ts")
	if err := q.Enqueue(context.Background(), first); err != nil {
		t.Fatalf("enqueue first: %v", err)
	}
	if err := q.Enqueue(context.Background(), second); err != nil {
		t.Fatalf("enqueue second: %v", err)
	}
	dequeued, ok, err := q.Dequeue(context.Background())
	if err != nil || !ok {
		t.Fatalf("dequeue first: ok=%t err=%v", ok, err)
	}
	if dequeued.ID != first.ID || dequeued.Status != worker.JobStatusQueued {
		t.Fatalf("unexpected first dequeue: %#v", dequeued)
	}
	dequeued, ok, err = q.Dequeue(context.Background())
	if err != nil || !ok {
		t.Fatalf("dequeue second: ok=%t err=%v", ok, err)
	}
	if dequeued.ID != second.ID {
		t.Fatalf("expected second job, got %s", dequeued.ID)
	}
}

func TestMemoryQueueCapacity(t *testing.T) {
	q, err := NewMemoryQueue(1)
	if err != nil {
		t.Fatalf("NewMemoryQueue returned error: %v", err)
	}
	if err := q.Enqueue(context.Background(), testJob(t, "one.ts")); err != nil {
		t.Fatalf("first enqueue failed: %v", err)
	}
	if err := q.Enqueue(context.Background(), testJob(t, "two.ts")); !errors.Is(err, ErrQueueFull) {
		t.Fatalf("expected ErrQueueFull, got %v", err)
	}
}

func TestMemoryQueueSnapshotIsDefensive(t *testing.T) {
	q, err := NewMemoryQueue(2)
	if err != nil {
		t.Fatalf("NewMemoryQueue returned error: %v", err)
	}
	job := testJob(t, "copy.ts")
	job.Metadata = map[string]string{"original": "yes"}
	if err := q.Enqueue(context.Background(), job); err != nil {
		t.Fatalf("enqueue failed: %v", err)
	}
	snapshot := q.Snapshot()
	snapshot[0].Metadata["original"] = "changed"
	again := q.Snapshot()
	if again[0].Metadata["original"] != "yes" {
		t.Fatalf("snapshot mutated queue state")
	}
}

func TestMemoryQueueClose(t *testing.T) {
	q, err := NewMemoryQueue(1)
	if err != nil {
		t.Fatalf("NewMemoryQueue returned error: %v", err)
	}
	if err := q.Close(); err != nil {
		t.Fatalf("close failed: %v", err)
	}
	if err := q.Enqueue(context.Background(), testJob(t, "closed.ts")); !errors.Is(err, ErrQueueClosed) {
		t.Fatalf("expected ErrQueueClosed, got %v", err)
	}
	if _, _, err := q.Dequeue(context.Background()); !errors.Is(err, ErrQueueClosed) {
		t.Fatalf("expected ErrQueueClosed on dequeue, got %v", err)
	}
}

func TestMemoryQueueHonorsContextCancellation(t *testing.T) {
	q, err := NewMemoryQueue(1)
	if err != nil {
		t.Fatalf("NewMemoryQueue returned error: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := q.Enqueue(ctx, testJob(t, "cancel.ts")); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}
