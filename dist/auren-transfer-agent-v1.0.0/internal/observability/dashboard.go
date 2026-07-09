package observability

import "time"

const (
	// DashboardName is the canonical capability name for the local dashboard payload.
	DashboardName = "dashboard"
)

// Dashboard is the local foundation observability overview.
type Dashboard struct {
	Status       string        `json:"status"`
	Capabilities []string      `json:"capabilities"`
	Snapshot     SnapshotInput `json:"snapshot"`
	Alerts       []Alert       `json:"alerts"`
	GeneratedAt  time.Time     `json:"generated_at"`
}

// NewDashboard builds a stable local observability dashboard payload.
func NewDashboard(input SnapshotInput) Dashboard {
	generatedAt := input.GeneratedAt
	if generatedAt.IsZero() {
		generatedAt = time.Now().UTC()
		input.GeneratedAt = generatedAt
	}
	alerts := EvaluateAlerts(input)
	return Dashboard{Status: "ok", Capabilities: Capabilities(), Snapshot: input, Alerts: alerts, GeneratedAt: generatedAt}
}

// Capabilities returns the stable observability capability list.
func Capabilities() []string {
	return []string{PrometheusName, GrafanaName, TracingName, AuditName, AlertsName, DashboardName, CentralizedLogsName}
}
