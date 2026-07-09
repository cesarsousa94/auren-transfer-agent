# Changelog

## [v1.6.0] - 2026-07-09

### Added

- Added Linux Package & Zero-Touch Bootstrap delivery.
- Added CLI subcommands: `serve`, `bootstrap`, `doctor` and `status`.
- Added bootstrap flow that writes production config, registers with Media Hub using a one-time token and persists `node_uuid`/`node_secret`.
- Added Debian package build pipeline through `scripts/build-deb.sh`.
- Added `.deb` control scripts: `postinst`, `prerm`, `postrm` and `conffiles`.
- Added canonical Linux package layout under `/usr/bin`, `/etc/auren-transfer-agent`, `/var/lib/auren-transfer-agent`, `/var/log/auren-transfer-agent` and `/var/tmp/auren-transfer-agent`.
- Added canonical `auren-agent` system user/group for package installs.
- Added one-line installer script with `--media-hub`, `--token`, `--role`, `--enable-gateway`, `--public-base-url` and `--max-concurrent-jobs`.
- Added APT repository skeleton generator through `scripts/build-apt-repo.sh`.
- Added deployment guide `docs/deployment/linux-package-bootstrap.md`.

### Changed

- systemd now runs `/usr/bin/auren-transfer-agent serve --config /etc/auren-transfer-agent/agent.yaml`.
- Release pipeline now builds ZIP plus `.deb` and SHA-256 sidecar files.
- Docker, Compose, Kubernetes, CI and production docs now reference `v1.6.0`.

### Notes

- This version does not add new transfer/gateway business logic. It turns the v1.5.0 production Agent into a Linux-installable background service.
- APT repository publishing/signing is prepared as a skeleton; private GPG key management remains outside the repository.

## [v1.2.0] - 2026-07-09

### Added

- Added EPIC 15 — Real Transfer Executor.
- Added `internal/transfer` with Media Hub pull-claim manager, real transfer executor, local job-state persistence and worker handler adapter.
- Added Media Hub transfer-agent client methods for `/api/internal/transfer-agent/jobs/claim`, `started`, `progress`, `completed`, `failed`, `events`, `release` and `control`.
- Added guarded download execution with resume support, SHA-256 checksum, tiny-body validation, HTML response blocking and progress callbacks.
- Added upload execution for local storage, Auren Storage foundation adapter and signed/scoped upload URL payloads.
- Added transfer capacity reporting into heartbeat/metrics through `active_jobs` and `max_concurrent_jobs`.
- Added tests for transfer execution, HTML blocking and claim payload parsing.

### Changed

- Bootstrap now wires `transfer_executor` as the worker handler instead of the noop handler.
- Media Hub connector telemetry now includes live transfer executor capacity snapshots.
- Docker, Compose, systemd, Kubernetes, README, ROADMAP and deployment documentation now reference `v1.2.0`.

### Notes

- This version expects the Media Hub side to expose the 47.15/47.16 transfer-agent claim/callback endpoints.
- Public gateway runtime remains reserved for v1.6.0.
- Full production multipart Auren Storage adapter remains reserved for v1.3.0.

## [v1.1.0] - 2026-07-09

### Added

- Added EPIC 14 — Media Hub Connector Foundation.
- Added `media_hub` runtime configuration for Media Hub base URL, registration token, persisted node credentials, HMAC, telemetry intervals, role/provider/region, public URLs, capacity and capabilities.
- Added `internal/mediahub` with durable node state, NodeAgentContractService client, registration, config pull, heartbeat, metrics, events and v1 HMAC signing.
- Bootstrap now registers the Agent as a Media Hub `edge_node`, persists `node_uuid`/`node_secret`, pulls config and emits heartbeat/metrics/events when `media_hub.enabled=true`.
- Added connector contract tests for HMAC, response parsing and local state persistence.

### Notes

- Real transfer execution remains intentionally unchanged and still uses the existing noop worker foundation. Download/upload offload is reserved for v1.2.0.
- Public gateway runtime remains reserved for v1.6.0.

## [v1.0.0] - 2026-07-09

### Added

- Completed EPIC 13 — Production with Docker, Docker Compose, Linux installer, systemd unit, Kubernetes manifest, CI/CD workflow and release pipeline.
- Added production HTTP server lifecycle support when `server.enabled` / `AUREN_SERVER_ENABLED` is true, including graceful shutdown on SIGTERM/SIGINT.
- Added `docker/Dockerfile` and `docker/docker-compose.yml` for container packaging.
- Added Linux installer and systemd deployment files under `deploy/`.
- Added Kubernetes deployment manifest under `deploy/kubernetes/`.
- Added GitHub Actions CI workflow and `scripts/release.sh`.
- Added production deployment documentation and production artifact contract tests.

### Changed

- Updated runtime metadata to `v1.0.0` with status `production/ready`.
- Expanded Makefile and Taskfile with serve, Docker, Compose and release commands.

## [v0.1.38] - 2026-07-09

### Added

- Completed EPIC 12 — Security by delivering 12.1 — JWT, 12.2 — API Keys, 12.3 — mTLS, 12.4 — RBAC, 12.5 — Rate Limit and 12.6 — Secrets in one delivery.
- Added `internal/security` with foundation JWT, API-key, mTLS, RBAC, rate-limit and secrets contracts.
- Added SHA-256 API key hash support, constant-time API key verification and deterministic fixed-window rate limiting.
- Added local secrets loading/redaction and RBAC policy contracts for `admin`, `worker` and `observer` roles.
- Added security development documentation.

### Changed

- Expanded the `security` configuration section with JWT, API key hash, mTLS, RBAC, rate limit and secrets keys.
- Bootstrap now wires local security contracts and reports security diagnostics.
- Updated runtime version metadata to `v0.1.38` with status `foundation/security-foundation`.

### Notes

- Security primitives are foundation contracts only. The Agent still does not start a network listener, manage certificate authorities, issue production tokens to remote callers or connect to a centralized secrets backend.
- EPIC 12 — Security is complete. Next roadmap item: EPIC 13.1 — Docker.

## [v0.1.37] - 2026-07-09

### Added

- Completed EPIC 11 — Observability by delivering 11.1 — Prometheus, 11.2 — Grafana, 11.3 — Tracing, 11.4 — Audit, 11.5 — Alerts, 11.6 — Dashboard and 11.7 — Centralized Logs in one delivery.
- Added `internal/observability` with Prometheus exposition, Grafana dashboard export, local tracing, audit recording, alert evaluation, dashboard snapshots and centralized log sink.
- Added `internal/server/observability.go` with `GET /metrics`, `GET /api/v1/observability`, Grafana, traces, audit, alerts and centralized logs route contracts.
- Added bounded in-memory recorders for traces, audit events and centralized logs.
- Added local alert evaluation over queue, download and heartbeat snapshots.

