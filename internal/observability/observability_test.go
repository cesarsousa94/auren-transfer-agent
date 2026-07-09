package observability

import (
	"strings"
	"testing"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/download"
	"github.com/cesarsousa94/auren-transfer-agent/internal/heartbeat"
	"github.com/cesarsousa94/auren-transfer-agent/internal/queue"
	"github.com/cesarsousa94/auren-transfer-agent/internal/runtime"
)

func TestPrometheusRendersFoundationMetrics(t *testing.T) {
	rendered := Prometheus(SnapshotInput{Info: runtime.Info(), Heartbeat: heartbeat.Record{AgentID: "agent-1", Hostname: "host-1", Status: "idle"}, Queue: queue.Info{Driver: "memory", Mode: "local", Length: 2, Capacity: 5}, Download: download.DownloadSummary{Succeeded: 3, Failed: 1, BytesWritten: 99}, EventsCount: 4, GeneratedAt: time.Unix(10, 0).UTC()})
	for _, expected := range []string{"# HELP auren_agent_info", "auren_queue_length", "auren_download_total{result=\"failed\"} 1", "auren_observability_snapshot_timestamp_seconds 10"} {
		if !strings.Contains(rendered, expected) {
			t.Fatalf("expected Prometheus output to contain %q, got %s", expected, rendered)
		}
	}
}

func TestGrafanaDashboardHasPanels(t *testing.T) {
	dashboard := DefaultGrafanaDashboard()
	if dashboard.UID == "" || len(dashboard.Panels) < 6 {
		t.Fatalf("unexpected dashboard: %+v", dashboard)
	}
}

func TestRecordersStoreDefensiveCopies(t *testing.T) {
	traces := NewTraceRecorder(2)
	if _, err := traces.Record(SpanInput{Name: "bootstrap", Attributes: map[string]string{"a": "b"}}); err != nil {
		t.Fatal(err)
	}
	spans := traces.Snapshot()
	spans[0].Attributes["a"] = "changed"
	if traces.Snapshot()[0].Attributes["a"] != "b" {
		t.Fatal("trace recorder did not return defensive copies")
	}

	audit := NewAuditRecorder(2)
	if _, err := audit.Record(AuditInput{Action: "start", Resource: "agent"}); err != nil {
		t.Fatal(err)
	}
	logs := NewCentralLogSink(2)
	if _, err := logs.Record(LogInput{Message: "started"}); err != nil {
		t.Fatal(err)
	}
	if audit.Count() != 1 || logs.Count() != 1 {
		t.Fatalf("unexpected counts audit=%d logs=%d", audit.Count(), logs.Count())
	}
}

func TestDashboardEvaluatesAlerts(t *testing.T) {
	dashboard := NewDashboard(SnapshotInput{Queue: queue.Info{Length: 10, Capacity: 10}, Download: download.DownloadSummary{Failed: 1}, GeneratedAt: time.Unix(20, 0).UTC()})
	if len(dashboard.Alerts) != 3 {
		t.Fatalf("expected three active alerts, got %+v", dashboard.Alerts)
	}
	if len(dashboard.Capabilities) != 7 {
		t.Fatalf("unexpected capabilities: %+v", dashboard.Capabilities)
	}
}
