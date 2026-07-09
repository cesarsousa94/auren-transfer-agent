# Auren Transfer Agent Roadmap

**Document version:** 1.0  
**Status:** Approved  
**Current delivery:** v1.6.0

## Objective

Auren Transfer Agent is a Go service responsible for executing high-reliability media transfers.

It receives jobs from Auren Media Hub, resolves complex URLs such as Xtream, Shui, Cloudflare, redirects, tokens and cookies, performs resilient downloads, uploads results to Auren Storage and returns operation status.

The Agent is stateless, horizontally scalable and prepared for Docker, Kubernetes or direct Linux installation.

## Philosophy

The Agent never contains business rules.

It only executes jobs.

All decisions remain in Auren Media Hub.

## Definitive structure

```text
auren-transfer-agent/
├── cmd/
│   └── agent/
├── internal/
│   ├── api/
│   ├── bootstrap/
│   ├── config/
│   ├── logger/
│   ├── identity/
│   ├── server/
│   ├── worker/
│   ├── queue/
│   ├── scheduler/
│   ├── dispatcher/
│   ├── resolver/
│   ├── download/
│   ├── upload/
│   ├── storage/
│   ├── metrics/
│   ├── heartbeat/
│   ├── security/
│   └── runtime/
├── pkg/
│   └── plugins/
├── configs/
├── docker/
├── deploy/
├── scripts/
├── docs/
│   ├── architecture/
│   ├── adr/
│   ├── api/
│   ├── development/
│   ├── deployment/
│   └── plugins/
├── examples/
├── tests/
├── README.md
├── CHANGELOG.md
├── ROADMAP.md
├── LICENSE
├── go.mod
├── go.sum
├── Makefile
└── Taskfile.yml
```

This structure is definitive and must not be reorganized.

## Development rules

1. Each delivery generates a complete ZIP.
2. Never ship patches as the official artifact.
3. Each version completely replaces the previous version.
4. Every ZIP must compile.
5. README, CHANGELOG and ROADMAP are always updated.
6. Each phase modifies no more than about twenty files.
7. Existing functionality is never removed; only additions or improvements are allowed.

## Versioning line

- `v0.1.x` — Foundation
- `v0.2.x` — Worker Engine
- `v0.3.x` — Download Engine
- `v0.4.x` — Resolver Engine
- `v0.5.x` — Upload Engine
- `v0.6.x` — Communication
- `v0.7.x` — Cluster
- `v0.8.x` — Observability
- `v0.9.x` — Security
- `v1.0.0` — Production
- `v1.1.0` — Media Hub Connector Foundation
- `v1.2.0` — Real Transfer Executor
- `v1.3.0` — Auren Storage Production Adapter
- `v1.4.0` — Gateway Runtime
- `v1.5.0` — Operational Hardening
- `v1.6.0` — Linux Package & Zero-Touch Bootstrap

## EPIC 0 — Foundation Base

- `0.0` Project
- `0.0.1` Project structure
- `0.0.2` `go.mod`, `Makefile`, `Taskfile.yml`
- `0.0.3` `README`, `CHANGELOG`, `ROADMAP`, `LICENSE`
- `0.0.4` Bootstrap, main and build
- `0.0.5` Version package

**v0.1.0 status:** complete.

## EPIC 1 — Configuration

- `1.1` Viper — complete in `v0.1.1`
- `1.2` Config YAML — complete in `v0.1.2`
- `1.3` Environment Override — complete in `v0.1.3`
- `1.4` Defaults — complete in `v0.1.4`
- `1.5` Validation — complete in `v0.1.5`

## EPIC 2 — Logger

- `2.1` Zerolog — complete in `v0.1.6`
- `2.2` Context Logger — complete in `v0.1.7`
- `2.3` JSON Logger — complete in `v0.1.8`
- `2.4` Console Logger — complete in `v0.1.9`
- `2.5` Request Logger — complete in `v0.1.10`

