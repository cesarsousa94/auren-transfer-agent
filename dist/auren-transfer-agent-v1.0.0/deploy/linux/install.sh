#!/usr/bin/env bash
set -Eeuo pipefail

APP_NAME="auren-transfer-agent"
INSTALL_DIR="${INSTALL_DIR:-/opt/${APP_NAME}}"
CONFIG_DIR="${CONFIG_DIR:-/etc/${APP_NAME}}"
DATA_DIR="${DATA_DIR:-/var/lib/${APP_NAME}}"
USER_NAME="${USER_NAME:-auren-transfer}"
BINARY_SOURCE="${1:-./bin/${APP_NAME}}"

if [[ ! -f "${BINARY_SOURCE}" ]]; then
  echo "Binary not found: ${BINARY_SOURCE}" >&2
  echo "Build it first with: make build" >&2
  exit 1
fi

if [[ "${EUID}" -ne 0 ]]; then
  echo "This installer must run as root." >&2
  exit 1
fi

if ! id "${USER_NAME}" >/dev/null 2>&1; then
  useradd --system --home-dir "${DATA_DIR}" --shell /usr/sbin/nologin "${USER_NAME}"
fi

install -d -m 0755 "${INSTALL_DIR}" "${CONFIG_DIR}"
install -d -m 0750 -o "${USER_NAME}" -g "${USER_NAME}" "${DATA_DIR}" "${DATA_DIR}/storage"
install -m 0755 "${BINARY_SOURCE}" "${INSTALL_DIR}/${APP_NAME}"

if [[ -f ./configs/agent.yaml && ! -f "${CONFIG_DIR}/agent.yaml" ]]; then
  install -m 0640 -o root -g "${USER_NAME}" ./configs/agent.yaml "${CONFIG_DIR}/agent.yaml"
fi

install -m 0644 ./deploy/systemd/${APP_NAME}.service /etc/systemd/system/${APP_NAME}.service
if [[ -f ./deploy/systemd/${APP_NAME}.env.example && ! -f "${CONFIG_DIR}/${APP_NAME}.env" ]]; then
  install -m 0640 -o root -g "${USER_NAME}" ./deploy/systemd/${APP_NAME}.env.example "${CONFIG_DIR}/${APP_NAME}.env"
fi

systemctl daemon-reload
systemctl enable "${APP_NAME}.service"
echo "Installed ${APP_NAME}. Start it with: systemctl start ${APP_NAME}.service"
