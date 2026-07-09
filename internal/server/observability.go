package server

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/download"
	"github.com/cesarsousa94/auren-transfer-agent/internal/heartbeat"
	"github.com/cesarsousa94/auren-transfer-agent/internal/observability"
	"github.com/cesarsousa94/auren-transfer-agent/internal/queue"
	"github.com/cesarsousa94/auren-transfer-agent/internal/runtime"
)

const (
	// ObservabilityAPIBasePath is the canonical foundation observability API prefix.
	ObservabilityAPIBasePath = CommunicationAPIBasePath + "/observability"

	// ObservabilityGrafanaPath exposes the foundation Grafana dashboard JSON.
	ObservabilityGrafanaPath = ObservabilityAPIBasePath + "/grafana/dashboard"

	// ObservabilityTracesPath exposes local trace spans.
	ObservabilityTracesPath = ObservabilityAPIBasePath + "/traces"

	// ObservabilityAuditPath exposes local audit records.
	ObservabilityAuditPath = ObservabilityAPIBasePath + "/audit"

	// ObservabilityAlertsPath exposes local alert evaluations.
	ObservabilityAlertsPath = ObservabilityAPIBasePath + "/alerts"

	// ObservabilityLogsPath exposes local centralized log records.
	ObservabilityLogsPath = ObservabilityAPIBasePath + "/logs"

	ObservabilityPrometheusRouteName  = "observability.prometheus"
	ObservabilityDashboardRouteName   = "observability.dashboard"
	ObservabilityGrafanaRouteName     = "observability.grafana.dashboard"
	ObservabilityTracesListRouteName  = "observability.traces.list"
	ObservabilityTracesStoreRouteName = "observability.traces.store"
	ObservabilityAuditListRouteName   = "observability.audit.list"
	ObservabilityAuditStoreRouteName  = "observability.audit.store"
	ObservabilityAlertsRouteName      = "observability.alerts"
	ObservabilityLogsListRouteName    = "observability.logs.list"
	ObservabilityLogsStoreRouteName   = "observability.logs.store"
)

// ObservabilityOptions configures foundation observability endpoints.
type ObservabilityOptions struct {
	Info            runtime.VersionInfo
	Heartbeat       heartbeat.Record
	Queue           queue.ClusterQueue
	DownloadMetrics *download.MemoryMetricsRecorder
	Events          *EventRecorder
	Traces          *observability.TraceRecorder
	Audit           *observability.AuditRecorder
	Logs            *observability.CentralLogSink
	PrometheusPath  string
	Authenticator   Authenticator
}

// ObservabilityResponse is returned by GET /api/v1/observability.
type ObservabilityResponse struct {
	Status      string                  `json:"status"`
	Dashboard   observability.Dashboard `json:"dashboard"`
	GeneratedAt time.Time               `json:"generated_at"`
}

// TracesResponse is returned by trace list/create endpoints.
type TracesResponse struct {
	Status      string               `json:"status"`
	Count       int                  `json:"count"`
	Spans       []observability.Span `json:"spans"`
	Span        observability.Span   `json:"span,omitempty"`
	GeneratedAt time.Time            `json:"generated_at"`
}

// AuditResponse is returned by audit list/create endpoints.
type AuditResponse struct {
	Status      string                     `json:"status"`
	Count       int                        `json:"count"`
	Events      []observability.AuditEvent `json:"events"`
	Event       observability.AuditEvent   `json:"event,omitempty"`
	GeneratedAt time.Time                  `json:"generated_at"`
}

// AlertsResponse is returned by GET /api/v1/observability/alerts.
type AlertsResponse struct {
	Status      string                `json:"status"`
	Count       int                   `json:"count"`
	Alerts      []observability.Alert `json:"alerts"`
	GeneratedAt time.Time             `json:"generated_at"`
}

// CentralLogsResponse is returned by centralized log list/create endpoints.
type CentralLogsResponse struct {
	Status      string                    `json:"status"`
	Count       int                       `json:"count"`
	Records     []observability.LogRecord `json:"records"`
	Record      observability.LogRecord   `json:"record,omitempty"`
	GeneratedAt time.Time                 `json:"generated_at"`
}