### Changed

- Bootstrap now wires observability recorders and routes after communication telemetry routes.
- Bootstrap now reports Prometheus/Grafana/tracing/audit/alerts/dashboard/centralized-log diagnostics.
- Updated runtime version metadata to `v0.1.37` with status `foundation/observability-foundation`.

### Notes

- Prometheus, Grafana, tracing, audit, alerts, dashboard and centralized logs are local foundation contracts. No external exporter, Grafana server, tracing backend, alert notifier or log shipper is started in this version.
- EPIC 11 — Observability is complete. Next roadmap item: EPIC 12.1 — JWT.

## [v0.1.36] - 2026-07-09

### Added

- Completed EPIC 10 — Cluster by delivering 10.4 — NATS, 10.5 — Agent Registry, 10.6 — Load Balancer, 10.7 — Leader Election and 10.8 — Failover in one delivery.
- Added NATS foundation queue adapter under the existing `queue.ClusterQueue` contract.
- Added queue configuration keys `nats_url`, `nats_subject` and `nats_queue_group`.
- Added `internal/cluster` with Agent registry, load balancer, leader election and failover planner contracts.
- Added deterministic local leader election using the lowest Agent fingerprint among available agents.
- Added deterministic least-loaded Agent selection and mechanical failover planning for queued jobs.

### Changed

- Bootstrap now creates a local cluster registry record from the persisted Agent identity and reports registry/load-balancer/leader/failover diagnostics.
- Bootstrap queue diagnostics now include NATS configuration and active driver details.
- Updated runtime version metadata to `v0.1.36` with status `foundation/cluster-coordination`.

### Notes

- NATS, Redis Streams and RabbitMQ remain offline-compilable foundation adapters. They do not open external network connections in this version.
- Agent registry, load balancer, leader election and failover are local deterministic contracts only; distributed coordination remains reserved for later production phases.
- EPIC 10 — Cluster is complete. Next roadmap item: EPIC 11.1 — Prometheus.

## [v0.1.35] - 2026-07-09

### Added

- Continued EPIC 9 — Communication by delivering 9.6 — Metrics API and 9.7 — Events API.
- Started EPIC 10 — Cluster by delivering 10.1 — Queue Interface, 10.2 — Redis Streams and 10.3 — RabbitMQ.
- Added `internal/server/telemetry.go` with `GET /api/v1/metrics`, `GET /api/v1/events` and `POST /api/v1/events`.
- Added local bounded `EventRecorder` and JSON telemetry responses for queue, heartbeat and download summaries.
- Added `internal/queue/adapters.go` with `ClusterQueue`, `Info`, `NewQueue`, Redis Streams adapter foundation and RabbitMQ adapter foundation.
- Added queue configuration keys for Redis Streams and RabbitMQ while keeping `memory` as the default driver.

### Changed

- Bootstrap now mounts telemetry routes after communication routes, reports event counts and reports cluster queue adapter diagnostics.
- Bootstrap now creates queues through `queue.NewQueue`, allowing the same worker contracts to use memory, Redis Streams foundation or RabbitMQ foundation drivers.
- Updated runtime version metadata to `v0.1.35` with status `foundation/communication-metrics-events-cluster-queues`.

### Notes

- Redis Streams and RabbitMQ adapters are offline-compilable foundation contracts. They do not open external network connections in this version.
- Communication routes are mounted into the foundation router, but the Agent still does not start a network listener.
- Next roadmap item: EPIC 10.4 — NATS.

## [v0.1.34] - 2026-07-09

### Added

- Started EPIC 9 — Communication by delivering 9.1 — REST API, 9.2 — WebSocket, 9.3 — Registration, 9.4 — Heartbeat and 9.5 — Authentication in one delivery.
- Added `internal/server/communication.go` with communication REST, WebSocket handshake, registration, heartbeat and API-key authentication contracts.
- Added canonical communication routes: `GET /api/v1`, `GET /api/v1/ws`, `POST /api/v1/registration`, `GET /api/v1/heartbeat` and `POST /api/v1/heartbeat`.
- Added optional `security.api_key` configuration with environment override support via `AUREN_SECURITY_API_KEY`.
- Added communication route tests covering REST, registration, heartbeat, WebSocket upgrade and authentication.

### Changed

- Bootstrap now mounts communication routes after foundation and worker routes, reporting communication diagnostics and authentication mode.
- Updated runtime version metadata to `v0.1.34` with status `foundation/communication-rest-websocket-auth`.
- Updated README, ROADMAP and server development notes.

### Notes

- Communication routes are mounted into the foundation router, but the Agent still does not start a network listener.
- WebSocket support is a foundation upgrade handshake only; event streaming remains reserved for later communication/events phases.
- Next roadmap item: EPIC 9.6 — Metrics API.

## [v0.1.33] - 2026-07-09

### Added

- Completed EPIC 8 — Upload Engine by delivering 8.4 — Resume Upload, 8.5 — Integrity Validation, 8.6 — Callback Engine, 8.7 — Storage Adapter and 8.8 — Auren Storage Adapter in one delivery.
- Added `internal/upload/resume.go` with local resume state inspection and append-based resume upload.
- Added `internal/upload/integrity.go` with SHA-256 size/checksum validation for uploaded files.
- Added `internal/upload/callback.go` with JSON HTTP callback sender and structured callback payloads.
- Added `internal/storage` with generic storage adapter contracts, local storage adapter and Auren Storage HTTP adapter foundation.
- Added storage adapter development documentation.

### Changed

- Upload results now expose `resumed` and `already_complete` flags for resume-aware operations.
- Configuration validation now accepts `storage.driver=auren_storage` and requires endpoint/bucket when that driver is selected.
- Bootstrap now creates local storage adapter diagnostics and reports Auren Storage adapter readiness when endpoint and bucket are configured.
- Updated runtime version metadata to `v0.1.33` with status `foundation/upload-storage-adapters`.
- Updated README, ROADMAP and upload development notes.

### Notes

- EPIC 8 — Upload Engine is complete.
- Auren Storage adapter support is an HTTP foundation contract only; it does not implement Media Hub policy, tenant provisioning or billing decisions.
- Next roadmap item: EPIC 9.1 — REST API.

## [v0.1.32] - 2026-07-09

### Added

- Completed EPIC 7.12 — Plugin SDK and started EPIC 8 — Upload Engine with 8.1 — Uploader Interface, 8.2 — Local Upload and 8.3 — Multipart Upload in one delivery.
- Added `pkg/plugins` with public SDK contracts for resolver and uploader plugins.
- Added `plugins.Manifest`, `plugins.ResolverPlugin`, `plugins.UploaderPlugin`, request/result DTOs and manifest validation/normalization helpers.
- Added `internal/upload` with `Uploader`, `Request`, `Result` and defensive copies.
- Added local upload support rooted at `storage.local_path`, with safe relative destination resolution.
- Added deterministic multipart upload planning and local multipart copy with part metadata.
- Added plugin SDK and upload development documentation.

