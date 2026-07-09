package observability

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// TracingName is the canonical capability name for local tracing.
	TracingName = "tracing"
)

// Span describes one local foundation trace span.
type Span struct {
	ID         string            `json:"id"`
	TraceID    string            `json:"trace_id"`
	ParentID   string            `json:"parent_id,omitempty"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"`
	Status     string            `json:"status"`
	StartedAt  time.Time         `json:"started_at"`
	EndedAt    time.Time         `json:"ended_at"`
	DurationMS int64             `json:"duration_ms"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// SpanInput is accepted by the local tracing recorder.
type SpanInput struct {
	ID         string            `json:"id"`
	TraceID    string            `json:"trace_id"`
	ParentID   string            `json:"parent_id"`
	Name       string            `json:"name"`
	Kind       string            `json:"kind"`
	Status     string            `json:"status"`
	StartedAt  time.Time         `json:"started_at"`
	EndedAt    time.Time         `json:"ended_at"`
	Attributes map[string]string `json:"attributes"`
}

// TraceRecorder stores local spans in memory for diagnostics.
type TraceRecorder struct {
	mu    sync.Mutex
	limit int
	spans []Span
	now   func() time.Time
}

// NewTraceRecorder creates a bounded in-memory trace recorder.
func NewTraceRecorder(limit int) *TraceRecorder {
	if limit <= 0 {
		limit = 100
	}
	return &TraceRecorder{limit: limit, now: func() time.Time { return time.Now().UTC() }}
}

// Record validates and stores one span.
func (recorder *TraceRecorder) Record(input SpanInput) (Span, error) {
	if recorder == nil {
		return Span{}, fmt.Errorf("trace recorder cannot be nil")
	}
	span, err := NewSpan(input, recorder.now())
	if err != nil {
		return Span{}, err
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.spans = append(recorder.spans, span)
	if len(recorder.spans) > recorder.limit {
		recorder.spans = recorder.spans[len(recorder.spans)-recorder.limit:]
	}
	return span.Clone(), nil
}

// Snapshot returns spans in insertion order.
func (recorder *TraceRecorder) Snapshot() []Span {
	if recorder == nil {
		return nil
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	spans := make([]Span, len(recorder.spans))
	for index, span := range recorder.spans {
		spans[index] = span.Clone()
	}
	return spans
}

// Count returns the number of retained spans.
func (recorder *TraceRecorder) Count() int {
	if recorder == nil {
		return 0
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return len(recorder.spans)
}

// NewSpan normalizes and validates a span.
func NewSpan(input SpanInput, now time.Time) (Span, error) {
	name := strings.TrimSpace(input.Name)
	if name == "" {
		return Span{}, fmt.Errorf("span name is required")
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = stableID("span", name, now)
	}
	traceID := strings.TrimSpace(input.TraceID)
	if traceID == "" {
		traceID = id
	}
	kind := strings.ToLower(strings.TrimSpace(input.Kind))
	if kind == "" {
		kind = "internal"
	}
	status := strings.ToLower(strings.TrimSpace(input.Status))
	if status == "" {
		status = "ok"
	}
	started := input.StartedAt.UTC()
	if started.IsZero() {
		started = now.UTC()
	}
	ended := input.EndedAt.UTC()
	if ended.IsZero() || ended.Before(started) {
		ended = started
	}
	return Span{ID: id, TraceID: traceID, ParentID: strings.TrimSpace(input.ParentID), Name: name, Kind: kind, Status: status, StartedAt: started, EndedAt: ended, DurationMS: ended.Sub(started).Milliseconds(), Attributes: cloneStringMap(input.Attributes)}, nil
}

// Clone returns a defensive span copy.
func (span Span) Clone() Span {
	copy := span
	copy.Attributes = cloneStringMap(span.Attributes)
	return copy
}
