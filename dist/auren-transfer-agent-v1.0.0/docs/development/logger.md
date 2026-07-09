# Logger Development Notes

Auren Transfer Agent v0.1.10 completes EPIC 2 with subphase 2.5 — Request Logger.

The logger package remains the single foundation logging boundary for future server, worker, resolver, downloader, uploader and observability layers.

## Package

The logger contract lives at:

```text
internal/logger
```

The package exposes the foundation constructor:

```go
logger.New(cfg config.LoggerConfig, out io.Writer) (zerolog.Logger, error)
```

Callers receive a zerolog-compatible logger and should avoid importing the compatibility module directly.

## Supported formats

v0.1.10 supports two formats:

```text
json
console
```

`json` remains the default. It emits one JSON object followed by a newline and is intended for Docker, Kubernetes, journald forwarding, log shippers and future centralized logging.

`console` is opt-in and renders the same structured events as deterministic human-readable lines for local development, manual operations and simple shell diagnostics.

## JSON output contract

Canonical core fields:

```text
level
service
time
message
```

`time` is present only when `logger.timestamp=true` and uses UTC `RFC3339Nano` formatting.

Runtime startup fields:

```text
version
status
environment
component
```

Correlation fields:

```text
operation
request_id
job_id
agent_id
trace_id
```

Request logger fields:

```text
http_method
http_path
http_status
http_duration_ms
http_bytes
http_remote_addr
http_user_agent
```

The field constants are exported from `internal/logger` so future packages do not duplicate string literals.

## JSON helpers

```go
logger.JSONFormat
logger.JSONLineDelimiter
logger.JSONFieldNames()
logger.DecodeJSONLine(line string) (map[string]any, error)
logger.ValidateJSONLine(line string) error
logger.LogRuntimeStartup(log zerolog.Logger, event logger.RuntimeStartupEvent)
```

`DecodeJSONLine` and `ValidateJSONLine` are primarily contract and test helpers. Runtime code should normally emit events through the logger instance instead of manually constructing JSON.

## Console output contract

Console output is produced from the same JSON event payload before it reaches the final writer. This preserves one event model while allowing readable local logs.

Current helper surface:

```go
logger.ConsoleFormat
logger.ConsoleLineDelimiter
logger.NewConsoleWriter(out io.Writer) io.Writer
logger.FormatConsoleLine(line string) (string, error)
logger.ValidateConsoleLine(line string) error
```

Console lines use a stable separator:

```text
 | 
```

Example:

```text
2026-07-09T00:00:00Z | INFO | auren-transfer-agent | agent initialized | component=bootstrap version=v0.1.10 status=foundation/request-logger environment=local
```

Extra fields are sorted by key for deterministic tests and easier diffing. Runtime fields `version`, `status` and `environment` are kept at the end of the extra field group for readability.

The console renderer intentionally avoids ANSI color and terminal-specific behavior in this foundation phase so output remains stable in CI, containers and systemd logs.

## Context propagation

Context-aware helpers remain available:

```go
logger.IntoContext(ctx context.Context, log zerolog.Logger) context.Context
logger.FromContext(ctx context.Context) (zerolog.Logger, bool)
logger.FromContextOrDefault(ctx context.Context, fallback zerolog.Logger) zerolog.Logger
```

These helpers allow later HTTP handlers, workers and transfer engines to carry the active logger without global state.

## Context enrichment

Persistent fields can be attached to a logger or to the logger already stored in a context:

```go
child := logger.WithFields(base, logger.String(logger.FieldComponent, "worker"))
ctx = logger.EnrichContext(ctx, logger.String(logger.FieldJobID, "job_123"))
```

The field values are intentionally string-only in this foundation phase. Numeric metrics and transfer counters belong to the metrics, download and upload epics.

## Request logger

The request logger is implemented as standard `net/http` middleware:

```go
middleware := logger.RequestLogger(base)
handler = middleware(handler)
```

It does not start an HTTP server. EPIC 3 will wire it into the future router/server stack.

For every completed request, the middleware emits one structured event with:

```text
component=http_request
message="http request completed"
request_id=<X-Request-Id header when present>
http_method=<request method>
http_path=<URL path without query string>
http_status=<response status>
http_duration_ms=<handler duration in milliseconds>
http_bytes=<bytes written by the response writer>
http_remote_addr=<net/http remote address>
http_user_agent=<User-Agent header>
```

Severity mapping:

```text
2xx/3xx -> info
4xx     -> warn
5xx     -> error
```

The middleware also injects a request-enriched logger into `request.Context()`, so downstream handlers can log with the same `component`, `request_id`, method and path fields:

```go
log, ok := logger.FromContext(request.Context())
if ok {
    log.Info().Str(logger.FieldOperation, "handler").Msg("handling request")
}
```

Current request helper surface:

```go
logger.RequestLogger(log zerolog.Logger) func(http.Handler) http.Handler
logger.RequestLoggerFieldNames() []string
logger.RequestIDHeader
logger.RequestLoggerComponent
```

## Dependency boundary

The Agent imports:

```go
github.com/rs/zerolog
```

For the v0.1.x foundation line, `go.mod` replaces that module with:

```text
./internal/logger/zerologcompat
```

This keeps official ZIP deliveries self-contained and compilable in offline environments. Later production deliveries can remove the replace directive and use upstream zerolog without changing the Agent packages that depend on `internal/logger`.

## Configuration

Current logger YAML section:

```yaml
logger:
  level: info
  format: json
  timestamp: true
  service: auren-transfer-agent
```

Environment override examples:

```bash
AUREN_LOGGER_LEVEL=debug
AUREN_LOGGER_FORMAT=console
AUREN_LOGGER_TIMESTAMP=false
AUREN_LOGGER_SERVICE=auren-transfer-agent-edge-01
```

## Supported levels

```text
trace, debug, info, warn, error, fatal, panic, disabled
```

The active level is a threshold. For example, `warn` hides `trace`, `debug` and `info`, while emitting `warn` and above.

## Startup event

Default JSON startup line:

```json
{"component":"bootstrap","environment":"local","level":"info","message":"agent initialized","service":"auren-transfer-agent","status":"foundation/request-logger","time":"2026-07-09T00:00:00Z","version":"v0.1.10"}
```

Field order is not part of the JSON contract. Valid JSON keys and values are the contract.

Opt-in console startup line:

```text
2026-07-09T00:00:00Z | INFO | auren-transfer-agent | agent initialized | component=bootstrap version=v0.1.10 status=foundation/request-logger environment=local
```

## Design rules

- The logger package must not contain business rules.
- The logger package must not know about Auren Media Hub jobs.
- Runtime packages should receive logger instances or read them from context.
- Request logging must remain middleware-level until EPIC 3 wires an HTTP server.
- JSON events must remain newline-delimited.
- Console events must remain deterministic and generated from structured fields.
- Context enrichment must remain correlation-oriented and must not mutate transfer decisions.
- HTTP framework-specific routing belongs to EPIC 3, not to the logger package.
