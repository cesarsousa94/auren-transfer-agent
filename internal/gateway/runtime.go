package gateway

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/cesarsousa94/auren-transfer-agent/internal/config"
	"github.com/cesarsousa94/auren-transfer-agent/internal/mediahub"
	"github.com/cesarsousa94/auren-transfer-agent/internal/ops"
	"github.com/cesarsousa94/auren-transfer-agent/internal/server"
)

const (
	// RuntimeName identifies the public Media Hub gateway runtime implemented by the Agent.
	RuntimeName = "auren_gateway_runtime_v1"

	// GatewayPathPattern is the registered wildcard route used by the offline Chi compatibility router.
	GatewayPathPattern = "/_auren/gateway/*"

	// GatewayCanonicalPath is the Media Hub handoff path appended to the node public base URL.
	GatewayCanonicalPath = "/_auren/gateway/{token}/{kind}/{id}.{ext}"

	ModeProxy    = "proxy"
	ModeRedirect = "redirect"
)

// MediaHubClient is the minimal gateway subset implemented by *mediahub.Client.
type MediaHubClient interface {
	ResolveGateway(ctx context.Context, state mediahub.NodeState, payload mediahub.GatewayResolveRequest) (mediahub.GatewayResolveResult, error)
	SendGatewaySessionHeartbeat(ctx context.Context, state mediahub.NodeState, payload mediahub.GatewaySessionHeartbeatPayload) error
	CloseGatewaySession(ctx context.Context, state mediahub.NodeState, payload mediahub.GatewaySessionClosePayload) error
	SendGatewayEvents(ctx context.Context, state mediahub.NodeState, payload mediahub.GatewayEventsPayload) error
}

// RuntimeOptions wires the gateway runtime without pulling bootstrap concerns into this package.
type RuntimeOptions struct {
	Config     config.MediaHubConfig
	Client     MediaHubClient
	NodeState  func() mediahub.NodeState
	HTTPClient *http.Client
	Tracker    *Tracker
	Operations *ops.Runtime
	Now        func() time.Time
}

// Runtime serves public gateway handoff routes and reports session telemetry to Media Hub.
type Runtime struct {
	cfg        config.MediaHubConfig
	client     MediaHubClient
	nodeState  func() mediahub.NodeState
	http       *http.Client
	tracker    *Tracker
	operations *ops.Runtime
	now        func() time.Time
}

// NewRuntime creates a validated gateway runtime.
func NewRuntime(options RuntimeOptions) (*Runtime, error) {
	if options.Client == nil {
		return nil, fmt.Errorf("gateway media hub client cannot be nil")
	}
	if options.NodeState == nil {
		return nil, fmt.Errorf("gateway node state callback cannot be nil")
	}
	httpClient := options.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 0}
	}
	tracker := options.Tracker
	if tracker == nil {
		tracker = NewTracker()
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	return &Runtime{cfg: options.Config, client: options.Client, nodeState: options.NodeState, http: httpClient, tracker: tracker, operations: options.Operations, now: now}, nil
}

// Routes returns the public gateway GET/HEAD definitions.
func (runtime *Runtime) Routes() []server.RouteDefinition {
	if runtime == nil || !runtime.cfg.GatewayEnabled {
		return nil
	}
	return []server.RouteDefinition{
		{Name: "gateway.stream.get", Method: http.MethodGet, Pattern: GatewayPathPattern, Handler: runtime.handle},
		{Name: "gateway.stream.head", Method: http.MethodHead, Pattern: GatewayPathPattern, Handler: runtime.handle},
	}
}

// Stats returns a capacity snapshot consumed by Media Hub heartbeat/metrics.
func (runtime *Runtime) Stats() Stats {
	if runtime == nil || runtime.tracker == nil {
		return Stats{}
	}
	return runtime.tracker.Stats()
}

