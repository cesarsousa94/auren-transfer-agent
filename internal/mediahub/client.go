package mediahub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	RegisterPath  = "/api/internal/nodes/register"
	ConfigPath    = "/api/internal/nodes/config"
	HeartbeatPath = "/api/internal/nodes/heartbeat"
	MetricsPath   = "/api/internal/nodes/metrics"
	EventsPath    = "/api/internal/nodes/events"
)

// EndpointPaths allows deployments to keep the same auth/identity contract while
// moving Media Hub internal node endpoints behind a prefix or compatibility route.
type EndpointPaths struct {
	Register  string
	Config    string
	Heartbeat string
	Metrics   string
	Events    string
}

func (paths EndpointPaths) normalized() EndpointPaths {
	return EndpointPaths{
		Register:  defaultPath(paths.Register, RegisterPath),
		Config:    defaultPath(paths.Config, ConfigPath),
		Heartbeat: defaultPath(paths.Heartbeat, HeartbeatPath),
		Metrics:   defaultPath(paths.Metrics, MetricsPath),
		Events:    defaultPath(paths.Events, EventsPath),
	}
}

func defaultPath(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = fallback
	}
	return "/" + strings.TrimLeft(value, "/")
}

// ClientOptions configures the Media Hub HTTP client.
type ClientOptions struct {
	BaseURL       string
	HTTPClient    *http.Client
	HMACEnabled   bool
	UserAgent     string
	Now           func() time.Time
	Trace         RequestTraceFunc
	DetailedTrace RequestDetailedTraceFunc
	Paths         EndpointPaths
}

// RequestTraceFunc receives a compact diagnostic trace for outbound Media Hub requests.
type RequestTraceFunc func(method string, path string, status int, duration time.Duration, bytes int64, err error)

// RequestDetailedTraceFunc receives a rich sanitized trace for the local Dev Console.
type RequestDetailedTraceFunc func(RequestTrace)

// RequestTrace is a rich outbound Media Hub request/response trace.
type RequestTrace struct {
	Method          string
	URL             string
	Path            string
	Host            string
	Status          int
	Duration        time.Duration
	RequestBytes    int64
	ResponseBytes   int64
	Err             error
	ContentType     string
	RequestHeaders  map[string]string
	ResponseHeaders map[string]string
	RequestPayload  any
	ResponsePayload any
	RequestBody     []byte
	ResponseBody    []byte
}

// Client is a small typed client for the Media Hub NodeAgentContractService.
type Client struct {
	baseURL       *url.URL
	httpClient    *http.Client
	hmacEnabled   bool
	userAgent     string
	now           func() time.Time
	trace         RequestTraceFunc
	detailedTrace RequestDetailedTraceFunc
	paths         EndpointPaths
}

// NewClient creates a validated Media Hub client.
func NewClient(options ClientOptions) (*Client, error) {
	trimmedBaseURL := strings.TrimRight(strings.TrimSpace(options.BaseURL), "/")
	if trimmedBaseURL == "" {
		return nil, fmt.Errorf("media hub base_url is required")
	}
	parsed, err := url.Parse(trimmedBaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse media hub base_url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return nil, fmt.Errorf("media hub base_url must use http or https")
	}
	if parsed.Host == "" {
		return nil, fmt.Errorf("media hub base_url host is required")
	}
	client := options.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	now := options.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	userAgent := strings.TrimSpace(options.UserAgent)
	if userAgent == "" {
		userAgent = "AurenTransferAgent"
	}
	return &Client{baseURL: parsed, httpClient: client, hmacEnabled: options.HMACEnabled, userAgent: userAgent, now: now, trace: options.Trace, detailedTrace: options.DetailedTrace, paths: options.Paths.normalized()}, nil
}

// Register registers a local Agent as a Media Hub edge node using a one-time token.
func (client *Client) Register(ctx context.Context, request RegistrationPayload) (RegistrationResult, error) {
	if strings.TrimSpace(request.RegistrationToken) == "" {
		return RegistrationResult{}, fmt.Errorf("media hub registration_token is required")
	}
	var response map[string]any
	if err := client.doJSON(ctx, http.MethodPost, client.paths.Register, request, NodeState{}, false, &response); err != nil {
		return RegistrationResult{}, err
	}
	result, err := ParseRegistrationResult(response)
	if err != nil {
		return RegistrationResult{}, err
	}
	return result, nil
}

// FetchConfig pulls node configuration from the Media Hub.
func (client *Client) FetchConfig(ctx context.Context, state NodeState) (ConfigResult, error) {
	var response map[string]any
	if err := client.doJSON(ctx, http.MethodGet, client.paths.Config, nil, state, true, &response); err != nil {
		return ConfigResult{}, err
	}
	return ParseConfigResult(response), nil
}

// SendHeartbeat submits the current Agent heartbeat.
func (client *Client) SendHeartbeat(ctx context.Context, state NodeState, payload HeartbeatPayload) error {
	var response map[string]any
	return client.doJSON(ctx, http.MethodPost, client.paths.Heartbeat, payload, state, true, &response)
}

