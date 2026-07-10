#!/usr/bin/env bash
set -Eeuo pipefail
REPO_URL="${REPO_URL:-https://auren-storage-bucket.s3.us-east-2.amazonaws.com/agent/apt}"
CODENAME="${CODENAME:-stable}"
COMPONENT="${COMPONENT:-main}"
KEY_URL="${KEY_URL:-https://auren-storage-bucket.s3.us-east-2.amazonaws.com/agent/apt/auren-transfer-agent.gpg}"
REQUIRE_SIGNED="${REQUIRE_SIGNED:-false}"
APP_NAME="auren-transfer-agent"
MEDIA_HUB=""
TOKEN=""
ROLE="worker"
REGION="sa-east-1"
PUBLIC_BASE_URL=""
ENABLE_GATEWAY="false"
START_SERVICE="true"
MAX_CONCURRENT_JOBS="2"
SKIP_REGISTER="false"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo-url) REPO_URL="${2:-}"; shift 2 ;;
    --apt-key-url|--key-url) KEY_URL="${2:-}"; shift 2 ;;
    --codename|--channel) CODENAME="${2:-}"; shift 2 ;;
    --component) COMPONENT="${2:-}"; shift 2 ;;
    --media-hub) MEDIA_HUB="${2:-}"; shift 2 ;;
    --token) TOKEN="${2:-}"; shift 2 ;;
    --role) ROLE="${2:-}"; shift 2 ;;
    --region) REGION="${2:-}"; shift 2 ;;
    --enable-gateway) ENABLE_GATEWAY="true"; shift ;;
    --public-base-url) PUBLIC_BASE_URL="${2:-}"; shift 2 ;;
    --max-concurrent-jobs) MAX_CONCURRENT_JOBS="${2:-}"; shift 2 ;;
    --no-start) START_SERVICE="false"; shift ;;
    --skip-register) SKIP_REGISTER="true"; shift ;;
    --allow-unsigned) REQUIRE_SIGNED="false"; KEY_URL=""; shift ;;
    --help|-h)
      echo "Usage: curl -fsSL ${REPO_URL}/install-apt.sh | sudo bash -s -- --media-hub URL --token TOKEN [options]"; exit 0 ;;
    *) echo "Unknown option: $1" >&2; exit 1 ;;
  esac
done
if [[ "${EUID}" -ne 0 ]]; then echo "run as root" >&2; exit 1; fi
apt-get update -y || true
apt-get install -y ca-certificates curl gnupg
if [[ -n "${KEY_URL}" && "${REQUIRE_SIGNED}" == "true" ]]; then
  curl -fsSL "${KEY_URL}" | gpg --dearmor -o /usr/share/keyrings/auren-transfer-agent.gpg
  echo "deb [signed-by=/usr/share/keyrings/auren-transfer-agent.gpg] ${REPO_URL} ${CODENAME} ${COMPONENT}" > /etc/apt/sources.list.d/auren-transfer-agent.list
else
  if [[ "${REQUIRE_SIGNED}" == "true" ]]; then echo "signed install requires --apt-key-url/KEY_URL or --allow-unsigned" >&2; exit 1; fi
  echo "WARN: unsigned/trusted APT install enabled. Use only for lab/private repositories." >&2
  echo "deb [trusted=yes] ${REPO_URL} ${CODENAME} ${COMPONENT}" > /etc/apt/sources.list.d/auren-transfer-agent.list
fi
apt-get update -y
apt-get install -y "${APP_NAME}"
if [[ -n "${MEDIA_HUB}" || "${SKIP_REGISTER}" == "true" ]]; then
  BOOTSTRAP=(/usr/bin/${APP_NAME} bootstrap --media-hub "${MEDIA_HUB}" --role "${ROLE}" --region "${REGION}" --max-concurrent-jobs "${MAX_CONCURRENT_JOBS}")
  [[ -n "${TOKEN}" ]] && BOOTSTRAP+=(--token "${TOKEN}")
  [[ "${ENABLE_GATEWAY}" == "true" ]] && BOOTSTRAP+=(--enable-gateway --public-base-url "${PUBLIC_BASE_URL}")
  [[ "${START_SERVICE}" == "true" ]] && BOOTSTRAP+=(--start-service)
  [[ "${SKIP_REGISTER}" == "true" ]] && BOOTSTRAP+=(--skip-register)
  "${BOOTSTRAP[@]}"
fi