### Changed

- Bootstrap now creates the local uploader from validated configuration and reports plugin/upload diagnostics.
- Updated runtime version metadata to `v0.1.32` with status `foundation/plugin-sdk-upload-foundation`.
- Updated README and ROADMAP to reflect EPIC 7 completion and EPIC 8 progress.

### Notes

- Plugin SDK support defines contracts only; it does not dynamically load external plugins yet.
- Local upload remains mechanical and business-rule free. Resume upload, integrity validation, callbacks and Auren Storage adapter remain future EPIC 8 items.
- Next roadmap item: EPIC 8.4 — Resume Upload.

## [v0.1.31] - 2026-07-09

### Added

- Continued EPIC 7 — Resolver Engine with 7.9 — Google Drive Resolver, 7.10 — MEGA Resolver and 7.11 — OneDrive Resolver in one delivery.
- Added `internal/resolver/cloud_storage.go` with metadata-only cloud-sharing resolvers for Google Drive, MEGA and OneDrive/SharePoint links.
- Added Google Drive parsing for `/file/d/{id}`, `/drive/folders/{id}` and `uc?id=...`, including derived file download URLs when mechanically possible.
- Added MEGA parsing for modern and legacy public file/folder links with key-presence detection and masked keys.
- Added OneDrive parsing for `onedrive.live.com`, `1drv.ms` and SharePoint sharing URLs with `resid`, `id`, `cid`, `authkey` and short-link metadata.
- Added cloud-storage resolver tests covering parsing, masking, derived URLs and registry participation.

### Changed

- Bootstrap now registers resolver order `xtream -> shui -> cloudflare -> hls -> m3u8 -> google_drive -> mega -> onedrive -> redirect -> http`.
- Startup diagnostics now report cloud-storage resolver names.
- Updated runtime version metadata to `v0.1.31` with status `foundation/resolver-cloud-storage`.
- Updated README, ROADMAP and resolver development documentation.

### Notes

- Cloud-storage resolvers do not call provider APIs, bypass protections, decrypt remote metadata or make Media Hub policy decisions.
- Shared tokens and keys are represented by presence flags and masked fields in metadata.
- Next roadmap item: EPIC 7.12 — Plugin SDK.

## [v0.1.30] - 2026-07-09

### Added

- Continued EPIC 7 — Resolver Engine with 7.5 — Redirect Resolver, 7.6 — Cloudflare Resolver, 7.7 — M3U8 Resolver and 7.8 — HLS Resolver in one delivery.
- Added `internal/resolver/redirect.go` with redirect-aware metadata resolution, final URL reporting and redirect-chain diagnostics.
- Added `internal/resolver/cloudflare.go` with opt-in Cloudflare response classification that never bypasses protections.
- Added `internal/resolver/m3u8.go` with bounded M3U8 manifest fetching and playlist metadata parsing.
- Added `internal/resolver/hls.go` with HLS master/media playlist classification.
- Added tests for redirect, Cloudflare, M3U8 and HLS resolver contracts.

### Changed

- Bootstrap now registers resolver order `xtream -> shui -> cloudflare -> hls -> m3u8 -> redirect -> http`.
- Startup diagnostics now report manifest read limit and Cloudflare bypass status.
- Updated runtime version metadata to `v0.1.30` with status `foundation/resolver-redirect-cloudflare-m3u8-hls`.
- Updated README, ROADMAP and resolver development documentation.

### Notes

- Cloudflare support is classification-only and does not attempt challenge bypass.
- M3U8/HLS resolvers inspect manifests only; they do not download segments or make Media Hub policy decisions.
- Next roadmap item: EPIC 7.9 — Google Drive Resolver.

## [v0.1.29] - 2026-07-09

### Added

- Started EPIC 7 — Resolver Engine with 7.1 — Resolver Interface, 7.2 — HTTP Resolver, 7.3 — Xtream Resolver and 7.4 — Shui Resolver in one delivery.
- Added `internal/resolver` with `Resolver`, `Request`, `Result`, deterministic `Registry` and defensive request/result copies.
- Added `HTTPResolver` backed by the existing Download HTTP Client for metadata-only HTTP resolution with redirect support.
- Added `XtreamResolver` for direct Xtream stream paths and `player_api.php` endpoint metadata extraction.
- Added `ShuiResolver` for Shui/XUI Admin API-style access-code, `api_key` and `action` metadata extraction.
- Added resolver tests and `docs/development/resolver.md`.

### Changed

- Bootstrap now constructs a resolver registry in semantic order: `xtream`, `shui`, `http`.
- Startup diagnostics now report resolver registry size, resolver order and resolver configuration.
- Updated runtime version metadata to `v0.1.29` with status `foundation/resolver-http-xtream-shui`.
- Updated README and ROADMAP for the start of EPIC 7.

### Notes

- Resolver metadata masks provider secrets and does not perform business decisions.
- Xtream and Shui resolvers parse/classify URLs without contacting providers.
- Next roadmap item: EPIC 7.5 — Redirect Resolver.

## [v0.1.28] - 2026-07-09

### Added

- Completed EPIC 6 — Download Engine with 6.10 — Bandwidth Controller, 6.11 — Download Metrics and 6.12 — Download Tests in one delivery.
- Added `internal/download/bandwidth.go` with an optional mechanical bytes-per-second controller and writer wrapper.
- Added `internal/download/metrics.go` with `DownloadMetric`, `MemoryMetricsRecorder`, defensive snapshots and summary counters.
- Added metric emission from streaming and multipart download paths when a recorder is supplied.
- Added integrated Download Engine tests for bandwidth delay/writer behavior, metric validation/summaries and streaming metrics/failure metrics.

### Changed

- Updated bootstrap diagnostics to report bandwidth and download metrics capabilities.
- Updated runtime version metadata to `v0.1.28` with status `foundation/download-bandwidth-metrics-tests`.
- Updated README, ROADMAP and download development documentation.

### Notes

- EPIC 6 — Download Engine is complete.
- Bandwidth control and metrics are mechanical local primitives; provider policy, Media Hub orchestration and Prometheus export remain outside this EPIC.
- Next roadmap item: EPIC 7.1 — Resolver Interface.

## [v0.1.27] - 2026-07-09

### Added

