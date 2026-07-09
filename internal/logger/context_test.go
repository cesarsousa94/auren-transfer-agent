package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cesarsousa94/auren-transfer-agent/internal/config"
)

func TestContextLoggerRoundTrip(t *testing.T) {
	var buffer bytes.Buffer
	log, err := New(config.DefaultConfig().Logger, &buffer)
	if err != nil {
		t.Fatalf("expected logger to initialize: %v", err)
	}

	ctx := IntoContext(context.Background(), log)
	loaded, ok := FromContext(ctx)
	if !ok {
		t.Fatalf("expected logger in context")
	}

	loaded.Info().Msg("context logger ready")
	if !strings.Contains(buffer.String(), "context logger ready") {
		t.Fatalf("expected context logger to write event, got %q", buffer.String())
	}
}

func TestEnrichContextAddsPersistentFields(t *testing.T) {
	var buffer bytes.Buffer
	log, err := New(config.DefaultConfig().Logger, &buffer)
	if err != nil {
		t.Fatalf("expected logger to initialize: %v", err)
	}

	ctx := IntoContext(context.Background(), log)
	ctx = EnrichContext(ctx,
		String(FieldComponent, "worker"),
		String(FieldOperation, "download"),
		String(FieldRequestID, "req-001"),
		String(FieldJobID, "job-001"),
		String(" ", "ignored"),
	)

	loaded, ok := FromContext(ctx)
	if !ok {
		t.Fatalf("expected enriched logger in context")
	}
	loaded.Info().Msg("operation correlated")

	payload := decodeSingleLogLine(t, buffer.String())
	if payload[FieldComponent] != "worker" {
		t.Fatalf("unexpected component: %#v", payload[FieldComponent])
	}
	if payload[FieldOperation] != "download" {
		t.Fatalf("unexpected operation: %#v", payload[FieldOperation])
	}
	if payload[FieldRequestID] != "req-001" {
		t.Fatalf("unexpected request id: %#v", payload[FieldRequestID])
	}
	if payload[FieldJobID] != "job-001" {
		t.Fatalf("unexpected job id: %#v", payload[FieldJobID])
	}
	if _, exists := payload[" "]; exists {
		t.Fatalf("blank field key should be ignored: %#v", payload)
	}
}

func TestFromContextOrDefaultUsesFallback(t *testing.T) {
	var buffer bytes.Buffer
	fallback, err := New(config.DefaultConfig().Logger, &buffer)
	if err != nil {
		t.Fatalf("expected logger to initialize: %v", err)
	}

	log := FromContextOrDefault(context.Background(), fallback)
	log.Info().Msg("fallback used")

	if !strings.Contains(buffer.String(), "fallback used") {
		t.Fatalf("expected fallback logger output, got %q", buffer.String())
	}
}

func TestEnrichContextWithoutLoggerKeepsContextUsable(t *testing.T) {
	ctx := EnrichContext(nil, String(FieldRequestID, "req-001"))
	if ctx == nil {
		t.Fatalf("expected nil context to be normalized")
	}
	if _, ok := FromContext(ctx); ok {
		t.Fatalf("did not expect logger to be created implicitly")
	}
}

func decodeSingleLogLine(t *testing.T, line string) map[string]any {
	t.Helper()

	line = strings.TrimSpace(line)
	if line == "" {
		t.Fatalf("expected one log line")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatalf("expected JSON log line, got %q: %v", line, err)
	}
	return payload
}
