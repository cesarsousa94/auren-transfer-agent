# Linux Package & Zero-Touch Bootstrap — v1.6.0

Auren Transfer Agent v1.6.0 turns the Agent into a Debian/Ubuntu-installable service. The package installs a durable systemd runtime, creates the `auren-agent` Linux user, writes canonical directories under `/etc`, `/var/lib`, `/var/log` and `/var/tmp`, and exposes CLI bootstrap commands so a node can register itself against Auren Media Hub using a one-time registration token.

## Package layout

```text
/usr/bin/auren-transfer-agent
/etc/auren-transfer-agent/agent.yaml
/etc/auren-transfer-agent/auren-transfer-agent.env
/lib/systemd/system/auren-transfer-agent.service
/var/lib/auren-transfer-agent
/var/lib/auren-transfer-agent/media-hub/node.json
/var/lib/auren-transfer-agent/transfer
/var/lib/auren-transfer-agent/transfer/dead-letter
/var/log/auren-transfer-agent
/var/tmp/auren-transfer-agent
```

`node.json` is created after registration and stores `node_uuid` plus `node_secret` with file mode `0600` inside the durable data directory.

## One-line install

```bash
curl -fsSL https://downloads.auren.app/agent/install.sh | sudo bash -s -- \
  --media-hub=https://media.example.com \
  --token=REGISTRATION_TOKEN \
  --role=hybrid \
  --region=sa-east-1 \
  --max-concurrent-jobs=2
```

Gateway mode also needs a public URL:

```bash
curl -fsSL https://downloads.auren.app/agent/install.sh | sudo bash -s -- \
  --media-hub=https://media.example.com \
  --token=REGISTRATION_TOKEN \
  --role=hybrid \
  --enable-gateway \
  --public-base-url=https://node1.example.com
```

## Manual `.deb` install

```bash
sudo dpkg -i auren-transfer-agent_1.6.0_amd64.deb
sudo auren-transfer-agent bootstrap \
  --media-hub=https://media.example.com \
  --token=REGISTRATION_TOKEN \
  --role=worker \
  --start-service
```

The bootstrap command writes `/etc/auren-transfer-agent/agent.yaml`, registers the node, persists Media Hub credentials and can enable/start the service.

## Service lifecycle

```bash
sudo systemctl enable --now auren-transfer-agent
sudo systemctl status auren-transfer-agent
journalctl -u auren-transfer-agent -f
sudo systemctl restart auren-transfer-agent
```

The service runs:

```bash
/usr/bin/auren-transfer-agent serve --config /etc/auren-transfer-agent/agent.yaml
```

It restarts automatically after process failure and starts again after machine reboot.

## CLI commands

```bash
auren-transfer-agent bootstrap --media-hub URL --token TOKEN [options]
auren-transfer-agent doctor --config /etc/auren-transfer-agent/agent.yaml
auren-transfer-agent status --config /etc/auren-transfer-agent/agent.yaml
auren-transfer-agent serve --config /etc/auren-transfer-agent/agent.yaml
auren-transfer-agent --version
```

`doctor` validates config, data directories and Media Hub credential state. Add `--online` to test HTTP connectivity.

## Media Hub registration flow

1. Media Hub creates a node in provisioning state and issues a one-time registration token.
2. The Linux installer passes `--media-hub` and `--token` to `auren-transfer-agent bootstrap`.
3. The Agent calls `/api/internal/nodes/register`.
4. Media Hub returns `node_uuid` and `node_secret`.
5. The Agent stores those credentials under `/var/lib/auren-transfer-agent/media-hub/node.json`.
6. Runtime calls are then authenticated with `X-Auren-Node-UUID`, `X-Auren-Node-Secret` and HMAC headers.
7. The Agent sends heartbeat, metrics, events, transfer claims, transfer callbacks and gateway session telemetry in the background.

## Building packages

```bash
make build
./scripts/build-deb.sh v1.6.0
./scripts/release.sh v1.6.0
```

Release output includes:

```text
dist/auren-transfer-agent-v1.6.0.zip
dist/auren-transfer-agent_1.6.0_amd64.deb
dist/auren-transfer-agent_1.6.0_amd64.deb.sha256
```

## APT repository skeleton

```bash
./scripts/build-apt-repo.sh dist/apt 'dist/*.deb'
```

The result can be published behind `https://downloads.auren.app/agent/apt` and signed with your GPG release key. The script intentionally does not manage private signing keys.
