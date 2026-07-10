// Package devui provides the lightweight local development console used by the Agent.
package devui

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	DefaultPath      = "/_auren/dev"
	DefaultRetention = 500
	DefaultBodyLimit = 8192
)

// Config controls the local development UI.
type Config struct {
	Enabled         bool
	Path            string
	Retention       int
	RefreshInterval string
	CaptureBodies   bool
	BodyLimitBytes  int64
}

// BodyPreview stores a sanitized, bounded request/response body preview.
type BodyPreview struct {
	Truncated bool   `json:"truncated"`
	Size      int64  `json:"size"`
	Text      string `json:"text,omitempty"`
	JSON      any    `json:"json,omitempty"`
}

// RequestRecord is a safe, bounded request trace entry for local diagnostics.
type RequestRecord struct {
	ID              int64             `json:"id"`
	Direction       string            `json:"direction"`
	Kind            string            `json:"kind,omitempty"`
	Method          string            `json:"method"`
	URL             string            `json:"url,omitempty"`
	Path            string            `json:"path"`
	Host            string            `json:"host,omitempty"`
	Status          int               `json:"status"`
	DurationMS      int64             `json:"duration_ms"`
	Bytes           int64             `json:"bytes"`
	RemoteAddr      string            `json:"remote_addr,omitempty"`
	Error           string            `json:"error,omitempty"`
	ContentType     string            `json:"content_type,omitempty"`
	UserAgent       string            `json:"user_agent,omitempty"`
	CorrelationID   string            `json:"correlation_id,omitempty"`
	JobUUID         string            `json:"job_uuid,omitempty"`
	Operation       string            `json:"operation,omitempty"`
	Stage           string            `json:"stage,omitempty"`
	Message         string            `json:"message,omitempty"`
	SourceURL       string            `json:"source_url,omitempty"`
	Destination     string            `json:"destination,omitempty"`
	ObjectPath      string            `json:"object_path,omitempty"`
	StartedAt       time.Time         `json:"started_at"`
	FinishedAt      time.Time         `json:"finished_at"`
	Headers         map[string]string `json:"headers,omitempty"`
	ResponseHeaders map[string]string `json:"response_headers,omitempty"`
	RequestBody     *BodyPreview      `json:"request_body,omitempty"`
	ResponseBody    *BodyPreview      `json:"response_body,omitempty"`
	Metadata        map[string]any    `json:"metadata,omitempty"`
}

// Counters summarizes recent request activity.
type Counters struct {
	Total          int64 `json:"total"`
	Inbound        int64 `json:"inbound"`
	Outbound       int64 `json:"outbound"`
	Events         int64 `json:"events"`
	Errors         int64 `json:"errors"`
	LastStatus     int   `json:"last_status"`
	LastDurationMS int64 `json:"last_duration_ms"`
}

// Recorder stores a fixed-size ring of local and outbound request traces.
type Recorder struct {
	mu            sync.Mutex
	retention     int
	bodyLimit     int64
	captureBodies bool
	nextID        int64
	records       []RequestRecord
	counters      Counters
}

// NewRecorder creates a bounded request recorder.
func NewRecorder(retention int) *Recorder {
	return NewRecorderWithOptions(retention, true, DefaultBodyLimit)
}

// NewRecorderWithOptions creates a bounded request recorder with body capture controls.
func NewRecorderWithOptions(retention int, captureBodies bool, bodyLimit int64) *Recorder {
	if retention <= 0 {
		retention = DefaultRetention
	}
	if bodyLimit <= 0 {
		bodyLimit = DefaultBodyLimit
	}
	return &Recorder{retention: retention, records: make([]RequestRecord, 0, retention), captureBodies: captureBodies, bodyLimit: bodyLimit}
}

// Record stores a request trace entry.
func (recorder *Recorder) Record(record RequestRecord) RequestRecord {
	if recorder == nil {
		return record
	}
	record = recorder.sanitizeRecord(record)
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
	case "event":
		recorder.counters.Events++
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

// Middleware records inbound HTTP requests handled by the Agent with bounded request/response previews.
func (recorder *Recorder) Middleware(next http.Handler) http.Handler {
	if next == nil {
		next = http.NotFoundHandler()
	}
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		started := time.Now().UTC()
		var requestBody *BodyPreview
		if recorder != nil && recorder.captureBodies && request.Body != nil && shouldCaptureContentType(request.Header.Get("Content-Type")) && request.ContentLength >= 0 && request.ContentLength <= recorder.bodyLimit {
			payload, _ := io.ReadAll(request.Body)
			requestBody = bodyPreview(payload, recorder.bodyLimit)
			request.Body = io.NopCloser(bytes.NewReader(payload))
		}
		wrapped := &responseCapture{ResponseWriter: writer, status: http.StatusOK, capture: recorder != nil && recorder.captureBodies, limit: recorderBodyLimit(recorder)}
		next.ServeHTTP(wrapped, request)
		finished := time.Now().UTC()
		recorder.Record(RequestRecord{
			Direction:       "inbound",
			Kind:            "http",
			Method:          request.Method,
			URL:             sanitizeURL(request.URL.String()),
			Path:            sanitizeURL(request.URL.RequestURI()),
			Host:            request.Host,
			Status:          wrapped.status,
			DurationMS:      finished.Sub(started).Milliseconds(),
			Bytes:           wrapped.bytes,
			RemoteAddr:      request.RemoteAddr,
			ContentType:     wrapped.Header().Get("Content-Type"),
			UserAgent:       request.UserAgent(),
			CorrelationID:   firstHeader(request, "X-Request-ID", "X-Correlation-ID", "X-Auren-Request-ID"),
			StartedAt:       started,
			FinishedAt:      finished,
			Headers:         sanitizeHeaders(request.Header),
			ResponseHeaders: sanitizeHeaders(wrapped.Header()),
			RequestBody:     requestBody,
			ResponseBody:    bodyPreview(wrapped.body.Bytes(), recorderBodyLimit(recorder)),
		})
	})
}

