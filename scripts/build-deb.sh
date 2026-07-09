#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="auren-transfer-agent"
VERSION="${1:-v1.7.0}"
ARCH="${DEB_ARCH:-amd64}"
DIST_DIR="${DIST_DIR:-dist}"
BINARY="${BINARY:-bin/${APP_NAME}}"
PACKAGE_VERSION="${VERSION#v}"
ROOT="${DIST_DIR}/deb/${APP_NAME}_${PACKAGE_VERSION}_${ARCH}"
DEB_PATH="${DIST_DIR}/${APP_NAME}_${PACKAGE_VERSION}_${ARCH}.deb"

if [[ ! "${VERSION}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "version must look like v1.7.0" >&2
  exit 1
fi
if [[ ! -x "${BINARY}" ]]; then
  echo "binary not found or not executable: ${BINARY}" >&2
  echo "build first: CGO_ENABLED=0 go build -trimpath -o bin/${APP_NAME} ./cmd/agent" >&2
  exit 1
fi
if ! command -v dpkg-deb >/dev/null 2>&1; then
  echo "dpkg-deb is required to build Debian package" >&2
  exit 1
fi

rm -rf "${ROOT}" "${DEB_PATH}"
mkdir -p \
  "${ROOT}/DEBIAN" \
  "${ROOT}/usr/bin" \
  "${ROOT}/lib/systemd/system" \
  "${ROOT}/etc/${APP_NAME}" \
  "${ROOT}/usr/share/doc/${APP_NAME}" \
  "${ROOT}/usr/share/${APP_NAME}/scripts"

install -m 0755 "${BINARY}" "${ROOT}/usr/bin/${APP_NAME}"
install -m 0644 "deploy/systemd/${APP_NAME}.service" "${ROOT}/lib/systemd/system/${APP_NAME}.service"
install -m 0640 "configs/agent.yaml" "${ROOT}/etc/${APP_NAME}/agent.yaml"
install -m 0640 "deploy/systemd/${APP_NAME}.env.example" "${ROOT}/etc/${APP_NAME}/${APP_NAME}.env"
install -m 0644 "README.md" "CHANGELOG.md" "ROADMAP.md" "${ROOT}/usr/share/doc/${APP_NAME}/"
install -m 0644 "docs/deployment/linux-package-bootstrap.md" "docs/deployment/production.md" "docs/deployment/apt-repository.md" "${ROOT}/usr/share/doc/${APP_NAME}/"
install -m 0755 "deploy/linux/install.sh" "${ROOT}/usr/share/${APP_NAME}/scripts/install.sh"
install -m 0755 "scripts/build-apt-repo.sh" "scripts/publish-apt-s3.sh" "${ROOT}/usr/share/${APP_NAME}/scripts/"

cp deploy/debian/DEBIAN/postinst "${ROOT}/DEBIAN/postinst"
cp deploy/debian/DEBIAN/prerm "${ROOT}/DEBIAN/prerm"
cp deploy/debian/DEBIAN/postrm "${ROOT}/DEBIAN/postrm"
cp deploy/debian/DEBIAN/conffiles "${ROOT}/DEBIAN/conffiles"
chmod 0755 "${ROOT}/DEBIAN/postinst" "${ROOT}/DEBIAN/prerm" "${ROOT}/DEBIAN/postrm"

cat > "${ROOT}/DEBIAN/control" <<CONTROL
Package: ${APP_NAME}
Version: ${PACKAGE_VERSION}
Section: net
Priority: optional
Architecture: ${ARCH}
Maintainer: Auren <ops@auren.app>
Depends: ca-certificates, adduser, systemd
Homepage: https://auren.app
Description: Auren Transfer Agent
 Execution/data-plane agent for Auren Media Hub. Provides transfer execution,
 Auren Storage uploads, Media Hub heartbeat/callbacks and optional gateway runtime.
CONTROL

find "${ROOT}" -type d -exec chmod 0755 {} +
dpkg-deb --build --root-owner-group "${ROOT}" "${DEB_PATH}" >/dev/null
sha256sum "${DEB_PATH}" > "${DEB_PATH}.sha256"
echo "Created ${DEB_PATH}"