- Continued EPIC 6 — Download Engine with 6.7 — Multipart Download, 6.8 — SHA256 and 6.9 — Retry Download in one delivery.
- Added `internal/download/multipart.go` with deterministic range planning, part metadata and a mechanical multipart-to-file executor.
- Added `internal/download/checksum.go` with streaming SHA-256 calculation for readers/files and file verification against expected hex digests.
- Added `internal/download/retry.go` with a context-aware mechanical retry loop for download operations.
- Added tests for multipart range planning, multipart file merge, SHA-256 calculation/verification and retry success/failure/cancellation flows.

### Changed

- Updated bootstrap diagnostics to report multipart, checksum and retry download capabilities.
- Updated runtime version metadata to `v0.1.27` with status `foundation/download-multipart-checksum-retry`.
- Updated README, ROADMAP and download development documentation.

### Notes

- Multipart execution is range-based and mechanical; it does not decide provider policy or perform Media Hub orchestration.
- SHA-256 is local integrity plumbing only; upload validation and Storage callbacks remain for later epics.
- Download retry is a generic context-aware attempt loop and does not classify business/provider errors yet.
- Next roadmap item: EPIC 6.10 — Bandwidth Controller.

## [v0.1.26] - 2026-07-09

### Added

- Continued EPIC 6 — Download Engine with 6.4 — Headers, 6.5 — Resume Download and 6.6 — Streaming Download in one delivery.
- Added `internal/download/headers.go` with normalized header sets, canonical header name validation, defensive copies and a mechanical request builder.
- Added `internal/download/resume.go` with validated resume state, local partial-file inspection and safe `Range` header application.
- Added `internal/download/stream.go` with streaming downloads to `io.Writer` and local files, including append mode for resume flows.
- Added tests for header normalization, unsafe header rejection, resume decisions, range application, streaming response bodies and streaming-to-file append behavior.

### Changed

- Updated bootstrap diagnostics to report header, resume and streaming download capabilities.
- Updated runtime version metadata to `v0.1.26` with status `foundation/download-streaming`.
- Updated README, ROADMAP and download development documentation.

### Notes

- The Agent still does not start a network listener or execute real Media Hub transfer jobs in the background.
- Resume support is mechanical and only applies local byte offsets/range headers; provider policy decisions remain outside the Agent.
- Next roadmap item: EPIC 6.7 — Multipart Download.

## [v0.1.25] - 2026-07-09

### Added

- Started EPIC 6 — Download Engine by completing EPIC 6.1 — HTTP Client, EPIC 6.2 — Redirect Engine and EPIC 6.3 — Cookie Engine in one delivery.
- Added `internal/download/client.go` with the foundation HTTP client contract.
- Added `download.HTTPClientOptions`, `download.HTTPClient`, `download.OptionsFromConfig`, `download.NewHTTPClientFromConfig`, `download.NewHTTPClient` and `download.HTTPClient.Do`.
- Added configured TCP connect timeout, response header timeout, idle connection timeout and default User-Agent application.
- Added `internal/download/redirect.go` with the mechanical redirect engine.
- Added `download.RedirectEngine`, `download.RedirectOptions`, `download.RedirectEvent` and `download.NewRedirectEngine`.
- Added redirect follow/disable support, max redirect enforcement and redirect event snapshots.
- Added `internal/download/cookies.go` with the in-memory RFC6265 cookie engine.
- Added `download.CookieEngine`, `download.CookieInfo` and `download.NewCookieEngine`.
- Added HTTP client, redirect engine and cookie engine tests.

### Changed

- Bootstrap now constructs the download HTTP client from validated resolver/download configuration.
- Startup summary now reports download client, user agent, redirect policy, cookie engine and timeout diagnostics.
- Updated runtime version metadata to `v0.1.25` with status `foundation/download-http-client`.
- Updated README, ROADMAP and download development notes.

### Notes

- These are transport primitives only; the Agent still does not perform real file downloads, streaming, resume, checksum or upload work.
- The HTTP client remains business-rule free and applies only mechanical timeout, redirect, cookie and User-Agent behavior.
- Next roadmap item: EPIC 6.4 — Headers.

## [v0.1.24] - 2026-07-09

### Added

- Continued EPIC 5 — Worker Engine by completing EPIC 5.9 — Persistence and EPIC 5.10 — REST API in one delivery.
- Added `internal/queue/persistence.go` with local JSON queue snapshot persistence.
- Added `queue.Snapshot`, `queue.StoreResult`, `queue.FileStore`, `queue.DefaultPersistencePath`, `queue.NewFileStore`, `queue.FileStore.Ensure`, `queue.FileStore.Load`, `queue.FileStore.Save`, `queue.NewSnapshot`, `queue.ValidateSnapshot` and `queue.Restore`.
- Added atomic queue snapshot writes using a private temporary file and rename.
- Added `internal/server/worker_api.go` with worker REST route contracts.
- Added `GET /worker`, `GET /worker/jobs` and `POST /worker/jobs` route definitions.
- Added `server.WorkerAPIOptions`, `server.WorkerResponse`, `server.WorkerJobsResponse`, `server.CreateJobRequest`, `server.CreateJobResponse`, `server.WorkerHandler`, `server.WorkerJobsHandler`, `server.CreateWorkerJobHandler` and `server.WorkerRoutes`.
- Added persistence and worker REST API tests.

### Changed

- Bootstrap now creates a queue persistence store at `<runtime.data_dir>/worker/queue.json`.
- Bootstrap now initializes or loads queue persistence, restores queued jobs into the memory queue and saves the current queue snapshot.
- Bootstrap now registers worker REST API routes alongside foundation health, version, ready and identity routes.
- Startup summary now reports queue persistence source, restored job count, persistence path and worker API route paths.
- Updated runtime version metadata to `v0.1.24` with status `foundation/worker-persistence-rest-api`.
- Updated README, ROADMAP, server notes and worker development notes.

### Notes

- EPIC 5 — Worker Engine is complete.
- Persistence is local and mechanical; malformed snapshots are rejected rather than silently replaced.
- Worker REST API is still a route contract only because the foundation line does not start a listening HTTP server.
- `POST /worker/jobs` accepts only mechanical transfer jobs and does not perform Media Hub business decisions.
- Next roadmap item: EPIC 6.1 — HTTP Client.

## [v0.1.23] - 2026-07-09

### Added

- Continued EPIC 5 — Worker Engine by completing EPIC 5.6 — Dispatcher, EPIC 5.7 — Retry and EPIC 5.8 — Heartbeat in one delivery.
- Added `internal/dispatcher/dispatcher.go` with the foundation dispatcher orchestration contract.
- Added `dispatcher.Dispatcher`, `dispatcher.New`, `dispatcher.Options`, `dispatcher.DispatchResult`, `dispatcher.PoolRunner` and `dispatcher.RetryQueue`.
- Added `internal/dispatcher/retry.go` with an attempt-based mechanical retry policy.
- Added `dispatcher.RetryPolicy`, `dispatcher.AttemptsRetryPolicy`, `dispatcher.NewAttemptsRetryPolicy` and `dispatcher.RetryDecision`.
- Added `internal/heartbeat/heartbeat.go` with the foundation local heartbeat payload contract.
- Added `heartbeat.Record`, `heartbeat.Input`, `heartbeat.QueueStats`, `heartbeat.NewRecord` and `heartbeat.ValidateRecord`.
- Added dispatcher, retry and heartbeat tests.