// RecordOutbound records an outbound request made by a typed Agent client.
func (recorder *Recorder) RecordOutbound(method string, path string, status int, duration time.Duration, bytes int64, err error) {
	if recorder == nil {
		return
	}
	recorder.RecordOutboundDetailed(OutboundTrace{Method: method, Path: path, Status: status, Duration: duration, ResponseBytes: bytes, Err: err})
}

// OutboundTrace is a rich outbound HTTP trace from typed clients/adapters.
type OutboundTrace struct {
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
	Headers         map[string]string
	ResponseHeaders map[string]string
	RequestPayload  any
	ResponsePayload any
	RequestBody     []byte
	ResponseBody    []byte
	JobUUID         string
	Operation       string
	Stage           string
	Message         string
	SourceURL       string
	Destination     string
	ObjectPath      string
	Metadata        map[string]any
}

// RecordOutboundDetailed records a rich outbound HTTP trace.
func (recorder *Recorder) RecordOutboundDetailed(trace OutboundTrace) {
	if recorder == nil {
		return
	}
	message := ""
	if trace.Err != nil {
		message = trace.Err.Error()
	}
	finished := time.Now().UTC()
	started := finished.Add(-trace.Duration)
	metadata := cloneAnyMap(trace.Metadata)
	if trace.RequestPayload != nil {
		metadata["request_payload"] = sanitizeAny(trace.RequestPayload)
	}
	if trace.ResponsePayload != nil {
		metadata["response_payload"] = sanitizeAny(trace.ResponsePayload)
	}
	jobUUID, operation, stage, inferredMessage, sourceURL, destination, objectPath := inferOutboundTraceFields(trace)
	if message == "" {
		message = inferredMessage
	}
	recorder.Record(RequestRecord{Direction: "outbound", Kind: "http", Method: trace.Method, URL: sanitizeURL(firstNonEmpty(trace.URL, trace.Path)), Path: sanitizeURL(firstNonEmpty(trace.Path, trace.URL)), Host: trace.Host, Status: trace.Status, DurationMS: trace.Duration.Milliseconds(), Bytes: trace.ResponseBytes, Error: message, ContentType: trace.ContentType, StartedAt: started, FinishedAt: finished, Headers: cloneStringMap(trace.Headers), ResponseHeaders: cloneStringMap(trace.ResponseHeaders), RequestBody: bodyPreview(trace.RequestBody, recorder.bodyLimit), ResponseBody: bodyPreview(trace.ResponseBody, recorder.bodyLimit), JobUUID: jobUUID, Operation: operation, Stage: stage, Message: message, SourceURL: sourceURL, Destination: destination, ObjectPath: objectPath, Metadata: metadata})
}

// RecordTransferEvent records a detailed non-HTTP transfer step.
func (recorder *Recorder) RecordTransferEvent(jobUUID, operation, stage, message string, metadata map[string]any) {
	if recorder == nil {
		return
	}
	recorder.Record(RequestRecord{Direction: "event", Kind: "transfer", Method: "EVENT", Path: stage, Status: 0, StartedAt: time.Now().UTC(), FinishedAt: time.Now().UTC(), JobUUID: jobUUID, Operation: operation, Stage: stage, Message: message, SourceURL: stringFromMap(metadata, "source_url"), Destination: stringFromMap(metadata, "destination_driver"), ObjectPath: stringFromMap(metadata, "object_path"), Metadata: sanitizeMapAny(metadata)})
}

func (recorder *Recorder) sanitizeRecord(record RequestRecord) RequestRecord {
	record.URL = sanitizeURL(record.URL)
	record.Path = sanitizeURL(record.Path)
	record.SourceURL = sanitizeURL(record.SourceURL)
	record.Headers = sanitizeHeaderMap(record.Headers)
	record.ResponseHeaders = sanitizeHeaderMap(record.ResponseHeaders)
	record.Metadata = sanitizeMapAny(record.Metadata)
	return record
}

