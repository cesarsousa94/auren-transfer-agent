# Operational Hardening — v1.6.0

Auren Transfer Agent v1.6.0 adds production safety controls around the Media Hub connector, transfer executor, Auren Storage adapter and Gateway Runtime.

The Agent still does not make business decisions. These controls only determine whether the node is mechanically safe to accept more work.

## Controls

- **Drain:** when `media_hub.drain_enabled=true`, a marker file at `media_hub.drain_file` or a shutdown signal stops new transfer claims and new gateway sessions.
- **Media Hub drain callbacks:** shutdown-triggered drain sends `/api/internal/nodes/drain/started`; after HTTP shutdown, the Agent sends `/api/internal/nodes/drain/completed`.
- **Backpressure:** when `media_hub.backpressure_enabled=true`, the Agent rejects new claims/sessions once job, session or egress limits are reached.
- **Disk pressure guard:** when `media_hub.disk_guard_enabled=true`, the claim loop checks free bytes for `media_hub.work_dir` before accepting heavy transfer work.
- **Dead-letter:** failed transfer attempts are persisted as JSON under `media_hub.dead_letter_dir` when enabled.
- **Lease renewal:** long-running transfers send periodic progress callbacks with `lease_renewal=true`.
- **Control watcher:** the executor polls Media Hub `/control` and cancels local work when the operator asks for `cancel`, `pause`, `release` or `drain`.
- **Authenticated worker API:** local worker routes are wrapped by the configured Agent API-key authenticator.

## Configuration

```yaml
media_hub:
  drain_enabled: true
  drain_file: ./data/transfer/drain
  backpressure_enabled: true
  disk_guard_enabled: true
  disk_min_free_bytes: 1073741824
  dead_letter_enabled: true
  dead_letter_dir: ./data/transfer/dead-letter
  lease_renewal_enabled: true
  lease_renewal_interval: 30s
  secret_rotation_enabled: false
  secret_rotation_interval: 24h
```

## Drain operation

Create the marker file to stop new work:

```bash
mkdir -p ./data/transfer
echo "maintenance" > ./data/transfer/drain
```

Remove it to let the Agent accept new work again:

```bash
rm -f ./data/transfer/drain
```

Existing jobs and sessions are allowed to finish unless the process receives a shutdown signal or Media Hub sends a control action.

## Dead-letter format

Each failed attempt is saved as a JSON file containing:

- job UUID;
- operation;
- failed stage;
- error message;
- retryability flag;
- optional payload/metadata;
- creation timestamp.

This is intended for audit and support diagnostics. Media Hub remains the source of truth for retries and final job state.

## Secret rotation

`secret_rotation_enabled` and `secret_rotation_interval` are validated and exposed in diagnostics as a foundation hook. Actual live credential rotation depends on the Media Hub-side rotation endpoint and policy.
