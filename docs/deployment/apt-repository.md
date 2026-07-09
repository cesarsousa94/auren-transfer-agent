# APT Repository Distribution — v1.7.0

Auren Transfer Agent v1.7.0 adds a publishable Debian/Ubuntu APT repository layout. The repository is static, so it can be hosted by S3 + CloudFront, Nginx, Apache, Cloudflare R2, Bunny Storage or any HTTP origin that preserves paths.

## Build the package and repository

```bash
make release VERSION=v1.7.0
```

This generates:

```text
dist/auren-transfer-agent_1.7.0_amd64.deb
dist/apt/
dist/auren-transfer-agent-apt-repo-v1.7.0.tar.gz
```

The APT repository root is `dist/apt`:

```text
dist/apt/
├── dists/stable/Release
├── dists/stable/main/binary-amd64/Packages
├── dists/stable/main/binary-amd64/Packages.gz
├── pool/main/a/auren-transfer-agent/auren-transfer-agent_1.7.0_amd64.deb
├── SHA256SUMS
└── install-apt.sh
```

## Unsigned lab repository

For a private lab or temporary EC2 test, publish `dist/apt` and install with `trusted=yes`:

```bash
curl -fsSL https://downloads.example.com/agent/apt/install-apt.sh | sudo \
  REPO_URL=https://downloads.example.com/agent/apt \
  bash
```

Or manually:

```bash
echo 'deb [trusted=yes] https://downloads.example.com/agent/apt stable main' | \
  sudo tee /etc/apt/sources.list.d/auren-transfer-agent.list
sudo apt update
sudo apt install auren-transfer-agent
```

Use this only for controlled tests. Production repositories should be signed.

## Signed production repository

Import or create a GPG signing key on the release machine. Do not commit or ship the private key.

```bash
export APT_GPG_KEY_ID='YOUR_KEY_ID_OR_FINGERPRINT'
export APT_SIGN=true
./scripts/build-apt-repo.sh dist/apt 'dist/*.deb'
```

The script creates:

```text
dists/stable/InRelease
dists/stable/Release.gpg
```

Publish your public key separately, for example:

```text
https://downloads.example.com/agent/auren-transfer-agent.gpg
```

Then install on EC2/Ubuntu/Debian:

```bash
curl -fsSL https://downloads.example.com/agent/auren-transfer-agent.gpg | \
  sudo gpg --dearmor -o /usr/share/keyrings/auren-transfer-agent.gpg

echo 'deb [signed-by=/usr/share/keyrings/auren-transfer-agent.gpg] https://downloads.example.com/agent/apt stable main' | \
  sudo tee /etc/apt/sources.list.d/auren-transfer-agent.list

sudo apt update
sudo apt install auren-transfer-agent
```

## Publish to AWS S3 + CloudFront

Build the repository:

```bash
./scripts/release.sh v1.7.0
```

Publish the static repository:

```bash
S3_URI=s3://your-download-bucket/agent/apt \
CLOUDFRONT_DISTRIBUTION_ID=E1234567890 \
./scripts/publish-apt-s3.sh
```

For a dry-run:

```bash
DRY_RUN=true S3_URI=s3://your-download-bucket/agent/apt ./scripts/publish-apt-s3.sh
```

Expected public URL:

```text
https://downloads.example.com/agent/apt
```

## One-line install + bootstrap

After publishing, a test EC2 can be installed and registered like this:

```bash
curl -fsSL https://downloads.example.com/agent/install.sh | sudo bash -s -- \
  --apt \
  --repo-url https://downloads.example.com/agent/apt \
  --apt-key-url https://downloads.example.com/agent/auren-transfer-agent.gpg \
  --media-hub https://media.example.com \
  --token REGISTRATION_TOKEN \
  --role worker \
  --region sa-east-1
```

For temporary unsigned lab repositories, omit `--apt-key-url`. The installer will use `trusted=yes` and print a warning.

## Post-install commands

```bash
sudo systemctl status auren-transfer-agent
journalctl -u auren-transfer-agent -f
auren-transfer-agent status --config /etc/auren-transfer-agent/agent.yaml
auren-transfer-agent doctor --config /etc/auren-transfer-agent/agent.yaml --online
```

## Upgrade

Once the repository is configured:

```bash
sudo apt update
sudo apt install --only-upgrade auren-transfer-agent
sudo systemctl restart auren-transfer-agent
```

The package preserves `/etc/auren-transfer-agent/agent.yaml` as a conffile and keeps durable identity under `/var/lib/auren-transfer-agent`.

## Production notes

- The Agent uses outbound HTTPS to Media Hub for registration, heartbeat, claim, callbacks and metrics.
- Transfer-only nodes do not need a public inbound URL.
- Gateway nodes need `public_base_url`, TLS termination and inbound 80/443.
- Never publish the GPG private key.
- Prefer signed repositories for any customer or production environment.
