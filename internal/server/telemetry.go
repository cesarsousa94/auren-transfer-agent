package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/auren/auren-transfer-agent/internal/download"
	"github.com/auren/auren-transfer-agent/internal/heartbeat"
	"github.com/auren/auren-transfer-agent/internal/queue"
	"github.com/auren/auren-transfer-agent/internal/runtime"
)

const (
	// MetricsAPIPath is the canonical foundation communication metrics route.
	MetricsAPIPath = CommunicationAPIBasePath + "/metrics"

	// EventsAPIPath is the canonical foundation communication events route.
	EventsAPIPath = CommunicationAPIBasePath + "/events"

	// MetricsAPIRouteName is the route name for GET /api/v1/metrics.
	MetricsAPIRouteName = "communication.metrics"

	// EventsAPIListRouteName is the route name for GET /api/v1/events.
	EventsAPIListRouteName = "communication.events.list"

	// EventsAPIStoreRouteName is the route name for POST /api/v1/events.
	EventsAPIStoreRouteName = "communication.events.store"

	// EventLevelInfo is the default event level.
	EventLevelInfo = "info"
)

// MetricsAPIOptions configures the foundation metrics API endpoint.
type MetricsAPIOptions struct {
	Info            runtime.VersionInfo
	Heartbeat       heartbeat.Record
	Queue           queue.ClusterQueue
	DownloadMetrics *download.MemoryMetricsRecorder
}

// MetricsResponse is the JSON payload returned by GET /api/v1/metrics.
type MetricsResponse struct {
	Status    string                   `json:"status"`
	Runtime   runtime.VersionInfo      `json:"runtime"`
	Heartbeat heartbeat.Record         `json:"heartbeat"`
	Queue     queue.Info               `json:"queue"`
	Download  download.DownloadSummary `json:"download"`
	Generated time.Time                `json:"generated_at"`
}

