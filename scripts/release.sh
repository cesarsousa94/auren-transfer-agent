#!/usr/bin/env bash
# Current package baseline: v1.6.0
set -Eeuo pipefail

VERSION="${1:-}"
MODE="${2:-}"
APP_NAME="auren-transfer-agent"

if [[ -z "${VERSION}" ]]; then
  echo "usage: ./scripts/release.sh vX.Y.Z [--dry-run]" >&2
  exit 1
fi

if [[ ! "${VERSION}" =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "release version must look like v1.0.0" >&2
  exit 1
fi

ARCHIVE_DIR="dist/${APP_NAME}-${VERSION}"
ARCHIVE_PATH="dist/${APP_NAME}-${VERSION}.zip"
DEB_ARCH="${DEB_ARCH:-amd64}"
DEB_PATH="dist/${APP_NAME}_${VERSION#v}_${DEB_ARCH}.deb"

echo "Preparing ${APP_NAME} ${VERSION}"
if [[ "${MODE}" == "--dry-run" ]]; then
  echo "Dry run: would run go test, build binary, create ${ARCHIVE_PATH} and ${DEB_PATH}"
  exit 0
fi

go test ./...
mkdir -p bin dist
CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "bin/${APP_NAME}" ./cmd/agent
rm -rf "${ARCHIVE_DIR}" "${ARCHIVE_PATH}"
mkdir -p "${ARCHIVE_DIR}"
rsync -a --exclude='.git' --exclude='dist' --exclude='bin' ./ "${ARCHIVE_DIR}/"
mkdir -p "${ARCHIVE_DIR}/bin"
cp "bin/${APP_NAME}" "${ARCHIVE_DIR}/bin/${APP_NAME}"
./scripts/build-deb.sh "${VERSION}"
mkdir -p "${ARCHIVE_DIR}/dist"
cp "${DEB_PATH}" "${ARCHIVE_DIR}/dist/"
cp "${DEB_PATH}.sha256" "${ARCHIVE_DIR}/dist/"
(cd dist && zip -qr "${APP_NAME}-${VERSION}.zip" "${APP_NAME}-${VERSION}")
sha256sum "${ARCHIVE_PATH}" > "${ARCHIVE_PATH}.sha256"
echo "Created ${ARCHIVE_PATH}"
echo "Created ${DEB_PATH}"
