# Signed APT Repository Distribution — v1.9.0

Auren Transfer Agent v1.9.0 makes the online APT repository the official Linux distribution path. The direct `.deb` remains a build artifact, but production installs should use an HTTPS APT repository with a GPG-signed `Release` file and a public `install-apt.sh` bootstrap script.

## Release artifacts

Running the release pipeline creates:

```text
./scripts/release.sh v1.9.0

dist/auren-transfer-agent-v1.9.0.zip
dist/auren-transfer-agent_1.9.0_amd64.deb
dist/auren-transfer-agent-apt-repo-v1.9.0.tar.gz
dist/apt/
```

The APT repo contains:

```text
apt/
├── pool/main/a/auren-transfer-agent/auren-transfer-agent_1.9.0_amd64.deb
├── dists/stable/Release
├── dists/stable/InRelease                  # when signed
├── dists/stable/Release.gpg                # when signed
├── dists/stable/main/binary-amd64/Packages
├── dists/stable/main/binary-amd64/Packages.gz
├── dists/edge/...
├── auren-transfer-agent.gpg                # when signed
├── auren-transfer-agent.asc                # when signed
├── install-apt.sh
├── install.sh
├── install-command-template.json
├── SHA256SUMS
├── CHANNELS
├── SIGNED
└── VERSION
```

## Build a signed repository

Create or choose a GPG key on the release machine, then run:

```bash
APT_SIGN=true \
APT_REQUIRE_SIGNED=true \
APT_GPG_KEY_ID="YOUR_GPG_KEY_ID" \
APT_CHANNELS="stable,edge" \
APT_PUBLIC_REPO_URL="https://downloads.seudominio.com/agent/apt" \
APT_PUBLIC_KEY_URL="https://downloads.seudominio.com/agent/apt/auren-transfer-agent.gpg" \
./scripts/release.sh v1.9.0
```

For a lab repository only, signing can be skipped:

```bash
APT_SIGN=false ./scripts/release.sh v1.9.0
```

Production should use signed repositories.

## Publish to S3 + CloudFront

```bash
S3_URI=s3://seu-bucket/agent/apt \
CLOUDFRONT_DISTRIBUTION_ID=E1234567890 \
./scripts/publish-apt-s3.sh
```

The published URL should expose the repo root:

```text
https://downloads.seudominio.com/agent/apt
```

## Install from EC2/Ubuntu

Signed production install:

```bash
curl -fsSL https://downloads.seudominio.com/agent/apt/install-apt.sh | sudo bash -s -- \
  --repo-url https://downloads.seudominio.com/agent/apt \
  --apt-key-url https://downloads.seudominio.com/agent/apt/auren-transfer-agent.gpg \
  --media-hub https://media.seudominio.com \
  --token TOKEN_GERADO_NO_MEDIA_HUB \
  --role worker \
  --region sa-east-1
```

Gateway/hybrid install:

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

Lab unsigned install:

```bash
curl -fsSL https://downloads.seudominio.com/agent/apt/install-apt.sh | sudo bash -s -- \
  --allow-unsigned \
  --media-hub https://media.seudominio.com \
  --token TOKEN_GERADO_NO_MEDIA_HUB
```

## Raw apt commands

```bash
curl -fsSL https://downloads.seudominio.com/agent/apt/auren-transfer-agent.gpg | \
  sudo gpg --dearmor -o /usr/share/keyrings/auren-transfer-agent.gpg

echo "deb [signed-by=/usr/share/keyrings/auren-transfer-agent.gpg] https://downloads.seudominio.com/agent/apt stable main" | \
  sudo tee /etc/apt/sources.list.d/auren-transfer-agent.list

sudo apt update
sudo apt install auren-transfer-agent
```

Then register the node:

```bash
sudo auren-transfer-agent bootstrap \
  --media-hub https://media.seudominio.com \
  --token TOKEN_GERADO_NO_MEDIA_HUB \
  --role worker \
  --region sa-east-1 \
  --start-service
```

## Media Hub install command integration

The repo includes `install-command-template.json` so Auren Media Hub can render copy/paste commands in Provider Nodes. The template expects these variables:

```text
{{media_hub_url}}
{{registration_token}}
{{role}}
{{region}}
```

Operators can generate a command locally too:

```bash
MEDIA_HUB=https://media.seudominio.com \
TOKEN=auren-node-xxxx \
REPO_URL=https://downloads.seudominio.com/agent/apt \
KEY_URL=https://downloads.seudominio.com/agent/apt/auren-transfer-agent.gpg \
./scripts/generate-install-command.sh
```

## Channels

`APT_CHANNELS="stable,edge"` generates both channels with the same package set. In production, publish can be split by release branch: stable for approved releases and edge for canary/test nodes.

## Verification

```bash
apt-cache policy auren-transfer-agent
auren-transfer-agent --version
auren-transfer-agent doctor --config /etc/auren-transfer-agent/agent.yaml
systemctl status auren-transfer-agent
journalctl -u auren-transfer-agent -f
```
