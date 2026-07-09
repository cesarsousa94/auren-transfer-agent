package observability

import (
	"sort"
	"time"
)

const (
	// AlertsName is the canonical capability name for local alert evaluation.
	AlertsName = "alerts"
)

// Alert describes a mechanically evaluated foundation alert.
type Alert struct {
	Name        string    `json:"name"`
	Severity    string    `json:"severity"`
	Active      bool      `json:"active"`
	Message     string    `json:"message"`
	EvaluatedAt time.Time `json:"evaluated_at"`
}

// EvaluateAlerts applies local foundation alert rules without side effects.
func EvaluateAlerts(input SnapshotInput) []Alert {
	evaluatedAt := input.GeneratedAt
	if evaluatedAt.IsZero() {
		evaluatedAt = time.Now().UTC()
	}
	alerts := []Alert{
		{Name: "queue_capacity_exhausted", Severity: "critical", Active: input.Queue.Capacity > 0 && input.Queue.Length >= input.Queue.Capacity, Message: "local queue capacity is exhausted", EvaluatedAt: evaluatedAt},
		{Name: "download_failures_present", Severity: "warning", Active: input.Download.Failed > 0, Message: "retained download metrics include failures", EvaluatedAt: evaluatedAt},
		{Name: "heartbeat_missing", Severity: "critical", Active: input.Heartbeat.AgentID == "", Message: "heartbeat identity is missing", EvaluatedAt: evaluatedAt},
	}
	active := make([]Alert, 0, len(alerts))
	for _, alert := range alerts {
		if alert.Active {
			active = append(active, alert)
		}
	}
	sort.Slice(active, func(left, right int) bool { return active[left].Name < active[right].Name })
	return active
}
