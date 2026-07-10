# Local Dev Console — v1.9.1

Auren Transfer Agent v1.9.1 adds a lightweight local console for development and first production validation. It intentionally does not replace Prometheus or Grafana. Its goal is to make the Agent understandable while integrating it with Auren Media Hub.

## Pages

When `server.enabled=true` and `dev_ui.enabled=true`, the console is available at:

```text
http://127.0.0.1:8080/_auren/dev/metrics
http://127.0.0.1:8080/_auren/dev/requests
```

The metrics page shows:

- Media Hub base URL and node UUID;
- active transfer jobs and terminal counters;
- gateway sessions, bytes and approximate egress;
- local queue driver, length and capacity;
- hardening decision state;
- request counters and the latest request traces.

The requests page shows:

- inbound HTTP requests received by the Agent;
- outbound Media Hub calls made by the Agent;
- method, path, HTTP status, duration, bytes and error message.

## JSON endpoints

```text
GET /_auren/dev/api/snapshot
GET /_auren/dev/api/requests
```

The HTML pages poll these endpoints every `dev_ui.refresh_interval`.

## Configuration

```yaml
dev_ui:
  enabled: true
  path: /_auren/dev
  retention: 500
  refresh_interval: 2s
```

For package/bootstrap installs, the v1.9.1 bootstrap writes:

```yaml
dev_ui:
  enabled: true
  path: /_auren/dev
  retention: 1000
  refresh_interval: 2s
```

## Security note

The Local Dev Console is meant for development, labs and controlled node operations. In production, expose it only on private interfaces, VPN, SSH tunnel or behind a trusted reverse proxy. Do not expose it publicly without network-level protection.

## Typical AWS test flow

```bash
sudo auren-transfer-agent bootstrap \
  --media-hub=https://media.example.com \
  --token=TOKEN \
  --role=worker \
  --start-service

sudo systemctl status auren-transfer-agent
curl http://127.0.0.1:8080/_auren/dev/api/snapshot
```

Open an SSH tunnel if the EC2 security group should not expose the console directly:

```bash
ssh -L 8080:127.0.0.1:8080 ubuntu@NODE_PUBLIC_IP
```

Then open:

```text
http://127.0.0.1:8080/_auren/dev/metrics
```
