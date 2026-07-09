# Download Engine Foundation

Auren Transfer Agent v0.1.28 completes **EPIC 6 — Download Engine** by delivering:

- **6.10 — Bandwidth Controller**;
- **6.11 — Download Metrics**;
- **6.12 — Download Tests**.

v0.1.25 introduced the foundation HTTP client, redirect engine and cookie engine. v0.1.26 added headers, resume and streaming. v0.1.27 added range-based multipart planning/execution, local SHA-256 integrity primitives and context-aware retry loops. v0.1.28 completes the download foundation with optional bandwidth throttling, local metric recording and integrated tests.

These contracts are transport primitives only. The Agent still does not decide provider policy, execute Media Hub business rules, upload to Storage or export Prometheus metrics.

## Package

The download foundation lives at:

```text
internal/download
```

Current public surface includes:

```text
download.HTTPClientName
download.HTTPClientOptions
download.HTTPClient
download.OptionsFromConfig
download.NewHTTPClientFromConfig
download.NewHTTPClient
download.HTTPClient.Do
download.HTTPClient.Stream
download.HTTPClient.StreamToFile
download.HTTPClient.MultipartToFile
download.HeaderSet
download.NewHeaderSet
download.NewRequest
download.ResumeState
download.NewResumeState
download.ResumeFromFile
download.ApplyResume
download.StreamResult
download.BandwidthControllerName
download.BandwidthOptions
download.BandwidthController
download.NewBandwidthController
download.NewBandwidthControllerWithOptions
download.BandwidthController.Enabled
download.BandwidthController.LimitBytesPerSecond
download.BandwidthController.DelayFor
download.BandwidthController.Wait
download.BandwidthController.WrapWriter
download.DownloadMetricsName
download.DownloadMetric
download.DownloadSummary
download.MetricsRecorder
download.MemoryMetricsRecorder
download.NewMemoryMetricsRecorder
download.MemoryMetricsRecorder.RecordDownloadMetric
download.MemoryMetricsRecorder.Snapshot
download.MemoryMetricsRecorder.Count
download.MemoryMetricsRecorder.Summary
download.ValidateDownloadMetric
download.MultipartEngineName
download.MultipartOptions
download.MultipartPart
download.MultipartPlan
download.MultipartResult
download.NewMultipartPlan
download.MultipartPlan.PartsSnapshot
download.SHA256ChecksumName
download.SHA256Result
download.SHA256Reader
download.SHA256File
download.VerifySHA256File
download.ValidateSHA256Hex
download.IsSHA256Hex
download.RetryEngineName
download.RetryOptions
download.RetryPolicy
download.RetryAttempt
download.RetryResult
download.Operation
download.NewRetryPolicy
download.NewRetryPolicyFromConfig
download.RetryPolicy.MaxRetries
download.RetryPolicy.Backoff
download.RunWithRetry
```

## Bandwidth controller

`NewBandwidthController(0)` creates an unlimited controller. A positive limit enables mechanical bytes-per-second throttling. The controller exposes pure delay calculation through `DelayFor` and can wrap an `io.Writer` for streaming paths.

The limiter is intentionally local and mechanical. It does not decide whether a job deserves throttling; future orchestration can pass the correct controller per job.

## Download metrics

`MemoryMetricsRecorder` stores local `DownloadMetric` entries and exposes defensive snapshots plus an aggregate `DownloadSummary`.

Streaming and multipart paths can receive a recorder through their options. Metrics include engine, URL, status, bytes, content length, duration, resume/range metadata, multipart part count, bandwidth metadata and final error string when applicable.

This is not the Prometheus exporter. Central observability remains in EPIC 11.

## Multipart download

`NewMultipartPlan(totalSize, partSize)` splits a known byte size into deterministic inclusive ranges:

```text
TotalSize=10 PartSize=4 => bytes=0-3, bytes=4-7, bytes=8-9
```

`HTTPClient.MultipartToFile` downloads each range with `Range: bytes=start-end`, expects `206 Partial Content`, stores temporary part files under `<target>.parts`, merges parts in order and removes the temporary directory when complete.

## SHA-256

`SHA256Reader` computes a digest while streaming from any `io.Reader`. `SHA256File` computes a digest for a local file. `VerifySHA256File` validates the expected hex digest and compares it with the local file digest.

Checksum primitives do not decide whether a job should be accepted, retried or reported as failed. That decision remains in the orchestration layers.

## Retry download

`RunWithRetry` runs a typed operation until it succeeds, retries are exhausted or the context is cancelled. `NewRetryPolicyFromConfig` accepts the validated `download.max_retries` and `download.retry_backoff` values.

The retry loop records failed attempts and returns the final error unchanged. It does not classify provider-specific failures yet.

## Download tests

EPIC 6 includes tests for:

- HTTP client timeouts and User-Agent behavior;
- redirect policy and cookie jar behavior;
- header validation and request construction;
- resume state and range header application;
- streaming to writers/files;
- multipart range planning and merge behavior;
- SHA-256 calculation and verification;
- retry success/failure/cancellation behavior;
- bandwidth delay/writer behavior;
- streaming metrics, failure metrics and defensive metric snapshots.

## Current bootstrap behavior

The bootstrap constructs the download client, retry policy, unlimited bandwidth controller and in-memory metrics recorder and prints diagnostics similar to:

```text
download: client=http user_agent="AurenTransferAgent/0.1" redirects=true max_redirects=10 cookies=cookie_jar headers=headers resume=true streaming=streaming multipart=multipart checksum=sha256 retry=retry bandwidth=bandwidth bandwidth_limit=0 metrics=download_metrics metrics_count=0 max_retries=3 retry_backoff=2s chunk_size=8MiB connect_timeout=15s response_header_timeout=30s idle_timeout=1m0s
```

No outbound HTTP request is made during bootstrap.

## Next steps

EPIC 6 is complete. The next roadmap item is:

```text
7.1 — Resolver Interface
```
