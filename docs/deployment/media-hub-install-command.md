# Media Hub Provider Nodes Install Command — v1.13.1

Auren Transfer Agent v1.13.1 includes a repository-side `install-command-template.json` so Auren Media Hub can render a ready-to-copy Linux install command when an operator creates a Provider Node.

## Expected Media Hub flow

1. Operator opens `Provider Nodes > Novo Agent Linux`.
2. Media Hub creates or pre-provisions an `edge_node` and one-time registration token.
3. Media Hub renders a command using the repository metadata and the token.
4. Operator pastes the command into an EC2/Ubuntu host.
5. The host installs the Agent through APT, bootstraps the node and starts systemd.

## Template file

The APT repository publishes:

```text
https://downloads.seudominio.com/agent/apt/install-command-template.json
```

Example:

```json
{
  "version": "1.13.1",
  "repository_url": "https://downloads.seudominio.com/agent/apt",
  "key_url": "https://downloads.seudominio.com/agent/apt/auren-transfer-agent.gpg",
  "default_channel": "stable",
  "component": "main",
  "signed": true,
  "command_template": "curl -fsSL https://downloads.seudominio.com/agent/apt/install-apt.sh | sudo bash -s -- --media-hub {{media_hub_url}} --token {{registration_token}} --role {{role}} --region {{region}} --start-service"
}
```

## Worker command

```bash
curl -fsSL https://downloads.seudominio.com/agent/apt/install-apt.sh | sudo bash -s -- \
  --repo-url https://downloads.seudominio.com/agent/apt \
  --apt-key-url https://downloads.seudominio.com/agent/apt/auren-transfer-agent.gpg \
  --media-hub https://media.seudominio.com \
  --token auren-node-xxxx \
  --role worker \
  --region sa-east-1
```

## Hybrid gateway command

```bash
curl -fsSL https://downloads.seudominio.com/agent/apt/install-apt.sh | sudo bash -s -- \
  --repo-url https://downloads.seudominio.com/agent/apt \
  --apt-key-url https://downloads.seudominio.com/agent/apt/auren-transfer-agent.gpg \
  --media-hub https://media.seudominio.com \
  --token auren-node-xxxx \
  --role hybrid \
  --enable-gateway \
  --public-base-url https://node1.seudominio.com \
  --region sa-east-1
```

## Local command generation

```bash
MEDIA_HUB=https://media.seudominio.com \
TOKEN=auren-node-xxxx \
ROLE=worker \
REGION=sa-east-1 \
REPO_URL=https://downloads.seudominio.com/agent/apt \
KEY_URL=https://downloads.seudominio.com/agent/apt/auren-transfer-agent.gpg \
./scripts/generate-install-command.sh
```