func bodyPreview(payload []byte, limit int64) *BodyPreview {
	if len(payload) == 0 || limit == 0 {
		return nil
	}
	truncated := false
	if int64(len(payload)) > limit {
		payload = payload[:limit]
		truncated = true
	}
	preview := &BodyPreview{Truncated: truncated, Size: int64(len(payload)), Text: sanitizeBodyText(string(payload))}
	var decoded any
	if json.Unmarshal(payload, &decoded) == nil {
		preview.JSON = sanitizeAny(decoded)
	}
	return preview
}

func sanitizeBodyText(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	var decoded any
	if json.Unmarshal([]byte(trimmed), &decoded) == nil {
		encoded, err := json.MarshalIndent(sanitizeAny(decoded), "", "  ")
		if err == nil {
			return string(encoded)
		}
	}
	return sanitizeURL(trimmed)
}

func sanitizeAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := map[string]any{}
		for key, value := range typed {
			if isSensitiveKey(key) {
				out[key] = "[redacted]"
				continue
			}
			out[key] = sanitizeAny(value)
		}
		return out
	case map[string]string:
		out := map[string]any{}
		for key, value := range typed {
			if isSensitiveKey(key) {
				out[key] = "[redacted]"
			} else {
				out[key] = sanitizeURL(value)
			}
		}
		return out
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, sanitizeAny(item))
		}
		return out
	case string:
		return sanitizeURL(typed)
	default:
		return value
	}
}

func sanitizeHeaders(headers http.Header) map[string]string {
	return sanitizeHeaderMap(flattenHeaders(headers))
}

func flattenHeaders(headers http.Header) map[string]string {
	out := map[string]string{}
	for key, values := range headers {
		out[key] = strings.Join(values, ",")
	}
	return out
}

func sanitizeHeaderMap(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	out := map[string]string{}
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if isSensitiveKey(key) {
			out[key] = "[redacted]"
		} else {
			out[key] = sanitizeURL(headers[key])
		}
	}
	return out
}

func sanitizeURL(value string) string {
	if strings.TrimSpace(value) == "" {
		return value
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return value
	}
	if parsed.User != nil {
		parsed.User = url.UserPassword("[redacted]", "[redacted]")
	}
	query := parsed.Query()
	changed := false
	for key := range query {
		if isSensitiveKey(key) {
			query.Set(key, "[redacted]")
			changed = true
		}
	}
	if changed {
		parsed.RawQuery = query.Encode()
	}
	return parsed.String()
}

func isSensitiveKey(key string) bool {
	normalized := strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(key, "-", "_"), ".", "_"))
	sensitive := []string{"authorization", "cookie", "set_cookie", "token", "secret", "password", "passwd", "api_key", "apikey", "signature", "credential", "x_amz_security_token", "x_auren_node_secret", "registration_token", "node_secret", "access_key", "secret_key", "aws_secret_access_key", "aws_access_key_id"}
	for _, item := range sensitive {
		if normalized == item || strings.Contains(normalized, item) {
			return true
		}
	}
	return false
}

func shouldCaptureContentType(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return contentType == "" || strings.Contains(contentType, "json") || strings.Contains(contentType, "text") || strings.Contains(contentType, "xml") || strings.Contains(contentType, "form")
}

func firstHeader(request *http.Request, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(request.Header.Get(name)); value != "" {
			return value
		}
	}
	return ""
}

func recorderBodyLimit(recorder *Recorder) int64 {
	if recorder == nil || recorder.bodyLimit <= 0 {
		return DefaultBodyLimit
	}
	return recorder.bodyLimit
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}
func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	out := map[string]string{}
	for k, v := range input {
		out[k] = v
	}
	return out
}
func sanitizeMapAny(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	if sanitized, ok := sanitizeAny(input).(map[string]any); ok {
		return sanitized
	}
	return nil
}

func cloneAnyMap(input map[string]any) map[string]any {
	out := map[string]any{}
	for k, v := range input {
		out[k] = v
	}
	return out
}
func stringFromMap(input map[string]any, key string) string {
	if input == nil {
		return ""
	}
	if v, ok := input[key]; ok {
		return sanitizeURL(strings.TrimSpace(strings.TrimPrefix(strings.TrimSuffix(strings.ReplaceAll(strings.TrimSpace(toString(v)), "\n", " "), "\""), "\"")))
	}
	return ""
}
func toString(value any) string {
	if value == nil {
		return ""
	}
	if s, ok := value.(string); ok {
		return s
	}
	b, _ := json.Marshal(value)
	return string(b)
}

type responseCapture struct {
	http.ResponseWriter
	status  int
	bytes   int64
	capture bool
	limit   int64
	body    bytes.Buffer
}

func (capture *responseCapture) WriteHeader(status int) {
	capture.status = status
	capture.ResponseWriter.WriteHeader(status)
}
func (capture *responseCapture) Write(payload []byte) (int, error) {
	written, err := capture.ResponseWriter.Write(payload)
	capture.bytes += int64(written)
	if capture.capture && int64(capture.body.Len()) < capture.limit {
		remain := capture.limit - int64(capture.body.Len())
		if int64(len(payload)) > remain {
			payload = payload[:remain]
		}
		_, _ = capture.body.Write(payload)
	}
	return written, err
}