## EPIC 3 — HTTP Server

- `3.1` Chi — complete in `v0.1.11`
- `3.2` Router — complete in `v0.1.12`
- `3.3` Middlewares — complete in `v0.1.13`
- `3.4` Health — complete in `v0.1.14`
- `3.5` Version — complete in `v0.1.15`
- `3.6` Ready — complete in `v0.1.16`

## EPIC 4 — Identity

- `4.1` UUID — complete in `v0.1.17`
- `4.2` Identity Storage — complete in `v0.1.18`
- `4.3` Hostname — complete in `v0.1.19`
- `4.4` Fingerprint — complete in `v0.1.20`
- `4.5` Identity API — complete in `v0.1.20`

## EPIC 5 — Worker Engine

- `5.1` Job Model — complete in `v0.1.21`
- `5.2` Queue — complete in `v0.1.21`
- `5.3` Worker — complete in `v0.1.22`
- `5.4` Worker Pool — complete in `v0.1.22`
- `5.5` Scheduler — complete in `v0.1.22`
- `5.6` Dispatcher — complete in `v0.1.23`
- `5.7` Retry — complete in `v0.1.23`
- `5.8` Heartbeat — complete in `v0.1.23`
- `5.9` Persistence — complete in `v0.1.24`
- `5.10` REST API — complete in `v0.1.24`

## EPIC 6 — Download Engine

- `6.1` HTTP Client — complete in `v0.1.25`
- `6.2` Redirect Engine — complete in `v0.1.25`
- `6.3` Cookie Engine — complete in `v0.1.25`
- `6.4` Headers — complete in `v0.1.26`
- `6.5` Resume Download — complete in `v0.1.26`
- `6.6` Streaming Download — complete in `v0.1.26`
- `6.7` Multipart Download — complete in `v0.1.27`
- `6.8` SHA256 — complete in `v0.1.27`
- `6.9` Retry Download — complete in `v0.1.27`
- `6.10` Bandwidth Controller — complete in `v0.1.28`
- `6.11` Download Metrics — complete in `v0.1.28`
- `6.12` Download Tests — complete in `v0.1.28`

## EPIC 7 — Resolver Engine

- `7.1` Resolver Interface — complete in `v0.1.29`
- `7.2` HTTP Resolver — complete in `v0.1.29`
- `7.3` Xtream Resolver — complete in `v0.1.29`
- `7.4` Shui Resolver — complete in `v0.1.29`
- `7.5` Redirect Resolver — complete in `v0.1.30`
- `7.6` Cloudflare Resolver — complete in `v0.1.30`
- `7.7` M3U8 Resolver — complete in `v0.1.30`
- `7.8` HLS Resolver — complete in `v0.1.30`
- `7.9` Google Drive Resolver — complete in `v0.1.31`
- `7.10` MEGA Resolver — complete in `v0.1.31`
- `7.11` OneDrive Resolver — complete in `v0.1.31`
- `7.12` Plugin SDK — complete in `v0.1.32`

## EPIC 8 — Upload Engine

- `8.1` Uploader Interface — complete in `v0.1.32`
- `8.2` Local Upload — complete in `v0.1.32`
- `8.3` Multipart Upload — complete in `v0.1.32`
- `8.4` Resume Upload — complete in `v0.1.33`
- `8.5` Integrity Validation — complete in `v0.1.33`
- `8.6` Callback Engine — complete in `v0.1.33`
- `8.7` Storage Adapter — complete in `v0.1.33`
- `8.8` Auren Storage Adapter — complete in `v0.1.33`

## EPIC 9 — Communication

- `9.1` REST API — complete in `v0.1.34`
- `9.2` WebSocket — complete in `v0.1.34`
- `9.3` Registration — complete in `v0.1.34`
- `9.4` Heartbeat — complete in `v0.1.34`
- `9.5` Authentication — complete in `v0.1.34`
- `9.6` Metrics API — complete in `v0.1.35`
- `9.7` Events API — complete in `v0.1.35`

