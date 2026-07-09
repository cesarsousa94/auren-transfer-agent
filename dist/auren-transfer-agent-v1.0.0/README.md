# Auren Transfer Agent

**Version:** v1.0.0  
**Status:** Foundation / Security Foundation

The Auren Transfer Agent is a Go service responsible for executing high-reliability media transfers. It receives jobs from Auren Media Hub, resolves complex URLs, downloads media, uploads the result to Auren Storage and returns operation status.

This repository intentionally keeps business decisions outside the Agent. The Agent executes jobs. Auren Media Hub owns all business rules.

## v1.0.0 scope

This delivery completes **EPIC 12 — Security** by shipping six requested subphases together:

- **12.1 — JWT**;
- **12.2 — API Keys**;
- **12.3 — mTLS**;
- **12.4 — RBAC**;
- **12.5 — Rate Limit**;
- **12.6 — Secrets**.

Added in this version:

- `internal/security` with foundation JWT, API-key, mTLS, RBAC, rate-limit and secrets contracts;
- expanded security configuration for JWT, API key hashes, mTLS, RBAC, rate limiting and secrets provider selection;
- bootstrap diagnostics for every security primitive;
- `docs/development/security.md` with the new security foundation contract.

The Agent still does not start a listening HTTP server, issue production tokens to external systems, manage certificate authorities or connect to a centralized secrets backend. Security components are compiled and wired as local foundation contracts only.

## Architecture

```text
Auren Media Hub
      ↓
Auren Transfer Agent
      ↓
Resolver
      ↓
Downloader
      ↓
Uploader
      ↓
Auren Storage
```

## Requirements

- Go 1.22 or newer.
- Make is optional but recommended.
- Task is optional and only needed when using `Taskfile.yml`.

## Configuration

The agent loads configuration through the Viper contract. The v1.0.0 ZIP is self-contained for offline compilation.

Default search order when `--config` is not provided:

1. current working directory;
2. `./configs`;
3. `/etc/auren-transfer-agent`.

If no config file exists, built-in defaults from `DefaultConfig()` are used. Environment variables with the `AUREN_` prefix override both defaults and YAML values. The final merged configuration is validated before the Agent reports readiness.

Run with an explicit file:

```bash
go run ./cmd/agent --config ./configs/agent.yaml
```

Override values from the environment:

```bash
AUREN_RUNTIME_ENVIRONMENT=production AUREN_LOGGER_LEVEL=debug AUREN_LOGGER_FORMAT=console AUREN_SERVER_PORT=8188 AUREN_WORKER_ENABLED=true go run ./cmd/agent --config ./configs/agent.yaml
```

Current top-level YAML sections:

```text
app, runtime, logger, server, worker, queue, resolver, download, upload, storage, metrics, heartbeat, security
```

## HTTP foundation

The HTTP foundation provides routing, middleware composition and canonical diagnostic route contracts. It does not open a network listener yet.

