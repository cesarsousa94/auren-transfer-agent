package logger

import (
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
)

const (
	// RequestIDHeader is the canonical inbound request correlation header.
	RequestIDHeader = "X-Request-Id"

	// RequestLoggerComponent identifies events emitted by the request logger.
	RequestLoggerComponent = "http_request"

	// FieldHTTPMethod stores the inbound HTTP method.
	FieldHTTPMethod = "http_method"

	// FieldHTTPPath stores the request URL path.
	FieldHTTPPath = "http_path"

	// FieldHTTPStatus stores the response status code.
	FieldHTTPStatus = "http_status"

	// FieldHTTPDurationMS stores elapsed handler time in milliseconds.
	FieldHTTPDurationMS = "http_duration_ms"

	// FieldHTTPBytes stores bytes written by the response writer.
	FieldHTTPBytes = "http_bytes"

	// FieldHTTPRemoteAddr stores the remote network address reported by net/http.
	FieldHTTPRemoteAddr = "http_remote_addr"

	// FieldHTTPUserAgent stores the inbound HTTP user-agent header.
	FieldHTTPUserAgent = "http_user_agent"
)

// RequestLogger returns a standard net/http middleware that emits one structured
// log event per completed request and propagates a request-enriched logger to
// downstream handlers through context.Context.
func RequestLogger(log zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		if next == nil {
			next = http.NotFoundHandler()
		}

		return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			if request == nil {
				next.ServeHTTP(writer, request)
				return
			}

			startedAt := time.Now()
			observed := &requestResponseWriter{ResponseWriter: writer, statusCode: http.StatusOK}
			requestID := requestIDFrom(request)
			requestLog := WithFields(log,
				String(FieldComponent, RequestLoggerComponent),
				String(FieldHTTPMethod, request.Method),
				String(FieldHTTPPath, request.URL.Path),
				String(FieldRequestID, requestID),
			)

			next.ServeHTTP(observed, request.WithContext(IntoContext(request.Context(), requestLog)))

			durationMS := int(time.Since(startedAt).Milliseconds())
			if durationMS < 0 {
				durationMS = 0
			}

			event := requestLog.Info()
			if observed.statusCode >= http.StatusInternalServerError {
				event = requestLog.Error()
			} else if observed.statusCode >= http.StatusBadRequest {
				event = requestLog.Warn()
			}

			event.
				Int(FieldHTTPStatus, observed.statusCode).
				Int(FieldHTTPDurationMS, durationMS).
				Int(FieldHTTPBytes, observed.bytesWritten).
				Str(FieldHTTPRemoteAddr, request.RemoteAddr).
				Str(FieldHTTPUserAgent, request.UserAgent()).
				Msg("http request completed")
		})
	}
}

// RequestLoggerFieldNames returns the canonical request logging field names.
func RequestLoggerFieldNames() []string {
	fields := []string{
		FieldComponent,
		FieldRequestID,
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

func requestIDFrom(request *http.Request) string {
	if request == nil {
		return ""
	}
	return strings.TrimSpace(request.Header.Get(RequestIDHeader))
}

type requestResponseWriter struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (writer *requestResponseWriter) WriteHeader(statusCode int) {
	writer.statusCode = statusCode
	writer.ResponseWriter.WriteHeader(statusCode)
}

func (writer *requestResponseWriter) Write(payload []byte) (int, error) {
	if writer.statusCode == 0 {
		writer.statusCode = http.StatusOK
	}
	written, err := writer.ResponseWriter.Write(payload)
	writer.bytesWritten += written
	return written, err
}
