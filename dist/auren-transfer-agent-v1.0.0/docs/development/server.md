# HTTP Server Development Notes

Auren Transfer Agent v0.1.34 keeps the HTTP foundation from EPIC 3, identity and worker route contracts, and adds EPIC 9.1-9.5 communication REST/WebSocket/authentication contracts.

The HTTP foundation preserves the canonical health, version, ready, identity, worker and communication routes while intentionally avoiding a listening HTTP server. The Agent still starts as a foundation CLI process only; server lifecycle contracts are introduced in later production/server phases.

## Package

The server foundation lives at:

```text
internal/server
```

Current public surface:

```go
server.NewRouter(options server.RouterOptions) chi.Router
server.BuildRouter(options server.RouterOptions) (chi.Router, error)
server.RegisterRoute(router chi.Router, route server.RouteInfo, handler http.HandlerFunc)
server.RegisterRouteDefinition(router chi.Router, route server.RouteDefinition) error
server.RegisterRoutes(router chi.Router, routes ...server.RouteDefinition) error
server.NewRouteRegistry(routes ...server.RouteDefinition) (*server.RouteRegistry, error)
server.DefaultMiddlewareRegistry(options server.MiddlewareOptions) (*server.MiddlewareRegistry, error)
server.NewMiddlewareRegistry(middlewares ...server.MiddlewareDefinition) (*server.MiddlewareRegistry, error)
server.RegisterMiddlewares(router chi.Router, middlewares ...server.MiddlewareDefinition) error
server.Recoverer(log zerolog.Logger) func(http.Handler) http.Handler
server.HealthHandler(info runtime.VersionInfo) http.HandlerFunc
server.HealthRoute(info runtime.VersionInfo) server.RouteDefinition
server.HealthRoutes(info runtime.VersionInfo) []server.RouteDefinition
server.VersionHandler(info runtime.VersionInfo) http.HandlerFunc
server.VersionRoute(info runtime.VersionInfo) server.RouteDefinition
server.VersionRoutes(info runtime.VersionInfo) []server.RouteDefinition
server.ReadyHandler(info runtime.VersionInfo) http.HandlerFunc
server.ReadyRoute(info runtime.VersionInfo) server.RouteDefinition
server.ReadyRoutes(info runtime.VersionInfo) []server.RouteDefinition
server.IdentityHandler(info runtime.VersionInfo, snapshot identity.Snapshot) http.HandlerFunc
server.IdentityRoute(info runtime.VersionInfo, snapshot identity.Snapshot) server.RouteDefinition
server.IdentityRoutes(info runtime.VersionInfo, snapshot identity.Snapshot) []server.RouteDefinition
server.WorkerHandler(options server.WorkerAPIOptions) http.HandlerFunc
server.WorkerJobsHandler(options server.WorkerAPIOptions) http.HandlerFunc
server.CreateWorkerJobHandler(options server.WorkerAPIOptions) http.HandlerFunc
server.WorkerRoutes(options server.WorkerAPIOptions) []server.RouteDefinition
server.NewAuthenticator(options server.AuthOptions) (server.Authenticator, error)
server.RESTStatusHandler(options server.CommunicationOptions) http.HandlerFunc
server.WebSocketHandler(options server.CommunicationOptions) http.HandlerFunc
server.RegistrationHandler(options server.CommunicationOptions) http.HandlerFunc
server.CommunicationHeartbeatHandler(options server.CommunicationOptions, accepted bool) http.HandlerFunc
server.CommunicationRoutes(options server.CommunicationOptions) []server.RouteDefinition
server.FoundationRoutes(info runtime.VersionInfo, identitySnapshots ...identity.Snapshot) []server.RouteDefinition
server.RouterKindName() string
```

## Router implementation

The official router family is:

```text
chi
```

Application code imports the canonical upstream path:

```go
github.com/go-chi/chi/v5
```

The ZIP includes `internal/server/chicompat` and a `replace` directive so the project remains self-contained and offline-compilable during the foundation phases.

## Factory

The canonical router is created through:

```go
router, err := server.BuildRouter(server.RouterOptions{
    Logger:         log,
    RequestLogging: true,
    RecoverPanics:  true,
    Routes:         append(server.FoundationRoutes(runtime.Info(), identitySnapshot), server.WorkerRoutes(workerAPIOptions)...),
})
```

`server.NewRouter` remains available for simple construction and backward compatibility. `server.BuildRouter` is preferred when a caller needs route or middleware validation errors.

## Default middleware stack

Current canonical middleware names:

```text
request_logger
recoverer
```

When `RequestLogging` is true, the stack uses the EPIC 2 `logger.RequestLogger` middleware. When `RecoverPanics` is true, the stack uses `server.Recoverer`, which catches request panics at the HTTP transport boundary, emits a structured error log and returns HTTP 500.

Middleware order is deterministic. Registered middlewares execute in the same order they are stored in the registry, matching standard Chi behavior.

