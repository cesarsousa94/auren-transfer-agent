#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="auren-transfer-agent"
REPO_DIR="${1:-dist/apt}"
DEB_GLOB="${2:-dist/*.deb}"
CODENAME="${APT_CODENAME:-stable}"
CHANNELS_RAW="${APT_CHANNELS:-${CODENAME}}"
COMPONENT="${APT_COMPONENT:-main}"
ARCH="${APT_ARCH:-${DEB_ARCH:-amd64}}"
ORIGIN="${APT_ORIGIN:-Auren}"
LABEL="${APT_LABEL:-Auren Transfer Agent}"
DESCRIPTION="${APT_DESCRIPTION:-Auren Transfer Agent Debian repository}"
SIGN_MODE="${APT_SIGN:-auto}"
REQUIRE_SIGNED="${APT_REQUIRE_SIGNED:-false}"
GPG_KEY_ID="${APT_GPG_KEY_ID:-}"
PUBLIC_REPO_URL="${APT_PUBLIC_REPO_URL:-https://downloads.auren.app/agent/apt}"
PUBLIC_KEY_URL="${APT_PUBLIC_KEY_URL:-${PUBLIC_REPO_URL%/}/auren-transfer-agent.gpg}"
VERSION="${VERSION:-}"

usage() {
  cat <<USAGE
Usage:
  ./scripts/build-apt-repo.sh [repo_dir] [deb_glob]

Environment:
  APT_CHANNELS=stable,edge                 channels/codenames to generate
  APT_COMPONENT=main                       component
  APT_ARCH=amd64                           architecture
  APT_SIGN=true|false|auto                 sign Release files
  APT_REQUIRE_SIGNED=true                  fail when signing is unavailable
  APT_GPG_KEY_ID=KEYID                     GPG key used to sign Release
  APT_PUBLIC_REPO_URL=https://.../apt      used in generated install command template
  APT_PUBLIC_KEY_URL=https://.../key.gpg   used in generated install command template
USAGE
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi
if ! command -v dpkg-scanpackages >/dev/null 2>&1; then
  echo "dpkg-scanpackages not found; install dpkg-dev to build Packages index" >&2
  exit 1
fi

IFS=',' read -r -a CHANNELS <<< "${CHANNELS_RAW}"
if [[ ${#CHANNELS[@]} -eq 0 ]]; then
  CHANNELS=(stable)
fi

rm -rf "${REPO_DIR}"
mkdir -p "${REPO_DIR}/pool/${COMPONENT}/a/${APP_NAME}"
REPO_DIR="$(mkdir -p "${REPO_DIR}" && cd "${REPO_DIR}" && pwd)"

shopt -s nullglob
DEBS=( ${DEB_GLOB} )
if [[ ${#DEBS[@]} -eq 0 ]]; then
  echo "no .deb files matched: ${DEB_GLOB}" >&2
  exit 1
fi
for deb in "${DEBS[@]}"; do
  cp "${deb}" "${REPO_DIR}/pool/${COMPONENT}/a/${APP_NAME}/"
  if [[ -z "${VERSION}" ]]; then
    base="$(basename "${deb}")"
    VERSION="${base#${APP_NAME}_}"
    VERSION="${VERSION%%_*}"
  fi
done

append_checksums() {
  local release_path="$1"
  local channel="$2"
  local algo="$3"
  local command_name="$4"
  echo "${algo}:" >> "${release_path}"
  (
    cd "${REPO_DIR}/dists/${channel}"
    find . -type f ! -name Release ! -name InRelease ! -name Release.gpg | sort | while read -r file; do
      local clean_file="${file#./}"
      local size
      size="$(wc -c < "${file}" | tr -d ' ')"
      local digest
      digest="$(${command_name} "${file}" | awk '{print $1}')"
      printf ' %s %16s %s\n' "${digest}" "${size}" "${clean_file}" >> "${release_path}"
    done
  )
}

SIGNED="false"
SIGN_AVAILABLE="false"
if command -v gpg >/dev/null 2>&1 && [[ -n "${GPG_KEY_ID}" ]]; then
  SIGN_AVAILABLE="true"
fi
if [[ "${SIGN_MODE}" == "true" || "${REQUIRE_SIGNED}" == "true" ]]; then
  if [[ "${SIGN_AVAILABLE}" != "true" ]]; then
    echo "APT signing requires gpg and APT_GPG_KEY_ID" >&2
    exit 1
  fi
fi

for channel in "${CHANNELS[@]}"; do
  channel="$(echo "${channel}" | xargs)"
  [[ -n "${channel}" ]] || continue
  mkdir -p "${REPO_DIR}/dists/${channel}/${COMPONENT}/binary-${ARCH}"
  (
    cd "${REPO_DIR}"
    dpkg-scanpackages --arch "${ARCH}" "pool/${COMPONENT}" /dev/null > "dists/${channel}/${COMPONENT}/binary-${ARCH}/Packages"
    gzip -9c "dists/${channel}/${COMPONENT}/binary-${ARCH}/Packages" > "dists/${channel}/${COMPONENT}/binary-${ARCH}/Packages.gz"
  )

  RELEASE_PATH="${REPO_DIR}/dists/${channel}/Release"
  DATE_RFC2822="$(LC_ALL=C date -Ru)"
  cat > "${RELEASE_PATH}" <<RELEASE
Origin: ${ORIGIN}
Label: ${LABEL}
Suite: ${channel}
Codename: ${channel}
Date: ${DATE_RFC2822}
Architectures: ${ARCH}
Components: ${COMPONENT}
Description: ${DESCRIPTION}
RELEASE

  append_checksums "${RELEASE_PATH}" "${channel}" "MD5Sum" "md5sum"
  append_checksums "${RELEASE_PATH}" "${channel}" "SHA1" "sha1sum"
  append_checksums "${RELEASE_PATH}" "${channel}" "SHA256" "sha256sum"
  append_checksums "${RELEASE_PATH}" "${channel}" "SHA512" "sha512sum"

  if [[ "${SIGN_MODE}" != "false" && "${SIGN_AVAILABLE}" == "true" ]]; then
    gpg --batch --yes --local-user "${GPG_KEY_ID}" --armor --detach-sign -o "${RELEASE_PATH}.gpg" "${RELEASE_PATH}"
    gpg --batch --yes --local-user "${GPG_KEY_ID}" --clearsign -o "${REPO_DIR}/dists/${channel}/InRelease" "${RELEASE_PATH}"
    SIGNED="true"
  fi
done

(
  cd "${REPO_DIR}"
  find pool dists -type f | sort | xargs sha256sum > SHA256SUMS
  printf '%s\n' "${APP_NAME}" > PACKAGE_NAME
  printf '%s\n' "${VERSION:-unknown}" > VERSION
  printf '%s\n' "${CHANNELS_RAW}" > CHANNELS
  printf '%s\n' "${SIGNED}" > SIGNED
)

if [[ "${SIGNED}" == "true" ]]; then
  mkdir -p "${REPO_DIR}"
  gpg --batch --yes --export "${GPG_KEY_ID}" > "${REPO_DIR}/${APP_NAME}.gpg"
  gpg --batch --yes --armor --export "${GPG_KEY_ID}" > "${REPO_DIR}/${APP_NAME}.asc"
fi

cat > "${REPO_DIR}/install-apt.sh" <<INSTALL
#!/usr/bin/env bash
set -Eeuo pipefail
REPO_URL="\${REPO_URL:-${PUBLIC_REPO_URL}}"
CODENAME="\${CODENAME:-${CHANNELS[0]}}"
COMPONENT="\${COMPONENT:-${COMPONENT}}"
KEY_URL="\${KEY_URL:-${PUBLIC_KEY_URL}}"
REQUIRE_SIGNED="\${REQUIRE_SIGNED:-${SIGNED}}"
APP_NAME="${APP_NAME}"
MEDIA_HUB=""
TOKEN=""
ROLE="worker"
REGION="sa-east-1"
PUBLIC_BASE_URL=""
ENABLE_GATEWAY="false"
START_SERVICE="true"
MAX_CONCURRENT_JOBS="2"
SKIP_REGISTER="false"
while [[ \$# -gt 0 ]]; do
  case "\$1" in
    --repo-url) REPO_URL="\${2:-}"; shift 2 ;;
    --apt-key-url|--key-url) KEY_URL="\${2:-}"; shift 2 ;;
    --codename|--channel) CODENAME="\${2:-}"; shift 2 ;;
    --component) COMPONENT="\${2:-}"; shift 2 ;;
    --media-hub) MEDIA_HUB="\${2:-}"; shift 2 ;;
    --token) TOKEN="\${2:-}"; shift 2 ;;
    --role) ROLE="\${2:-}"; shift 2 ;;
    --region) REGION="\${2:-}"; shift 2 ;;
    --enable-gateway) ENABLE_GATEWAY="true"; shift ;;
    --public-base-url) PUBLIC_BASE_URL="\${2:-}"; shift 2 ;;
    --max-concurrent-jobs) MAX_CONCURRENT_JOBS="\${2:-}"; shift 2 ;;
    --no-start) START_SERVICE="false"; shift ;;
    --skip-register) SKIP_REGISTER="true"; shift ;;
    --allow-unsigned) REQUIRE_SIGNED="false"; KEY_URL=""; shift ;;
    --help|-h)
      echo "Usage: curl -fsSL \${REPO_URL}/install-apt.sh | sudo bash -s -- --media-hub URL --token TOKEN [options]"; exit 0 ;;
    *) echo "Unknown option: \$1" >&2; exit 1 ;;
  esac
