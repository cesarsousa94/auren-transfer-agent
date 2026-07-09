// Package queue contains foundation queue contracts for worker jobs.
package queue

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/auren/auren-transfer-agent/internal/worker"
)

const (
	// MemoryDriver is the foundation in-process queue driver.
	MemoryDriver = "memory"
)

var (
	// ErrQueueFull is returned when a bounded queue reaches capacity.
	ErrQueueFull = errors.New("queue is full")

	// ErrQueueClosed is returned when enqueue or dequeue is attempted after Close.
	ErrQueueClosed = errors.New("queue is closed")
)

// Queue is the foundation worker queue contract.
type Queue interface {
	Enqueue(context.Context, worker.Job) error
	Dequeue(context.Context) (worker.Job, bool, error)
	Len() int
	Capacity() int
	Snapshot() []worker.Job
	Close() error
}

// MemoryQueue is a bounded FIFO in-memory queue used by the v0.1.x foundation.
type MemoryQueue struct {
	mu       sync.Mutex
	capacity int
	jobs     []worker.Job
	closed   bool
}

// NewMemoryQueue creates a bounded FIFO memory queue.
func NewMemoryQueue(capacity int) (*MemoryQueue, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("memory queue capacity must be greater than zero")
	}
	return &MemoryQueue{capacity: capacity, jobs: make([]worker.Job, 0, capacity)}, nil
}

// Enqueue validates and appends a job to the queue.
func (queue *MemoryQueue) Enqueue(ctx context.Context, job worker.Job) error {
	if queue == nil {
		return fmt.Errorf("queue cannot be nil")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := worker.ValidateJob(job); err != nil {
		return err
	}
	if job.Status != worker.JobStatusPending && job.Status != worker.JobStatusQueued {
		return fmt.Errorf("queue only accepts pending or queued jobs")
	}

	queued, err := job.WithStatus(worker.JobStatusQueued, time.Now().UTC())
	if err != nil {
		return err
	}

	queue.mu.Lock()
	defer queue.mu.Unlock()

	if queue.closed {
		return ErrQueueClosed
	}
	if len(queue.jobs) >= queue.capacity {
		return ErrQueueFull
	}

	queue.jobs = append(queue.jobs, queued.Clone())
	return nil
}

// Dequeue removes and returns the oldest queued job.
func (queue *MemoryQueue) Dequeue(ctx context.Context) (worker.Job, bool, error) {
	if queue == nil {
		return worker.Job{}, false, fmt.Errorf("queue cannot be nil")
	}
	if err := ctx.Err(); err != nil {
		return worker.Job{}, false, err
	}

	queue.mu.Lock()
	defer queue.mu.Unlock()

	if queue.closed {
		return worker.Job{}, false, ErrQueueClosed
	}
	if len(queue.jobs) == 0 {
		return worker.Job{}, false, nil
	}

	job := queue.jobs[0].Clone()
	copy(queue.jobs, queue.jobs[1:])
	queue.jobs[len(queue.jobs)-1] = worker.Job{}
	queue.jobs = queue.jobs[:len(queue.jobs)-1]
	return job, true, nil
}

// Len returns the current number of queued jobs.
func (queue *MemoryQueue) Len() int {
	if queue == nil {
		return 0
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return len(queue.jobs)
}

// Capacity returns the configured queue capacity.
func (queue *MemoryQueue) Capacity() int {
	if queue == nil {
		return 0
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	return queue.capacity
}

// Snapshot returns defensive copies of all queued jobs in FIFO order.
func (queue *MemoryQueue) Snapshot() []worker.Job {
	if queue == nil {
		return nil
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()

	output := make([]worker.Job, len(queue.jobs))
	for index, job := range queue.jobs {
		output[index] = job.Clone()
	}
	return output
}

// Close prevents further queue operations.
func (queue *MemoryQueue) Close() error {
	if queue == nil {
		return fmt.Errorf("queue cannot be nil")
	}
	queue.mu.Lock()
	defer queue.mu.Unlock()
	queue.closed = true
	return nil
}
