package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/heartbeat"
	workerqueue "github.com/cesarsousa94/auren-transfer-agent/internal/queue"
	"github.com/cesarsousa94/auren-transfer-agent/internal/runtime"
	"github.com/cesarsousa94/auren-transfer-agent/internal/worker"
)

const (
	// WorkerRouteName is the canonical route name for the worker status endpoint.
	WorkerRouteName = "worker.status"

	// WorkerJobsListRouteName is the canonical route name for the worker job list endpoint.
	WorkerJobsListRouteName = "worker.jobs.index"

	// WorkerJobsCreateRouteName is the canonical route name for the worker job creation endpoint.
	WorkerJobsCreateRouteName = "worker.jobs.create"

	// WorkerPath is the canonical REST path for worker status.
	WorkerPath = "/worker"

	// WorkerJobsPath is the canonical REST path for queued worker jobs.
	WorkerJobsPath = "/worker/jobs"

	// WorkerAPIStatus is the stable status value returned by worker REST endpoints.
	WorkerAPIStatus = "ok"
)

// WorkerQueue exposes the queue capabilities used by the worker REST API.
type WorkerQueue interface {
	Enqueue(ctx context.Context, job worker.Job) error
	Len() int
	Capacity() int
	Snapshot() []worker.Job
}

// QueuePersister saves queued jobs after REST mutations.
type QueuePersister interface {
	Save(ctx context.Context, driver string, jobs []worker.Job) (workerqueue.StoreResult, error)
}

// WorkerAPIOptions configures the foundation worker REST routes.
type WorkerAPIOptions struct {
	Info      runtime.VersionInfo
	Heartbeat heartbeat.Record
	Queue     WorkerQueue
	Driver    string
	Persister QueuePersister
}

// WorkerQueueStats is the queue state rendered by worker REST responses.
type WorkerQueueStats struct {
	Driver   string `json:"driver"`
	Length   int    `json:"length"`
	Capacity int    `json:"capacity"`
}

// WorkerResponse is the stable foundation worker status payload.
type WorkerResponse struct {
	Status        string           `json:"status"`
	Name          string           `json:"name"`
	Version       string           `json:"version"`
	RuntimeStatus string           `json:"runtime_status"`
	Router        string           `json:"router"`
	Heartbeat     heartbeat.Record `json:"heartbeat"`
	Queue         WorkerQueueStats `json:"queue"`
	GeneratedAt   time.Time        `json:"generated_at"`
}

// WorkerJobsResponse lists currently queued foundation jobs.
type WorkerJobsResponse struct {
	Status string           `json:"status"`
	Queue  WorkerQueueStats `json:"queue"`
	Jobs   []worker.Job     `json:"jobs"`
}

// CreateJobRequest is the REST input for a mechanical transfer job.
type CreateJobRequest struct {
	ID             string            `json:"id"`
	ExternalID     string            `json:"external_id"`
	Type           worker.JobType    `json:"type"`
	Priority       int               `json:"priority"`
	MaxAttempts    int               `json:"max_attempts"`
	SourceURL      string            `json:"source_url"`
	DestinationKey string            `json:"destination_key"`
	Headers        map[string]string `json:"headers"`
	Metadata       map[string]string `json:"metadata"`
}

// CreateJobResponse is returned after a job is accepted into the local queue.
type CreateJobResponse struct {
	Status      string                   `json:"status"`
	Accepted    bool                     `json:"accepted"`
	Job         worker.Job               `json:"job"`
	Queue       WorkerQueueStats         `json:"queue"`
	Persistence *workerqueue.StoreResult `json:"persistence,omitempty"`
}

// ErrorResponse is the stable JSON error payload for worker REST endpoints.
type ErrorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// WorkerHandler returns worker status based on current local queue state.
func WorkerHandler(options WorkerAPIOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		queueStats := workerQueueStats(options)
		payload := WorkerResponse{
			Status:        WorkerAPIStatus,
			Name:          options.Info.Name,
			Version:       options.Info.Version,
			RuntimeStatus: options.Info.Status,
			Router:        RouterKindName(),
			Heartbeat:     options.Heartbeat.Clone(),
			Queue:         queueStats,
			GeneratedAt:   time.Now().UTC(),
		}
		writeJSON(writer, http.StatusOK, payload)
		_ = request
	}
}