// SendMetrics submits periodic node metrics.
func (client *Client) SendMetrics(ctx context.Context, state NodeState, payload MetricsPayload) error {
	var response map[string]any
	return client.doJSON(ctx, http.MethodPost, client.paths.Metrics, payload, state, true, &response)
}

// SendEvents submits batched node events.
func (client *Client) SendEvents(ctx context.Context, state NodeState, payload EventsPayload) error {
	var response map[string]any
	return client.doJSON(ctx, http.MethodPost, client.paths.Events, payload, state, true, &response)
}

// SendDrainStarted tells Media Hub this node must stop receiving new work.
func (client *Client) SendDrainStarted(ctx context.Context, state NodeState, payload DrainPayload) error {
	var response map[string]any
	return client.doJSON(ctx, http.MethodPost, "/api/internal/nodes/drain/started", payload, state, true, &response)
}

// SendDrainCompleted tells Media Hub this node has no active work left after drain.
func (client *Client) SendDrainCompleted(ctx context.Context, state NodeState, payload DrainPayload) error {
	var response map[string]any
	return client.doJSON(ctx, http.MethodPost, "/api/internal/nodes/drain/completed", payload, state, true, &response)
}

// ClaimTransferJob asks the Media Hub for one executable transfer job when capacity is available.
func (client *Client) ClaimTransferJob(ctx context.Context, state NodeState, payload ClaimRequest) (ClaimResponse, error) {
	var response map[string]any
	if err := client.doJSON(ctx, http.MethodPost, TransferClaimPath, payload, state, true, &response); err != nil {
		return ClaimResponse{}, err
	}
	return ParseClaimResponse(response)
}

// SendTransferStarted notifies Media Hub that a job has started on this Agent.
func (client *Client) SendTransferStarted(ctx context.Context, state NodeState, jobUUID string, payload TransferProgressPayload) error {
	var response map[string]any
	return client.doJSON(ctx, http.MethodPost, transferJobPath(jobUUID, "started"), payload, state, true, &response)
}

// SendTransferProgress sends a progress callback for a claimed job.
func (client *Client) SendTransferProgress(ctx context.Context, state NodeState, jobUUID string, payload TransferProgressPayload) error {
	var response map[string]any
	return client.doJSON(ctx, http.MethodPost, transferJobPath(jobUUID, "progress"), payload, state, true, &response)
}

// SendTransferCompleted sends the terminal success callback for a claimed job.
func (client *Client) SendTransferCompleted(ctx context.Context, state NodeState, jobUUID string, payload TransferCompletedPayload) error {
	var response map[string]any
	return client.doJSON(ctx, http.MethodPost, transferJobPath(jobUUID, "completed"), payload, state, true, &response)
}

// SendTransferFailed sends the terminal failure callback for a claimed job.
func (client *Client) SendTransferFailed(ctx context.Context, state NodeState, jobUUID string, payload TransferFailedPayload) error {
	var response map[string]any
	return client.doJSON(ctx, http.MethodPost, transferJobPath(jobUUID, "failed"), payload, state, true, &response)
}

// SendTransferEvents sends job-scoped event batches to Media Hub.
func (client *Client) SendTransferEvents(ctx context.Context, state NodeState, jobUUID string, payload TransferEventsPayload) error {
	var response map[string]any
	return client.doJSON(ctx, http.MethodPost, transferJobPath(jobUUID, "events"), payload, state, true, &response)
}

// ReleaseTransferJob releases a claimed job without marking it successful.
func (client *Client) ReleaseTransferJob(ctx context.Context, state NodeState, jobUUID string, payload map[string]any) error {
	var response map[string]any
	return client.doJSON(ctx, http.MethodPost, transferJobPath(jobUUID, "release"), payload, state, true, &response)
}

// FetchTransferControl returns the latest operator control action for the job.
func (client *Client) FetchTransferControl(ctx context.Context, state NodeState, jobUUID string) (TransferControlResult, error) {
	var response map[string]any
	if err := client.doJSON(ctx, http.MethodGet, transferJobPath(jobUUID, "control"), nil, state, true, &response); err != nil {
		return TransferControlResult{}, err
	}
	result := TransferControlResult{Raw: response}
	result.Action = firstString(response, "action", "command")
	result.Reason = firstString(response, "reason", "message")
	if nested, ok := response["control"].(map[string]any); ok {
		if result.Action == "" {
			result.Action = firstString(nested, "action", "command")
		}
		if result.Reason == "" {
			result.Reason = firstString(nested, "reason", "message")
		}
	}
	return result, nil
}

// ResolveGateway validates a public gateway handoff token and returns proxy/redirect instructions.
func (client *Client) ResolveGateway(ctx context.Context, state NodeState, payload GatewayResolveRequest) (GatewayResolveResult, error) {
	var response map[string]any
	if err := client.doJSON(ctx, http.MethodPost, GatewayResolvePath, payload, state, true, &response); err != nil {
		return GatewayResolveResult{}, err
	}
	return ParseGatewayResolveResult(response)
}

