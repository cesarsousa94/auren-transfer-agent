package logger

import (
	"context"
	"strings"

	"github.com/rs/zerolog"
)

const (
	// FieldComponent identifies the local subsystem emitting a log event.
	FieldComponent = "component"

	// FieldOperation identifies the operation currently being executed.
	FieldOperation = "operation"

	// FieldRequestID carries an inbound HTTP/API request correlation id.
	FieldRequestID = "request_id"

	// FieldJobID carries a future Media Hub transfer job correlation id.
	FieldJobID = "job_id"

	// FieldAgentID carries the local Agent identity once EPIC 4 is available.
	FieldAgentID = "agent_id"

	// FieldTraceID carries a distributed trace id once tracing is available.
	FieldTraceID = "trace_id"
)

type contextLoggerKey struct{}

// Field represents a persistent string field added to a contextual logger.
//
// v0.1.7 intentionally keeps fields string-only. This preserves a compact
// foundation contract while remaining compatible with request, worker and
// transfer correlation data introduced by later epics.
type Field struct {
	Key   string
	Value string
}

// String creates a context logger field.
func String(key string, value string) Field {
	return Field{Key: key, Value: value}
}

// WithFields returns a child logger that carries the provided persistent fields.
func WithFields(log zerolog.Logger, fields ...Field) zerolog.Logger {
	contextBuilder := log.With()
	applied := false

	for _, field := range fields {
		key := strings.TrimSpace(field.Key)
		if key == "" {
			continue
		}
		contextBuilder = contextBuilder.Str(key, field.Value)
		applied = true
	}

	if !applied {
		return log
	}

	return contextBuilder.Logger()
}

// IntoContext stores a logger in ctx and returns the derived context.
func IntoContext(ctx context.Context, log zerolog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, contextLoggerKey{}, log)
}

// FromContext returns the logger stored in ctx.
func FromContext(ctx context.Context) (zerolog.Logger, bool) {
	if ctx == nil {
		return zerolog.Logger{}, false
	}

	log, ok := ctx.Value(contextLoggerKey{}).(zerolog.Logger)
	return log, ok
}

// FromContextOrDefault returns the logger stored in ctx or fallback when absent.
func FromContextOrDefault(ctx context.Context, fallback zerolog.Logger) zerolog.Logger {
	if log, ok := FromContext(ctx); ok {
		return log
	}
	return fallback
}

// EnrichContext adds persistent fields to the logger stored in ctx.
//
// If ctx has no logger, the original context is returned unchanged. A nil
// context is normalized to context.Background().
func EnrichContext(ctx context.Context, fields ...Field) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}

	log, ok := FromContext(ctx)
	if !ok {
		return ctx
	}

	return IntoContext(ctx, WithFields(log, fields...))
}
