package worker

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// DefaultWorkerID is used when a foundation worker receives no explicit id.
	DefaultWorkerID = "worker-1"
)

var (
	// ErrNoJob means a worker poll found no queued work.
	ErrNoJob = errors.New("no queued job available")
)

// JobQueue is the minimum queue capability required by a Worker.
type JobQueue interface {
	Dequeue(context.Context) (Job, bool, error)
}

// Handler executes one mechanical job already claimed by a Worker.
type Handler interface {
	HandleJob(context.Context, Job) (HandlerResult, error)
}

// HandlerFunc adapts a function into a Handler.
type HandlerFunc func(context.Context, Job) (HandlerResult, error)

// HandleJob executes fn.
func (fn HandlerFunc) HandleJob(ctx context.Context, job Job) (HandlerResult, error) {
	if fn == nil {
		return HandlerResult{}, fmt.Errorf("handler function cannot be nil")
	}
	return fn(ctx, job)
}

// HandlerResult is the mechanical result returned by a worker handler.
type HandlerResult struct {
	Status   JobStatus
	Message  string
	Metadata map[string]string
	Now      time.Time
}

// RunResult summarizes one worker poll/execution cycle.
type RunResult struct {
	WorkerID  string
	Executed  bool
	Job       Job
	StartedAt time.Time
	EndedAt   time.Time
	Duration  time.Duration
	Error     string
}

// WorkerOptions configure a foundation Worker.
type WorkerOptions struct {
	ID      string
	Queue   JobQueue
	Handler Handler
	Now     func() time.Time
}

// Worker claims one queued job at a time and delegates execution to a Handler.
type Worker struct {
	id      string
	queue   JobQueue
	handler Handler
	now     func() time.Time
}

// NewWorker creates a validated foundation Worker.
func NewWorker(options WorkerOptions) (*Worker, error) {
	workerID := NormalizeWorkerID(options.ID)
	if workerID == "" {
		workerID = DefaultWorkerID
	}
	if options.Queue == nil {
		return nil, fmt.Errorf("worker queue cannot be nil")
	}
	if options.Handler == nil {
		return nil, fmt.Errorf("worker handler cannot be nil")
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Worker{id: workerID, queue: options.Queue, handler: options.Handler, now: now}, nil
}

// ID returns the stable local worker id.
func (worker *Worker) ID() string {
	if worker == nil {
		return ""
	}
	return worker.id
}

// RunOnce polls the queue once and executes at most one job.
func (worker *Worker) RunOnce(ctx context.Context) (RunResult, error) {
	if worker == nil {
		return RunResult{}, fmt.Errorf("worker cannot be nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return RunResult{WorkerID: worker.id}, err
	}

	queued, ok, err := worker.queue.Dequeue(ctx)
	if err != nil {
		return RunResult{WorkerID: worker.id, Error: err.Error()}, err
	}
	if !ok {
		return RunResult{WorkerID: worker.id}, nil
	}

	started := worker.now().UTC()
	running, err := queued.WithAttemptStatus(JobStatusRunning, started)
	if err != nil {
		return RunResult{WorkerID: worker.id, Executed: true, Job: queued.Clone(), StartedAt: started, EndedAt: started, Error: err.Error()}, err
	}

	handlerResult, handlerErr := worker.handler.HandleJob(ctx, running.Clone())
	ended := handlerResult.Now.UTC()
	if ended.IsZero() {
		ended = worker.now().UTC()
	}

	status := handlerResult.Status
	if status == "" {
		status = JobStatusSucceeded
	}
	if handlerErr != nil {
		status = JobStatusFailed
	}
	if !IsTerminalJobStatus(status) {
		status = JobStatusFailed
		if handlerErr == nil {
			handlerErr = fmt.Errorf("handler returned non-terminal status %q", handlerResult.Status)
		}
	}

	final := running.Clone()
	final.Status = status
	final.UpdatedAt = ended
	if len(handlerResult.Metadata) > 0 {
		if final.Metadata == nil {
			final.Metadata = map[string]string{}
		}
		for key, value := range handlerResult.Metadata {
			trimmedKey := strings.TrimSpace(key)
			if trimmedKey != "" {
				final.Metadata[trimmedKey] = value
			}
		}
	}
	if strings.TrimSpace(handlerResult.Message) != "" {
		if final.Metadata == nil {
			final.Metadata = map[string]string{}
		}
		final.Metadata["worker_message"] = strings.TrimSpace(handlerResult.Message)
	}
	if err := ValidateJob(final); err != nil {
		return RunResult{WorkerID: worker.id, Executed: true, Job: running.Clone(), StartedAt: started, EndedAt: ended, Duration: ended.Sub(started), Error: err.Error()}, err
	}

	runResult := RunResult{WorkerID: worker.id, Executed: true, Job: final.Clone(), StartedAt: started, EndedAt: ended, Duration: ended.Sub(started)}
	if handlerErr != nil {
		runResult.Error = handlerErr.Error()
		return runResult, handlerErr
	}
	return runResult, nil
}

// NormalizeWorkerID trims and canonicalizes a local worker id.
func NormalizeWorkerID(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

// NoopHandler returns a deterministic handler that marks jobs as succeeded.
func NoopHandler() Handler {
	return HandlerFunc(func(context.Context, Job) (HandlerResult, error) {
		return HandlerResult{Status: JobStatusSucceeded, Message: "noop"}, nil
	})
}
