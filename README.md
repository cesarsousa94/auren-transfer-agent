# Auren Transfer Agent

**Version:** v1.9.1  
**Status:** Production / Signed APT Distribution

The Auren Transfer Agent is a Go service responsible for executing high-reliability media transfers and serving Media Hub gateway handoff traffic. It receives jobs from Auren Media Hub, resolves complex URLs, downloads media, uploads the result to Auren Storage, returns operation status and can proxy or redirect public streaming sessions without Laravel carrying the video body.

This repository intentionally keeps business decisions outside the Agent. The Agent executes jobs. Auren Media Hub owns all business rules.

## v1.9.1 scope

This delivery turns the online APT repository into the recommended production distribution path. It keeps the v1.6.0 Linux package/bootstrap baseline and the v1.7.0 static APT repository, then adds signed-release-first publishing, `stable`/`edge` channel generation, exported public GPG key artifacts and a Media Hub install-command template.

Added in this version:

- signed APT repository release files (`InRelease` and `Release.gpg`) when `APT_SIGN=true`;
- public key artifacts `auren-transfer-agent.gpg` and `.asc`;
- default `stable,edge` channel generation in the release pipeline;
- generated `install-apt.sh` and `install.sh` in the repository root;
- `install-command-template.json` for Media Hub Provider Nodes;
- `scripts/generate-install-command.sh`;
- `scripts/export-apt-gpg-key.sh`;
- improved S3/CloudFront publishing for key, metadata and installer files;
- release artifact `auren-transfer-agent-apt-repo-v1.9.1.tar.gz`.

Build the package and repository:

```bash
APT_SIGN=true APT_GPG_KEY_ID="YOUR_GPG_KEY_ID" ./scripts/release.sh v1.9.1
```

Install from an online APT repository and bootstrap the node:

```bash
curl -fsSL https://downloads.auren.app/agent/apt/install-apt.sh | sudo bash -s -- \
  --repo-url https://downloads.auren.app/agent/apt \
  --apt-key-url https://downloads.auren.app/agent/apt/auren-transfer-agent.gpg \
  --media-hub https://media.example.com \
  --token REGISTRATION_TOKEN \
  --role worker \
  --region sa-east-1
```

For gateway mode add `--enable-gateway --public-base-url=https://node1.example.com`.

## Architecture

```text
Auren Media Hub
      ↓ claim/callback + gateway resolve
Auren Transfer Agent
      ├─ transfer executor → downloader/uploader → Auren Storage
      └─ gateway runtime → proxy/redirect → upstream IPTV/media source
```

## Requirements

- Go 1.22 or newer.
- Make is optional but recommended.
- Task is optional and only needed when using `Taskfile.yml`.


## Linux package CLI

```bash
auren-transfer-agent bootstrap --media-hub https://media.example.com --token TOKEN --start-service
auren-transfer-agent doctor --config /etc/auren-transfer-agent/agent.yaml
auren-transfer-agent status --config /etc/auren-transfer-agent/agent.yaml
auren-transfer-agent serve --config /etc/auren-transfer-agent/agent.yaml
```

The `.deb` package installs the binary as `/usr/bin/auren-transfer-agent` and the service as `auren-transfer-agent.service`. The service survives terminal logout and machine reboot because systemd restarts and enables it.


## Local Dev Console

Auren Transfer Agent v1.9.1 includes a lightweight local console for development and node validation. With `server.enabled=true` and `dev_ui.enabled=true`, open:

```text
http://127.0.0.1:8080/_auren/dev/metrics
http://127.0.0.1:8080/_auren/dev/requests
```

The metrics page shows Media Hub registration state, transfer jobs, gateway sessions, queue state, hardening decisions and request counters. The requests page shows inbound Agent HTTP requests and outbound Media Hub calls with status, duration, bytes and error messages.

For EC2 testing, keep the console private and access it through an SSH tunnel:

```bash
ssh -L 8080:127.0.0.1:8080 ubuntu@NODE_PUBLIC_IP
```

Then open `http://127.0.0.1:8080/_auren/dev/metrics` locally.

## Configuration