done
if [[ "\${EUID}" -ne 0 ]]; then echo "run as root" >&2; exit 1; fi
apt-get update -y || true
apt-get install -y ca-certificates curl gnupg
if [[ -n "\${KEY_URL}" && "\${REQUIRE_SIGNED}" == "true" ]]; then
  curl -fsSL "\${KEY_URL}" | gpg --dearmor -o /usr/share/keyrings/auren-transfer-agent.gpg
  echo "deb [signed-by=/usr/share/keyrings/auren-transfer-agent.gpg] \${REPO_URL} \${CODENAME} \${COMPONENT}" > /etc/apt/sources.list.d/auren-transfer-agent.list
else
  if [[ "\${REQUIRE_SIGNED}" == "true" ]]; then echo "signed install requires --apt-key-url/KEY_URL or --allow-unsigned" >&2; exit 1; fi
  echo "WARN: unsigned/trusted APT install enabled. Use only for lab/private repositories." >&2
  echo "deb [trusted=yes] \${REPO_URL} \${CODENAME} \${COMPONENT}" > /etc/apt/sources.list.d/auren-transfer-agent.list
fi
apt-get update -y
apt-get install -y "\${APP_NAME}"
if [[ -n "\${MEDIA_HUB}" || "\${SKIP_REGISTER}" == "true" ]]; then
  BOOTSTRAP=(/usr/bin/\${APP_NAME} bootstrap --media-hub "\${MEDIA_HUB}" --role "\${ROLE}" --region "\${REGION}" --max-concurrent-jobs "\${MAX_CONCURRENT_JOBS}")
  [[ -n "\${TOKEN}" ]] && BOOTSTRAP+=(--token "\${TOKEN}")
  [[ "\${ENABLE_GATEWAY}" == "true" ]] && BOOTSTRAP+=(--enable-gateway --public-base-url "\${PUBLIC_BASE_URL}")
  [[ "\${START_SERVICE}" == "true" ]] && BOOTSTRAP+=(--start-service)
  [[ "\${SKIP_REGISTER}" == "true" ]] && BOOTSTRAP+=(--skip-register)
  "\${BOOTSTRAP[@]}"
fi
INSTALL
chmod 0755 "${REPO_DIR}/install-apt.sh"
cp "${REPO_DIR}/install-apt.sh" "${REPO_DIR}/install.sh"

cat > "${REPO_DIR}/install-command-template.json" <<JSON
{
  "version": "${VERSION:-unknown}",
  "repository_url": "${PUBLIC_REPO_URL}",
  "key_url": "${PUBLIC_KEY_URL}",
  "default_channel": "${CHANNELS[0]}",
  "component": "${COMPONENT}",
  "signed": ${SIGNED},
  "command_template": "curl -fsSL ${PUBLIC_REPO_URL}/install-apt.sh | sudo bash -s -- --media-hub {{media_hub_url}} --token {{registration_token}} --role {{role}} --region {{region}} --start-service"
}
JSON

echo "APT repository prepared at ${REPO_DIR}"
echo "channels=${CHANNELS_RAW} component=${COMPONENT} arch=${ARCH} signed=${SIGNED}"
echo "public repo URL: ${PUBLIC_REPO_URL}"