## EPIC 10 — Cluster

- `10.1` Queue Interface — complete in `v0.1.35`
- `10.2` Redis Streams — complete in `v0.1.35`
- `10.3` RabbitMQ — complete in `v0.1.35`
- `10.4` NATS — complete in `v0.1.36`
- `10.5` Agent Registry — complete in `v0.1.36`
- `10.6` Load Balancer — complete in `v0.1.36`
- `10.7` Leader Election — complete in `v0.1.36`
- `10.8` Failover — complete in `v0.1.36`

## EPIC 11 — Observability

- `11.1` Prometheus — complete in `v0.1.37`
- `11.2` Grafana — complete in `v0.1.37`
- `11.3` Tracing — complete in `v0.1.37`
- `11.4` Audit — complete in `v0.1.37`
- `11.5` Alerts — complete in `v0.1.37`
- `11.6` Dashboard — complete in `v0.1.37`
- `11.7` Centralized Logs — complete in `v0.1.37`

## EPIC 12 — Security

- `12.1` JWT — complete in `v0.1.38`
- `12.2` API Keys — complete in `v0.1.38`
- `12.3` mTLS — complete in `v0.1.38`
- `12.4` RBAC — complete in `v0.1.38`
- `12.5` Rate Limit — complete in `v0.1.38`
- `12.6` Secrets — complete in `v0.1.38`

**EPIC 12 status:** complete.

## EPIC 13 — Production

- `13.1` Docker — complete in `v1.0.0`
- `13.2` Docker Compose — complete in `v1.0.0`
- `13.3` Linux Installer — complete in `v1.0.0`
- `13.4` systemd — complete in `v1.0.0`
- `13.5` Kubernetes — complete in `v1.0.0`
- `13.6` CI/CD — complete in `v1.0.0`
- `13.7` Release Pipeline — complete in `v1.0.0`

**EPIC 13 status:** complete.

**Roadmap status:** complete through `v1.0.0` production baseline.

## Current implementation status