### Changed

- Bootstrap now wires a dispatcher using the worker pool, memory queue and attempt retry policy.
- Scheduler task now delegates to the dispatcher instead of calling the worker pool directly.
- Startup summary now reports dispatcher and heartbeat diagnostics.
- Updated runtime version metadata to `v0.1.23` with status `foundation/dispatcher-retry-heartbeat`.
- Updated README, ROADMAP and worker development notes.

### Notes

- Retry remains mechanical: a failed job is requeued only while `attempt < max_attempts`.
- Heartbeat is a local foundation snapshot only; it is not sent to Media Hub yet.
- The default handler remains `noop`; real transfer execution remains reserved for Download, Resolver and Upload epics.
- Persistence and REST API remain reserved for later EPIC 5 subphases.
- Next roadmap item: EPIC 5.9 — Persistence.

## [v0.1.22] - 2026-07-09

### Added

- Continued EPIC 5 — Worker Engine by completing EPIC 5.3 — Worker, EPIC 5.4 — Worker Pool and EPIC 5.5 — Scheduler in one delivery.
- Added `internal/worker/worker.go` with the foundation worker execution contract.
- Added `worker.JobQueue`, `worker.Handler`, `worker.HandlerFunc`, `worker.HandlerResult`, `worker.RunResult`, `worker.Worker`, `worker.NewWorker` and `worker.NoopHandler`.
- Added worker execution statuses `running`, `succeeded` and `failed`, plus terminal-status and attempt-status helpers.
- Added `internal/worker/pool.go` with bounded worker pool composition.
- Added `worker.Pool`, `worker.NewPool`, `worker.PoolOptions`, `worker.PoolStats`, `worker.Pool.RunOnce`, `worker.Pool.Size`, `worker.Pool.WorkerIDs` and `worker.Pool.Stats`.
- Added `internal/scheduler/scheduler.go` with a fixed interval scheduler contract.
- Added `scheduler.Task`, `scheduler.RunResult`, `scheduler.FixedIntervalScheduler`, `scheduler.NewFixedInterval`, `RunOnce` and `Start`.
- Added worker, pool and scheduler tests.

### Changed

- Bootstrap now wires memory queue, noop handler, worker pool and fixed interval scheduler without starting background execution.
- Startup summary now reports worker pool and scheduler diagnostics.
- Queue enqueue now rejects non-queueable execution/terminal statuses.
- Updated runtime version metadata to `v0.1.22` with status `foundation/worker-scheduler`.
- Updated README, ROADMAP and worker development notes.

### Notes

- Worker execution is still mechanical and business-rule-free.
- The default handler is `noop`; real transfer execution remains reserved for Download, Resolver and Upload epics.
- Dispatcher, retry, heartbeat, persistence and REST API remain reserved for later EPIC 5 subphases.
- Next roadmap item: EPIC 5.6 — Dispatcher.

## [v0.1.21] - 2026-07-09

### Added

- Started EPIC 5 — Worker Engine by completing EPIC 5.1 — Job Model and EPIC 5.2 — Queue in one delivery.
- Added `internal/worker/job.go` with the canonical foundation transfer job model.
- Added `worker.Job`, `worker.JobInput`, `worker.JobType`, `worker.JobStatus`, `worker.NewJob`, `worker.ValidateJob`, `worker.Job.Clone` and `worker.Job.WithStatus`.
- Added foundation job type `transfer` and statuses `pending` and `queued`.
- Added `internal/queue/memory.go` with the canonical queue interface and bounded FIFO memory queue.
- Added `queue.Queue`, `queue.MemoryQueue`, `queue.NewMemoryQueue`, `queue.MemoryDriver`, `queue.ErrQueueFull` and `queue.ErrQueueClosed`.
- Added worker and queue tests for validation, defensive copies, FIFO behavior, capacity, close behavior and context cancellation.
- Added `docs/development/worker.md`.

### Changed

- Bootstrap now creates the foundation memory queue using `queue.memory_capacity`.
- Startup summary now reports queue driver, capacity, queued count and poll interval.
- Updated runtime version metadata to `v0.1.21` with status `foundation/worker-queue`.
- Updated README and ROADMAP for the start of EPIC 5.

### Notes

- Jobs are modeled and queued only; there is no worker execution loop yet.
- The queue is process-local and non-persistent; persistence remains reserved for EPIC 5.9.
- Scheduler, dispatcher, retry and heartbeat remain reserved for later EPIC 5 subphases.
- Next roadmap item: EPIC 5.3 — Worker.

## [v0.1.20] - 2026-07-09

### Added

- Completed EPIC 4.4 — Fingerprint and EPIC 4.5 — Identity API in one delivery.
- Added `internal/identity/fingerprint.go` with deterministic SHA-256 Agent fingerprint generation.
- Added `identity.Snapshot`, `identity.NewFingerprint`, `identity.ValidateFingerprint`, `identity.IsFingerprint` and `identity.NewSnapshot`.
- Added `internal/server/identity.go` with the canonical `GET /identity` route contract.
- Added `server.IdentityResponse`, `server.IdentityHandler`, `server.IdentityRoute` and `server.IdentityRoutes`.
- Added fingerprint and identity API tests for deterministic hashing, validation, payload shape, route metadata and router integration.

### Changed

- `server.FoundationRoutes` can now include the identity route when provided an `identity.Snapshot`.
- Bootstrap now builds the foundation router with `/health`, `/version`, `/ready` and `/identity`.
- Structured startup logs now include `fingerprint` in addition to `agent_id`, `hostname` and `hostname_source`.
- Startup summary now reports fingerprint and fingerprint algorithm.
- Updated runtime version metadata to `v0.1.20` with status `foundation/identity-api`.
- Updated README, ROADMAP, server notes and identity notes for completed EPIC 4.

### Notes

- EPIC 4 — Identity is complete.
- `/identity` is still a route contract only; the Agent does not start a listening HTTP server yet.
- Fingerprint is technical metadata, not a credential or authorization secret.
- Next roadmap item: EPIC 5.1 — Job Model.

## [v0.1.19] - 2026-07-09

### Added

