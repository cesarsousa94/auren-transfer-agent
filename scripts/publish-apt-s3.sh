#!/usr/bin/env bash
set -Eeuo pipefail

REPO_DIR="${REPO_DIR:-dist/apt}"
S3_URI="${S3_URI:-}"
CLOUDFRONT_DISTRIBUTION_ID="${CLOUDFRONT_DISTRIBUTION_ID:-}"
DRY_RUN="${DRY_RUN:-false}"
CACHE_CONTROL_INDEX="max-age=60, public"
CACHE_CONTROL_POOL="max-age=31536000, public, immutable"

usage() {
  cat <<USAGE
Usage:
  S3_URI=s3://bucket/agent/apt ./scripts/publish-apt-s3.sh

Environment:
  REPO_DIR=dist/apt
  S3_URI=s3://bucket/agent/apt              required
  CLOUDFRONT_DISTRIBUTION_ID=E123          optional invalidation
  DRY_RUN=true                             show aws sync commands
USAGE
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi
if [[ -z "${S3_URI}" ]]; then
  echo "S3_URI is required, for example: S3_URI=s3://downloads-auren/agent/apt" >&2
  exit 1
fi
if [[ ! -d "${REPO_DIR}/dists" || ! -d "${REPO_DIR}/pool" ]]; then
  echo "APT repo not found at ${REPO_DIR}; run ./scripts/build-apt-repo.sh first" >&2
  exit 1
fi
if ! command -v aws >/dev/null 2>&1; then
  echo "aws CLI is required to publish to S3" >&2
  exit 1
fi

run() {
  if [[ "${DRY_RUN}" == "true" ]]; then
    printf 'DRY-RUN '
    printf '%q ' "$@"
    printf '\n'
  else
    "$@"
  fi
}

run aws s3 sync "${REPO_DIR}/pool" "${S3_URI%/}/pool" --delete --cache-control "${CACHE_CONTROL_POOL}"
run aws s3 sync "${REPO_DIR}/dists" "${S3_URI%/}/dists" --delete --cache-control "${CACHE_CONTROL_INDEX}"
run aws s3 cp "${REPO_DIR}/install-apt.sh" "${S3_URI%/}/install-apt.sh" --content-type "text/x-shellscript" --cache-control "${CACHE_CONTROL_INDEX}"
run aws s3 cp "${REPO_DIR}/SHA256SUMS" "${S3_URI%/}/SHA256SUMS" --content-type "text/plain" --cache-control "${CACHE_CONTROL_INDEX}"

if [[ -n "${CLOUDFRONT_DISTRIBUTION_ID}" ]]; then
  run aws cloudfront create-invalidation --distribution-id "${CLOUDFRONT_DISTRIBUTION_ID}" --paths "/agent/apt/dists/*" "/agent/apt/install-apt.sh" "/agent/apt/SHA256SUMS"
fi

echo "APT repository published to ${S3_URI}"
