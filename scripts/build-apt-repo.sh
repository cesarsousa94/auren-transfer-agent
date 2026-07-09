#!/usr/bin/env bash
set -Eeuo pipefail

REPO_DIR="${1:-dist/apt}"
DEB_GLOB="${2:-dist/*.deb}"
CODENAME="${CODENAME:-stable}"
COMPONENT="${COMPONENT:-main}"
ARCH="${DEB_ARCH:-amd64}"

mkdir -p "${REPO_DIR}/pool/${COMPONENT}" "${REPO_DIR}/dists/${CODENAME}/${COMPONENT}/binary-${ARCH}"
shopt -s nullglob
for deb in ${DEB_GLOB}; do
  cp "${deb}" "${REPO_DIR}/pool/${COMPONENT}/"
done
if ! command -v dpkg-scanpackages >/dev/null 2>&1; then
  echo "dpkg-scanpackages not found; install dpkg-dev to build Packages index" >&2
  exit 1
fi
(
  cd "${REPO_DIR}"
  dpkg-scanpackages "pool/${COMPONENT}" /dev/null | gzip -9c > "dists/${CODENAME}/${COMPONENT}/binary-${ARCH}/Packages.gz"
)
echo "APT repository prepared at ${REPO_DIR}"