- Completed EPIC 4.3 — Hostname.
- Added `internal/identity/hostname.go` with local host name discovery and canonical normalization.
- Added `identity.HostInfo`, `identity.ResolveHostname`, `identity.ResolveHostnameWith`, `identity.NormalizeHostname`, `identity.ValidateHostname` and `identity.IsHostname`.
- Added controlled `unknown-host` fallback for hosts where the operating-system hostname is unavailable or invalid.
- Added hostname tests for canonicalization, validation, fallback behavior and helper predicates.

### Changed

- Bootstrap now resolves host identity diagnostics during startup.
- Structured startup logs now include `hostname` and `hostname_source` fields.
- Startup summary now reports host name, source and raw value.
- Updated runtime version metadata to `v0.1.19`.
- Updated README, ROADMAP and identity development notes for EPIC 4.3 progress.

### Notes

- Hostname is diagnostic Agent identity metadata, not a business decision input.
- Hostname resolution does not fail bootstrap; invalid or unavailable names become `unknown-host`.
- Fingerprint and identity API remain reserved for later EPIC 4 deliveries.
- The Agent still does not start a listening HTTP server or transfer engines yet.

## [v0.1.18] - 2026-07-09

### Added

- Completed EPIC 4.2 — Identity Storage.
- Added `internal/identity/store.go` with durable local JSON identity storage.
- Added canonical identity store path resolution under `runtime.data_dir/identity/agent.json`.
- Added `identity.Record`, `identity.StoreResult`, `identity.FileStore`, `identity.NewFileStore`, `identity.DefaultStorePath`, `identity.NewRecord` and `identity.ValidateRecord`.
- Added atomic identity writes with private directory/file permissions.
- Added identity storage tests for creation, reload, canonical JSON, validation and malformed files.

### Changed

- Bootstrap now loads or creates a persistent local `agent_id` instead of generating a new transient ID on every boot.
- Startup logging continues to include the `agent_id` correlation field.
- Startup summary now reports identity persistence mode, source and local path.
- Updated runtime version metadata to `v0.1.18`.
- Updated README, ROADMAP and identity development notes for EPIC 4.2 progress.

### Notes

- The identity file stores only Agent technical identity metadata.
- The Agent remains stateless with respect to jobs, media, business decisions and queue state.
- Hostname, fingerprint and identity API remain reserved for later EPIC 4 deliveries.
- The Agent still does not start a listening HTTP server or transfer engines yet.

## [v0.1.17] - 2026-07-09

### Added

- Started EPIC 4 — Identity with subphase 4.1 — UUID.
- Added `internal/identity/uuid.go` with cryptographically random RFC 4122 version 4 UUID generation.
- Added `identity.ValidateUUID`, `identity.NormalizeUUID` and `identity.IsUUID`.
- Added UUID tests for generation, uniqueness, normalization, invalid values and helper predicates.
- Added identity development notes for the transient UUID contract.

### Changed

- Bootstrap now generates a transient `agent_id` at startup.
- Startup logging is enriched with the `agent_id` correlation field.
- Startup summary now reports the transient identity state.
- Updated runtime version metadata to `v0.1.17`.
- Updated README and ROADMAP for EPIC 4.1 progress.

### Notes

- The generated `agent_id` is intentionally not persisted yet.
- Durable identity storage remains reserved for EPIC 4.2.
- The Agent remains stateless and business-rule-free.
- The Agent still does not start a listening HTTP server or transfer engines yet.

## [v0.1.16] - 2026-07-09

### Added

- Completed EPIC 3.6 — Ready.
- Added `internal/server/ready.go` with the canonical foundation ready route.
- Added `server.ReadyResponse`, `server.ReadyHandler`, `server.ReadyRoute` and `server.ReadyRoutes`.
- Added stable JSON readiness payload fields for readiness status, boolean readiness, app name, version, runtime status and router family.
- Added ready tests for payload shape, route metadata, router integration and defensive route slices.

### Changed

- `server.FoundationRoutes` now includes `/health`, `/version` and `/ready`.
- Bootstrap now registers all foundation diagnostic routes while still avoiding a listening HTTP server.
- Updated runtime version metadata to `v0.1.16`.
- Updated startup output to report ready route readiness and include `routes=3` in the configuration summary.
- Updated README, ROADMAP and server development notes for EPIC 3.6 progress.

### Notes

- EPIC 3 — HTTP Server is complete at the contract level.
- The Agent still does not start a listening HTTP server.
- Real queue, storage and worker readiness checks remain reserved for their later roadmap phases.
- The Agent remains business-rule-free and does not start transfer engines yet.


## [v0.1.15] - 2026-07-09

### Added

- Completed EPIC 3.5 — Version.
- Added `internal/server/version.go` with the canonical foundation version route.
- Added `server.VersionResponse`, `server.VersionHandler`, `server.VersionRoute` and `server.VersionRoutes`.
- Added `server.FoundationRoutes` for the current foundation route set.
- Added stable JSON runtime metadata payload fields for app name, version, runtime status and router family.
- Added version tests for payload shape, route metadata, router integration and defensive route slices.

### Changed

- Bootstrap now registers both `/health` and `/version` routes while still avoiding a listening HTTP server.
- Updated runtime version metadata to `v0.1.15`.
- Updated startup output to report version route readiness and include `routes=2` in the configuration summary.
- Updated README, ROADMAP and server development notes for EPIC 3.5 progress.

### Notes

- The Agent still does not start a listening HTTP server.
- `/ready` remains reserved for EPIC 3.6.
- The Agent remains business-rule-free and does not start transfer engines yet.


## [v0.1.14] - 2026-07-09

### Added

- Completed EPIC 3.4 — Health.
- Added `internal/server/health.go` with the canonical foundation health route.
- Added `server.HealthResponse`, `server.HealthHandler`, `server.HealthRoute` and `server.HealthRoutes`.
- Added stable JSON liveness payload fields for status, app name, version, runtime status and router family.
- Added health tests for payload shape, route metadata, router integration and defensive route slices.

### Changed

- Bootstrap now registers the `/health` route while still avoiding a listening HTTP server.
- Updated runtime version metadata to `v0.1.14`.
- Updated startup output to report health route readiness and include `routes=1` in the configuration summary.
- Updated README, ROADMAP and server development notes for EPIC 3.4 progress.

### Notes

- The Agent still does not start a listening HTTP server.
- `/version` and `/ready` remain reserved for later EPIC 3 deliveries.
- The Agent remains business-rule-free and does not start transfer engines yet.

## [v0.1.13] - 2026-07-09

### Added

- Completed EPIC 3.3 — Middlewares.
- Added `server.MiddlewareDefinition`, `server.MiddlewareInfo`, `server.MiddlewareOptions` and `server.MiddlewareRegistry`.
- Added `server.DefaultMiddlewareRegistry` for the standard foundation middleware stack.
- Added `server.RegisterMiddlewares`, `server.NormalizeMiddlewareDefinition` and `server.NormalizeMiddlewareName`.
- Added `server.Recoverer` for panic recovery at the HTTP transport boundary.
- Added middleware tests for canonical stack creation, validation, defensive copies, execution order, request logging and panic recovery.

