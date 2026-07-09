#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="auren-transfer-agent"
VERSION="${VERSION:-v1.7.0}"
DOWNLOAD_BASE="${DOWNLOAD_BASE:-https://auren-storage-bucket.s3.us-east-2.amazonaws.com/agent}"
APT_REPO_URL="${APT_REPO_URL:-https://auren-storage-bucket.s3.us-east-2.amazonaws.com/agent/apt}"
APT_CODENAME="${APT_CODENAME:-stable}"
APT_COMPONENT="${APT_COMPONENT:-main}"
APT_KEY_URL="${APT_KEY_URL:-}"
INSTALL_METHOD="auto"
DEB_PATH=""
MEDIA_HUB=""
TOKEN=""
ROLE="hybrid"
REGION="sa-east-1"
PUBLIC_BASE_URL=""
ENABLE_GATEWAY="false"
MAX_CONCURRENT_JOBS="2"
START_SERVICE="true"
SKIP_REGISTER="false"

usage() {
  cat <<USAGE
Usage:
  curl -fsSL ${DOWNLOAD_BASE}/install.sh | sudo bash -s -- --media-hub URL --token TOKEN [options]

Install source:
  --apt                         install from APT repository
  --repo-url URL                APT repository root URL, default: ${APT_REPO_URL}
  --apt-key-url URL             public GPG key URL for signed repository
  --codename NAME               APT codename/channel, default: ${APT_CODENAME}
  --component NAME              APT component, default: ${APT_COMPONENT}
  --deb PATH                    install a local .deb instead of downloading
  --version v1.7.0              release version to download in direct .deb mode

Bootstrap:
  --media-hub URL               Auren Media Hub URL
  --token TOKEN                 one-time node registration token
  --role worker|gateway|hybrid  node role
  --region REGION               node region
  --enable-gateway              enable public Gateway Runtime
  --public-base-url URL         public node URL for gateway mode
  --max-concurrent-jobs N       transfer concurrency
  --no-start                    do not start systemd service
  --skip-register               write config only, do not register immediately
USAGE
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --apt) INSTALL_METHOD="apt"; shift ;;
    --repo-url) APT_REPO_URL="${2:-}"; INSTALL_METHOD="apt"; shift 2 ;;
    --apt-key-url|--gpg-key-url) APT_KEY_URL="${2:-}"; shift 2 ;;
    --codename|--channel) APT_CODENAME="${2:-}"; shift 2 ;;
    --component) APT_COMPONENT="${2:-}"; shift 2 ;;
    --deb) DEB_PATH="${2:-}"; INSTALL_METHOD="deb"; shift 2 ;;
    --version) VERSION="${2:-}"; shift 2 ;;
    --media-hub) MEDIA_HUB="${2:-}"; shift 2 ;;
    --token) TOKEN="${2:-}"; shift 2 ;;
    --role) ROLE="${2:-}"; shift 2 ;;
    --region) REGION="${2:-}"; shift 2 ;;
    --public-base-url) PUBLIC_BASE_URL="${2:-}"; shift 2 ;;
    --enable-gateway) ENABLE_GATEWAY="true"; shift ;;
    --max-concurrent-jobs) MAX_CONCURRENT_JOBS="${2:-}"; shift 2 ;;
    --no-start) START_SERVICE="false"; shift ;;
    --skip-register) SKIP_REGISTER="true"; shift ;;
    --help|-h) usage; exit 0 ;;
    *) echo "Unknown option: $1" >&2; usage; exit 1 ;;
  esac
done

if [[ "${EUID}" -ne 0 ]]; then
  echo "This installer must run as root." >&2
  exit 1
fi

if [[ -z "${MEDIA_HUB}" ]]; then
  echo "--media-hub is required" >&2
  exit 1
fi
if [[ -z "${TOKEN}" && "${SKIP_REGISTER}" != "true" ]]; then
  echo "--token is required unless --skip-register is used" >&2
  exit 1
fi
if [[ "${ENABLE_GATEWAY}" == "true" && -z "${PUBLIC_BASE_URL}" ]]; then
  echo "--public-base-url is required with --enable-gateway" >&2
  exit 1
fi

install_prerequisites() {
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update -y || true
    apt-get install -y ca-certificates curl gnupg adduser systemd
  fi
}

install_from_apt() {
  install_prerequisites
  if [[ -n "${APT_KEY_URL}" ]]; then
    curl -fsSL "${APT_KEY_URL}" | gpg --dearmor -o /usr/share/keyrings/auren-transfer-agent.gpg
    echo "deb [signed-by=/usr/share/keyrings/auren-transfer-agent.gpg] ${APT_REPO_URL} ${APT_CODENAME} ${APT_COMPONENT}" > /etc/apt/sources.list.d/auren-transfer-agent.list
  else
    echo "WARN: --apt-key-url was not provided; using trusted=yes. Use only for lab/private repositories." >&2
    echo "deb [trusted=yes] ${APT_REPO_URL} ${APT_CODENAME} ${APT_COMPONENT}" > /etc/apt/sources.list.d/auren-transfer-agent.list
  fi
  apt-get update -y
  apt-get install -y "${APP_NAME}"
}

install_from_deb() {
  if [[ -z "${DEB_PATH}" ]]; then
    TMP_DIR="$(mktemp -d)"
    trap 'rm -rf "${TMP_DIR}"' EXIT
    ARCH="$(dpkg --print-architecture 2>/dev/null || echo amd64)"
    DEB_NAME="${APP_NAME}_${VERSION#v}_${ARCH}.deb"
    DEB_URL="${DOWNLOAD_BASE}/${VERSION}/${DEB_NAME}"
    DEB_PATH="${TMP_DIR}/${DEB_NAME}"
    echo "Downloading ${DEB_URL}"
    if command -v curl >/dev/null 2>&1; then
      curl -fL "${DEB_URL}" -o "${DEB_PATH}"
    elif command -v wget >/dev/null 2>&1; then
      wget -O "${DEB_PATH}" "${DEB_URL}"
    else
      echo "curl or wget is required" >&2
      exit 1
    fi
  fi
  install_prerequisites
  dpkg -i "${DEB_PATH}" || apt-get install -f -y
}

case "${INSTALL_METHOD}" in
  apt) install_from_apt ;;
  deb|auto) install_from_deb ;;
  *) echo "unsupported install method: ${INSTALL_METHOD}" >&2; exit 1 ;;
esac

BOOTSTRAP=(/usr/bin/${APP_NAME} bootstrap --media-hub "${MEDIA_HUB}" --role "${ROLE}" --region "${REGION}" --max-concurrent-jobs "${MAX_CONCURRENT_JOBS}")
if [[ -n "${TOKEN}" ]]; then
  BOOTSTRAP+=(--token "${TOKEN}")
fi
if [[ "${ENABLE_GATEWAY}" == "true" ]]; then
  BOOTSTRAP+=(--enable-gateway --public-base-url "${PUBLIC_BASE_URL}")
fi
if [[ "${START_SERVICE}" == "true" ]]; then
  BOOTSTRAP+=(--start-service)
fi
if [[ "${SKIP_REGISTER}" == "true" ]]; then
  BOOTSTRAP+=(--skip-register)
fi

"${BOOTSTRAP[@]}"
/usr/bin/${APP_NAME} status --config /etc/${APP_NAME}/agent.yaml || true
