# Detailed Request Inspector — v1.13.1

Auren Transfer Agent v1.13.1 expands the Local Dev Console so development can follow the real transfer flow without Prometheus/Grafana.

Open:

```text
/_auren/dev/requests
```

The page now shows:

- inbound Agent HTTP requests;
- outbound Media Hub HTTP calls;
- transfer lifecycle events;
- job UUID;
- operation;
- stage;
- source URL;
- destination driver;
- object path;
- status;
- duration;
- bytes;
- sanitized request headers;
- sanitized response headers;
- bounded sanitized request body;
- bounded sanitized response body;
- full sanitized record JSON.

Sensitive fields are redacted automatically when keys contain values such as `authorization`, `token`, `secret`, `password`, `api_key`, `cookie`, `signature`, `credential`, `node_secret`, `registration_token`, `aws_access_key_id` or `aws_secret_access_key`.

The request inspector is intentionally a development tool. Use SSH tunnel, VPN or protected reverse proxy. Do not expose `/_auren/dev/*` directly on the public internet.

## Useful filters

Use the search field to filter by:

```text
job uuid
source url
object path
destination driver
progress
completed
failed
s3
auren_storage
```

## Transfer events

The executor records these non-HTTP events:

```text
claim
download.start
upload.start
upload.completed
```

These events make it possible to see a Media Hub download request even when the heavy I/O is happening as local download/upload work rather than as an HTTP request handled by the Agent.
