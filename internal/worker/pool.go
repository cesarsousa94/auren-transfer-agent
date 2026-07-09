package worker

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PoolOptions configure a bounded local worker pool.
type PoolOptions struct {
	Concurrency int
	Queue       JobQueue
	Handler     Handler
	Now         func() time.Time
}

// PoolStats summarizes a foundation worker pool.
type PoolStats struct {
	Concurrency int
	WorkerIDs   []string
}

// Pool executes bounded worker polls using a fixed set of workers.
type Pool struct {
	workers []*Worker
}

// NewPool builds a worker pool with deterministic worker ids.
func NewPool(options PoolOptions) (*Pool, error) {
	if options.Concurrency <= 0 {
		return nil, fmt.Errorf("worker pool concurrency must be greater than zero")
	}
	workers := make([]*Worker, 0, options.Concurrency)
	for index := 1; index <= options.Concurrency; index++ {
		worker, err := NewWorker(WorkerOptions{ID: fmt.Sprintf("worker-%d", index), Queue: options.Queue, Handler: options.Handler, Now: options.Now})
		if err != nil {
			return nil, err
		}
		workers = append(workers, worker)
	}
	return &Pool{workers: workers}, nil
}

// Size returns the number of workers configured in the pool.
func (pool *Pool) Size() int {
	if pool == nil {
		return 0
	}
	return len(pool.workers)
}

// WorkerIDs returns a defensive list of local worker ids.
func (pool *Pool) WorkerIDs() []string {
	if pool == nil {
		return nil
	}
	ids := make([]string, len(pool.workers))
	for index, worker := range pool.workers {
		ids[index] = worker.ID()
	}
	return ids
}

// Stats returns a defensive snapshot of the pool configuration.
func (pool *Pool) Stats() PoolStats {
	return PoolStats{Concurrency: pool.Size(), WorkerIDs: pool.WorkerIDs()}
}

// RunOnce asks each worker to poll once, bounded by pool size.
func (pool *Pool) RunOnce(ctx context.Context) ([]RunResult, error) {
	if pool == nil {
		return nil, fmt.Errorf("worker pool cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	results := make([]RunResult, len(pool.workers))
	errors := make([]error, len(pool.workers))

	var wait sync.WaitGroup
	for index, worker := range pool.workers {
		wait.Add(1)
		go func(index int, worker *Worker) {
			defer wait.Done()
			results[index], errors[index] = worker.RunOnce(ctx)
		}(index, worker)
	}
	wait.Wait()

	for _, err := range errors {
		if err != nil {
			return results, err
		}
	}
	return results, nil
}
