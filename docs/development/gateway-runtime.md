# Gateway Runtime — v1.9.1

Auren Transfer Agent v1.9.1 implements the public Media Hub gateway runtime. The Agent remains an execution/data-plane component: Media Hub validates the public token, chooses the delivery policy and returns the upstream instructions.

## Public handoff route

The runtime exposes the canonical Media Hub node handoff path:

```text
GET  /_auren/gateway/{token}/{kind}/{id}.{ext}
HEAD /_auren/gateway/{token}/{kind}/{id}.{ext}
```

The offline Chi compatibility router registers this internally as `/_auren/gateway/*`; the parser enforces the canonical token/kind/id/ext shape.

## Media Hub calls

Before opening upstream traffic, the Agent calls:

```text
POST /api/internal/gateway/resolve
```

The request includes token, media kind, id, extension, method, Range, User-Agent, selected request headers and timestamp. Node authentication uses the same `X-Auren-Node-*` and HMAC contract introduced by the Media Hub Connector.

Media Hub returns a mode:

- `proxy`: the Agent opens the upstream URL and streams bytes to the player;
- `redirect`: the Agent returns an HTTP 302 to the resolved upstream URL.

During proxy sessions, the Agent reports:

```text
POST /api/internal/gateway/sessions/heartbeat
POST /api/internal/gateway/sessions/close
POST /api/internal/gateway/events
```

## Proxy behavior

Proxy mode preserves Range requests and allows Media Hub to inject upstream headers such as User-Agent, cookies or provider-specific headers. Hop-by-hop response headers are stripped before returning upstream response headers to the player.

The runtime tracks bytes sent per session and flushes the response writer when possible, which keeps long-running IPTV streams responsive.

## Capacity telemetry

Gateway runtime metrics are folded into node heartbeat and metrics payloads:

- `active_sessions` comes from active gateway session count;
- `current_egress_mbps` is approximated from bytes sent by the gateway tracker;
- `max_sessions` and `max_egress_mbps` still come from `media_hub` config.

This allows Media Hub Capacity Routing to move traffic away from saturated nodes.

## Configuration

```yaml
media_hub:
  enabled: true
  gateway_enabled: true
  gateway_proxy_enabled: true
  gateway_redirect_enabled: true
  gateway_heartbeat_interval: 10s
  gateway_token_ttl: 5m
  public_base_url: https://node-1.example.com
  capabilities: transfer,gateway,download,upload,auren_storage,xtream,shui,m3u8,hls,live,movie,series
```

`server.enabled=true` is required because the runtime is served through the Agent HTTP server.

## Not included

v1.9.1 adds drain/backpressure admission around this gateway runtime. The gateway still delegates token validation and upstream policy to Media Hub.