The agent loads configuration through the Viper contract. The v1.9.1 ZIP is self-contained for offline compilation.

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
app, runtime, logger, server, worker, queue, resolver, download, upload, storage, metrics, heartbeat, security, media_hub
```


### Media Hub connector quick start

```bash
AUREN_MEDIA_HUB_ENABLED=true \
AUREN_MEDIA_HUB_BASE_URL=https://mediahub.example.com \
AUREN_MEDIA_HUB_REGISTRATION_TOKEN=one-time-token \
AUREN_MEDIA_HUB_PUBLIC_BASE_URL=https://node-1.example.com \
AUREN_MEDIA_HUB_TRANSFER_ENABLED=true \
AUREN_MEDIA_HUB_CLAIM_ENABLED=true \
AUREN_MEDIA_HUB_MAX_CONCURRENT_JOBS=2 \
AUREN_SERVER_ENABLED=true \
go run ./cmd/agent --config ./configs/agent.yaml
```

The connector persists Media Hub-issued credentials at `runtime.data_dir/media-hub/node.json`. After the first registration, the same node UUID/secret are reused automatically. When `media_hub.transfer_enabled=true` and `media_hub.claim_enabled=true`, the Agent claims remote transfer jobs from Media Hub, downloads payloads, uploads them through the production Auren Storage adapter when requested, and reports started/progress/completed/failed callbacks. When `media_hub.gateway_enabled=true`, the Agent also exposes `/_auren/gateway/{token}/{kind}/{id}.{ext}` for Media Hub Capacity Routing handoff.

## HTTP foundation

The HTTP runtime provides routing, middleware composition and canonical diagnostic route contracts. It opens a network listener when `server.enabled=true`.

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

EPIC 11 is complete in the production baseline.

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

EPIC 8 is complete in v0.1.33 and EPIC 16 upgrades the Auren Storage adapter to the production v1 contract. Current upload and storage contracts include local upload, multipart upload, resume upload, integrity validation, callback sender, generic storage adapter, local storage adapter and Auren Storage v1 HTTP adapter.

The local uploader writes files below `storage.local_path` and accepts only relative destination paths. The Auren Storage adapter uploads to `POST /api/v1/buckets/{bucket_uuid}/objects` using `multipart/form-data` for normal files and initiate/parts/complete endpoints for large files when `upload.multipart_enabled=true`. The adapter propagates `directory_path`, `relative_path`, `visibility`, `mime_type`, metadata and SHA-256 checksum, and returns object UUID/path/size/checksum details to the Media Hub callback.

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
{"agent_id":"...","component":"bootstrap","environment":"local","fingerprint":"...","hostname":"...","hostname_source":"os","level":"info","message":"agent initialized","service":"auren-transfer-agent","status":"production/ready","time":"...","version":"v1.9.1"}
auren-transfer-agent v1.9.1 initialized
status: production-ready
identity: agent_id=... fingerprint=... algorithm=sha256 persistence=persistent source=created path=data/identity/agent.json
host: hostname=... source=os raw="..."
queue: driver=memory mode=local endpoint= name= capacity=100 queued=0 poll_interval=1s source=created restored=0 path=data/worker/queue.json snapshot=saved
communication: rest=/api/v1 websocket=/api/v1/ws registration=/api/v1/registration heartbeat=/api/v1/heartbeat metrics=/api/v1/metrics events=/api/v1/events authentication=disabled token_header=Authorization ...
observability: prometheus=prometheus path=/metrics grafana=grafana tracing=tracing spans=1 audit=audit audit_events=1 alerts=alerts active_alerts=0 dashboard=dashboard centralized_logs=centralized_logs log_records=1 ...
config: app=auren-transfer-agent environment=local logger=info/json router=chi middlewares=2 routes=25 server=0.0.0.0:8080 worker=false queue=memory storage=local metrics=0.0.0.0:9090 media_hub=false
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
auren-transfer-agent v1.9.1 (production/ready)
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

This repository is currently at **v1.9.1 — Signed APT Repository & Media Hub Install Command**. EPIC 1 through EPIC 16 remain the production/connector/transfer/storage baseline, and EPIC 17 adds Media Hub public streaming handoff through the Agent.


## Production deployment

The v1.9.1 ZIP includes the complete EPIC 13 production baseline, EPIC 14 Media Hub connector foundation, EPIC 15 real transfer executor, EPIC 16 Auren Storage production adapter and EPIC 17 Operational Hardening:

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