Current foundation endpoint contracts include:

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
GET /api/v1/metrics
GET /api/v1/events
POST /api/v1/events
GET /metrics
GET /api/v1/observability
GET /api/v1/observability/grafana/dashboard
GET /api/v1/observability/traces
POST /api/v1/observability/traces
GET /api/v1/observability/audit
POST /api/v1/observability/audit
GET /api/v1/observability/alerts
GET /api/v1/observability/logs
POST /api/v1/observability/logs
```

Authentication is disabled by default. When `security.api_key_required=true`, communication, telemetry and observability routes require the configured key in `security.token_header`, either as the raw key or as `Bearer <key>` / `ApiKey <key>`.

## Observability foundation

EPIC 11 is complete in v1.0.0.

Current observability contracts include:

```text
observability.Prometheus
observability.DefaultGrafanaDashboard
observability.TraceRecorder
observability.NewTraceRecorder
observability.AuditRecorder
observability.NewAuditRecorder
observability.EvaluateAlerts
observability.NewDashboard
observability.CentralLogSink
observability.NewCentralLogSink
server.ObservabilityRoutes
server.ObservabilityCapabilities
server.PrometheusHandler
server.ObservabilityDashboardHandler
server.GrafanaDashboardHandler
server.TracesListHandler
server.TracesStoreHandler
server.AuditListHandler
server.AuditStoreHandler
server.AlertsHandler
server.CentralLogsListHandler
server.CentralLogsStoreHandler
```

The Prometheus endpoint renders a stable text exposition format from local Agent state. The Grafana endpoint returns a static importable dashboard definition. Tracing, audit and centralized logs are bounded in-memory stores for local diagnostics and future exporters. Alerts are mechanically evaluated from local queue, heartbeat and download summaries. Dashboard output combines the local snapshot, capabilities and active alerts.

## Resolver foundation

EPIC 7 is complete in v0.1.32. Bootstrap registers resolvers in deterministic order:

```text
xtream -> shui -> cloudflare -> hls -> m3u8 -> google_drive -> mega -> onedrive -> redirect -> http
```

Xtream and Shui/XUI resolvers classify provider-specific URLs without contacting the provider. Cloudflare classification never attempts to bypass challenges. HLS/M3U8 resolvers fetch only bounded manifests. Cloud storage resolvers classify public sharing URLs and mask share keys or tokens in metadata.

## Upload and storage foundation

EPIC 8 is complete in v0.1.33. Current upload and storage contracts include local upload, multipart upload, resume upload, integrity validation, callback sender, generic storage adapter, local storage adapter and Auren Storage HTTP foundation adapter.

The local uploader writes files below `storage.local_path` and accepts only relative destination paths. The Auren Storage adapter streams a source file to the configured endpoint and bucket when `storage.endpoint` and `storage.bucket` are configured.

## Communication and cluster foundation

EPIC 9 is complete through communication metrics/events. EPIC 10 is complete in v0.1.36. Current cluster contracts include memory queue, Redis Streams foundation adapter, RabbitMQ foundation adapter, NATS foundation adapter, local Agent registry, least-loaded selection, deterministic leader election and mechanical failover planning.

Redis Streams, RabbitMQ and NATS are offline-compilable foundation adapters. They do not open external network connections in v0.1.x.

## Security foundation

Security primitives are available in `internal/security` and configured through the `security` YAML section. The foundation includes JWT HS256 signing/validation, API key raw/hash verification, mTLS state validation, deterministic RBAC, fixed-window rate limiting and local secret redaction/loading.

See `docs/development/security.md` for the contract details.

## Build

```bash
make build
```

The binary will be created at:

```text
bin/auren-transfer-agent
```

## Run

```bash
make run
```

Expected default output includes one structured JSON startup line followed by the foundation readiness summary:

```text
{"agent_id":"...","component":"bootstrap","environment":"local","fingerprint":"...","hostname":"...","hostname_source":"os","level":"info","message":"agent initialized","service":"auren-transfer-agent","status":"production/ready","time":"...","version":"v1.0.0"}
auren-transfer-agent v1.0.0 initialized
status: observability-foundation-ready
identity: agent_id=... fingerprint=... algorithm=sha256 persistence=persistent source=created path=data/identity/agent.json
host: hostname=... source=os raw="..."
queue: driver=memory mode=local endpoint= name= capacity=100 queued=0 poll_interval=1s source=created restored=0 path=data/worker/queue.json snapshot=saved
communication: rest=/api/v1 websocket=/api/v1/ws registration=/api/v1/registration heartbeat=/api/v1/heartbeat metrics=/api/v1/metrics events=/api/v1/events authentication=disabled token_header=Authorization ...
observability: prometheus=prometheus path=/metrics grafana=grafana tracing=tracing spans=1 audit=audit audit_events=1 alerts=alerts active_alerts=0 dashboard=dashboard centralized_logs=centralized_logs log_records=1 ...
config: app=auren-transfer-agent environment=local logger=info/json router=chi middlewares=2 routes=25 server=0.0.0.0:8080 worker=false queue=memory storage=local metrics=0.0.0.0:9090
```

Console output can be enabled explicitly:

```bash
AUREN_LOGGER_FORMAT=console go run ./cmd/agent
```

## Version

```bash
make version
```

Expected output:

```text
auren-transfer-agent v1.0.0 (production/ready)
```

## Test

```bash
make test
```

## Development rules

- Every delivery is a complete ZIP.
- Never ship patches as the official artifact.
- Every new version replaces the previous one completely.
- Every ZIP must compile.
- README, CHANGELOG and ROADMAP must always be updated.
- Each phase should modify no more than about twenty files.
- Existing functionality must not be removed.
- The definitive project structure must not be reorganized.

## Current phase

This repository is currently at **v1.0.0 — Production**. EPIC 1 through EPIC 13 are complete as the official production baseline.


## Production deployment

The v1.0.0 ZIP includes the complete EPIC 13 production baseline:

- Docker image definition: `docker/Dockerfile`;
- Docker Compose stack: `docker/docker-compose.yml`;
- Linux installer: `deploy/linux/install.sh`;
- systemd service and env template: `deploy/systemd/`;
- Kubernetes manifest: `deploy/kubernetes/auren-transfer-agent.yaml`;
- CI workflow: `.github/workflows/ci.yml`;
- Release pipeline: `scripts/release.sh`;
- Deployment guide: `docs/deployment/production.md`.

Build locally:

```bash
make build
./bin/auren-transfer-agent --version
```

Run the HTTP runtime locally:

```bash
AUREN_SERVER_ENABLED=true go run ./cmd/agent --config ./configs/agent.yaml
```

Build the Docker image:

```bash
make docker-build
```

Create a release archive:

```bash
make release
```

The Agent still does not contain Media Hub business rules. Production artifacts only package, supervise and deploy the already-defined Agent runtime contracts.