### Changed

- `server.RouterOptions` now supports `RecoverPanics` and custom middleware definitions.
- `server.BuildRouter` now applies the canonical middleware registry before registering routes.
- Updated runtime version metadata to `v0.1.13`.
- Updated startup output to report middleware stack readiness and include `middlewares=2` in the configuration summary.
- Updated README, ROADMAP and server development notes for EPIC 3.3 progress.

### Notes

- The Agent still does not start a listening HTTP server.
- Health, version and ready endpoints remain reserved for later EPIC 3 deliveries.
- The Agent remains business-rule-free and does not start transfer engines yet.

## [v0.1.12] - 2026-07-09

### Added

- Completed EPIC 3.2 — Router.
- Added canonical route composition contracts through `server.RouteDefinition`, `server.RouteRegistry` and `server.BuildRouter`.
- Added route validation, method normalization, pattern normalization, duplicate route protection and generated route names.
- Added `server.RegisterRoutes`, `server.RegisterRouteDefinition`, `server.NormalizeRouteDefinition`, `server.SupportedHTTPMethods` and related helpers.
- Added router contract tests for route batches, registry snapshots, duplicate validation, invalid definitions, nil router errors and defensive method-list copies.

### Changed

- `server.RouterOptions` can now receive route definitions for validated router construction.
- `server.RegisterRoute` remains backward-compatible while delegating to the new validated route definition path.
- Updated runtime version metadata to `v0.1.12`.
- Updated startup output to report router contract readiness and include `routes=0` in the configuration summary.
- Updated README, ROADMAP and server development notes for EPIC 3.2 progress.

### Notes

- The Agent still does not start a listening HTTP server.
- Health, version and ready endpoints remain reserved for later EPIC 3 deliveries.
- The Agent remains business-rule-free and does not start transfer engines yet.

## [v0.1.11] - 2026-07-09

### Added

- Started EPIC 3.1 — Chi.
- Added `internal/server` as the foundation HTTP routing package.
- Added the canonical Chi dependency boundary through `github.com/go-chi/chi/v5`.
- Added local `internal/server/chicompat` module so official ZIPs remain self-contained and offline-compilable.
- Added `server.NewRouter`, `server.RouterOptions`, `server.RouteInfo`, `server.RegisterRoute` and `server.RouterKindName`.
- Added optional request logger middleware wiring at router creation.
- Added router tests for Chi-style route registration, structured request logging and nil-input handling.
- Added `docs/development/server.md` with the current EPIC 3 server contract.

### Changed

- Updated runtime version metadata to `v0.1.11`.
- Updated startup output to report Chi router readiness and include `router=chi` in the configuration summary.
- Updated README and ROADMAP for EPIC 3.1 progress.

### Notes

- EPIC 3 has started, but the Agent still does not start a listening HTTP server.
- Health, version and ready endpoints remain reserved for later EPIC 3 deliveries.
- The Agent remains business-rule-free and does not start transfer engines yet.

## [v0.1.10] - 2026-07-09

### Added

- Completed EPIC 2.5 — Request Logger.
- Added `internal/logger/request.go` with standard `net/http` request logging middleware.
- Added structured request completion fields for method, path, status, duration, bytes, remote address, user-agent and request id.
- Added request-context logger propagation so future HTTP handlers can reuse the active request logger without global state.
- Added `logger.RequestLogger`, `logger.RequestLoggerFieldNames`, `logger.RequestIDHeader` and request field constants.
- Added request logger tests for structured events, context propagation, warn/error severity mapping and defensive field-name copies.

### Changed

- Updated runtime version metadata to `v0.1.10`.
- Updated startup output to report request logger readiness.
- Extended JSON field discovery with request logger field names.
- Updated README, ROADMAP, configuration notes and logger development notes for EPIC 2 completion.

### Notes

- EPIC 2 — Logger is now complete.
- Request logging is implemented as reusable middleware only; EPIC 3 will introduce the actual HTTP server.
- The Agent remains business-rule-free and does not start transfer engines yet.

## [v0.1.9] - 2026-07-09

### Added

- Completed EPIC 2.4 — Console Logger.
- Added `internal/logger/console.go` with a deterministic console writer for local operations.
- Added `logger.ConsoleFormat`, `logger.ConsoleLineDelimiter`, `logger.NewConsoleWriter`, `logger.FormatConsoleLine` and `logger.ValidateConsoleLine`.
- Added console logger tests for readable output, stable extra-field ordering, level filtering and raw JSON rejection.

### Changed

- `logger.New` now supports both `json` and `console` output formats.
- Configuration validation now accepts `logger.format=console` while keeping `json` as the default.
- Updated runtime version metadata to `v0.1.9`.
- Updated startup output to report console logger readiness.
- Updated README, ROADMAP, configuration notes and logger development notes for EPIC 2.4 progress.

### Notes

- JSON logging remains the default production-oriented format.
- Console logging is explicitly opt-in through YAML or `AUREN_LOGGER_FORMAT=console`.
- Request logger remains reserved for EPIC 2.5.
- The Agent remains business-rule-free and does not start transfer engines yet.

## [v0.1.8] - 2026-07-09

### Added

- Completed EPIC 2.3 — JSON Logger.
- Added `internal/logger/json.go` with the canonical newline-delimited JSON logging contract.
- Added exported JSON constants for format, delimiter, core fields, runtime fields and environment metadata.
- Added `logger.RuntimeStartupEvent` and `logger.LogRuntimeStartup` for the official startup log event.
- Added `logger.JSONFieldNames`, `logger.DecodeJSONLine` and `logger.ValidateJSONLine` as contract helpers for tests and future integrations.
- Added JSON logger tests for valid foundation events, timestamp omission, invalid payload rejection and defensive field-name copies.

### Changed

- Updated runtime version metadata to `v0.1.8`.
- Updated bootstrap startup logging to use the canonical JSON startup helper.
- Updated startup output to report JSON logger readiness.
- Updated README, ROADMAP and logger development notes for EPIC 2.3 progress.

### Notes

- JSON output remains the only enabled logger format in this subphase.
- Console logger remains reserved for EPIC 2.4.
- Request logger remains reserved for EPIC 2.5.
- The Agent remains business-rule-free and does not start transfer engines yet.

## [v0.1.7] - 2026-07-09

### Added

- Completed EPIC 2.2 — Context Logger.
- Added context-aware logger propagation through `logger.IntoContext` and `logger.FromContext`.
- Added `logger.FromContextOrDefault` for safe fallback logging when a context has no logger.
- Added persistent contextual field helpers through `logger.WithFields` and `logger.EnrichContext`.
- Added official correlation field constants for `component`, `operation`, `request_id`, `job_id`, `agent_id` and `trace_id`.
- Added context logger tests for round-trip storage, enrichment, fallback and nil-context behavior.

