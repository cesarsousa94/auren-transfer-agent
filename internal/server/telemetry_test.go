package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/auren/auren-transfer-agent/internal/download"
	"github.com/auren/auren-transfer-agent/internal/heartbeat"
	"github.com/auren/auren-transfer-agent/internal/identity"
	"github.com/auren/auren-transfer-agent/internal/queue"
	"github.com/auren/auren-transfer-agent/internal/runtime"
)

func TestMetricsAPIHandlerReturnsQueueAndDownloadSummary(t *testing.T) {
	q, err := queue.NewQueue(queue.Options{Driver: queue.MemoryDriver, MemoryCapacity: 5})
	if err != nil {
		t.Fatalf("queue: %v", err)
	}
	defer q.Close()
	recorder := download.NewMemoryMetricsRecorder()
	if err := recorder.RecordDownloadMetric(nil, download.DownloadMetric{Engine: "stream", BytesWritten: 42, ContentLength: 42}); err != nil {
		t.Fatalf("RecordDownloadMetric: %v", err)
	}
	snapshot := identity.Snapshot{AgentID: "00000000-0000-4000-8000-000000000001", Hostname: "test-host", Fingerprint: strings.Repeat("a", 64), FingerprintAlgorithm: "sha256"}
	hb, err := heartbeat.NewRecord(heartbeat.Input{Identity: snapshot, Version: runtime.Info(), Status: heartbeat.StatusIdle, QueueStats: heartbeat.QueueStats{Driver: queue.MemoryDriver, Capacity: 5}})
	if err != nil {
		t.Fatalf("heartbeat: %v", err)
	}

	recorderHTTP := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, MetricsAPIPath, nil)
	MetricsAPIHandler(MetricsAPIOptions{Info: runtime.Info(), Heartbeat: hb, Queue: q, DownloadMetrics: recorder}).ServeHTTP(recorderHTTP, request)
	if recorderHTTP.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", recorderHTTP.Code, recorderHTTP.Body.String())
	}
	var payload MetricsResponse
	if err := json.Unmarshal(recorderHTTP.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.Status != CommunicationStatusOK || payload.Queue.Driver != queue.MemoryDriver || payload.Download.Count != 1 || payload.Download.BytesWritten != 42 {
		t.Fatalf("unexpected payload: %#v", payload)
	}
}

func TestEventRecorderStoresBoundedDefensiveEvents(t *testing.T) {
	recorder := NewEventRecorder(1)
	first, err := recorder.Record(EventInput{Level: "warn", Type: "first", Message: "first event", Metadata: map[string]string{"a": "b"}})
	if err != nil {
		t.Fatalf("record first: %v", err)
	}
	first.Metadata["a"] = "mutated"
	if _, err := recorder.Record(EventInput{Type: "second", Message: "second event"}); err != nil {
		t.Fatalf("record second: %v", err)
	}
	events := recorder.Snapshot()
	if len(events) != 1 || events[0].Type != "second" || events[0].Level != EventLevelInfo {
		t.Fatalf("unexpected events: %#v", events)
	}
}

func TestEventsAPIHandlers(t *testing.T) {
	recorder := NewEventRecorder(10)
	options := EventsAPIOptions{Info: runtime.Info(), Recorder: recorder}
	body := bytes.NewBufferString(`{"type":"diagnostic","message":"created"}`)
	store := httptest.NewRecorder()
	EventsStoreHandler(options).ServeHTTP(store, httptest.NewRequest(http.MethodPost, EventsAPIPath, body))
	if store.Code != http.StatusAccepted {
		t.Fatalf("store status=%d body=%s", store.Code, store.Body.String())
	}
	list := httptest.NewRecorder()
	EventsListHandler(options).ServeHTTP(list, httptest.NewRequest(http.MethodGet, EventsAPIPath, nil))
	if list.Code != http.StatusOK {
		t.Fatalf("list status=%d body=%s", list.Code, list.Body.String())
	}
	var payload EventsResponse
	if err := json.Unmarshal(list.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if payload.Count != 1 || payload.Events[0].Type != "diagnostic" {
		t.Fatalf("unexpected list: %#v", payload)
	}
}

func TestTelemetryRoutesAreAuthenticated(t *testing.T) {
	auth, err := NewAuthenticator(AuthOptions{Required: true, APIKey: "secret", TokenHeader: "Authorization"})
	if err != nil {
		t.Fatalf("auth: %v", err)
	}
	routes := TelemetryRoutes(MetricsAPIOptions{Info: runtime.Info()}, EventsAPIOptions{Info: runtime.Info(), Recorder: NewEventRecorder(1)}, auth)
	if len(routes) != 3 {
		t.Fatalf("expected three telemetry routes, got %d", len(routes))
	}
	router, err := BuildRouter(RouterOptions{Routes: routes})
	if err != nil {
		t.Fatalf("BuildRouter: %v", err)
	}
	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, MetricsAPIPath, nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized, got %d", unauthorized.Code)
	}
	authorized := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, MetricsAPIPath, nil)
	request.Header.Set("Authorization", "Bearer secret")
	router.ServeHTTP(authorized, request)
	if authorized.Code != http.StatusOK {
		t.Fatalf("expected ok, got %d body=%s", authorized.Code, authorized.Body.String())
	}
}
