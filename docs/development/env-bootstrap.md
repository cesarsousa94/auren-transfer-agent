# Auren Transfer Agent v1.13.1 — Env Bootstrap & Node Authentication

This version makes the Agent easier to run in development by allowing the full runtime and Media Hub bootstrap flow to be driven from a root `.env` file.

## Files

Root examples:

- `.env.example`
- `env.example`
- `env.exemplo`

Linux package runtime file:

- `/etc/auren-transfer-agent/auren-transfer-agent.env`

The CLI reads `.env` by default. You can override it with `--env-file`.

```bash
auren-transfer-agent bootstrap --env-file .env --start-service
auren-transfer-agent serve --env-file .env --config ./configs/agent.yaml
auren-transfer-agent doctor --env-file .env --config ./configs/agent.yaml --online
```

Existing process environment variables win over values loaded from `.env`.

## Simple node registration flow

The preferred development flow is:

1. Media Hub creates or returns a one-time node registration token.
2. The Agent receives the Media Hub URL and the token through `.env` or CLI flags.
3. The Agent calls the Media Hub node registration endpoint.
4. Media Hub returns `node_uuid` and `node_secret`.
5. The Agent persists those credentials in `runtime.data_dir/media-hub/node.json` using `0600` permissions.
6. The Agent rewrites `agent.yaml` without the one-time registration token.

The initial token is only bootstrap material. The durable node credentials live in `node.json`.

## Minimal .env

```dotenv
AUREN_MEDIA_HUB_BASE_URL=http://127.0.0.1:8000
AUREN_MEDIA_HUB_REGISTRATION_TOKEN=auren-node-...
AUREN_AGENT_ROLE=worker
AUREN_AGENT_REGION=sa-east-1
AUREN_AGENT_START_SERVICE=false
```

Run:

```bash
sudo auren-transfer-agent bootstrap --env-file .env
```

## Optional token endpoint flow

For a later Media Hub integration, the Agent can request a one-time registration token from a Media Hub bootstrap endpoint instead of receiving the token directly.

```dotenv
AUREN_MEDIA_HUB_BASE_URL=http://127.0.0.1:8000
AUREN_MEDIA_HUB_BOOTSTRAP_TOKEN_ENDPOINT=/api/internal/nodes/bootstrap-token
AUREN_MEDIA_HUB_BOOTSTRAP_TOKEN_SECRET=shared-lab-secret
AUREN_AGENT_ROLE=worker
AUREN_AGENT_REGION=sa-east-1
```

Run:

```bash
sudo auren-transfer-agent bootstrap --env-file .env
```

The Agent sends a JSON request with role, region, capacity and capabilities and expects a response containing one of:

```json
{"registration_token":"..."}
```

or:

```json
{"data":{"registration_token":"..."}}
```

The request includes `X-Auren-Bootstrap-Secret` when `AUREN_MEDIA_HUB_BOOTSTRAP_TOKEN_SECRET` is set.

## Custom Media Hub paths

Defaults:

```dotenv
AUREN_MEDIA_HUB_REGISTER_PATH=/api/internal/nodes/register
AUREN_MEDIA_HUB_CONFIG_PATH=/api/internal/nodes/config
AUREN_MEDIA_HUB_HEARTBEAT_PATH=/api/internal/nodes/heartbeat
AUREN_MEDIA_HUB_METRICS_PATH=/api/internal/nodes/metrics
AUREN_MEDIA_HUB_EVENTS_PATH=/api/internal/nodes/events
```

Use these only if the Media Hub exposes compatibility routes or a versioned internal prefix.

## Supported aliases

For bootstrap convenience, these aliases are also accepted:

- `AUREN_AGENT_MEDIA_HUB_BASE_URL`, `AUREN_MEDIA_HUB_URL`, `MEDIA_HUB_URL`
- `AUREN_NODE_REGISTRATION_TOKEN`, `AUREN_AGENT_REGISTRATION_TOKEN`, `REGISTRATION_TOKEN`
- `AUREN_BOOTSTRAP_TOKEN_ENDPOINT`, `AUREN_NODE_TOKEN_ENDPOINT`, `MEDIA_HUB_NODE_TOKEN_ENDPOINT`
- `AUREN_BOOTSTRAP_TOKEN_SECRET`, `AUREN_NODE_TOKEN_SECRET`, `MEDIA_HUB_NODE_TOKEN_SECRET`
- `AUREN_AGENT_ROLE`, `AUREN_NODE_ROLE`
- `AUREN_AGENT_REGION`, `AUREN_NODE_REGION`

## Security note

Do not commit `.env` with real tokens or S3 credentials. Commit only `.env.example`/`env.example`/`env.exemplo`.