func (runtime *Runtime) handle(writer http.ResponseWriter, request *http.Request) {
	if runtime == nil {
		http.Error(writer, "gateway runtime not initialized", http.StatusServiceUnavailable)
		return
	}
	token, kind, id, ext := parseHandoffPath(request.URL.Path)
	if token == "" || kind == "" || id == "" || ext == "" {
		http.Error(writer, "invalid gateway handoff path", http.StatusBadRequest)
		return
	}

	if decision := runtime.gatewayAdmissionDecision(); !decision.Allowed {
		http.Error(writer, firstNonEmpty(decision.Message, "gateway node is not accepting new sessions"), http.StatusServiceUnavailable)
		return
	}
	state := runtime.nodeState()
	if state.Empty() {
		http.Error(writer, "gateway node is not registered", http.StatusServiceUnavailable)
		return
	}

	input := mediahub.GatewayResolveRequest{
		Token:       token,
		Kind:        kind,
		ID:          id,
		Extension:   ext,
		Method:      request.Method,
		Range:       request.Header.Get("Range"),
		UserAgent:   request.UserAgent(),
		RemoteAddr:  request.RemoteAddr,
		Headers:     captureHeaders(request.Header),
		GeneratedAt: runtime.now().UTC(),
	}
	resolved, err := runtime.client.ResolveGateway(request.Context(), state, input)
	if err != nil {
		runtime.emitEvent(request.Context(), state, "gateway.resolve_failed", "Gateway resolve failed", map[string]string{"kind": kind, "id": id, "error": err.Error()})
		http.Error(writer, "gateway resolve failed", http.StatusBadGateway)
		return
	}
	if err := resolved.Validate(); err != nil {
		runtime.emitEvent(request.Context(), state, "gateway.resolve_invalid", "Gateway resolve response is invalid", map[string]string{"kind": kind, "id": id, "error": err.Error()})
		http.Error(writer, "invalid gateway resolve response", http.StatusBadGateway)
		return
	}

	sessionID := resolved.SessionID
	if sessionID == "" {
		sessionID = token
	}
	record := runtime.tracker.Start(SessionInput{SessionID: sessionID, Token: token, Kind: kind, ID: id, Extension: ext, Mode: resolved.Mode})
	defer runtime.tracker.Close(record.SessionID)

	switch resolved.NormalizedMode() {
	case ModeRedirect:
		runtime.sendHeartbeat(request.Context(), state, resolved, record, http.StatusFound, "redirect")
		target := resolved.UpstreamURL
		if request.Method == http.MethodHead {
			writer.Header().Set("Location", target)
			writer.WriteHeader(http.StatusFound)
		} else {
			http.Redirect(writer, request, target, http.StatusFound)
		}
		runtime.closeSession(request.Context(), state, resolved, record, http.StatusFound, "redirect")
	case ModeProxy:
		runtime.proxy(writer, request, state, resolved, record)
	default:
		http.Error(writer, "unsupported gateway mode", http.StatusBadGateway)
		runtime.closeSession(request.Context(), state, resolved, record, http.StatusBadGateway, "unsupported_mode")
	}
}

func captureHeaders(headers http.Header) map[string]string {
	output := map[string]string{}
	for _, key := range []string{"Range", "User-Agent", "Accept", "Accept-Encoding", "If-Range", "If-None-Match", "If-Modified-Since"} {
		if value := strings.TrimSpace(headers.Get(key)); value != "" {
			output[key] = value
		}
	}
	return output
}

func parseHandoffPath(path string) (string, string, string, string) {
	trimmed := strings.Trim(strings.TrimPrefix(path, "/_auren/gateway/"), "/")
	parts := strings.Split(trimmed, "/")
	if len(parts) != 3 {
		return "", "", "", ""
	}
	token := strings.TrimSpace(parts[0])
	kind := strings.TrimSpace(parts[1])
	leaf := strings.TrimSpace(parts[2])
	dot := strings.LastIndex(leaf, ".")
	if dot <= 0 || dot >= len(leaf)-1 {
		return "", "", "", ""
	}
	id := strings.TrimSpace(leaf[:dot])
	ext := strings.Trim(strings.TrimSpace(leaf[dot+1:]), ".")
	return token, kind, id, ext
}

func (runtime *Runtime) gatewayAdmissionDecision() ops.Decision {
	if runtime == nil || runtime.operations == nil {
		return ops.Decision{Allowed: true, Reason: ops.DecisionAllowed}
	}
	stats := runtime.tracker.Stats()
	return runtime.operations.CanAcceptGatewaySession(ops.ClaimSnapshot{ActiveSessions: stats.ActiveSessions, MaxSessions: runtime.cfg.MaxSessions, CurrentEgressMbps: stats.CurrentEgressBps / 125000, MaxEgressMbps: runtime.cfg.MaxEgressMbps})
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
