# Docker Auto Bootstrap Runtime

Auren Transfer Agent v1.13.1 can register itself when the container starts.
This is the recommended lab flow when validating Media Hub ↔ Agent behavior
without repeatedly installing a remote machine.

## 1. Prepare Media Hub

Enable the Media Hub bootstrap-token endpoint and choose a shared secret:

```dotenv
AUREN_NODE_BOOTSTRAP_TOKEN_ENABLED=true
AUREN_NODE_BOOTSTRAP_TOKEN_SECRET=dev-agent-secret
AUREN_NODE_BOOTSTRAP_TOKEN_REQUIRE_SECRET=true
```

Then clear Laravel config/cache.

## 2. Prepare Agent `.env`

Copy the root dotenv example:

```bash
cp .env.example .env
```

Set at least:

```dotenv
AUREN_MEDIA_HUB_BASE_URL=http://host.docker.internal:8000
AUREN_MEDIA_HUB_BOOTSTRAP_TOKEN_ENDPOINT=/api/internal/nodes/bootstrap-token
AUREN_MEDIA_HUB_BOOTSTRAP_TOKEN_SECRET=dev-agent-secret
AUREN_MEDIA_HUB_ROLE=worker
AUREN_MEDIA_HUB_REGION=sa-east-1
AUREN_DOCKER_AUTO_BOOTSTRAP=true
AUREN_AGENT_HTTP_PORT=8080
```

On Linux, if `host.docker.internal` is unavailable, use the reachable Media Hub
host/IP on the Docker bridge/network.

## 3. Start the container

```bash
docker compose -f docker/docker-compose.yml up --build
```

The entrypoint will:

1. check `/var/lib/auren-transfer-agent/media-hub/node.json`;
2. skip bootstrap if a node identity already exists;
3. request a registration token from Media Hub when needed;
4. run `auren-transfer-agent bootstrap`;
5. write `/etc/auren-transfer-agent/agent.yaml`;
6. persist `node_uuid`/`node_secret` in the Docker volume;
7. start `auren-transfer-agent serve`.

## 4. Inspect

```text
http://127.0.0.1:8080/_auren/dev/metrics
http://127.0.0.1:8080/_auren/dev/requests
```

## 5. Force re-bootstrap

Only for lab resets:

```bash
docker compose -f docker/docker-compose.yml down -v
```

or set:

```dotenv
AUREN_DOCKER_FORCE_BOOTSTRAP=true
```

Do not force-bootstrap in production unless you are intentionally replacing the
node identity.
