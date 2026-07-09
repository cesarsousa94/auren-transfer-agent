# Production Deployment

Version `v1.0.0` completes EPIC 13 and provides the official production deployment foundation for the Auren Transfer Agent.

The Agent remains business-rule free. It executes transport, queue, resolver, download, upload, telemetry and security primitives exposed by previous EPICs. Media Hub decisions stay outside this repository.

## Docker

Build the image:

```bash
docker build -f docker/Dockerfile -t auren-transfer-agent:v1.0.0 .
```

Run the container:

```bash
docker run --rm -p 8080:8080 \
  -e AUREN_SERVER_ENABLED=true \
  -e AUREN_RUNTIME_ENVIRONMENT=production \
  auren-transfer-agent:v1.0.0
```

## Docker Compose

```bash
cd docker
docker compose up --build
```

The compose file mounts durable data volumes and enables the HTTP runtime through environment overrides.

## Linux Installer

Build the binary, then install it as a system service:

```bash
make build
sudo ./deploy/linux/install.sh ./bin/auren-transfer-agent
sudo systemctl start auren-transfer-agent.service
sudo systemctl status auren-transfer-agent.service
```

The installer creates the `auren-transfer` system user when missing, installs the binary under `/opt/auren-transfer-agent`, copies configuration into `/etc/auren-transfer-agent` and stores runtime state under `/var/lib/auren-transfer-agent`.

## systemd

The unit is available at:

```text
deploy/systemd/auren-transfer-agent.service
```

Runtime environment overrides can be placed in:

```text
/etc/auren-transfer-agent/auren-transfer-agent.env
```

## Kubernetes

Apply the foundation manifest:

```bash
kubectl apply -f deploy/kubernetes/auren-transfer-agent.yaml
```

The manifest includes Namespace, ConfigMap, Deployment, Service, readiness probe and liveness probe. The image name is intentionally local by default and should be replaced by the target registry during release.

## CI/CD

GitHub Actions workflow:

```text
.github/workflows/ci.yml
```

It runs formatting checks, tests, build, version output and release archive dry-run.

## Release pipeline

Create a release archive:

```bash
./scripts/release.sh v1.0.0
```

Dry-run the release pipeline:

```bash
./scripts/release.sh v1.0.0 --dry-run
```

## Runtime server

Production assets set:

```text
AUREN_SERVER_ENABLED=true
```

When enabled, the Agent starts the HTTP server with the already-registered foundation routes and stops gracefully on `SIGTERM`/`SIGINT`.