// PrometheusHandler returns Prometheus text exposition.
func PrometheusHandler(options ObservabilityOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		snapshot := buildObservabilitySnapshot(options)
		writer.Header().Set("Content-Type", observability.PrometheusContentType)
		writer.WriteHeader(http.StatusOK)
		_, _ = writer.Write([]byte(observability.Prometheus(snapshot)))
		_ = request
	}
}

// ObservabilityDashboardHandler returns the local foundation dashboard payload.
func ObservabilityDashboardHandler(options ObservabilityOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		dashboard := observability.NewDashboard(buildObservabilitySnapshot(options))
		writeJSON(writer, http.StatusOK, ObservabilityResponse{Status: CommunicationStatusOK, Dashboard: dashboard, GeneratedAt: dashboard.GeneratedAt})
		_ = request
	}
}

// GrafanaDashboardHandler returns the foundation Grafana dashboard JSON.
func GrafanaDashboardHandler() http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		writeJSON(writer, http.StatusOK, observability.DefaultGrafanaDashboard())
		_ = request
	}
}

// TracesListHandler returns local spans.
func TracesListHandler(options ObservabilityOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		spans := []observability.Span(nil)
		if options.Traces != nil {
			spans = options.Traces.Snapshot()
		}
		writeJSON(writer, http.StatusOK, TracesResponse{Status: CommunicationStatusOK, Count: len(spans), Spans: spans, GeneratedAt: time.Now().UTC()})
		_ = request
	}
}

// TracesStoreHandler records one local span.
func TracesStoreHandler(options ObservabilityOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		if options.Traces == nil {
			writeCommunicationError(writer, http.StatusServiceUnavailable, "trace recorder unavailable")
			return
		}
		var input observability.SpanInput
		if err := decodeObservabilityJSON(writer, request, &input); err != nil {
			writeCommunicationError(writer, http.StatusBadRequest, err.Error())
			return
		}
		span, err := options.Traces.Record(input)
		if err != nil {
			writeCommunicationError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(writer, http.StatusAccepted, TracesResponse{Status: CommunicationStatusOK, Count: options.Traces.Count(), Span: span, GeneratedAt: time.Now().UTC()})
	}
}

// AuditListHandler returns local audit events.
func AuditListHandler(options ObservabilityOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		events := []observability.AuditEvent(nil)
		if options.Audit != nil {
			events = options.Audit.Snapshot()
		}
		writeJSON(writer, http.StatusOK, AuditResponse{Status: CommunicationStatusOK, Count: len(events), Events: events, GeneratedAt: time.Now().UTC()})
		_ = request
	}
}

// AuditStoreHandler records one local audit event.
func AuditStoreHandler(options ObservabilityOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		if options.Audit == nil {
			writeCommunicationError(writer, http.StatusServiceUnavailable, "audit recorder unavailable")
			return
		}
		var input observability.AuditInput
		if err := decodeObservabilityJSON(writer, request, &input); err != nil {
			writeCommunicationError(writer, http.StatusBadRequest, err.Error())
			return
		}
		event, err := options.Audit.Record(input)
		if err != nil {
			writeCommunicationError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(writer, http.StatusAccepted, AuditResponse{Status: CommunicationStatusOK, Count: options.Audit.Count(), Event: event, GeneratedAt: time.Now().UTC()})
	}
}

// AlertsHandler returns currently active local alerts.
func AlertsHandler(options ObservabilityOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		alerts := observability.EvaluateAlerts(buildObservabilitySnapshot(options))
		writeJSON(writer, http.StatusOK, AlertsResponse{Status: CommunicationStatusOK, Count: len(alerts), Alerts: alerts, GeneratedAt: time.Now().UTC()})
		_ = request
	}
}

// CentralLogsListHandler returns local centralized logs.
func CentralLogsListHandler(options ObservabilityOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		records := []observability.LogRecord(nil)
		if options.Logs != nil {
			records = options.Logs.Snapshot()
		}
		writeJSON(writer, http.StatusOK, CentralLogsResponse{Status: CommunicationStatusOK, Count: len(records), Records: records, GeneratedAt: time.Now().UTC()})
		_ = request
	}
}

