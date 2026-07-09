package logger

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

const (
	// ConsoleFormat is the human-readable logger output format for local work.
	ConsoleFormat = "console"

	// ConsoleLineDelimiter is the official delimiter for console log streams.
	ConsoleLineDelimiter = '\n'
)

var consoleReservedFields = map[string]struct{}{
	FieldTimestamp:   {},
	FieldLevel:       {},
	FieldService:     {},
	FieldMessage:     {},
	FieldVersion:     {},
	FieldStatus:      {},
	FieldEnvironment: {},
}

// NewConsoleWriter converts newline-delimited JSON log events into stable,
// human-readable console lines while preserving the same structured fields.
func NewConsoleWriter(out io.Writer) io.Writer {
	return &consoleWriter{out: out}
}

type consoleWriter struct {
	out    io.Writer
	buffer bytes.Buffer
	mu     sync.Mutex
}

func (writer *consoleWriter) Write(payload []byte) (int, error) {
	if writer.out == nil {
		return 0, fmt.Errorf("console writer output cannot be nil")
	}

	writer.mu.Lock()
	defer writer.mu.Unlock()

	accepted := len(payload)
	if _, err := writer.buffer.Write(payload); err != nil {
		return 0, err
	}

	for {
		line, err := writer.buffer.ReadString(ConsoleLineDelimiter)
		if err != nil {
			writer.buffer.WriteString(line)
			break
		}

		formatted, err := FormatConsoleLine(line)
		if err != nil {
			formatted = strings.TrimSpace(line)
		}
		if strings.TrimSpace(formatted) == "" {
			continue
		}
		if _, err := fmt.Fprintln(writer.out, formatted); err != nil {
			return 0, err
		}
	}

	return accepted, nil
}

// FormatConsoleLine renders one JSON log line as deterministic console text.
func FormatConsoleLine(line string) (string, error) {
	payload, err := DecodeJSONLine(line)
	if err != nil {
		return "", err
	}

	parts := make([]string, 0, 4)
	if timestamp := stringField(payload, FieldTimestamp); timestamp != "" {
		parts = append(parts, timestamp)
	}

	level := strings.ToUpper(stringField(payload, FieldLevel))
	if level == "" {
		level = "INFO"
	}
	parts = append(parts, level)

	if service := stringField(payload, FieldService); service != "" {
		parts = append(parts, service)
	}

	message := stringField(payload, FieldMessage)
	if message == "" {
		message = "log event"
	}
	parts = append(parts, message)

	fields := consoleExtraFields(payload)
	if len(fields) > 0 {
		parts = append(parts, strings.Join(fields, " "))
	}

	return strings.Join(parts, " | "), nil
}

// ValidateConsoleLine validates the stable console rendering contract.
func ValidateConsoleLine(line string) error {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return fmt.Errorf("console log line is empty")
	}
	if strings.HasPrefix(trimmed, "{") {
		return fmt.Errorf("console log line must not be raw JSON")
	}
	if !strings.Contains(trimmed, " | ") {
		return fmt.Errorf("console log line must contain console separators")
	}
	return nil
}

func consoleExtraFields(payload map[string]any) []string {
	keys := make([]string, 0, len(payload))
	for key := range payload {
		if _, reserved := consoleReservedFields[key]; reserved {
			continue
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)

	fields := make([]string, 0, len(keys)+3)
	for _, key := range keys {
		fields = append(fields, fmt.Sprintf("%s=%s", key, consoleValue(payload[key])))
	}

	if value := stringField(payload, FieldVersion); value != "" {
		fields = append(fields, fmt.Sprintf("%s=%s", FieldVersion, value))
	}
	if value := stringField(payload, FieldStatus); value != "" {
		fields = append(fields, fmt.Sprintf("%s=%s", FieldStatus, value))
	}
	if value := stringField(payload, FieldEnvironment); value != "" {
		fields = append(fields, fmt.Sprintf("%s=%s", FieldEnvironment, value))
	}

	return fields
}

func stringField(payload map[string]any, field string) string {
	value, ok := payload[field]
	if !ok || value == nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return strings.TrimSpace(consoleValue(value))
	}
	return strings.TrimSpace(text)
}

func consoleValue(value any) string {
	switch typed := value.(type) {
	case string:
		return quoteConsoleValue(typed)
	case fmt.Stringer:
		return quoteConsoleValue(typed.String())
	default:
		encoded, err := json.Marshal(typed)
		if err != nil {
			return quoteConsoleValue(fmt.Sprint(typed))
		}
		return quoteConsoleValue(string(encoded))
	}
}

func quoteConsoleValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return `""`
	}
	if strings.ContainsAny(trimmed, " \t\n\r\"") {
		return fmt.Sprintf("%q", trimmed)
	}
	return trimmed
}
