// Package zerolog is a compact local compatibility layer for the subset of
// github.com/rs/zerolog used by the Auren Transfer Agent v0.1.x foundation.
//
// It keeps the ZIP self-contained and offline-compilable while preserving the
// public dependency boundary expected by internal/logger. Later deliveries can
// remove the replace directive and use upstream zerolog without changing Agent
// code that already depends on the internal/logger package.
package zerolog

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

// Level represents a log severity threshold.
type Level int8

const (
	TraceLevel Level = -1
	DebugLevel Level = 0
	InfoLevel  Level = 1
	WarnLevel  Level = 2
	ErrorLevel Level = 3
	FatalLevel Level = 4
	PanicLevel Level = 5
	Disabled   Level = 6
)

// ParseLevel converts a textual level into a Level value.
func ParseLevel(value string) (Level, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "trace":
		return TraceLevel, nil
	case "debug":
		return DebugLevel, nil
	case "", "info":
		return InfoLevel, nil
	case "warn", "warning":
		return WarnLevel, nil
	case "error":
		return ErrorLevel, nil
	case "fatal":
		return FatalLevel, nil
	case "panic":
		return PanicLevel, nil
	case "disabled", "off":
		return Disabled, nil
	default:
		return InfoLevel, fmt.Errorf("unknown log level %q", value)
	}
}

func (level Level) String() string {
	switch level {
	case TraceLevel:
		return "trace"
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warn"
	case ErrorLevel:
		return "error"
	case FatalLevel:
		return "fatal"
	case PanicLevel:
		return "panic"
	case Disabled:
		return "disabled"
	default:
		return "info"
	}
}

// Logger writes structured JSON log events.
type Logger struct {
	out       io.Writer
	level     Level
	fields    map[string]any
	timestamp bool
	mu        *sync.Mutex
}

// New creates a logger that writes to out.
func New(out io.Writer) Logger {
	return Logger{
		out:    out,
		level:  InfoLevel,
		fields: map[string]any{},
		mu:     &sync.Mutex{},
	}
}

// Level returns a copy with the provided severity threshold.
func (logger Logger) Level(level Level) Logger {
	logger.level = level
	return logger
}

// With starts a context builder for common fields.
func (logger Logger) With() Context {
	return Context{logger: logger, fields: cloneFields(logger.fields), timestamp: logger.timestamp}
}

// Trace starts a trace event.
func (logger Logger) Trace() *Event { return logger.newEvent(TraceLevel) }

// Debug starts a debug event.
func (logger Logger) Debug() *Event { return logger.newEvent(DebugLevel) }

// Info starts an info event.
func (logger Logger) Info() *Event { return logger.newEvent(InfoLevel) }

// Warn starts a warning event.
func (logger Logger) Warn() *Event { return logger.newEvent(WarnLevel) }

// Error starts an error event.
func (logger Logger) Error() *Event { return logger.newEvent(ErrorLevel) }

func (logger Logger) newEvent(level Level) *Event {
	if logger.out == nil || logger.level == Disabled || level < logger.level {
		return &Event{disabled: true}
	}

	fields := cloneFields(logger.fields)
	fields["level"] = level.String()
	if logger.timestamp {
		fields["time"] = time.Now().UTC().Format(time.RFC3339Nano)
	}

	return &Event{logger: logger, fields: fields}
}

// Context builds a logger with persistent fields.
type Context struct {
	logger    Logger
	fields    map[string]any
	timestamp bool
}

// Timestamp enables UTC timestamp emission.
func (context Context) Timestamp() Context {
	context.timestamp = true
	return context
}

// Str adds a persistent string field.
func (context Context) Str(key string, value string) Context {
	if context.fields == nil {
		context.fields = map[string]any{}
	}
	context.fields[key] = value
	return context
}

// Logger finalizes the context into a Logger.
func (context Context) Logger() Logger {
	logger := context.logger
	logger.fields = cloneFields(context.fields)
	logger.timestamp = context.timestamp
	return logger
}

// Event represents a pending structured log event.
type Event struct {
	logger   Logger
	fields   map[string]any
	disabled bool
}

// Str adds a string field to the event.
func (event *Event) Str(key string, value string) *Event {
	if event.disabled {
		return event
	}
	event.fields[key] = value
	return event
}

// Bool adds a boolean field to the event.
func (event *Event) Bool(key string, value bool) *Event {
	if event.disabled {
		return event
	}
	event.fields[key] = value
	return event
}

// Int adds an integer field to the event.
func (event *Event) Int(key string, value int) *Event {
	if event.disabled {
		return event
	}
	event.fields[key] = value
	return event
}

// Err adds an error field when err is not nil.
func (event *Event) Err(err error) *Event {
	if event.disabled || err == nil {
		return event
	}
	event.fields["error"] = err.Error()
	return event
}

// Msg writes the event as one JSON line.
func (event *Event) Msg(message string) {
	if event.disabled {
		return
	}
	event.fields["message"] = message
	payload, err := json.Marshal(event.fields)
	if err != nil {
		payload = []byte(`{"level":"error","message":"failed to encode log event"}`)
	}

	event.logger.mu.Lock()
	defer event.logger.mu.Unlock()
	_, _ = event.logger.out.Write(append(payload, '\n'))
}

func cloneFields(input map[string]any) map[string]any {
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
