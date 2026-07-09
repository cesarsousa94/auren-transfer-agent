package logger

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/cesarsousa94/auren-transfer-agent/internal/config"
)

func TestNewWritesStructuredJSON(t *testing.T) {
	var buffer bytes.Buffer

	log, err := New(config.DefaultConfig().Logger, &buffer)
	if err != nil {
		t.Fatalf("expected logger to initialize: %v", err)
	}

	log.Info().Str("version", "test").Msg("agent initialized")

	line := strings.TrimSpace(buffer.String())
	if line == "" {
		t.Fatalf("expected one log line")
	}

	var payload map[string]any
	if err := ValidateJSONLine(line); err != nil {
		t.Fatalf("expected valid JSON log line, got %q: %v", line, err)
	}
	if err := json.Unmarshal([]byte(line), &payload); err != nil {
		t.Fatalf("expected JSON log line, got %q: %v", line, err)
	}

	if payload["level"] != "info" {
		t.Fatalf("unexpected level: %#v", payload["level"])
	}
	if payload["service"] != "auren-transfer-agent" {
		t.Fatalf("unexpected service: %#v", payload["service"])
	}
	if payload["version"] != "test" {
		t.Fatalf("unexpected version: %#v", payload["version"])
	}
	if payload["message"] != "agent initialized" {
		t.Fatalf("unexpected message: %#v", payload["message"])
	}
	if _, ok := payload["time"]; !ok {
		t.Fatalf("expected timestamp field")
	}
}

func TestNewHonorsLogLevel(t *testing.T) {
	var buffer bytes.Buffer
	cfg := config.DefaultConfig().Logger
	cfg.Level = "warn"

	log, err := New(cfg, &buffer)
	if err != nil {
		t.Fatalf("expected logger to initialize: %v", err)
	}

	log.Info().Msg("hidden")
	if buffer.Len() != 0 {
		t.Fatalf("expected info message to be filtered, got %q", buffer.String())
	}

	log.Warn().Msg("visible")
	if !strings.Contains(buffer.String(), "visible") {
		t.Fatalf("expected warn message to be emitted, got %q", buffer.String())
	}
}

func TestNewRejectsInvalidFormat(t *testing.T) {
	cfg := config.DefaultConfig().Logger
	cfg.Format = "xml"

	_, err := New(cfg, &bytes.Buffer{})
	if err == nil {
		t.Fatalf("expected invalid format to fail")
	}
	if !strings.Contains(err.Error(), "unsupported logger format") {
		t.Fatalf("unexpected error: %v", err)
	}
}
