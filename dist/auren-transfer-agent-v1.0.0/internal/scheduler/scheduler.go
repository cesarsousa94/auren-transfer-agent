// Package scheduler contains foundation polling contracts for the worker engine.
package scheduler

import (
	"context"
	"fmt"
	"time"
)

// Task is called by a Scheduler tick.
type Task func(context.Context) error

// RunResult describes one scheduler tick.
type RunResult struct {
	StartedAt time.Time
	EndedAt   time.Time
	Duration  time.Duration
	Error     string
}

// FixedIntervalScheduler runs one task at a stable interval.
type FixedIntervalScheduler struct {
	interval time.Duration
	task     Task
	now      func() time.Time
}

// NewFixedInterval creates a scheduler that can be run manually or as a loop.
func NewFixedInterval(interval time.Duration, task Task) (*FixedIntervalScheduler, error) {
	if interval <= 0 {
		return nil, fmt.Errorf("scheduler interval must be greater than zero")
	}
	if task == nil {
		return nil, fmt.Errorf("scheduler task cannot be nil")
	}
	return &FixedIntervalScheduler{interval: interval, task: task, now: func() time.Time { return time.Now().UTC() }}, nil
}

// Interval returns the configured tick interval.
func (scheduler *FixedIntervalScheduler) Interval() time.Duration {
	if scheduler == nil {
		return 0
	}
	return scheduler.interval
}

// RunOnce executes one scheduler tick immediately.
func (scheduler *FixedIntervalScheduler) RunOnce(ctx context.Context) RunResult {
	if scheduler == nil {
		return RunResult{Error: "scheduler cannot be nil"}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	started := scheduler.now().UTC()
	err := scheduler.task(ctx)
	ended := scheduler.now().UTC()
	result := RunResult{StartedAt: started, EndedAt: ended, Duration: ended.Sub(started)}
	if err != nil {
		result.Error = err.Error()
	}
	return result
}

// Start runs ticks until ctx is canceled and emits each result on a channel.
func (scheduler *FixedIntervalScheduler) Start(ctx context.Context) (<-chan RunResult, error) {
	if scheduler == nil {
		return nil, fmt.Errorf("scheduler cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	results := make(chan RunResult, 1)
	go func() {
		defer close(results)
		ticker := time.NewTicker(scheduler.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				results <- scheduler.RunOnce(ctx)
			}
		}
	}()
	return results, nil
}
