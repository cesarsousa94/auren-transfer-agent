package logger

import (
	"bytes"
	"strings"
	"testing"

	"github.com/auren/auren-transfer-agent/internal/config"
)

func TestJSONLoggerEmitsValidContract(t *testing.T) {
	var buffer bytes.Buffer
	log, err := New(config.DefaultConfig().Logger, &buffer)
	if err != nil {
		t.Fatalf("expected logger to initialize: %v", err)
	}

	log = WithFields(log, String(FieldComponent, "bootstrap"))
	LogRuntimeStartup(log, RuntimeStartupEvent{Version: "v-test", Status: "testing", Environment: "test"})

	line := strings.TrimSpace(buffer.String())
	if err := ValidateJSONLine(line); err != nil {
		t.Fatalf("expected valid json log line: %v\nline=%s", err, line)
	}

	payload, err := DecodeJSONLine(line)
	if err != nil {
		t.Fatalf("expected payload to decode: %v", err)
	}

	assertJSONField(t, payload, FieldLevel, "info")
	assertJSONField(t, payload, FieldService, "auren-transfer-agent")
	assertJSONField(t, payload, FieldComponent, "bootstrap")
	assertJSONField(t, payload, FieldVersion, "v-test")
	assertJSONField(t, payload, FieldStatus, "testing")
	assertJSONField(t, payload, FieldEnvironment, "test")
	assertJSONField(t, payload, FieldMessage, "agent initialized")
}

func TestJSONLoggerCanOmitTimestamp(t *testing.T) {
	var buffer bytes.Buffer
	cfg := config.DefaultConfig().Logger
	cfg.Timestamp = false

	log, err := New(cfg, &buffer)
	if err != nil {
		t.Fatalf("expected logger to initialize: %v", err)
	}

	log.Info().Msg("no timestamp")
	payload, err := DecodeJSONLine(buffer.String())
	if err != nil {
		t.Fatalf("expected payload to decode: %v", err)
	}

	if _, exists := payload[FieldTimestamp]; exists {
		t.Fatalf("did not expect timestamp field when disabled: %#v", payload)
	}
	if err := ValidateJSONLine(buffer.String()); err != nil {
		t.Fatalf("expected timestamp-free line to remain valid: %v", err)
	}
}

func TestValidateJSONLineRejectsInvalidEvents(t *testing.T) {
	cases := map[string]string{
		"empty":         "",
		"not json":      "not-json",
		"missing level": `{"service":"auren-transfer-agent","message":"missing"}`,
		"bad level":     `{"level":"verbose","service":"auren-transfer-agent","message":"bad"}`,
		"bad time":      `{"level":"info","service":"auren-transfer-agent","message":"bad","time":"yesterday"}`,
	}

	for name, line := range cases {
		t.Run(name, func(t *testing.T) {
			if err := ValidateJSONLine(line); err == nil {
				t.Fatalf("expected line to be invalid")
			}
		})
	}
}

func TestJSONFieldNamesReturnsDefensiveCopy(t *testing.T) {
	fields := JSONFieldNames()
	if len(fields) == 0 {
		t.Fatalf("expected field names")
	}

	fields[0] = "mutated"
	next := JSONFieldNames()
	if next[0] == "mutated" {
		t.Fatalf("expected defensive copy")
	}
}

func assertJSONField(t *testing.T, payload map[string]any, field string, expected string) {
	t.Helper()

	value, exists := payload[field]
	if !exists {
		t.Fatalf("expected field %s in payload %#v", field, payload)
	}
	if value != expected {
		t.Fatalf("unexpected %s: got %#v want %#v", field, value, expected)
	}
}
