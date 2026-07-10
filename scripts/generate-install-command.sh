#!/usr/bin/env bash
# Auren Transfer Agent v1.9.1 distribution helper
set -Eeuo pipefail

REPO_URL="${REPO_URL:-https://auren-storage-bucket.s3.us-east-2.amazonaws.com/agent/apt}"
INSTALL_URL="${INSTALL_URL:-${REPO_URL%/}/install-apt.sh}"
KEY_URL="${KEY_URL:-${APT_KEY_URL:-${REPO_URL%/}/auren-transfer-agent.gpg}}"
MEDIA_HUB="${MEDIA_HUB:-}"
TOKEN="${TOKEN:-}"
ROLE="${ROLE:-worker}"
REGION="${REGION:-sa-east-1}"
CODENAME="${CODENAME:-${APT_CODENAME:-stable}}"
COMPONENT="${COMPONENT:-${APT_COMPONENT:-main}}"
PUBLIC_BASE_URL="${PUBLIC_BASE_URL:-}"
ENABLE_GATEWAY="${ENABLE_GATEWAY:-false}"
START_SERVICE="${START_SERVICE:-true}"
SIGNED="${SIGNED:-true}"

usage() {
  cat <<USAGE
Usage:
  MEDIA_HUB=https://media.example.com TOKEN=auren-node-... ./scripts/generate-install-command.sh

Environment:
  REPO_URL=https://auren-storage-bucket.s3.us-east-2.amazonaws.com/agent/apt
  INSTALL_URL=https://auren-storage-bucket.s3.us-east-2.amazonaws.com/agent/apt/install-apt.sh
  KEY_URL=https://auren-storage-bucket.s3.us-east-2.amazonaws.com/agent/apt/auren-transfer-agent.gpg
  MEDIA_HUB=https://media.example.com       required for ready-to-run bootstrap
  TOKEN=auren-node-...                      optional; use placeholder if omitted
  ROLE=worker|gateway|hybrid                default worker
  REGION=sa-east-1                          default sa-east-1
  CODENAME=stable                           default stable
  COMPONENT=main                            default main
  ENABLE_GATEWAY=true                       append --enable-gateway
  PUBLIC_BASE_URL=https://node.example.com  required when ENABLE_GATEWAY=true
  SIGNED=true|false                         include --apt-key-url when true
USAGE
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi
if [[ -z "${MEDIA_HUB}" ]]; then
  MEDIA_HUB="https://media.example.com"
fi
if [[ -z "${TOKEN}" ]]; then
  TOKEN="TOKEN_GERADO_NO_MEDIA_HUB"
fi

args=(--apt --repo-url "${REPO_URL}" --codename "${CODENAME}" --component "${COMPONENT}")
if [[ "${SIGNED}" == "true" ]]; then
  args+=(--apt-key-url "${KEY_URL}")
fi
args+=(--media-hub "${MEDIA_HUB}" --token "${TOKEN}" --role "${ROLE}" --region "${REGION}")
if [[ "${ENABLE_GATEWAY}" == "true" ]]; then
  args+=(--enable-gateway --public-base-url "${PUBLIC_BASE_URL:-https://node.example.com}")
fi
if [[ "${START_SERVICE}" != "true" ]]; then
  args+=(--no-start)
fi

printf 'curl -fsSL %q | sudo bash -s --' "${INSTALL_URL}"
printf ' %q' "${args[@]}"
printf '\n'
