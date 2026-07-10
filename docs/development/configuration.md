# Configuration Development Notes

Auren Transfer Agent v1.9.0 keeps the configuration contract stable and expands the Media Hub section with operational hardening controls while retaining Gateway Runtime controls and the Auren Storage production adapter fallback key. This file defines the configuration surface, precedence rules, default APIs and validation rules used by the production runtime.

## Search order

When no explicit path is provided, the agent searches for `agent.yaml` in the directories returned by `config.DefaultSearchPaths()`:

1. current working directory;
2. `./configs`;
3. `/etc/auren-transfer-agent`.

If no file is found, the agent starts with built-in defaults.

## Precedence

Configuration values are resolved in this order:

1. built-in defaults from `config.DefaultConfig()`;
2. YAML file values;
3. environment variables with the `AUREN_` prefix.

Environment variables always win over YAML and defaults. The merged result is validated before startup readiness is reported.

## Defaults contract

The defaults contract is centralized in `internal/config/defaults.go`.

Current public helpers:

- `DefaultConfig()` returns the typed built-in `Config` baseline;
- `DefaultValues()` returns dotted-key defaults used by the Viper registration layer;
- `DefaultSearchPaths()` returns the canonical discovery directories;
- `DefaultEnvPrefix` defines the environment override prefix.

These helpers return independent values where mutation could otherwise leak state.

## Explicit file

```bash
auren-transfer-agent --config ./configs/agent.yaml
```

When `--config` is provided, the file must exist and must be readable.

## Environment overrides

All environment overrides use the `AUREN_` prefix. Dotted keys are converted to uppercase snake case.

Examples:

| YAML key | Environment variable |
| --- | --- |
| `runtime.environment` | `AUREN_RUNTIME_ENVIRONMENT` |
| `logger.level` | `AUREN_LOGGER_LEVEL` |
| `server.host` | `AUREN_SERVER_HOST` |
| `server.port` | `AUREN_SERVER_PORT` |
| `worker.enabled` | `AUREN_WORKER_ENABLED` |
| `queue.memory_capacity` | `AUREN_QUEUE_MEMORY_CAPACITY` |
| `download.max_retries` | `AUREN_DOWNLOAD_MAX_RETRIES` |
| `storage.endpoint` | `AUREN_STORAGE_ENDPOINT` |
| `storage.bucket` | `AUREN_STORAGE_BUCKET` |
| `storage.api_key` | `AUREN_STORAGE_API_KEY` |
| `security.api_key` | `AUREN_SECURITY_API_KEY` |
| `security.allow_insecure_http` | `AUREN_SECURITY_ALLOW_INSECURE_HTTP` |
| `media_hub.transfer_enabled` | `AUREN_MEDIA_HUB_TRANSFER_ENABLED` |
| `media_hub.claim_enabled` | `AUREN_MEDIA_HUB_CLAIM_ENABLED` |
| `media_hub.max_concurrent_jobs` | `AUREN_MEDIA_HUB_MAX_CONCURRENT_JOBS` |

Example run:

```bash
AUREN_RUNTIME_ENVIRONMENT=production \
AUREN_LOGGER_LEVEL=debug \
AUREN_SERVER_PORT=8188 \
AUREN_WORKER_ENABLED=true \
auren-transfer-agent --config ./configs/agent.yaml
```

## Official YAML sections

The current contract reserves these top-level sections:

- `app` for process metadata;
- `runtime` for local directories and environment name;
- `logger` for structured logger settings;
- `server` for future HTTP bind/timeouts;
- `worker` for future worker pool sizing;
- `queue` for future queue behavior;
- `resolver` for future URL resolution behavior;
- `download` for future download engine behavior;
- `upload` for future upload engine behavior;
- `storage` for local storage and Auren Storage v1 adapter behavior;
- `metrics` for future metrics endpoint behavior;
- `heartbeat` for future heartbeat loop behavior;
- `security` for authentication/security behavior;
- `media_hub` for Media Hub registration, persisted node credentials, HMAC, telemetry intervals, role/provider/region, public URLs, capacity and capabilities.

## Current full example

```yaml
app:
  name: auren-transfer-agent
  description: High-reliability media transfer agent

runtime:
  environment: local
  data_dir: ./data
  temp_dir: ./tmp

logger:
  level: info
  format: json
  timestamp: true
  service: auren-transfer-agent

server:
  enabled: false
  host: 0.0.0.0
  port: 8080
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 60s

worker:
  enabled: false
  concurrency: 1
  shutdown_timeout: 30s

queue:
  driver: memory
  memory_capacity: 100
  poll_interval: 1s

resolver:
  default_user_agent: AurenTransferAgent/0.1
  follow_redirects: true
  max_redirects: 10

download:
  temp_dir: ./tmp/downloads
  connect_timeout: 15s
  response_header_timeout: 30s
  idle_timeout: 60s
  max_retries: 3
  retry_backoff: 2s
  chunk_size: 8MiB
  resume_enabled: true
  checksum: sha256

upload:
  driver: local
  max_retries: 3
  retry_backoff: 2s
  multipart_enabled: true
  part_size: 16MiB

storage:
  driver: local
  endpoint: ""
  bucket: ""
  api_key: ""
  region: us-east-1
  local_path: ./data/storage
  use_path_style: true

metrics:
  enabled: false
  host: 0.0.0.0
  port: 9090
  path: /metrics

heartbeat:
  enabled: false
  interval: 30s
  timeout: 10s

security:
  api_key_required: false
  api_key: ""
  token_header: Authorization
  allow_insecure_http: true

media_hub:
  enabled: false
  base_url: ""
  registration_token: ""
  node_uuid: ""
  node_secret: ""
  hmac_enabled: true
  poll_enabled: false
  poll_interval: 2s
  heartbeat_interval: 30s
  metrics_interval: 60s
  events_flush_interval: 10s
  role: gateway
  provider: auren_transfer_agent
  region: sa-east-1
  public_base_url: ""
  health_url: ""
  max_sessions: 500
  max_egress_mbps: 1000
  capabilities: transfer,gateway,download,upload,auren_storage,xtream,shui,m3u8,hls,live,movie,series
```