// WorkerJobsHandler returns queued jobs without mutating queue state.
func WorkerJobsHandler(options WorkerAPIOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if options.Queue == nil {
			writeWorkerError(writer, http.StatusServiceUnavailable, "worker queue is unavailable")
			return
		}
		payload := WorkerJobsResponse{Status: WorkerAPIStatus, Queue: workerQueueStats(options), Jobs: cloneWorkerJobs(options.Queue.Snapshot())}
		writeJSON(writer, http.StatusOK, payload)
		_ = request
	}
}

// CreateWorkerJobHandler validates and enqueues a mechanical job from REST input.
func CreateWorkerJobHandler(options WorkerAPIOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		if options.Queue == nil {
			writeWorkerError(writer, http.StatusServiceUnavailable, "worker queue is unavailable")
			return
		}
		defer request.Body.Close()
		var input CreateJobRequest
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeWorkerError(writer, http.StatusBadRequest, "invalid job payload")
			return
		}
		job, err := worker.NewJob(worker.JobInput{
			ID:             input.ID,
			ExternalID:     input.ExternalID,
			Type:           input.Type,
			Priority:       input.Priority,
			MaxAttempts:    input.MaxAttempts,
			SourceURL:      input.SourceURL,
			DestinationKey: input.DestinationKey,
			Headers:        input.Headers,
			Metadata:       input.Metadata,
		})
		if err != nil {
			writeWorkerError(writer, http.StatusBadRequest, err.Error())
			return
		}
		if err := options.Queue.Enqueue(request.Context(), job); err != nil {
			status := http.StatusConflict
			if errors.Is(err, workerqueue.ErrQueueFull) {
				status = http.StatusTooManyRequests
			}
			writeWorkerError(writer, status, err.Error())
			return
		}

		var persistence *workerqueue.StoreResult
		if options.Persister != nil {
			result, err := options.Persister.Save(request.Context(), normalizedWorkerDriver(options.Driver), options.Queue.Snapshot())
			if err != nil {
				writeWorkerError(writer, http.StatusInternalServerError, err.Error())
				return
			}
			persistence = &result
		}

		queued := findQueuedJob(options.Queue.Snapshot(), job.ID)
		payload := CreateJobResponse{Status: WorkerAPIStatus, Accepted: true, Job: queued, Queue: workerQueueStats(options), Persistence: persistence}
		writeJSON(writer, http.StatusAccepted, payload)
	}
}

// WorkerRoutes returns the foundation REST API route set for worker state and local job submission.
func WorkerRoutes(options WorkerAPIOptions) []RouteDefinition {
	return WorkerRoutesWithAuth(options, Authenticator{})
}

// WorkerRoutesWithAuth returns worker routes wrapped by an optional authenticator.
func WorkerRoutesWithAuth(options WorkerAPIOptions, auth Authenticator) []RouteDefinition {
	return []RouteDefinition{
		{Name: WorkerRouteName, Method: http.MethodGet, Pattern: WorkerPath, Handler: auth.Wrap(WorkerHandler(options))},
		{Name: WorkerJobsListRouteName, Method: http.MethodGet, Pattern: WorkerJobsPath, Handler: auth.Wrap(WorkerJobsHandler(options))},
		{Name: WorkerJobsCreateRouteName, Method: http.MethodPost, Pattern: WorkerJobsPath, Handler: auth.Wrap(CreateWorkerJobHandler(options))},
	}
}

func workerQueueStats(options WorkerAPIOptions) WorkerQueueStats {
	stats := WorkerQueueStats{Driver: normalizedWorkerDriver(options.Driver)}
	if options.Queue != nil {
		stats.Length = options.Queue.Len()
		stats.Capacity = options.Queue.Capacity()
	}
	return stats
}

func normalizedWorkerDriver(driver string) string {
	trimmed := strings.TrimSpace(driver)
	if trimmed == "" {
		return workerqueue.MemoryDriver
	}
	return trimmed
}

func findQueuedJob(jobs []worker.Job, id string) worker.Job {
	for _, job := range jobs {
		if job.ID == id {
			return job.Clone()
		}
	}
	return worker.Job{}
}

func writeWorkerError(writer http.ResponseWriter, status int, message string) {
	if strings.TrimSpace(message) == "" {
		message = fmt.Sprintf("worker api error: %d", status)
	}
	writeJSON(writer, status, ErrorResponse{Status: "error", Message: message})
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", HealthContentType)
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}

func cloneWorkerJobs(jobs []worker.Job) []worker.Job {
	if jobs == nil {
		return nil
	}
	cloned := make([]worker.Job, len(jobs))
	for index, job := range jobs {
		cloned[index] = job.Clone()
	}
	return cloned
}