- `v0.1.0` completed EPIC 0 — Foundation Base.
- `v0.1.1` completed EPIC 1.1 — Viper configuration foundation.
- `v0.1.2` completed EPIC 1.2 — Config YAML.
- `v0.1.3` completed EPIC 1.3 — Environment Override.
- `v0.1.4` completed EPIC 1.4 — Defaults.
- `v0.1.5` completed EPIC 1.5 — Validation.
- `v0.1.6` completed EPIC 2.1 — Zerolog.
- `v0.1.7` completed EPIC 2.2 — Context Logger.
- `v0.1.8` completed EPIC 2.3 — JSON Logger.
- `v0.1.9` completed EPIC 2.4 — Console Logger.
- `v0.1.10` completed EPIC 2.5 — Request Logger.
- `v0.1.11` completed EPIC 3.1 — Chi.
- `v0.1.12` completed EPIC 3.2 — Router.
- `v0.1.13` completed EPIC 3.3 — Middlewares.
- `v0.1.14` completed EPIC 3.4 — Health.
- `v0.1.15` completed EPIC 3.5 — Version.
- `v0.1.16` completed EPIC 3.6 — Ready.
- `v0.1.17` completed EPIC 4.1 — UUID.
- `v0.1.18` completed EPIC 4.2 — Identity Storage.
- `v0.1.19` completed EPIC 4.3 — Hostname.
- `v0.1.20` completed EPIC 4.4 — Fingerprint and EPIC 4.5 — Identity API.
- `v0.1.21` completed EPIC 5.1 — Job Model and EPIC 5.2 — Queue in one delivery.
- `v0.1.22` completed EPIC 5.3 — Worker, EPIC 5.4 — Worker Pool and EPIC 5.5 — Scheduler in one delivery.
- `v0.1.23` completed EPIC 5.6 — Dispatcher, EPIC 5.7 — Retry and EPIC 5.8 — Heartbeat in one delivery.
- `v0.1.24` completed EPIC 5.9 — Persistence and EPIC 5.10 — REST API in one delivery.
- `v0.1.25` completed EPIC 6.1 — HTTP Client, EPIC 6.2 — Redirect Engine and EPIC 6.3 — Cookie Engine in one delivery.
- `v0.1.26` completed EPIC 6.4 — Headers, EPIC 6.5 — Resume Download and EPIC 6.6 — Streaming Download in one delivery.
- `v0.1.27` completed EPIC 6.7 — Multipart Download, EPIC 6.8 — SHA256 and EPIC 6.9 — Retry Download in one delivery.
- `v0.1.28` completed EPIC 6.10 — Bandwidth Controller, EPIC 6.11 — Download Metrics and EPIC 6.12 — Download Tests in one delivery.
- `v0.1.29` completed EPIC 7.1 — Resolver Interface, EPIC 7.2 — HTTP Resolver, EPIC 7.3 — Xtream Resolver and EPIC 7.4 — Shui Resolver in one delivery.
- `v0.1.30` completed EPIC 7.5 — Redirect Resolver, EPIC 7.6 — Cloudflare Resolver, EPIC 7.7 — M3U8 Resolver and EPIC 7.8 — HLS Resolver in one delivery.
- `v0.1.31` completed EPIC 7.9 — Google Drive Resolver, EPIC 7.10 — MEGA Resolver and EPIC 7.11 — OneDrive Resolver in one delivery.
- `v0.1.32` completed EPIC 7.12, EPIC 8.1, EPIC 8.2 and EPIC 8.3 — Plugin SDK, Uploader Interface, Local Upload and Multipart Upload in one delivery.
- `v0.1.33` completed EPIC 8.4 — Resume Upload, EPIC 8.5 — Integrity Validation, EPIC 8.6 — Callback Engine, EPIC 8.7 — Storage Adapter and EPIC 8.8 — Auren Storage Adapter in one delivery.
- `v0.1.34` completed EPIC 9.1 — REST API, EPIC 9.2 — WebSocket, EPIC 9.3 — Registration, EPIC 9.4 — Heartbeat and EPIC 9.5 — Authentication in one delivery.
- `v0.1.35` completed EPIC 9.6 — Metrics API, EPIC 9.7 — Events API, EPIC 10.1 — Queue Interface, EPIC 10.2 — Redis Streams and EPIC 10.3 — RabbitMQ in one delivery.
- `v0.1.36` completed EPIC 10.4 — NATS, EPIC 10.5 — Agent Registry, EPIC 10.6 — Load Balancer, EPIC 10.7 — Leader Election and EPIC 10.8 — Failover in one delivery.
- `v0.1.37` completed EPIC 11.1 — Prometheus, EPIC 11.2 — Grafana, EPIC 11.3 — Tracing, EPIC 11.4 — Audit, EPIC 11.5 — Alerts, EPIC 11.6 — Dashboard and EPIC 11.7 — Centralized Logs in one delivery.
- `v0.1.38` completed EPIC 12.1 — JWT, EPIC 12.2 — API Keys, EPIC 12.3 — mTLS, EPIC 12.4 — RBAC, EPIC 12.5 — Rate Limit and EPIC 12.6 — Secrets in one delivery.
- EPIC 3 — HTTP Server is complete.
- EPIC 4 — Identity is complete.
- EPIC 5 — Worker Engine is complete.
- EPIC 6 — Download Engine is complete.
- EPIC 7 — Resolver Engine is complete.
- EPIC 8 — Upload Engine is complete.
- EPIC 10 — Cluster is complete.
- EPIC 11 — Observability is complete.
- EPIC 12 — Security is complete.
- Next expected delivery: EPIC 13.1 — Docker.

