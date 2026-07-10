#!/usr/bin/env bash
# Auren Transfer Agent v1.9.1 distribution helper
set -Eeuo pipefail

KEY_ID="${APT_GPG_KEY_ID:-${1:-}}"
OUT="${2:-dist/apt/auren-transfer-agent.gpg}"
ARMOR_OUT="${APT_PUBLIC_KEY_ASC:-dist/apt/auren-transfer-agent.asc}"

usage() {
  cat <<USAGE
Usage:
  APT_GPG_KEY_ID=KEYID ./scripts/export-apt-gpg-key.sh [KEYID] [output.gpg]

Exports the public APT signing key in binary keyring form for apt signed-by usage.
Also writes an armored copy next to it for operators.
USAGE
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi
if [[ -z "${KEY_ID}" ]]; then
  echo "APT_GPG_KEY_ID or KEYID argument is required" >&2
  exit 1
fi
if ! command -v gpg >/dev/null 2>&1; then
  echo "gpg is required" >&2
  exit 1
fi
mkdir -p "$(dirname "${OUT}")" "$(dirname "${ARMOR_OUT}")"
gpg --batch --yes --armor --export "${KEY_ID}" > "${ARMOR_OUT}"
gpg --batch --yes --export "${KEY_ID}" > "${OUT}"
sha256sum "${OUT}" > "${OUT}.sha256"
sha256sum "${ARMOR_OUT}" > "${ARMOR_OUT}.sha256"
echo "Exported ${OUT}"
echo "Exported ${ARMOR_OUT}"
