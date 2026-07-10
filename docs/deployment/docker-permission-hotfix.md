# Docker Permission Hotfix — v1.13.1

Auren Transfer Agent v1.13.1 fixes Docker auto-bootstrap permission errors when the container runs as the non-root `auren` user.

The v1.13.0 entrypoint called `bootstrap` without overriding the Linux package default log directory. The bootstrap command therefore tried to create:

```text
/var/log/auren-transfer-agent
```

Inside the Docker image, the runtime intentionally runs without root privileges. v1.13.1 redirects Docker bootstrap logs to the writable persistent volume:

```text
/var/lib/auren-transfer-agent/logs
```

The Dockerfile and Compose file now set:

```dotenv
AUREN_AGENT_LOG_DIR=/var/lib/auren-transfer-agent/logs
```

The entrypoint also passes:

```bash
--log-dir "$AUREN_AGENT_LOG_DIR"
```

to the automatic bootstrap command.

## Resetting a failed v1.13.0 lab container

```bash
docker compose -f docker/docker-compose.yml down -v
docker compose -f docker/docker-compose.yml up --build
```
