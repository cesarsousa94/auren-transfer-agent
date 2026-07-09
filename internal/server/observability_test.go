package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/download"
	"github.com/cesarsousa94/auren-transfer-agent/internal/heartbeat"
	"github.com/cesarsousa94/auren-transfer-agent/internal/observability"
	"github.com/cesarsousa94/auren-transfer-agent/internal/queue"
	"github.com/cesarsousa94/auren-transfer-agent/internal/runtime"
)

func TestObservabilityRoutesExposePrometheusAndDashboard(t *testing.T) {
	memory, err := queue.NewMemoryQueue(10)
	if err != nil {
		t.Fatal(err)
	}
	q := &queue.InstrumentedQueue{Queue: memory}
	opts := ObservabilityOptions{Info: runtime.Info(), Heartbeat: heartbeat.Record{AgentID: "agent-1", Hostname: "host-1", Status: "idle"}, Queue: q, DownloadMetrics: download.NewMemoryMetricsRecorder(), Events: NewEventRecorder(10), Traces: observability.NewTraceRecorder(10), Audit: observability.NewAuditRecorder(10), Logs: observability.NewCentralLogSink(10), PrometheusPath: "/metrics"}
	router, err := BuildRouter(RouterOptions{Routes: ObservabilityRoutes(opts)})
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "auren_agent_info") {
		t.Fatalf("unexpected prometheus response code=%d body=%s", response.Code, response.Body.String())
	}

	response = httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, ObservabilityAPIBasePath, nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "prometheus") {
		t.Fatalf("unexpected dashboard response code=%d body=%s", response.Code, response.Body.String())
	}
}

func TestObservabilityRoutesRecordTraceAuditAndLog(t *testing.T) {
	opts := ObservabilityOptions{Info: runtime.Info(), Traces: observability.NewTraceRecorder(10), Audit: observability.NewAuditRecorder(10), Logs: observability.NewCentralLogSink(10), PrometheusPath: "/metrics"}
	router, err := BuildRouter(RouterOptions{Routes: ObservabilityRoutes(opts)})
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ path, body string }{
		{ObservabilityTracesPath, `{"name":"bootstrap","started_at":"` + time.Unix(1, 0).UTC().Format(time.RFC3339Nano) + `"}`},
		{ObservabilityAuditPath, `{"action":"start","resource":"agent"}`},
		{ObservabilityLogsPath, `{"message":"started","component":"bootstrap"}`},
	}
	for _, item := range cases {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodPost, item.path, strings.NewReader(item.body)))
		if response.Code != http.StatusAccepted {
			t.Fatalf("%s code=%d body=%s", item.path, response.Code, response.Body.String())
		}
	}
	if opts.Traces.Count() != 1 || opts.Audit.Count() != 1 || opts.Logs.Count() != 1 {
		t.Fatalf("unexpected counts traces=%d audit=%d logs=%d", opts.Traces.Count(), opts.Audit.Count(), opts.Logs.Count())
	}
}