// Event is the foundation event payload used by the local communication API.
type Event struct {
	ID        string            `json:"id"`
	Level     string            `json:"level"`
	Type      string            `json:"type"`
	Message   string            `json:"message"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// EventInput is accepted by POST /api/v1/events.
type EventInput struct {
	ID       string            `json:"id"`
	Level    string            `json:"level"`
	Type     string            `json:"type"`
	Message  string            `json:"message"`
	Metadata map[string]string `json:"metadata"`
}

// EventsAPIOptions configures the foundation events API endpoints.
type EventsAPIOptions struct {
	Info      runtime.VersionInfo
	Recorder  *EventRecorder
	MaxEvents int
}

// EventsResponse is returned by GET /api/v1/events.
type EventsResponse struct {
	Status      string              `json:"status"`
	Runtime     runtime.VersionInfo `json:"runtime"`
	Count       int                 `json:"count"`
	Events      []Event             `json:"events"`
	GeneratedAt time.Time           `json:"generated_at"`
}

// EventStoreResponse is returned by POST /api/v1/events.
type EventStoreResponse struct {
	Status    string `json:"status"`
	Accepted  bool   `json:"accepted"`
	Event     Event  `json:"event"`
	Generated string `json:"generated_at"`
}

// EventRecorder stores local communication events in memory for diagnostics.
type EventRecorder struct {
	mu     sync.Mutex
	limit  int
	events []Event
	now    func() time.Time
}

// NewEventRecorder creates a bounded in-memory event recorder.
func NewEventRecorder(limit int) *EventRecorder {
	if limit <= 0 {
		limit = 100
	}
	return &EventRecorder{limit: limit, now: func() time.Time { return time.Now().UTC() }}
}

// Record validates and stores an event.
func (recorder *EventRecorder) Record(input EventInput) (Event, error) {
	if recorder == nil {
		return Event{}, fmt.Errorf("event recorder cannot be nil")
	}
	event, err := NewEvent(input, recorder.now())
	if err != nil {
		return Event{}, err
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.events = append(recorder.events, event)
	if len(recorder.events) > recorder.limit {
		recorder.events = recorder.events[len(recorder.events)-recorder.limit:]
	}
	return event.Clone(), nil
}

// Snapshot returns a defensive copy of events in insertion order.
func (recorder *EventRecorder) Snapshot() []Event {
	if recorder == nil {
		return nil
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return cloneEvents(recorder.events)
}

// Count returns the current number of retained events.
func (recorder *EventRecorder) Count() int {
	if recorder == nil {
		return 0
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return len(recorder.events)
}

// NewEvent normalizes and validates one event.
func NewEvent(input EventInput, now time.Time) (Event, error) {
	level := strings.ToLower(strings.TrimSpace(input.Level))
	if level == "" {
		level = EventLevelInfo
	}
	if !isSupportedEventLevel(level) {
		return Event{}, fmt.Errorf("event level must be debug, info, warn or error")
	}
	typeName := strings.TrimSpace(input.Type)
	if typeName == "" {
		return Event{}, fmt.Errorf("event type is required")
	}
	message := strings.TrimSpace(input.Message)
	if message == "" {
		return Event{}, fmt.Errorf("event message is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Event{ID: strings.TrimSpace(input.ID), Level: level, Type: typeName, Message: message, Metadata: cloneStringMap(input.Metadata), CreatedAt: now.UTC()}, nil
}

// Clone returns a defensive event copy.
func (event Event) Clone() Event {
	copy := event
	copy.Metadata = cloneStringMap(event.Metadata)
	return copy
}

// MetricsAPIHandler returns local Agent metrics as JSON.
func MetricsAPIHandler(options MetricsAPIOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		queueInfo := queue.Info{}
		if options.Queue != nil {
			queueInfo = options.Queue.Info()
		}
		downloadSummary := download.DownloadSummary{}
		if options.DownloadMetrics != nil {
			downloadSummary = options.DownloadMetrics.Summary()
		}
		writeJSON(writer, http.StatusOK, MetricsResponse{Status: CommunicationStatusOK, Runtime: options.Info, Heartbeat: options.Heartbeat.Clone(), Queue: queueInfo, Download: downloadSummary, Generated: time.Now().UTC()})
		_ = request
	}
}

// EventsListHandler returns retained local events.
func EventsListHandler(options EventsAPIOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		events := []Event(nil)
		if options.Recorder != nil {
			events = options.Recorder.Snapshot()
		}
		writeJSON(writer, http.StatusOK, EventsResponse{Status: CommunicationStatusOK, Runtime: options.Info, Count: len(events), Events: events, GeneratedAt: time.Now().UTC()})
		_ = request
	}
}

// EventsStoreHandler accepts a local event for diagnostics.
func EventsStoreHandler(options EventsAPIOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		var input EventInput
		decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeCommunicationError(writer, http.StatusBadRequest, "invalid event payload")
			return
		}
		if options.Recorder == nil {
			writeCommunicationError(writer, http.StatusServiceUnavailable, "event recorder unavailable")
			return
		}
		event, err := options.Recorder.Record(input)
		if err != nil {
			writeCommunicationError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(writer, http.StatusAccepted, EventStoreResponse{Status: CommunicationStatusOK, Accepted: true, Event: event, Generated: time.Now().UTC().Format(time.RFC3339Nano)})
	}
}

// TelemetryRoutes returns metrics and events communication route definitions.
func TelemetryRoutes(metrics MetricsAPIOptions, events EventsAPIOptions, auth Authenticator) []RouteDefinition {
	return []RouteDefinition{
		{Name: MetricsAPIRouteName, Method: http.MethodGet, Pattern: MetricsAPIPath, Handler: auth.Wrap(MetricsAPIHandler(metrics))},
		{Name: EventsAPIListRouteName, Method: http.MethodGet, Pattern: EventsAPIPath, Handler: auth.Wrap(EventsListHandler(events))},
		{Name: EventsAPIStoreRouteName, Method: http.MethodPost, Pattern: EventsAPIPath, Handler: auth.Wrap(EventsStoreHandler(events))},
	}
}

// TelemetryCapabilities returns the stable foundation telemetry capability list.
func TelemetryCapabilities() []string {
	capabilities := []string{"metrics_api", "events_api", "local_event_recorder"}
	sort.Strings(capabilities)
	return capabilities
}

func isSupportedEventLevel(level string) bool {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

func cloneEvents(events []Event) []Event {
	if events == nil {
		return nil
	}
	cloned := make([]Event, len(events))
	for index, event := range events {
		cloned[index] = event.Clone()
	}
	return cloned
}
