package observability

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	// CentralizedLogsName is the canonical capability name for centralized log collection.
	CentralizedLogsName = "centralized_logs"
)

// LogRecord describes one retained centralized log event.
type LogRecord struct {
	ID        string            `json:"id"`
	Level     string            `json:"level"`
	Component string            `json:"component"`
	Message   string            `json:"message"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// LogInput is accepted by the centralized log sink.
type LogInput struct {
	ID        string            `json:"id"`
	Level     string            `json:"level"`
	Component string            `json:"component"`
	Message   string            `json:"message"`
	Metadata  map[string]string `json:"metadata"`
}

// CentralLogSink stores structured logs in memory for future forwarding.
type CentralLogSink struct {
	mu      sync.Mutex
	limit   int
	records []LogRecord
	now     func() time.Time
}

// NewCentralLogSink creates a bounded local centralized log sink.
func NewCentralLogSink(limit int) *CentralLogSink {
	if limit <= 0 {
		limit = 100
	}
	return &CentralLogSink{limit: limit, now: func() time.Time { return time.Now().UTC() }}
}

// Record validates and stores one centralized log record.
func (sink *CentralLogSink) Record(input LogInput) (LogRecord, error) {
	if sink == nil {
		return LogRecord{}, fmt.Errorf("central log sink cannot be nil")
	}
	record, err := NewLogRecord(input, sink.now())
	if err != nil {
		return LogRecord{}, err
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	sink.records = append(sink.records, record)
	if len(sink.records) > sink.limit {
		sink.records = sink.records[len(sink.records)-sink.limit:]
	}
	return record.Clone(), nil
}

// Snapshot returns a defensive centralized log copy.
func (sink *CentralLogSink) Snapshot() []LogRecord {
	if sink == nil {
		return nil
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	records := make([]LogRecord, len(sink.records))
	for index, record := range sink.records {
		records[index] = record.Clone()
	}
	return records
}

// Count returns the number of retained log records.
func (sink *CentralLogSink) Count() int {
	if sink == nil {
		return 0
	}
	sink.mu.Lock()
	defer sink.mu.Unlock()
	return len(sink.records)
}

// NewLogRecord normalizes and validates one centralized log record.
func NewLogRecord(input LogInput, now time.Time) (LogRecord, error) {
	message := strings.TrimSpace(input.Message)
	if message == "" {
		return LogRecord{}, fmt.Errorf("log message is required")
	}
	component := strings.TrimSpace(input.Component)
	if component == "" {
		component = "agent"
	}
	level := strings.ToLower(strings.TrimSpace(input.Level))
	if level == "" {
		level = "info"
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	id := strings.TrimSpace(input.ID)
	if id == "" {
		id = stableID("log", component+":"+message, now)
	}
	return LogRecord{ID: id, Level: level, Component: component, Message: message, Metadata: cloneStringMap(input.Metadata), CreatedAt: now.UTC()}, nil
}

// Clone returns a defensive log record copy.
func (record LogRecord) Clone() LogRecord {
	copy := record
	copy.Metadata = cloneStringMap(record.Metadata)
	return copy
}

func stableID(prefix string, seed string, now time.Time) string {
	digest := sha256.Sum256([]byte(prefix + ":" + seed + ":" + now.UTC().Format(time.RFC3339Nano)))
	return prefix + "_" + hex.EncodeToString(digest[:])[:24]
}

func cloneStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	output := make(map[string]string, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
