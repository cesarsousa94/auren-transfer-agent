# Media Hub Connector — v1.9.0

Auren Transfer Agent v1.1.0 added the Media Hub Connector Foundation. v1.9.0 keeps that contract and uses it for transfer claim/callback execution, production Auren Storage upload results and Gateway Runtime session telemetry. The Agent can now register itself as a Media Hub `edge_node`, persist the issued `node_uuid` and `node_secret`, pull node configuration and send heartbeat, metrics and events through the existing Media Hub Node Agent contract.

The connector remains the operational control-plane link: it owns registration, durable node credentials, config pull, heartbeat, metrics and events. The transfer executor and gateway runtime use the same node credentials/HMAC contract.

## Configuration

```yaml
media_hub:
  enabled: true
  base_url: https://mediahub.example.com
  registration_token: ""
  node_uuid: ""
  node_secret: ""
  hmac_enabled: true
  poll_enabled: true
  poll_interval: 2s
  gateway_enabled: true
  gateway_proxy_enabled: true
  gateway_redirect_enabled: true
  gateway_heartbeat_interval: 10s
  gateway_token_ttl: 5m
  heartbeat_interval: 30s
  metrics_interval: 60s
  events_flush_interval: 10s
  role: gateway
  provider: auren_transfer_agent
  region: sa-east-1
  availability_zone: ""
  public_base_url: https://node-1.example.com
  health_url: https://node-1.example.com/health
  max_sessions: 500
  max_egress_mbps: 1000
  capabilities: transfer,gateway,download,upload,auren_storage,xtream,shui,m3u8,hls,live,movie,series
```

`capabilities` is comma-separated because the current lightweight config contract does not support YAML lists.

## Registration flow

1. Start the Agent with `media_hub.enabled=true`, a valid `media_hub.base_url` and a one-time `media_hub.registration_token` issued by Media Hub.
2. The Agent posts to `/api/internal/nodes/register` with local identity, runtime version, role, provider, region, public URLs, capacity and capabilities.
3. The Agent stores the returned `node_uuid` and `node_secret` in `runtime.data_dir/media-hub/node.json` with `0600` permissions.
4. On restart, the Agent reuses the persisted state and does not require the registration token again.
5. If `media_hub.node_uuid` and `media_hub.node_secret` are provided explicitly, they are imported into the local state store.

## Auth and HMAC

Authenticated requests include:

- `X-Auren-Node-UUID`
- `X-Auren-Node-Secret`
- `X-Auren-Agent-Timestamp`
- `X-Auren-Agent-Nonce`
- `X-Auren-Agent-Signature-Version: v1`
- `X-Auren-Agent-Signature`

The v1 canonical HMAC string is:

```text
METHOD
/path?query
timestamp
nonce
sha256(body)
```

The signature is a lowercase hex HMAC-SHA256 using the Media Hub-issued node secret.

## Telemetry

After registration, bootstrap performs one immediate config pull, heartbeat, metrics flush and events flush. When `server.enabled=true`, the connector keeps loops alive until SIGTERM/SIGINT:

- config pull: `media_hub.poll_interval`, only when `media_hub.poll_enabled=true`;
- heartbeat: `media_hub.heartbeat_interval`;
- metrics: `media_hub.metrics_interval`;
- events: `media_hub.events_flush_interval`.

## Current limits

- Worker execution now uses the `transfer_executor` handler introduced in v1.2.0.
- Transfer job claim, progress and completion callbacks are implemented by `internal/transfer` in v1.2.0.
- Auren Storage production upload adapter alignment was implemented in v1.3.0.
- Public gateway runtime and session telemetry are implemented in v1.9.0.