## Validation contract

Validation runs after the final configuration is assembled from defaults, YAML and environment variables. It is intentionally structural and does not contain Auren Media Hub business rules.

Current checks include:

- required strings for process metadata, paths, hosts and headers;
- `runtime.environment` limited to `local`, `development`, `test`, `staging` or `production`;
- `logger.level` limited to `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic` or `disabled`;
- `logger.format` limited to `json` or `console`;
- TCP ports in the `1..65535` range;
- positive values for worker concurrency and queue capacity;
- non-negative retry and redirect counters;
- positive Go duration strings such as `500ms`, `1s`, `30s` and `2m`;
- positive byte sizes using plain bytes or `B`, `KB`, `MB`, `GB`, `KiB`, `MiB` and `GiB`;
- `metrics.path` starting with `/`;
- current foundation drivers: `queue.driver=memory|redis_streams|rabbitmq|nats`, `upload.driver=local`, `storage.driver=local|auren_storage`;
- current checksum options: `sha256` or `none`;
- when `media_hub.enabled=true`, `media_hub.base_url`, role/provider/region, durations and capabilities are validated structurally.

Invalid configurations return one aggregated error with field-level messages so operators can fix every issue in one pass.

## Compatibility notes

- Environment overrides are complete in EPIC 1.3.
- Defaults are centralized in EPIC 1.4.
- Validation is complete in EPIC 1.5.
- The logger section starts EPIC 2.1 and currently supports structured JSON output plus opt-in console rendering.
- The local compatibility module supports the YAML subset used by this official contract: nested maps, strings, integers and booleans. Lists are not part of the current lightweight contract, so `media_hub.capabilities` is comma-separated.
- Durations and sizes are parsed by configuration validation, but engines will still create their own typed runtime adapters in later phases.


## Queue adapter configuration — v0.1.36

The default queue driver remains `memory`. Foundation cluster adapters can be selected for contract testing and diagnostics:

```yaml
queue:
  driver: memory # memory, redis_streams, rabbitmq, nats
  memory_capacity: 100
  poll_interval: 1s
  redis_address: redis://localhost:6379
  redis_stream: auren.transfer.jobs
  redis_consumer_group: auren-transfer-agents
  rabbitmq_url: amqp://guest:guest@localhost:5672/
  rabbitmq_queue: auren.transfer.jobs
  nats_url: nats://localhost:4222
  nats_subject: auren.transfer.jobs
  nats_queue_group: auren-transfer-agents
```

`redis_streams` requires `redis_address`, `redis_stream` and `redis_consumer_group`. `rabbitmq` requires `rabbitmq_url` and `rabbitmq_queue`. `nats` requires `nats_url`, `nats_subject` and `nats_queue_group`. All cluster adapters are offline-compilable foundation contracts in v0.1.36 and do not open external network connections.


## Gateway runtime keys

- `media_hub.gateway_enabled`: exposes the public `/_auren/gateway/*` handoff route when the HTTP server is enabled.
- `media_hub.gateway_proxy_enabled`: allows Media Hub resolve responses to use proxy mode.
- `media_hub.gateway_redirect_enabled`: allows Media Hub resolve responses to use redirect mode.
- `media_hub.gateway_heartbeat_interval`: fallback session heartbeat interval when Media Hub does not provide one.
- `media_hub.gateway_token_ttl`: local policy hint for public handoff token lifetime; Media Hub remains the source of truth.

## v1.9.0 operational hardening keys

| Key | Environment variable |
| --- | --- |
| `media_hub.drain_enabled` | `AUREN_MEDIA_HUB_DRAIN_ENABLED` |
| `media_hub.drain_file` | `AUREN_MEDIA_HUB_DRAIN_FILE` |
| `media_hub.backpressure_enabled` | `AUREN_MEDIA_HUB_BACKPRESSURE_ENABLED` |
| `media_hub.disk_guard_enabled` | `AUREN_MEDIA_HUB_DISK_GUARD_ENABLED` |
| `media_hub.disk_min_free_bytes` | `AUREN_MEDIA_HUB_DISK_MIN_FREE_BYTES` |
| `media_hub.dead_letter_enabled` | `AUREN_MEDIA_HUB_DEAD_LETTER_ENABLED` |
| `media_hub.dead_letter_dir` | `AUREN_MEDIA_HUB_DEAD_LETTER_DIR` |
| `media_hub.lease_renewal_enabled` | `AUREN_MEDIA_HUB_LEASE_RENEWAL_ENABLED` |
| `media_hub.lease_renewal_interval` | `AUREN_MEDIA_HUB_LEASE_RENEWAL_INTERVAL` |
| `media_hub.secret_rotation_enabled` | `AUREN_MEDIA_HUB_SECRET_ROTATION_ENABLED` |
| `media_hub.secret_rotation_interval` | `AUREN_MEDIA_HUB_SECRET_ROTATION_INTERVAL` |
