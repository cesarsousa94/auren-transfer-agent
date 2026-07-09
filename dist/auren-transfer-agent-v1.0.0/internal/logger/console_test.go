package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/auren/auren-transfer-agent/internal/config"
)

func TestConsoleLoggerEmitsHumanReadableLine(t *testing.T) {
	var buffer bytes.Buffer
	cfg := config.DefaultConfig().Logger
	cfg.Format = ConsoleFormat
	cfg.Timestamp = false

	log, err := New(cfg, &buffer)
	if err != nil {
		t.Fatalf("expected console logger to initialize: %v", err)
	}

	log.Info().Str(FieldComponent, "bootstrap").Str(FieldVersion, "v-test").Msg("agent initialized")

	line := strings.TrimSpace(buffer.String())
	if err := ValidateConsoleLine(line); err != nil {
		t.Fatalf("expected valid console line: %v\nline=%s", err, line)
	}
	for _, expected := range []string{"INFO", "auren-transfer-agent", "agent initialized", "component=bootstrap", "version=v-test"} {
		if !strings.Contains(line, expected) {
			t.Fatalf("expected console line to contain %q, got %q", expected, line)
		}
	}
	if strings.HasPrefix(line, "{") {
		t.Fatalf("expected human-readable console line, got raw JSON: %q", line)
	}
}

func TestFormatConsoleLineSortsExtraFields(t *testing.T) {
	line := `{"level":"info","service":"auren-transfer-agent","message":"ready","zeta":"last","alpha":"first"}`

	formatted, err := FormatConsoleLine(line)
	if err != nil {
		t.Fatalf("expected console formatting to succeed: %v", err)
	}

	alpha := strings.Index(formatted, "alpha=first")
	zeta := strings.Index(formatted, "zeta=last")
	if alpha < 0 || zeta < 0 || alpha > zeta {
		t.Fatalf("expected extra fields to be sorted, got %q", formatted)
	}
}

func TestConsoleLoggerHonorsLogLevel(t *testing.T) {
	var buffer bytes.Buffer
	cfg := config.DefaultConfig().Logger
	cfg.Format = ConsoleFormat
	cfg.Level = "error"

	log, err := New(cfg, &buffer)
	if err != nil {
		t.Fatalf("expected console logger to initialize: %v", err)
	}

	log.Info().Msg("hidden")
	if buffer.Len() != 0 {
		t.Fatalf("expected info message to be filtered, got %q", buffer.String())
	}

	log.Error().Msg("visible")
	if !strings.Contains(buffer.String(), "visible") {
		t.Fatalf("expected error message to be emitted, got %q", buffer.String())
	}
}

func TestValidateConsoleLineRejectsRawJSON(t *testing.T) {
	if err := ValidateConsoleLine(`{"level":"info","service":"auren-transfer-agent","message":"raw"}`); err == nil {
		t.Fatalf("expected raw JSON to fail console validation")
	}
}
