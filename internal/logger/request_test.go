package logger

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/cesarsousa94/auren-transfer-agent/internal/config"
)

func TestRequestLoggerEmitsStructuredRequestEvent(t *testing.T) {
	var buffer bytes.Buffer
	cfg := config.DefaultConfig().Logger
	cfg.Timestamp = false

	log, err := New(cfg, &buffer)
	if err != nil {
		t.Fatalf("expected logger to initialize: %v", err)
	}

	handler := RequestLogger(log)(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.WriteHeader(http.StatusCreated)
		_, _ = writer.Write([]byte("created"))
	}))

	request := httptest.NewRequest(http.MethodPost, "/jobs/transfer?debug=1", nil)
	request.Header.Set(RequestIDHeader, "req-123")
	request.Header.Set("User-Agent", "agent-test")
	response := httptest.NewRecorder()

	handler.ServeHTTP(response, request)

	line := strings.TrimSpace(buffer.String())
	if err := ValidateJSONLine(line); err != nil {
		t.Fatalf("expected valid request log line: %v\nline=%s", err, line)
	}

	payload, err := DecodeJSONLine(line)
	if err != nil {
		t.Fatalf("expected payload to decode: %v", err)
	}

	assertJSONField(t, payload, FieldLevel, "info")
	assertJSONField(t, payload, FieldMessage, "http request completed")
	assertJSONField(t, payload, FieldComponent, RequestLoggerComponent)
	assertJSONField(t, payload, FieldRequestID, "req-123")
	assertJSONField(t, payload, FieldHTTPMethod, http.MethodPost)
	assertJSONField(t, payload, FieldHTTPPath, "/jobs/transfer")
	assertJSONField(t, payload, FieldHTTPUserAgent, "agent-test")
	assertJSONNumber(t, payload, FieldHTTPStatus, http.StatusCreated)
	assertJSONNumber(t, payload, FieldHTTPBytes, len("created"))
}

func TestRequestLoggerPropagatesLoggerIntoRequestContext(t *testing.T) {
	var buffer bytes.Buffer
	cfg := config.DefaultConfig().Logger
	cfg.Timestamp = false

	log, err := New(cfg, &buffer)
	if err != nil {
		t.Fatalf("expected logger to initialize: %v", err)
	}

	handler := RequestLogger(log)(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		ctxLog, ok := FromContext(request.Context())
		if !ok {
			t.Fatalf("expected logger in request context")
		}
		ctxLog.Info().Str(FieldOperation, "inside-handler").Msg("handler observed request")
		writer.WriteHeader(http.StatusNoContent)
	}))

	request := httptest.NewRequest(http.MethodGet, "/ready", nil)
	request.Header.Set(RequestIDHeader, "req-context")
	handler.ServeHTTP(httptest.NewRecorder(), request)

	lines := nonEmptyLines(buffer.String())
	if len(lines) != 2 {
		t.Fatalf("expected handler and completion log lines, got %d: %#v", len(lines), lines)
	}

	payload, err := DecodeJSONLine(lines[0])
	if err != nil {
		t.Fatalf("expected handler payload to decode: %v", err)
	}
	assertJSONField(t, payload, FieldComponent, RequestLoggerComponent)
	assertJSONField(t, payload, FieldRequestID, "req-context")
	assertJSONField(t, payload, FieldOperation, "inside-handler")
}

func TestRequestLoggerUsesWarnAndErrorLevels(t *testing.T) {
	cases := map[string]struct {
		status int
		level  string
	}{
		"client error": {status: http.StatusNotFound, level: "warn"},
		"server error": {status: http.StatusBadGateway, level: "error"},
	}

	for name, testCase := range cases {
		t.Run(name, func(t *testing.T) {
			var buffer bytes.Buffer
			cfg := config.DefaultConfig().Logger
			cfg.Timestamp = false

			log, err := New(cfg, &buffer)
			if err != nil {
				t.Fatalf("expected logger to initialize: %v", err)
			}

			handler := RequestLogger(log)(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				writer.WriteHeader(testCase.status)
			}))

			handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/status", nil))

			payload, err := DecodeJSONLine(buffer.String())
			if err != nil {
				t.Fatalf("expected payload to decode: %v", err)
			}
			assertJSONField(t, payload, FieldLevel, testCase.level)
			assertJSONNumber(t, payload, FieldHTTPStatus, testCase.status)
		})
	}
}

func TestRequestLoggerFieldNamesReturnsDefensiveCopy(t *testing.T) {
	fields := RequestLoggerFieldNames()
	if len(fields) == 0 {
		t.Fatalf("expected request logger fields")
	}

	fields[0] = "mutated"
	if RequestLoggerFieldNames()[0] == "mutated" {
		t.Fatalf("expected defensive copy")
	}
}

func assertJSONNumber(t *testing.T, payload map[string]any, field string, expected int) {
	t.Helper()

	value, exists := payload[field]
	if !exists {
		t.Fatalf("expected field %s in payload %#v", field, payload)
	}

	number, ok := value.(float64)
	if !ok {
		t.Fatalf("expected numeric field %s, got %#v", field, value)
	}
	if int(number) != expected {
		t.Fatalf("unexpected %s: got %#v want %#v", field, value, expected)
	}
}

func nonEmptyLines(value string) []string {
	parts := strings.Split(value, "\n")
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return lines
}
