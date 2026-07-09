package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/auren/auren-transfer-agent/internal/heartbeat"
	workerqueue "github.com/auren/auren-transfer-agent/internal/queue"
	"github.com/auren/auren-transfer-agent/internal/runtime"
	"github.com/auren/auren-transfer-agent/internal/worker"
)

func TestWorkerHandlerReturnsStatusPayload(t *testing.T) {
	options := testWorkerAPIOptions(t)
	recorder := httptest.NewRecorder()

	WorkerHandler(options).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, WorkerPath, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var payload WorkerResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Status != WorkerAPIStatus || payload.Version != runtime.Version || payload.Queue.Capacity != 10 {
		t.Fatalf("payload = %#v", payload)
	}
}

func TestWorkerJobsHandlerListsQueuedJobsDefensively(t *testing.T) {
	options := testWorkerAPIOptions(t)
	job := testServerJob(t, "queued.ts")
	if err := options.Queue.Enqueue(httptest.NewRequest(http.MethodGet, "/", nil).Context(), job); err != nil {
		t.Fatalf("Enqueue() error = %v", err)
	}

	recorder := httptest.NewRecorder()
	WorkerJobsHandler(options).ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, WorkerJobsPath, nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
	var payload WorkerJobsResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Jobs) != 1 || payload.Jobs[0].ID != job.ID || payload.Queue.Length != 1 {
		t.Fatalf("payload = %#v", payload)
	}
	payload.Jobs[0].Metadata = map[string]string{"mutated": "true"}
	if _, ok := options.Queue.Snapshot()[0].Metadata["mutated"]; ok {
		t.Fatal("jobs response was not defensive")
	}
}

func TestCreateWorkerJobHandlerAcceptsAndPersistsJob(t *testing.T) {
	options := testWorkerAPIOptions(t)
	body := bytes.NewBufferString(`{"source_url":"https://example.com/movie.mp4","destination_key":"media/movie.mp4","max_attempts":2,"metadata":{"tenant":"demo"}}`)
	recorder := httptest.NewRecorder()

	CreateWorkerJobHandler(options).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, WorkerJobsPath, body))

	if recorder.Code != http.StatusAccepted {
		t.Fatalf("status = %d body=%s, want %d", recorder.Code, recorder.Body.String(), http.StatusAccepted)
	}
	var payload CreateJobResponse
	if err := json.NewDecoder(recorder.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !payload.Accepted || payload.Job.Status != worker.JobStatusQueued || payload.Queue.Length != 1 || payload.Persistence == nil || payload.Persistence.Jobs != 1 {
		t.Fatalf("payload = %#v", payload)
	}

	snapshot, ok, err := options.Persister.(*workerqueue.FileStore).Load(httptest.NewRequest(http.MethodGet, "/", nil).Context())
	if err != nil || !ok {
		t.Fatalf("Load() ok=%t err=%v", ok, err)
	}
	if len(snapshot.Jobs) != 1 || snapshot.Jobs[0].ID != payload.Job.ID {
		t.Fatalf("snapshot = %#v", snapshot)
	}
}

func TestCreateWorkerJobHandlerRejectsInvalidPayload(t *testing.T) {
	options := testWorkerAPIOptions(t)
	recorder := httptest.NewRecorder()

	CreateWorkerJobHandler(options).ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, WorkerJobsPath, strings.NewReader(`{"source_url":"ftp://example.com/file"}`)))

	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}

func TestWorkerRoutesBuildCanonicalRoutes(t *testing.T) {
	routes := WorkerRoutes(testWorkerAPIOptions(t))
	if len(routes) != 3 {
		t.Fatalf("len(routes) = %d, want 3", len(routes))
	}
	router, err := BuildRouter(RouterOptions{Routes: routes})
	if err != nil {
		t.Fatalf("BuildRouter() error = %v", err)
	}
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, WorkerPath, nil))
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
	}
}

func testWorkerAPIOptions(t *testing.T) WorkerAPIOptions {
	t.Helper()
	q, err := workerqueue.NewMemoryQueue(10)
	if err != nil {
		t.Fatalf("NewMemoryQueue() error = %v", err)
	}
	record, err := heartbeat.NewRecord(heartbeat.Input{
		Identity:      testIdentitySnapshot(t),
		Version:       runtime.Info(),
		Status:        heartbeat.StatusIdle,
		GeneratedAt:   time.Date(2026, 7, 9, 12, 0, 0, 0, time.UTC),
		Interval:      30 * time.Second,
		WorkerEnabled: false,
		PoolStats:     worker.PoolStats{Concurrency: 1, WorkerIDs: []string{"worker-1"}},
		QueueStats:    heartbeat.QueueStats{Driver: workerqueue.MemoryDriver, Length: q.Len(), Capacity: q.Capacity()},
	})
	if err != nil {
		t.Fatalf("heartbeat.NewRecord() error = %v", err)
	}
	return WorkerAPIOptions{Info: runtime.Info(), Heartbeat: record, Queue: q, Driver: workerqueue.MemoryDriver, Persister: workerqueue.NewFileStore(filepath.Join(t.TempDir(), "queue.json"))}
}

func testServerJob(t *testing.T, destination string) worker.Job {
	t.Helper()
	job, err := worker.NewJob(worker.JobInput{SourceURL: "https://example.com/source.ts", DestinationKey: destination})
	if err != nil {
		t.Fatalf("NewJob() error = %v", err)
	}
	return job
}