### Changed

- Updated runtime version metadata to `v0.1.7`.
- Updated bootstrap startup logging to enrich events with `component=bootstrap`.
- Updated startup output to report context logger readiness.
- Updated README, ROADMAP and logger development notes for EPIC 2.2 progress.

### Notes

- The context logger does not create business rules or job behavior.
- JSON logger hardening remains reserved for EPIC 2.3.
- Console logger and request logger remain reserved for later EPIC 2 deliveries.
- The Agent remains business-rule-free and does not start transfer engines yet.

## [v0.1.6] - 2026-07-09

### Added

- Started EPIC 2.1 — Zerolog.
- Added `internal/logger` as the structured logger package for the Agent foundation.
- Added zerolog-compatible dependency boundary through `github.com/rs/zerolog`.
- Added local `internal/logger/zerologcompat` module so official ZIPs remain self-contained and offline-compilable.
- Added `logger` configuration section with level, format, timestamp and service settings.
- Added structured JSON startup diagnostic event.
- Added logger tests for JSON event shape, severity filtering and invalid format handling.

### Changed

- Updated runtime version metadata to `v0.1.6`.
- Updated startup output to report zerolog logger readiness.
- Updated configuration docs with the new `logger` section.
- Updated README and ROADMAP for EPIC 2 progress.

### Notes

- Only JSON logger output is enabled in this subphase.
- Context logger, console logger and request logger remain reserved for later EPIC 2 deliveries.
- The Agent remains business-rule-free and does not start transfer engines yet.

## [v0.1.5] - 2026-07-09

### Added

- Completed EPIC 1.5 — Validation.
- Added `internal/config/validation.go` with structural validation for the configuration contract.
- Added aggregated field-level validation errors through `ValidationError` and `ValidationIssue`.
- Added validation for required strings, ports, positive integers, non-negative integers, durations, byte sizes, slash-prefixed paths and supported foundation drivers.
- Added byte-size parsing for `B`, `KB`, `MB`, `GB`, `KiB`, `MiB`, `GiB` and plain bytes.
- Added tests for default validation, invalid YAML, invalid environment overrides and size unit parsing.

### Changed

- Configuration loading now validates the final merged values after defaults, YAML and `AUREN_*` overrides are applied.
- Updated runtime version metadata to `v0.1.5`.
- Updated startup output to report validation-aware configuration readiness.
- Updated configuration development notes with the official validation contract.

### Notes

- EPIC 1 — Configuration is now complete.
- Logger work starts in EPIC 2.
- The Agent remains business-rule-free and does not start transfer engines yet.

## [v0.1.4] - 2026-07-09

### Added

- Completed EPIC 1.4 — Defaults.
- Added `internal/config/defaults.go` as the centralized built-in default contract.
- Added `DefaultConfig()` for typed defaults used by the Agent foundation.
- Added `DefaultValues()` for dotted-key default registration through Viper.
- Added `DefaultSearchPaths()` for canonical config discovery paths.
- Added tests proving `Load(LoadOptions{})` matches `DefaultConfig()`.
- Added defensive-copy tests for default maps and search paths.

### Changed

- Replaced hardcoded default registration inside the loader with the centralized defaults contract.
- Updated runtime version metadata to `v0.1.4`.
- Updated startup output to report defaults-aware configuration readiness.
- Updated configuration development notes with the official default precedence and API.

### Notes

- Validation remains reserved for EPIC 1.5.
- Logger work starts after the configuration epic is complete.
- The Agent remains business-rule-free and does not start transfer engines yet.

## [v0.1.3] - 2026-07-09

### Added

- Completed EPIC 1.3 — Environment Override.
- Added `AUREN_*` environment override support through the Viper configuration contract.
- Added automatic mapping from dotted keys to uppercase env names, such as `server.port` to `AUREN_SERVER_PORT`.
- Added env override precedence above defaults and YAML files.
- Added tests for environment overrides over defaults and YAML values.

### Changed

- Updated runtime version metadata to `v0.1.3`.
- Updated startup output to report environment-aware configuration readiness.
- Updated configuration development notes with deployment-oriented env examples.

### Notes

- Stronger defaults remain reserved for EPIC 1.4.
- Validation remains reserved for EPIC 1.5.
- The Agent remains business-rule-free and does not start transfer engines yet.

## [v0.1.2] - 2026-07-09

### Added

- Completed EPIC 1.2 — Config YAML.
- Expanded the official `configs/agent.yaml` contract.
- Added configuration structs for future server, worker, queue, resolver, download, upload, storage, metrics, heartbeat and security layers.
- Added built-in defaults for every key in the current YAML contract.
- Added helper methods for server and metrics bind addresses.
- Added tests for full YAML loading and partial YAML/default merging.
- Updated configuration development notes.

### Changed

- Updated runtime version metadata to `v0.1.2`.
- Updated startup output to summarize the expanded YAML configuration.

### Notes

- Environment overrides remain reserved for EPIC 1.3.
- Stronger defaults remain reserved for EPIC 1.4.
- Validation remains reserved for EPIC 1.5.
- The Agent remains business-rule-free and does not start transfer engines yet.

## [v0.1.1] - 2026-07-09

### Added

- Started EPIC 1 — Configuration with subphase 1.1 — Viper.
- Added `internal/config` with a Viper-backed loading contract.
- Added optional `--config` CLI flag for explicit YAML configuration files.
- Added default config discovery for `agent.yaml` in `.`, `./configs` and `/etc/auren-transfer-agent`.
- Added sample configuration at `configs/agent.yaml`.
- Added configuration unit tests.
- Added configuration development notes in `docs/development/configuration.md`.

### Changed

- Updated runtime version metadata to `v0.1.1`.
- Updated startup output to report configuration readiness and the loaded bind address.

### Notes

- Environment overrides, stronger defaults and validation remain reserved for later EPIC 1 subphases.
- The Agent remains business-rule-free and does not start transfer engines yet.

## [v0.1.0] - 2026-07-09

### Added

- Created the official Foundation Base delivery.
- Added the definitive project directory structure.
- Added `go.mod` and `go.sum`.
- Added CLI entrypoint at `cmd/agent/main.go`.
- Added bootstrap package with `--help` and `--version` support.
- Added runtime version package.
- Added `Makefile` with build, run, version, test, format, tidy and clean targets.
- Added `Taskfile.yml` with equivalent development tasks.
- Added README, CHANGELOG, ROADMAP and LICENSE.

### Notes

- This version intentionally contains no business rules.
- Configuration, logging, HTTP server, identity and worker behavior begin in later phases.