// SendGatewaySessionHeartbeat reports active gateway session bytes and status.
func (client *Client) SendGatewaySessionHeartbeat(ctx context.Context, state NodeState, payload GatewaySessionHeartbeatPayload) error {
	var response map[string]any
	return client.doJSON(ctx, http.MethodPost, GatewaySessionHeartbeatPath, payload, state, true, &response)
}

// CloseGatewaySession closes a gateway stream session in Media Hub.
func (client *Client) CloseGatewaySession(ctx context.Context, state NodeState, payload GatewaySessionClosePayload) error {
	var response map[string]any
	return client.doJSON(ctx, http.MethodPost, GatewaySessionClosePath, payload, state, true, &response)
}

// SendGatewayEvents submits gateway runtime events to Media Hub.
func (client *Client) SendGatewayEvents(ctx context.Context, state NodeState, payload GatewayEventsPayload) error {
	var response map[string]any
	return client.doJSON(ctx, http.MethodPost, GatewayEventsPath, payload, state, true, &response)
}

func transferJobPath(jobUUID string, action string) string {
	return "/api/internal/transfer-agent/jobs/" + url.PathEscape(strings.TrimSpace(jobUUID)) + "/" + strings.Trim(strings.TrimSpace(action), "/")
}

func (client *Client) doJSON(ctx context.Context, method string, path string, input any, state NodeState, authenticate bool, output any) error {
	if client == nil {
		return fmt.Errorf("media hub client cannot be nil")
	}
	started := time.Now()
	statusCode := 0
	responseBytes := int64(0)
	var traceErr error
	var endpoint string
	var body []byte
	var responsePayload any
	var responseHeaders map[string]string
	var responseBody []byte
	defer func() {
		duration := time.Since(started)
		if client.trace != nil {
			client.trace(method, path, statusCode, duration, responseBytes, traceErr)
		}
		if client.detailedTrace != nil {
			client.detailedTrace(RequestTrace{Method: method, URL: endpoint, Path: path, Host: hostFromURL(endpoint), Status: statusCode, Duration: duration, RequestBytes: int64(len(body)), ResponseBytes: responseBytes, Err: traceErr, ContentType: "application/json", RequestHeaders: map[string]string{"Accept": "application/json", "Content-Type": contentTypeFor(input), "User-Agent": client.userAgent}, ResponseHeaders: responseHeaders, RequestPayload: input, ResponsePayload: responsePayload, RequestBody: body, ResponseBody: responseBody})
		}
	}()
	endpoint = client.resolve(path)
	var err error
	if input != nil {
		body, err = json.Marshal(input)
		if err != nil {
			traceErr = fmt.Errorf("encode media hub request: %w", err)
			return traceErr
		}
	}
	req, err := http.NewRequestWithContext(ctx, method, endpoint, bytes.NewReader(body))
	if err != nil {
		traceErr = fmt.Errorf("create media hub request: %w", err)
		return traceErr
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", client.userAgent)
	if input != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if authenticate {
		if err := ApplyNodeAuthentication(req, body, state, client.hmacEnabled, client.now(), ""); err != nil {
			traceErr = err
			return traceErr
		}
	}
	resp, err := client.httpClient.Do(req)
	if err != nil {
		traceErr = fmt.Errorf("media hub %s %s failed: %w", method, path, err)
		return traceErr
	}
	statusCode = resp.StatusCode
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, 4<<20)
	payload, err := io.ReadAll(limited)
	responseBytes = int64(len(payload))
	responseBody = payload
	responseHeaders = flattenHeaders(resp.Header)
	if len(bytes.TrimSpace(payload)) > 0 {
		var decoded any
		if json.Unmarshal(payload, &decoded) == nil {
			responsePayload = decoded
		}
	}
	if err != nil {
		traceErr = fmt.Errorf("read media hub response: %w", err)
		return traceErr
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		traceErr = fmt.Errorf("media hub %s %s returned HTTP %d: %s", method, path, resp.StatusCode, strings.TrimSpace(string(payload)))
		return traceErr
	}
	if output == nil || len(bytes.TrimSpace(payload)) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, output); err != nil {
		traceErr = fmt.Errorf("decode media hub response: %w", err)
		return traceErr
	}
	return nil
}

func contentTypeFor(input any) string {
	if input == nil {
		return ""
	}
	return "application/json"
}

func hostFromURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Host
}

func flattenHeaders(headers http.Header) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := map[string]string{}
	for key, values := range headers {
		out[key] = strings.Join(values, ",")
	}
	return out
}

func (client *Client) resolve(path string) string {
	resolved := *client.baseURL
	basePath := strings.TrimRight(resolved.Path, "/")
	path = "/" + strings.TrimLeft(path, "/")
	resolved.Path = basePath + path
	resolved.RawQuery = ""
	return resolved.String()
}
