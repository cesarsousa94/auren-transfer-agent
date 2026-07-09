# Production Deployment

Version `v1.7.0` keeps the production runtime, Media Hub connector, transfer executor, Auren Storage v1 adapter, Gateway Runtime and operational hardening baseline, then adds Debian/Ubuntu packaging, zero-touch Media Hub bootstrap and online APT repository distribution.

The Agent remains business-rule free. It executes transport, queue, resolver, download, upload, Auren Storage adapter, gateway proxy/redirect, telemetry and security primitives exposed by previous EPICs. Media Hub decisions stay outside this repository.

## Docker

Build the image:

```bash
docker build -f docker/Dockerfile -t auren-transfer-agent:v1.7.0 .
```

Run the container:

```bash
docker run --rm -p 8080:8080 \
  -e AUREN_SERVER_ENABLED=true \
  -e AUREN_RUNTIME_ENVIRONMENT=production \
  auren-transfer-agent:v1.7.0
```

## Docker Compose

```bash
cd docker
docker compose up --build
```

The compose file mounts durable data volumes and enables the HTTP runtime through environment overrides.

## Linux Package / Installer

Build a `.deb` package:

```bash
make build
./scripts/build-deb.sh v1.7.0
```

Install manually:

```bash
sudo dpkg -i dist/auren-transfer-agent_1.7.0_amd64.deb
sudo auren-transfer-agent bootstrap \
  --media-hub=https://media.example.com \
  --token=REGISTRATION_TOKEN \
  --role=worker \
  --start-service
```

One-line installer shape for Media Hub-generated commands:

```bash
curl -fsSL https://downloads.auren.app/agent/install.sh | sudo bash -s -- \
  --media-hub=https://media.example.com \
  --token=REGISTRATION_TOKEN \
  --role=hybrid
```

The package creates the `auren-agent` system user, installs the binary under `/usr/bin`, stores config under `/etc/auren-transfer-agent`, stores durable node state under `/var/lib/auren-transfer-agent` and registers the service with systemd.


## Online APT repository

Build the static APT repository:

```bash
./scripts/release.sh v1.7.0
```

Publish `dist/apt` to S3/CloudFront, Nginx or another HTTPS static origin. For AWS S3:

```bash
S3_URI=s3://your-download-bucket/agent/apt \
CLOUDFRONT_DISTRIBUTION_ID=E1234567890 \
./scripts/publish-apt-s3.sh
```

Install from the online APT repository and register the node:

```bash
curl -fsSL https://downloads.auren.app/agent/install.sh | sudo bash -s -- \
  --apt \
  --repo-url https://downloads.auren.app/agent/apt \
  --apt-key-url https://downloads.auren.app/agent/auren-transfer-agent.gpg \
  --media-hub=https://media.example.com \
  --token=REGISTRATION_TOKEN \
  --role=worker
```

Unsigned `trusted=yes` mode is supported for private lab tests only. Production repositories should be GPG-signed. See `docs/deployment/apt-repository.md`.

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
./scripts/release.sh v1.7.0
```

Dry-run the release pipeline:

```bash
./scripts/release.sh v1.7.0 --dry-run
```

## Runtime server

Production assets set:

```text
AUREN_SERVER_ENABLED=true
```

When enabled, the Agent starts the HTTP server with the already-registered foundation routes and stops gracefully on `SIGTERM`/`SIGINT`.


## Gateway runtime

To serve Media Hub Capacity Routing handoff traffic, expose the HTTP port publicly and set:

```text
AUREN_SERVER_ENABLED=true
AUREN_MEDIA_HUB_ENABLED=true
AUREN_MEDIA_HUB_GATEWAY_ENABLED=true
AUREN_MEDIA_HUB_PUBLIC_BASE_URL=https://node-1.example.com
```

The public route is `/_auren/gateway/{token}/{kind}/{id}.{ext}`. Media Hub remains responsible for issuing the token and returning `proxy` or `redirect` instructions through `/api/internal/gateway/resolve`.


## Operational hardening

For production nodes, keep these controls enabled unless a controlled lab environment needs to disable them:

```text
AUREN_MEDIA_HUB_DRAIN_ENABLED=true
AUREN_MEDIA_HUB_BACKPRESSURE_ENABLED=true
AUREN_MEDIA_HUB_DISK_GUARD_ENABLED=true
AUREN_MEDIA_HUB_DEAD_LETTER_ENABLED=true
AUREN_MEDIA_HUB_LEASE_RENEWAL_ENABLED=true
```

Create the drain marker file configured by `AUREN_MEDIA_HUB_DRAIN_FILE` before maintenance or scale-in to stop new claims/sessions while letting active work finish.
