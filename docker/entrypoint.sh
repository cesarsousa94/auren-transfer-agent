#!/usr/bin/env sh
# Auren Transfer Agent v1.13.1 Docker automatic bootstrap entrypoint.
set -eu

log() {
  printf '%s\n' "docker-entrypoint: $*"
}

is_true() {
  case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    1|true|yes|y|on|enabled) return 0 ;;
    *) return 1 ;;
  esac
}

is_false() {
  case "$(printf '%s' "${1:-}" | tr '[:upper:]' '[:lower:]')" in
    0|false|no|n|off|disabled) return 0 ;;
    *) return 1 ;;
  esac
}

CONFIG_PATH="${AUREN_AGENT_CONFIG_PATH:-/etc/auren-transfer-agent/agent.yaml}"
ENV_FILE="${AUREN_AGENT_ENV_FILE:-/etc/auren-transfer-agent/.env}"
DATA_DIR="${AUREN_RUNTIME_DATA_DIR:-/var/lib/auren-transfer-agent}"
LOG_DIR="${AUREN_AGENT_LOG_DIR:-${AUREN_LOG_DIR:-${DATA_DIR}/logs}}"
STATE_FILE="${AUREN_MEDIA_HUB_STATE_FILE:-${DATA_DIR}/media-hub/node.json}"
AUTO_BOOTSTRAP="${AUREN_DOCKER_AUTO_BOOTSTRAP:-true}"
FORCE_BOOTSTRAP="${AUREN_DOCKER_FORCE_BOOTSTRAP:-false}"
MEDIA_HUB_ENABLED="${AUREN_MEDIA_HUB_ENABLED:-true}"

# Keep the container useful with the shortest possible env file. If the operator
# provides only the shared bootstrap secret, default to the Media Hub endpoint
# introduced for the Agent env bootstrap contract.
if [ -z "${AUREN_MEDIA_HUB_BOOTSTRAP_TOKEN_ENDPOINT:-}" ] && [ -z "${AUREN_BOOTSTRAP_TOKEN_ENDPOINT:-}" ]; then
  if [ -n "${AUREN_MEDIA_HUB_BOOTSTRAP_TOKEN_SECRET:-${AUREN_BOOTSTRAP_TOKEN_SECRET:-}}" ]; then
    export AUREN_MEDIA_HUB_BOOTSTRAP_TOKEN_ENDPOINT="/api/internal/nodes/bootstrap-token"
  fi
fi

if [ -z "${AUREN_MEDIA_HUB_ENABLED:-}" ]; then
  export AUREN_MEDIA_HUB_ENABLED="true"
fi

mkdir -p "$(dirname "$CONFIG_PATH")" "$DATA_DIR" "$LOG_DIR" "${AUREN_RUNTIME_TEMP_DIR:-/tmp/auren-transfer-agent}" "${AUREN_MEDIA_HUB_WORK_DIR:-${DATA_DIR}/transfer}"

cmd="${1:-serve}"

should_bootstrap=false
if is_true "$AUTO_BOOTSTRAP" && ! is_false "$MEDIA_HUB_ENABLED"; then
  case "$cmd" in
    serve|"") should_bootstrap=true ;;
    *) should_bootstrap=false ;;
  esac
fi

if [ "$should_bootstrap" = "true" ]; then
  if [ -s "$STATE_FILE" ] && grep -q '"node_uuid"' "$STATE_FILE" && ! is_true "$FORCE_BOOTSTRAP"; then
    log "node state already exists at $STATE_FILE; bootstrap skipped"
  else
    media_hub_url="${AUREN_MEDIA_HUB_BASE_URL:-${AUREN_AGENT_MEDIA_HUB_BASE_URL:-${AUREN_MEDIA_HUB_URL:-${MEDIA_HUB_URL:-}}}}"
    registration_token="${AUREN_MEDIA_HUB_REGISTRATION_TOKEN:-${AUREN_NODE_REGISTRATION_TOKEN:-${AUREN_AGENT_REGISTRATION_TOKEN:-${REGISTRATION_TOKEN:-}}}}"
    token_endpoint="${AUREN_MEDIA_HUB_BOOTSTRAP_TOKEN_ENDPOINT:-${AUREN_BOOTSTRAP_TOKEN_ENDPOINT:-${AUREN_NODE_TOKEN_ENDPOINT:-${MEDIA_HUB_NODE_TOKEN_ENDPOINT:-}}}}"

    if [ -z "$media_hub_url" ]; then
      log "AUREN_MEDIA_HUB_BASE_URL is required for automatic Docker bootstrap"
      exit 64
    fi
    if [ -z "$registration_token" ] && [ -z "$token_endpoint" ]; then
      log "AUREN_MEDIA_HUB_REGISTRATION_TOKEN or AUREN_MEDIA_HUB_BOOTSTRAP_TOKEN_ENDPOINT is required for automatic Docker bootstrap"
      exit 64
    fi

    log "running automatic bootstrap against $media_hub_url"
    /usr/local/bin/auren-transfer-agent bootstrap \
      --config "$CONFIG_PATH" \
      --env-file "$ENV_FILE" \
      --log-dir "$LOG_DIR"
    log "automatic bootstrap complete"
  fi
fi

case "$cmd" in
  serve)
    shift || true
    log "starting serve with config=$CONFIG_PATH env_file=$ENV_FILE"
    exec /usr/local/bin/auren-transfer-agent serve --config "$CONFIG_PATH" --env-file "$ENV_FILE" "$@"
    ;;
  bootstrap|doctor|status)
    exec /usr/local/bin/auren-transfer-agent "$@" --config "$CONFIG_PATH" --env-file "$ENV_FILE"
    ;;
  --version|-version|version)
    exec /usr/local/bin/auren-transfer-agent --version
    ;;
  *)
    exec "$@"
    ;;
esac
