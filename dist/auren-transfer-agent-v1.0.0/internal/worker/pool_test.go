package worker

import (
	"context"
	"testing"
)

func TestPoolRunOnceRespectsConcurrency(t *testing.T) {
	q := newTestQueue(workerTestJob(t, "a.ts"), workerTestJob(t, "b.ts"), workerTestJob(t, "c.ts"))
	pool, err := NewPool(PoolOptions{Concurrency: 2, Queue: q, Handler: NoopHandler()})
	if err != nil {
		t.Fatalf("NewPool returned error: %v", err)
	}
	results, err := pool.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce returned error: %v", err)
	}
	executed := 0
	for _, result := range results {
		if result.Executed {
			executed++
		}
	}
	if executed != 2 || q.Len() != 1 {
		t.Fatalf("expected two executions and one queued job, executed=%d queued=%d", executed, q.Len())
	}
}

func TestPoolStatsAreDefensive(t *testing.T) {
	pool, err := NewPool(PoolOptions{Concurrency: 2, Queue: newTestQueue(), Handler: NoopHandler()})
	if err != nil {
		t.Fatalf("NewPool returned error: %v", err)
	}
	stats := pool.Stats()
	stats.WorkerIDs[0] = "mutated"
	again := pool.Stats()
	if again.WorkerIDs[0] == "mutated" || again.Concurrency != 2 {
		t.Fatalf("stats were not defensive: %#v", again)
	}
}

func TestNewPoolRejectsInvalidOptions(t *testing.T) {
	if _, err := NewPool(PoolOptions{Concurrency: 0, Handler: NoopHandler()}); err == nil {
		t.Fatalf("expected invalid concurrency error")
	}
}
