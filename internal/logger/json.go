package logger

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const (
	// JSONFormat is the canonical structured logger output format.
	JSONFormat = "json"

	// JSONLineDelimiter is the official event delimiter for log streams.
	JSONLineDelimiter = '\n'

	// FieldLevel stores the event severity.
	FieldLevel = "level"

	// FieldTimestamp stores the UTC RFC3339Nano event timestamp.
	FieldTimestamp = "time"

	// FieldService stores the emitting service name.
	FieldService = "service"

	// FieldMessage stores the human-readable event message.
	FieldMessage = "message"

	// FieldVersion stores the Agent runtime version.
	FieldVersion = "version"

	// FieldStatus stores the Agent runtime delivery status.
	FieldStatus = "status"

	// FieldEnvironment stores the active runtime environment.
	FieldEnvironment = "environment"
)

var validJSONLevels = map[string]struct{}{
	"trace":    {},
	"debug":    {},
	"info":     {},
	"warn":     {},
	"error":    {},
	"fatal":    {},
	"panic":    {},
	"disabled": {},
}

// RuntimeStartupEvent contains foundation metadata emitted when the Agent boots.
type RuntimeStartupEvent struct {
	Version     string
	Status      string
	Environment string
}

// LogRuntimeStartup writes the canonical JSON startup event.
func LogRuntimeStartup(log zerolog.Logger, event RuntimeStartupEvent) {
	log.Info().
		Str(FieldVersion, event.Version).
		Str(FieldStatus, event.Status).
		Str(FieldEnvironment, event.Environment).
		Msg("agent initialized")
}

// JSONFieldNames returns the canonical foundation JSON field names.
func JSONFieldNames() []string {
	fields := []string{
		FieldLevel,
		FieldTimestamp,
		FieldService,
		FieldMessage,
		FieldVersion,
		FieldStatus,
		FieldEnvironment,
		FieldComponent,
		FieldOperation,
		FieldRequestID,
		FieldJobID,
		FieldAgentID,
		FieldTraceID,
		FieldHTTPMethod,
		FieldHTTPPath,
		FieldHTTPStatus,
		FieldHTTPDurationMS,
		FieldHTTPBytes,
		FieldHTTPRemoteAddr,
		FieldHTTPUserAgent,
	}

	copyOfFields := make([]string, len(fields))
	copy(copyOfFields, fields)
	return copyOfFields
}

// DecodeJSONLine decodes one newline-delimited JSON log event.
func DecodeJSONLine(line string) (map[string]any, error) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil, fmt.Errorf("json log line is empty")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(trimmed), &payload); err != nil {
		return nil, fmt.Errorf("decode json log line: %w", err)
	}
	if len(payload) == 0 {
		return nil, fmt.Errorf("json log line has no fields")
	}

	return payload, nil
}

// ValidateJSONLine validates the foundation JSON log event contract.
func ValidateJSONLine(line string) error {
	payload, err := DecodeJSONLine(line)
	if err != nil {
		return err
	}

	if err := requireJSONString(payload, FieldLevel); err != nil {
		return err
	}
	if _, ok := validJSONLevels[strings.ToLower(strings.TrimSpace(payload[FieldLevel].(string)))]; !ok {
		return fmt.Errorf("json log field %s has unsupported level %q", FieldLevel, payload[FieldLevel])
	}

	if err := requireJSONString(payload, FieldService); err != nil {
		return err
	}
	if err := requireJSONString(payload, FieldMessage); err != nil {
		return err
	}

	if value, exists := payload[FieldTimestamp]; exists {
		timestamp, ok := value.(string)
		if !ok || strings.TrimSpace(timestamp) == "" {
			return fmt.Errorf("json log field %s must be a non-empty string when present", FieldTimestamp)
		}
		if _, err := time.Parse(time.RFC3339Nano, timestamp); err != nil {
			return fmt.Errorf("json log field %s must be RFC3339Nano UTC-compatible: %w", FieldTimestamp, err)
		}
	}

	return nil
}

func requireJSONString(payload map[string]any, field string) error {
	value, exists := payload[field]
	if !exists {
		return fmt.Errorf("json log field %s is required", field)
	}

	text, ok := value.(string)
	if !ok || strings.TrimSpace(text) == "" {
		return fmt.Errorf("json log field %s must be a non-empty string", field)
	}

	return nil
}
