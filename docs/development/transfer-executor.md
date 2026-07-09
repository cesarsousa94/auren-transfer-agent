# Real Transfer Executor — v1.6.0

Auren Transfer Agent v1.2.0 added the real execution plane for Media Hub transfer jobs. v1.6.0 keeps that executor unchanged and adds the public Gateway Runtime as a separate HTTP path. Transfer jobs and streaming sessions remain independent execution lanes.

The Media Hub still owns all tenant, catalog, entitlement, storage and orchestration decisions. The Agent only executes the mechanical job it receives.

## Runtime flow

```text
Media Hub claim API
  ↓
Agent claim loop
  ↓
Download remote source
  ↓
Validate bytes/checksum/content type
  ↓
Upload destination object
  ↓
Media Hub callbacks
```

## Media Hub endpoints used

The Agent expects the Media Hub 47.15/47.16 transfer-agent API shape:

```text
POST /api/internal/transfer-agent/jobs/claim
POST /api/internal/transfer-agent/jobs/{uuid}/started
POST /api/internal/transfer-agent/jobs/{uuid}/progress
POST /api/internal/transfer-agent/jobs/{uuid}/completed
POST /api/internal/transfer-agent/jobs/{uuid}/failed
POST /api/internal/transfer-agent/jobs/{uuid}/events
POST /api/internal/transfer-agent/jobs/{uuid}/release
GET  /api/internal/transfer-agent/jobs/{uuid}/control
```

All calls use the same node credentials and HMAC signature mechanism introduced in v1.1.0.

## Configuration

Transfer execution is enabled separately from the Media Hub connector:

```yaml
media_hub:
  enabled: true
  transfer_enabled: true
  claim_enabled: true
  claim_interval: 2s
  progress_interval: 5s
  control_interval: 10s
  max_concurrent_jobs: 2
  accepted_operations: media_remote_download,playlist_m3u_download,remote_download
  work_dir: /var/lib/auren-transfer-agent/transfer
  min_bytes: 65536
  block_html: true
```

`server.enabled=true` is required for a long-running process. Without the HTTP runtime, the Agent initializes, prints diagnostics and exits.

## Download guards

The executor currently protects remote downloads with:

- HTTP/HTTPS request validation;
- provider redirects through the configured HTTP client;
- resume via existing `.part` files and `Range` headers;
- progress callbacks during copy;
- tiny-file guard through `media_hub.min_bytes` or `job.source.min_bytes`;
- HTML/XHTML blocking to catch login/error pages returned as successful HTTP responses;
- SHA-256 computation and optional expected SHA-256 verification.

## Upload targets

v1.3.0+ supports:

- local storage through the existing local storage adapter;
- Auren Storage production adapter using endpoint/bucket/token hints from the job payload or local config;
- direct `multipart/form-data` object upload to Auren Storage v1;
- large-object multipart upload lifecycle when `upload.multipart_enabled=true`;
- signed/scoped upload URL using `job.destination.upload_url`.

## Local state

Every claimed/executed job gets a JSON state file under:

```text
{media_hub.work_dir}/jobs/{job_uuid}.json
```

This is intentionally local and lightweight. It is used for crash/debug visibility, not as the source of business truth. Media Hub remains the source of truth for the transfer lifecycle.

## Worker handler

The foundation worker queue now uses `transfer_executor` instead of `noop`. A manually queued foundation job can still be executed locally by converting `worker.Job` into the richer transfer job payload.

## Deferred scope

Not included in v1.6.0 transfer executor scope:

- drain/dead-letter/secret rotation hardening;
- Media Hub UI/policy decisions.

Public streaming gateway runtime is implemented separately in `internal/gateway` and documented in `docs/development/gateway-runtime.md`.