## Post-v1.0 Media Hub integration roadmap

After v1.0.0, a dedicated Auren Media Hub integration roadmap begins, including automatic Agent registration, Agent discovery, Agent dashboard, intelligent job distribution, remote version updates, real-time monitoring, centralized logs, remote console, queue management and operational diagnostics.


- `v1.0.0` completed EPIC 13.1 — Docker, EPIC 13.2 — Docker Compose, EPIC 13.3 — Linux Installer, EPIC 13.4 — systemd, EPIC 13.5 — Kubernetes, EPIC 13.6 — CI/CD and EPIC 13.7 — Release Pipeline in one production delivery.

## EPIC 15 — Real Transfer Executor

**v1.2.0 status:** complete.

Delivered:

- Media Hub transfer claim loop;
- transfer job payload parsing;
- started/progress/completed/failed callbacks;
- local transfer job state persistence;
- guarded HTTP download with resume/checksum/tiny-file/HTML protections;
- local/Auren Storage/signed URL upload dispatch;
- replacement of the noop worker handler with `transfer_executor`;
- active job capacity reporting to Media Hub.

Completed after EPIC 15:

- v1.3.0 — complete Auren Storage production multipart adapter.
- v1.4.0 — complete public Gateway Runtime.

Completed after EPIC 17:

- v1.6.0 — operational hardening/drain/dead-letter/secret rotation.

## EPIC 16 — Auren Storage Production Adapter

**v1.3.0 status:** complete.

Delivered:

- production `auren_storage` adapter aligned with Auren Storage v1 object upload;
- direct upload through `POST /api/v1/buckets/{bucket_uuid}/objects` using `multipart/form-data`;
- multipart upload lifecycle for large files with initiate, part upload, complete and abort-on-error;
- checksum SHA-256 propagation and response parsing;
- support for `bucket_uuid`, `directory_path`, `relative_path`, `visibility`, `mime_type` and metadata;
- Media Hub completion payload enrichment with object UUID, path, size, checksum, URL, visibility and MIME type;
- direct and multipart adapter contract tests.

Completed after EPIC 17:

- v1.6.0 — operational hardening/drain/dead-letter/secret rotation.



## EPIC 17 — Gateway Runtime

**v1.4.0 status:** complete.

Delivered:

- public gateway route `/_auren/gateway/{token}/{kind}/{id}.{ext}` through the Agent HTTP runtime;
- Media Hub resolve request before opening upstream traffic;
- proxy and redirect delivery modes controlled by the Media Hub control plane;
- Range passthrough and upstream header injection;
- session heartbeat and close callbacks;
- gateway runtime event callbacks;
- active session, byte and egress capacity reporting;
- route, proxy, redirect and resolve parsing tests.

Completed after EPIC 17:

- v1.6.0 — operational hardening/drain/dead-letter/secret rotation.


## EPIC 18 — Operational Hardening

**v1.5.0 status:** complete.

## EPIC 19 — Linux Package & Zero-Touch Bootstrap

- `.deb` package — complete in `v1.6.0`
- systemd persistent runtime — complete in `v1.6.0`
- Linux user/group and canonical directories — complete in `v1.6.0`
- `bootstrap`, `doctor` and `status` commands — complete in `v1.6.0`
- one-line installer — complete in `v1.6.0`
- APT repository skeleton — complete in `v1.6.0`

**v1.6.0 status:** complete.

Delivered:

- operational hardening runtime `internal/ops`;
- graceful drain for transfer claim and gateway session admission;
- Media Hub drain started/completed callbacks;
- backpressure by active jobs, sessions and egress;
- disk pressure guard before heavy transfer claims;
- local dead-letter storage for failed attempts;
- lease renewal callbacks for long-running jobs;
- operator control watcher for cancel/pause/release/drain;
- authenticated worker routes;
- hardening config, deployment examples and tests.
