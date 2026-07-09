package observability

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// AuditName is the canonical capability name for local audit events.
	AuditName = "audit"
)

// AuditEvent describes one mechanical audit record.
type AuditEvent struct {
	ID        string            `json:"id"`
	Actor     string            `json:"actor"`
	Action    string            `json:"action"`
	Resource  string            `json:"resource"`
	Outcome   string            `json:"outcome"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// AuditInput is accepted by the local audit recorder.
type AuditInput struct {
	ID       string            `json:"id"`
	Actor    string            `json:"actor"`
	Action   string            `json:"action"`
	Resource string            `json:"resource"`
	Outcome  string            `json:"outcome"`
	Metadata map[string]string `json:"metadata"`
}

// AuditRecorder stores audit events in memory for local diagnostics.
type AuditRecorder struct {
	mu     sync.Mutex
	limit  int
	events []AuditEvent
	now    func() time.Time
}

// NewAuditRecorder creates a bounded local audit recorder.
func NewAuditRecorder(limit int) *AuditRecorder {
	if limit <= 0 {
		limit = 100
	}
	return &AuditRecorder{limit: limit, now: func() time.Time { return time.Now().UTC() }}
}

// Record validates and stores one audit event.
func (recorder *AuditRecorder) Record(input AuditInput) (AuditEvent, error) {
	if recorder == nil {
		return AuditEvent{}, fmt.Errorf("audit recorder cannot be nil")
	}
	event, err := NewAuditEvent(input, recorder.now())
	if err != nil {
		return AuditEvent{}, err
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	recorder.events = append(recorder.events, event)
	if len(recorder.events) > recorder.limit {
		recorder.events = recorder.events[len(recorder.events)-recorder.limit:]
	}
	return event.Clone(), nil
}

// Snapshot returns a defensive audit event copy.
func (recorder *AuditRecorder) Snapshot() []AuditEvent {
	if recorder == nil {
		return nil
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	events := make([]AuditEvent, len(recorder.events))
	for index, event := range recorder.events {
		events[index] = event.Clone()
	}
	return events
}

// Count returns the number of retained audit events.
func (recorder *AuditRecorder) Count() int {
	if recorder == nil {
		return 0
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	return len(recorder.events)
}

// NewAuditEvent normalizes and validates an audit event.
func NewAuditEvent(input AuditInput, now time.Time) (AuditEvent, error) {
	action := strings.TrimSpace(input.Action)
	if action == "" {
		return AuditEvent{}, fmt.Errorf("audit action is required")
	}
	resource := strings.TrimSpace(input.Resource)
	if resource == "" {
		return AuditEvent{}, fmt.Errorf("audit resource is required")
	}
	actor := strings.TrimSpace(input.Actor)
	if actor == "" {
		actor = "system"
	}
	outcome := strings.ToLower(strings.TrimSpace(input.Outcome))
	if outcome == "" {
		outcome = "success"
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = stableID("audit", actor+":"+action+":"+resource, now)
	}
	return AuditEvent{ID: id, Actor: actor, Action: action, Resource: resource, Outcome: outcome, Metadata: cloneStringMap(input.Metadata), CreatedAt: now.UTC()}, nil
}

// Clone returns a defensive audit event copy.
func (event AuditEvent) Clone() AuditEvent {
	copy := event
	copy.Metadata = cloneStringMap(event.Metadata)
	return copy
}
