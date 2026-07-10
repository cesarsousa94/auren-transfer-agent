# Production Deployment — v1.9.0

Version `v1.9.0` keeps the runtime, Media Hub connector, transfer executor, Auren Storage adapter, Gateway Runtime, operational hardening and Debian packaging baseline, then makes signed online APT distribution the recommended deployment path.

## Recommended production install path

1. Build a signed APT repository.
2. Publish it to S3 + CloudFront or another HTTPS static origin.
3. Let Media Hub generate a one-time registration token.
4. Run the generated install command on the EC2/Ubuntu node.
5. The Agent registers, persists `node_uuid/node_secret`, enables systemd and starts sending heartbeat.

## Build release

```bash
APT_SIGN=true \
APT_REQUIRE_SIGNED=true \
APT_GPG_KEY_ID="YOUR_GPG_KEY_ID" \
APT_CHANNELS="stable,edge" \
APT_PUBLIC_REPO_URL="https://downloads.seudominio.com/agent/apt" \
APT_PUBLIC_KEY_URL="https://downloads.seudominio.com/agent/apt/auren-transfer-agent.gpg" \
./scripts/release.sh v1.9.0
```

## Publish release

```bash
S3_URI=s3://seu-bucket/agent/apt \
CLOUDFRONT_DISTRIBUTION_ID=E1234567890 \
./scripts/publish-apt-s3.sh
```

## Install worker node

```bash
curl -fsSL https://downloads.seudominio.com/agent/apt/install-apt.sh | sudo bash -s -- \
  --repo-url https://downloads.seudominio.com/agent/apt \
  --apt-key-url https://downloads.seudominio.com/agent/apt/auren-transfer-agent.gpg \
  --media-hub https://media.seudominio.com \
  --token TOKEN_GERADO_NO_MEDIA_HUB \
  --role worker \
  --region sa-east-1
```

## Install hybrid gateway/worker node

```bash
curl -fsSL https://downloads.seudominio.com/agent/apt/install-apt.sh | sudo bash -s -- \
  --repo-url https://downloads.seudominio.com/agent/apt \
  --apt-key-url https://downloads.seudominio.com/agent/apt/auren-transfer-agent.gpg \
  --media-hub https://media.seudominio.com \
  --token TOKEN_GERADO_NO_MEDIA_HUB \
  --role hybrid \
  --enable-gateway \
  --public-base-url https://node1.seudominio.com \
  --region sa-east-1
```

## Service lifecycle

```bash
sudo systemctl status auren-transfer-agent
sudo systemctl restart auren-transfer-agent
sudo journalctl -u auren-transfer-agent -f
```

The service runs:

```text
/usr/bin/auren-transfer-agent serve --config /etc/auren-transfer-agent/agent.yaml
```

## Direct `.deb` fallback

Direct package installs remain useful for offline troubleshooting:

```bash
sudo dpkg -i dist/auren-transfer-agent_1.9.0_amd64.deb
sudo apt-get install -f -y
```

For normal operations, prefer APT so upgrades are one command:

```bash
sudo apt update
sudo apt upgrade auren-transfer-agent
```
