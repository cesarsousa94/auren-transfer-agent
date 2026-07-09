#!/usr/bin/env bash
set -Eeuo pipefail
REPO_URL="${REPO_URL:-https://auren-storage-bucket.s3.us-east-2.amazonaws.com/agent/apt}"
CODENAME="${CODENAME:-stable}"
COMPONENT="${COMPONENT:-main}"
KEY_URL="${KEY_URL:-}"
APP_NAME="auren-transfer-agent"
if [[ "${EUID}" -ne 0 ]]; then
  echo "run as root" >&2
  exit 1
fi
apt-get update -y || true
apt-get install -y ca-certificates curl gnupg
if [[ -n "${KEY_URL}" ]]; then
  curl -fsSL "${KEY_URL}" | gpg --dearmor -o /usr/share/keyrings/auren-transfer-agent.gpg
  echo "deb [signed-by=/usr/share/keyrings/auren-transfer-agent.gpg] ${REPO_URL} ${CODENAME} ${COMPONENT}" > /etc/apt/sources.list.d/auren-transfer-agent.list
else
  echo "WARN: KEY_URL not provided; using trusted=yes for lab installs" >&2
  echo "deb [trusted=yes] ${REPO_URL} ${CODENAME} ${COMPONENT}" > /etc/apt/sources.list.d/auren-transfer-agent.list
fi
apt-get update -y
apt-get install -y "${APP_NAME}"
