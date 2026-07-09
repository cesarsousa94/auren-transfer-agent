package dispatcher

import (
	"context"
	"fmt"
	"time"

	"github.com/auren/auren-transfer-agent/internal/worker"
)

// PoolRunner is the minimum pool capability required by the dispatcher.
type PoolRunner interface {
	RunOnce(context.Context) ([]worker.RunResult, error)
	Stats() worker.PoolStats
}

// RetryQueue is the minimum queue capability required for mechanical retries.
type RetryQueue interface {
	Enqueue(context.Context, worker.Job) error
	Len() int
	Capacity() int
}

// DispatchResult summarizes one dispatcher cycle.
type DispatchResult struct {
	StartedAt     time.Time
	EndedAt       time.Time
	Duration      time.Duration
	PoolStats     worker.PoolStats
	WorkerResults []worker.RunResult
	Executed      int
	Succeeded     int
	Failed        int
	Retried       int
	QueueLength   int
	QueueCapacity int
	Error         string
}

// Options configure a foundation Dispatcher.
type Options struct {
	Pool        PoolRunner
	RetryQueue  RetryQueue
	RetryPolicy RetryPolicy
	Now         func() time.Time
}

// Dispatcher coordinates one worker-pool polling cycle and optional retries.
type Dispatcher struct {
	pool        PoolRunner
	retryQueue  RetryQueue
	retryPolicy RetryPolicy
	now         func() time.Time
}

// New creates a dispatcher with validated foundation dependencies.
func New(options Options) (*Dispatcher, error) {
	if options.Pool == nil {
		return nil, fmt.Errorf("dispatcher pool cannot be nil")
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	policy := options.RetryPolicy
	if policy == nil {
		policy = NewAttemptsRetryPolicy()
	}
	return &Dispatcher{pool: options.Pool, retryQueue: options.RetryQueue, retryPolicy: policy, now: now}, nil
}

// RunOnce executes one dispatcher cycle.
func (dispatcher *Dispatcher) RunOnce(ctx context.Context) (DispatchResult, error) {
	if dispatcher == nil {
		return DispatchResult{}, fmt.Errorf("dispatcher cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	started := dispatcher.now().UTC()
	results, runErr := dispatcher.pool.RunOnce(ctx)
	ended := dispatcher.now().UTC()

	output := DispatchResult{
		StartedAt:     started,
		EndedAt:       ended,
		Duration:      ended.Sub(started),
		PoolStats:     dispatcher.pool.Stats(),
		WorkerResults: cloneRunResults(results),
	}
	if runErr != nil {
		output.Error = runErr.Error()
	}

	for _, result := range results {
		if !result.Executed {
			continue
		}
		output.Executed++
		switch result.Job.Status {
		case worker.JobStatusSucceeded:
			output.Succeeded++
		case worker.JobStatusFailed:
			output.Failed++
			if dispatcher.retryQueue != nil {
				decision, err := dispatcher.retryPolicy.Decide(result.Job, ended)
				if err != nil {
					output.Error = err.Error()
					return output, err
				}
				if decision.Retry {
					if err := dispatcher.retryQueue.Enqueue(ctx, decision.Job); err != nil {
						output.Error = err.Error()
						return output, err
					}
					output.Retried++
				}
			}
		}
	}

	if dispatcher.retryQueue != nil {
		output.QueueLength = dispatcher.retryQueue.Len()
		output.QueueCapacity = dispatcher.retryQueue.Capacity()
	}
	return output, runErr
}

func cloneRunResults(results []worker.RunResult) []worker.RunResult {
	if results == nil {
		return nil
	}
	cloned := make([]worker.RunResult, len(results))
	for index, result := range results {
		cloned[index] = result
		cloned[index].Job = result.Job.Clone()
	}
	return cloned
}
