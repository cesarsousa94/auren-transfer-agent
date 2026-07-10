// Package devui provides the lightweight local development console used by the Agent.
package devui

import (
	"net/http"
	"strings"
	"sync"
	"time"
)

const (
	DefaultPath      = "/_auren/dev"
	DefaultRetention = 500
)

// Config controls the local development UI.
type Config struct {
	Enabled         bool
	Path            string
	Retention       int
	RefreshInterval string
}

// RequestRecord is a safe, bounded request trace entry for local diagnostics.
type RequestRecord struct {
	ID             int64             `json:"id"`
	Direction      string            `json:"direction"`
	Method         string            `json:"method"`
	Path           string            `json:"path"`
	Status         int               `json:"status"`
	DurationMS     int64             `json:"duration_ms"`
	Bytes          int64             `json:"bytes"`
	RemoteAddr     string            `json:"remote_addr,omitempty"`
	Error          string            `json:"error,omitempty"`
	ContentType    string            `json:"content_type,omitempty"`
	UserAgent      string            `json:"user_agent,omitempty"`
	CorrelationID  string            `json:"correlation_id,omitempty"`
	StartedAt      time.Time         `json:"started_at"`
	FinishedAt     time.Time         `json:"finished_at"`
	Headers        map[string]string `json:"headers,omitempty"`
}

// Counters summarizes recent request activity.
type Counters struct {
	Total          int64 `json:"total"`
	Inbound        int64 `json:"inbound"`
	Outbound       int64 `json:"outbound"`
	Errors         int64 `json:"errors"`
	LastStatus     int   `json:"last_status"`
	LastDurationMS int64 `json:"last_duration_ms"`
}

// Recorder stores a fixed-size ring of local and outbound request traces.
type Recorder struct {
	mu        sync.Mutex
	retention int
	nextID    int64
	records   []RequestRecord
	counters  Counters
}

// NewRecorder creates a bounded request recorder.
func NewRecorder(retention int) *Recorder {
	if retention <= 0 {
		retention = DefaultRetention
	}
	return &Recorder{retention: retention, records: make([]RequestRecord, 0, retention)}
}

// Record stores a request trace entry.
func (recorder *Recorder) Record(record RequestRecord) RequestRecord {
	if recorder == nil {
		return record
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.nextID++
	record.ID = recorder.nextID
	if record.StartedAt.IsZero() {
		record.StartedAt = time.Now().UTC()
	}
	if record.FinishedAt.IsZero() {
		record.FinishedAt = record.StartedAt
	}
	if record.DurationMS == 0 && record.FinishedAt.After(record.StartedAt) {
		record.DurationMS = record.FinishedAt.Sub(record.StartedAt).Milliseconds()
	}
	recorder.records = append(recorder.records, record)
	if len(recorder.records) > recorder.retention {
		copy(recorder.records, recorder.records[len(recorder.records)-recorder.retention:])
		recorder.records = recorder.records[:recorder.retention]
	}
	recorder.counters.Total++
	switch strings.ToLower(record.Direction) {
	case "outbound":
		recorder.counters.Outbound++
	default:
		recorder.counters.Inbound++
	}
	if record.Status >= 400 || record.Error != "" {
		recorder.counters.Errors++
	}
	recorder.counters.LastStatus = record.Status
	recorder.counters.LastDurationMS = record.DurationMS
	return record
}

// Snapshot returns the newest request records first.
func (recorder *Recorder) Snapshot(limit int) []RequestRecord {
	if recorder == nil {
		return []RequestRecord{}
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if limit <= 0 || limit > len(recorder.records) {
		limit = len(recorder.records)
	}
	output := make([]RequestRecord, 0, limit)
	for index := len(recorder.records) - 1; index >= 0 && len(output) < limit; index-- {
		output = append(output, recorder.records[index])
	}
	return output
}

// Counters returns aggregate request counters.
func (recorder *Recorder) Counters() Counters {
	if recorder == nil {
		return Counters{}
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return recorder.counters
}

// Middleware records inbound HTTP requests handled by the Agent.
func (recorder *Recorder) Middleware(next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now().UTC()
		wrapped := &responseCapture{ResponseWriter: writer, status: http.StatusOK}
		next.ServeHTTP(wrapped, request)
		finished := time.Now().UTC()
		recorder.Record(RequestRecord{
			Direction:     "inbound",
			Method:        request.Method,
			Path:          request.URL.RequestURI(),
			Status:        wrapped.status,
			DurationMS:    finished.Sub(started).Milliseconds(),
			Bytes:         wrapped.bytes,
			RemoteAddr:    request.RemoteAddr,
			ContentType:   wrapped.Header().Get("Content-Type"),
			UserAgent:     request.UserAgent(),
			CorrelationID: firstHeader(request, "X-Request-ID", "X-Correlation-ID", "X-Auren-Request-ID"),
			StartedAt:     started,
			FinishedAt:    finished,
		})
	})
}

// RecordOutbound records an outbound request made by a typed Agent client.
func (recorder *Recorder) RecordOutbound(method string, path string, status int, duration time.Duration, bytes int64, err error) {
	if recorder == nil {
		return
	}
	message := ""
	if err != nil {
		message = err.Error()
	}
	finished := time.Now().UTC()
	recorder.Record(RequestRecord{Direction: "outbound", Method: method, Path: path, Status: status, DurationMS: duration.Milliseconds(), Bytes: bytes, Error: message, StartedAt: finished.Add(-duration), FinishedAt: finished})
}

func firstHeader(request *http.Request, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(request.Header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

type responseCapture struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (capture *responseCapture) WriteHeader(status int) {
	capture.status = status
	capture.ResponseWriter.WriteHeader(status)
}

func (capture *responseCapture) Write(payload []byte) (int, error) {
	written, err := capture.ResponseWriter.Write(payload)
	capture.bytes += int64(written)
	return written, err
}
