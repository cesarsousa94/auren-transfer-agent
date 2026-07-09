#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="auren-transfer-agent"
REPO_DIR="${1:-dist/apt}"
DEB_GLOB="${2:-dist/*.deb}"
CODENAME="${APT_CODENAME:-stable}"
SUITE="${APT_SUITE:-${CODENAME}}"
COMPONENT="${APT_COMPONENT:-main}"
ARCH="${APT_ARCH:-${DEB_ARCH:-amd64}}"
ORIGIN="${APT_ORIGIN:-Auren}"
LABEL="${APT_LABEL:-Auren Transfer Agent}"
DESCRIPTION="${APT_DESCRIPTION:-Auren Transfer Agent Debian repository}"
SIGN_MODE="${APT_SIGN:-auto}"
GPG_KEY_ID="${APT_GPG_KEY_ID:-}"

if ! command -v dpkg-scanpackages >/dev/null 2>&1; then
  echo "dpkg-scanpackages not found; install dpkg-dev to build Packages index" >&2
  exit 1
fi

rm -rf "${REPO_DIR}"
mkdir -p \
  "${REPO_DIR}/pool/${COMPONENT}/a/${APP_NAME}" \
  "${REPO_DIR}/dists/${CODENAME}/${COMPONENT}/binary-${ARCH}"
REPO_DIR="$(cd "${REPO_DIR}" && pwd)"

shopt -s nullglob
DEBS=( ${DEB_GLOB} )
if [[ ${#DEBS[@]} -eq 0 ]]; then
  echo "no .deb files matched: ${DEB_GLOB}" >&2
  exit 1
fi
for deb in "${DEBS[@]}"; do
  cp "${deb}" "${REPO_DIR}/pool/${COMPONENT}/a/${APP_NAME}/"
done

(
  cd "${REPO_DIR}"
  dpkg-scanpackages --arch "${ARCH}" "pool/${COMPONENT}" /dev/null > "dists/${CODENAME}/${COMPONENT}/binary-${ARCH}/Packages"
  gzip -9c "dists/${CODENAME}/${COMPONENT}/binary-${ARCH}/Packages" > "dists/${CODENAME}/${COMPONENT}/binary-${ARCH}/Packages.gz"
)

RELEASE_PATH="${REPO_DIR}/dists/${CODENAME}/Release"
DATE_RFC2822="$(LC_ALL=C date -Ru)"
cat > "${RELEASE_PATH}" <<RELEASE
Origin: ${ORIGIN}
Label: ${LABEL}
Suite: ${SUITE}
Codename: ${CODENAME}
Date: ${DATE_RFC2822}
Architectures: ${ARCH}
Components: ${COMPONENT}
Description: ${DESCRIPTION}
RELEASE

append_checksums() {
  local algo="$1"
  local command_name="$2"
  echo "${algo}:" >> "${RELEASE_PATH}"
  (
    cd "${REPO_DIR}/dists/${CODENAME}"
    find . -type f ! -name Release ! -name InRelease ! -name Release.gpg | sort | while read -r file; do
      local clean_file="${file#./}"
      local size
      size="$(wc -c < "${file}" | tr -d ' ')"
      local digest
      digest="$(${command_name} "${file}" | awk '{print $1}')"
      printf ' %s %16s %s\n' "${digest}" "${size}" "${clean_file}" >> "${RELEASE_PATH}"
    done
  )
}

append_checksums "MD5Sum" "md5sum"
append_checksums "SHA1" "sha1sum"
append_checksums "SHA256" "sha256sum"
append_checksums "SHA512" "sha512sum"

(
  cd "${REPO_DIR}"
  find pool dists -type f | sort | xargs sha256sum > SHA256SUMS
  printf '%s\n' "${APP_NAME}" > PACKAGE_NAME
  printf '%s\n' "${CODENAME}" > CHANNEL
)

SIGNED="false"
if [[ "${SIGN_MODE}" != "false" ]]; then
  if command -v gpg >/dev/null 2>&1 && [[ -n "${GPG_KEY_ID}" ]]; then
    gpg --batch --yes --local-user "${GPG_KEY_ID}" --armor --detach-sign -o "${RELEASE_PATH}.gpg" "${RELEASE_PATH}"
    gpg --batch --yes --local-user "${GPG_KEY_ID}" --clearsign -o "${REPO_DIR}/dists/${CODENAME}/InRelease" "${RELEASE_PATH}"
    SIGNED="true"
  elif [[ "${SIGN_MODE}" == "true" ]]; then
    echo "APT_SIGN=true requires gpg and APT_GPG_KEY_ID" >&2
    exit 1
  fi
fi

cat > "${REPO_DIR}/install-apt.sh" <<INSTALL
#!/usr/bin/env bash
set -Eeuo pipefail
REPO_URL="\${REPO_URL:-https://auren-storage-bucket.s3.us-east-2.amazonaws.com/agent/apt}"
CODENAME="\${CODENAME:-${CODENAME}}"
COMPONENT="\${COMPONENT:-${COMPONENT}}"
KEY_URL="\${KEY_URL:-}"
APP_NAME="${APP_NAME}"
if [[ "\${EUID}" -ne 0 ]]; then
  echo "run as root" >&2
  exit 1
fi
apt-get update -y || true
apt-get install -y ca-certificates curl gnupg
if [[ -n "\${KEY_URL}" ]]; then
  curl -fsSL "\${KEY_URL}" | gpg --dearmor -o /usr/share/keyrings/auren-transfer-agent.gpg
  echo "deb [signed-by=/usr/share/keyrings/auren-transfer-agent.gpg] \${REPO_URL} \${CODENAME} \${COMPONENT}" > /etc/apt/sources.list.d/auren-transfer-agent.list
else
  echo "WARN: KEY_URL not provided; using trusted=yes for lab installs" >&2
  echo "deb [trusted=yes] \${REPO_URL} \${CODENAME} \${COMPONENT}" > /etc/apt/sources.list.d/auren-transfer-agent.list
fi
apt-get update -y
apt-get install -y "\${APP_NAME}"
INSTALL
chmod 0755 "${REPO_DIR}/install-apt.sh"

echo "APT repository prepared at ${REPO_DIR}"
echo "codename=${CODENAME} component=${COMPONENT} arch=${ARCH} signed=${SIGNED}"
echo "publish root URL should point to: ${REPO_DIR}"