## Route definitions

`server.RouteDefinition` remains the canonical route registration unit:

```go
server.RouteDefinition{
    Name:    "foundation.example",
    Method:  http.MethodGet,
    Pattern: "/foundation",
    Handler: handler,
}
```

The route contract validates:

- non-empty HTTP method;
- supported HTTP method;
- non-empty route pattern;
- slash-prefixed canonical route pattern;
- non-nil handler;
- duplicate method + pattern pairs inside a registry.

## Foundation routes

Current foundation route contracts:

```text
GET /health
GET /version
GET /ready
GET /identity
GET /worker
GET /worker/jobs
POST /worker/jobs
GET /api/v1
GET /api/v1/ws
POST /api/v1/registration
GET /api/v1/heartbeat
POST /api/v1/heartbeat
```

`/health` returns liveness metadata. `/version` returns immutable runtime metadata. `/ready` returns foundation readiness metadata. `/identity` returns local technical Agent identity metadata. Worker routes return local worker/queue state and accept mechanical transfer jobs into the local queue. Communication routes expose REST status, a WebSocket upgrade handshake, local registration acknowledgement, heartbeat payloads and optional API-key authentication.

## Identity route

v0.1.20 introduces the canonical foundation identity route:

```text
GET /identity
```

The handler returns HTTP 200 and a stable JSON payload:

```json
{
  "status": "ok",
  "name": "auren-transfer-agent",
  "version": "v0.1.24",
  "runtime_status": "foundation/worker-persistence-rest-api",
  "router": "chi",
  "agent_id": "123e4567-e89b-42d3-a456-426614174000",
  "fingerprint": "...",
  "fingerprint_algorithm": "sha256",
  "hostname": "agent-node-01",
  "hostname_source": "os",
  "persistence": "persistent",
  "store_source": "loaded",
  "store_path": "data/identity/agent.json"
}
```

The endpoint intentionally exposes only local technical identity metadata. Registration with Media Hub, authentication, cluster membership and remote registry state remain reserved for later roadmap phases.


## Worker REST routes

v0.1.24 introduces worker REST route contracts:

```text
GET /worker
GET /worker/jobs
POST /worker/jobs
```

`GET /worker` returns runtime, heartbeat and queue summary metadata. `GET /worker/jobs` lists queued jobs from the local queue snapshot. `POST /worker/jobs` validates a mechanical transfer job with `worker.NewJob`, enqueues it and persists the queue snapshot when a persister is configured.

Example create payload:

```json
{
  "source_url": "https://example.com/movie.mp4",
  "destination_key": "media/org/movie.mp4",
  "max_attempts": 2,
  "metadata": {
    "source": "media-hub"
  }
}
```

The route remains business-rule-free. Customer ownership, subscription state, catalog placement and any Media Hub workflow decisions stay outside the Agent.


## Communication routes

v0.1.34 introduces foundation communication routes:

```text
GET /api/v1
GET /api/v1/ws
POST /api/v1/registration
GET /api/v1/heartbeat
POST /api/v1/heartbeat
```

`GET /api/v1` returns capability and route metadata. `GET /api/v1/ws` performs only the RFC 6455 upgrade handshake and closes the foundation request; it does not implement event streaming yet. `POST /api/v1/registration` acknowledges local technical Agent registration using the current identity snapshot. Heartbeat endpoints return the current local heartbeat payload.

Authentication is controlled by:

```yaml
security:
  api_key_required: false
  api_key: ""
  token_header: Authorization
```

When enabled, routes accept the raw key, `Bearer <key>` or `ApiKey <key>` in the configured header. Authentication is intentionally transport-level only and does not implement RBAC, JWT or Media Hub policy.

## Runtime behavior

The bootstrap creates a route registry with foundation, worker and communication routes, a default middleware registry and builds the router to prove wiring, but it does not call `ListenAndServe` and does not expose network endpoints yet.

Current startup summary includes:

```text
router=chi middlewares=2 routes=12
status: communication-foundation-ready
```

## Reserved for later phases

The following items are intentionally not implemented in v0.1.24:

- HTTP server lifecycle;
- graceful shutdown;
- WebSocket event streaming;
- Media Hub remote registration loop;
- HTTP network listener lifecycle.

Those features remain assigned to later communication, worker, production and security epics.


## Communication telemetry — v0.1.35

Telemetry route contracts are mounted under the communication prefix:

```text
GET /api/v1/metrics
GET /api/v1/events
POST /api/v1/events
```

`GET /api/v1/metrics` returns JSON summary data for runtime, heartbeat, queue and download metrics. `GET /api/v1/events` returns retained local diagnostic events. `POST /api/v1/events` validates and stores a local event in the bounded in-memory recorder.

These endpoints are transport-level diagnostics only. They do not publish centralized logs, observability dashboards or Media Hub policy.