// CentralLogsStoreHandler records one local centralized log event.
func CentralLogsStoreHandler(options ObservabilityOptions) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		defer request.Body.Close()
		if options.Logs == nil {
			writeCommunicationError(writer, http.StatusServiceUnavailable, "central log sink unavailable")
			return
		}
		var input observability.LogInput
		if err := decodeObservabilityJSON(writer, request, &input); err != nil {
			writeCommunicationError(writer, http.StatusBadRequest, err.Error())
			return
		}
		record, err := options.Logs.Record(input)
		if err != nil {
			writeCommunicationError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(writer, http.StatusAccepted, CentralLogsResponse{Status: CommunicationStatusOK, Count: options.Logs.Count(), Record: record, GeneratedAt: time.Now().UTC()})
	}
}

// ObservabilityRoutes returns route definitions for local observability endpoints.
func ObservabilityRoutes(options ObservabilityOptions) []RouteDefinition {
	auth := options.Authenticator
	prometheusPath := strings.TrimSpace(options.PrometheusPath)
	if prometheusPath == "" {
		prometheusPath = "/metrics"
	}
	if !strings.HasPrefix(prometheusPath, "/") {
		prometheusPath = "/" + prometheusPath
	}
	return []RouteDefinition{
		{Name: ObservabilityPrometheusRouteName, Method: http.MethodGet, Pattern: prometheusPath, Handler: auth.Wrap(PrometheusHandler(options))},
		{Name: ObservabilityDashboardRouteName, Method: http.MethodGet, Pattern: ObservabilityAPIBasePath, Handler: auth.Wrap(ObservabilityDashboardHandler(options))},
		{Name: ObservabilityGrafanaRouteName, Method: http.MethodGet, Pattern: ObservabilityGrafanaPath, Handler: auth.Wrap(GrafanaDashboardHandler())},
		{Name: ObservabilityTracesListRouteName, Method: http.MethodGet, Pattern: ObservabilityTracesPath, Handler: auth.Wrap(TracesListHandler(options))},
		{Name: ObservabilityTracesStoreRouteName, Method: http.MethodPost, Pattern: ObservabilityTracesPath, Handler: auth.Wrap(TracesStoreHandler(options))},
		{Name: ObservabilityAuditListRouteName, Method: http.MethodGet, Pattern: ObservabilityAuditPath, Handler: auth.Wrap(AuditListHandler(options))},
		{Name: ObservabilityAuditStoreRouteName, Method: http.MethodPost, Pattern: ObservabilityAuditPath, Handler: auth.Wrap(AuditStoreHandler(options))},
		{Name: ObservabilityAlertsRouteName, Method: http.MethodGet, Pattern: ObservabilityAlertsPath, Handler: auth.Wrap(AlertsHandler(options))},
		{Name: ObservabilityLogsListRouteName, Method: http.MethodGet, Pattern: ObservabilityLogsPath, Handler: auth.Wrap(CentralLogsListHandler(options))},
		{Name: ObservabilityLogsStoreRouteName, Method: http.MethodPost, Pattern: ObservabilityLogsPath, Handler: auth.Wrap(CentralLogsStoreHandler(options))},
	}
}

// ObservabilityCapabilities returns the stable foundation observability capability list.
func ObservabilityCapabilities() []string {
	return observability.Capabilities()
}

func buildObservabilitySnapshot(options ObservabilityOptions) observability.SnapshotInput {
	queueInfo := queue.Info{}
	if options.Queue != nil {
		queueInfo = options.Queue.Info()
	}
	downloadSummary := download.DownloadSummary{}
	if options.DownloadMetrics != nil {
		downloadSummary = options.DownloadMetrics.Summary()
	}
	eventsCount := 0
	if options.Events != nil {
		eventsCount = options.Events.Count()
	}
	auditCount := 0
	if options.Audit != nil {
		auditCount = options.Audit.Count()
	}
	traceCount := 0
	if options.Traces != nil {
		traceCount = options.Traces.Count()
	}
	logCount := 0
	if options.Logs != nil {
		logCount = options.Logs.Count()
	}
	snapshot := observability.SnapshotInput{Info: options.Info, Heartbeat: options.Heartbeat.Clone(), Queue: queueInfo, Download: downloadSummary, EventsCount: eventsCount, AuditCount: auditCount, TraceCount: traceCount, CentralLogCount: logCount, GeneratedAt: time.Now().UTC()}
	snapshot.AlertCount = len(observability.EvaluateAlerts(snapshot))
	return snapshot
}

func decodeObservabilityJSON(writer http.ResponseWriter, request *http.Request, target any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(writer, request.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}
