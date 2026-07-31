#!/usr/bin/env bash
set -Eeuo pipefail

REPOSITORY="ClaraCora/CPanel"
AGENT_REPOSITORY="ClaraCora/CPanelde"
SERVICE_NAME="cpanel.service"
BINARY_PATH="/usr/local/bin/cpanel"
CONFIG_DIR="/etc/cpanel"
ENV_FILE="${CONFIG_DIR}/cpanel.env"
DATA_DIR="/var/lib/cpanel"
AGENT_ARTIFACT_DIR="${DATA_DIR}/agent-artifacts"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}"
ACTION="install"
VERSION="latest"
EXTERNAL_URL=""
LISTEN_ADDRESS="${CPANEL_ADDR:-127.0.0.1:8256}"
ADMIN_EMAIL="${CPANEL_ADMIN_EMAIL:-admin@cpanel.local}"
ADMIN_NAME="${CPANEL_ADMIN_NAME:-管理员}"
ADMIN_PASSWORD="${CPANEL_ADMIN_PASSWORD:-}"

usage() {
  cat <<'HELP'
CPanel one-click installer for Debian and Ubuntu

Usage:
  install.sh [install] [options]
  install.sh upgrade [--version VERSION]

Install options:
  --external-url URL       Public panel URL (auto-detected when omitted)
  --listen-address ADDR    Listen address (default: 127.0.0.1:8256)
  --admin-email EMAIL      Initial administrator email
  --admin-name NAME        Initial administrator name
  --version VERSION        Release tag (default: latest)
  --help                   Show this help

The initial password can be supplied through CPANEL_ADMIN_PASSWORD. When it is
omitted, a random password is generated and printed once after installation.
HELP
}

log() { printf '[cpanel] %s\n' "$*"; }
fail() { printf '[cpanel] ERROR: %s\n' "$*" >&2; exit 1; }

if [ "${1:-}" = "install" ] || [ "${1:-}" = "upgrade" ]; then
  ACTION="$1"
  shift
fi

while [ "$#" -gt 0 ]; do
  case "$1" in
    --external-url) EXTERNAL_URL="${2:-}"; shift 2 ;;
    --listen-address) LISTEN_ADDRESS="${2:-}"; shift 2 ;;
    --admin-email) ADMIN_EMAIL="${2:-}"; shift 2 ;;
    --admin-name) ADMIN_NAME="${2:-}"; shift 2 ;;
    --version) VERSION="${2:-}"; shift 2 ;;
    --help|-h) usage; exit 0 ;;
    *) fail "unknown argument: $1" ;;
  esac
done

[ "$(id -u)" -eq 0 ] || fail "run this installer as root"
command -v systemctl >/dev/null 2>&1 || fail "systemd is required"

case "$(uname -m)" in
  x86_64|amd64) ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) fail "unsupported architecture: $(uname -m)" ;;
esac

TMP_DIR=$(mktemp -d)
trap 'rm -rf -- "$TMP_DIR"' EXIT

install_prerequisites() {
  if command -v curl >/dev/null 2>&1 && command -v openssl >/dev/null 2>&1 && command -v psql >/dev/null 2>&1 && id postgres >/dev/null 2>&1; then
    return
  fi
  command -v apt-get >/dev/null 2>&1 || fail "automatic installation currently supports Debian and Ubuntu; install curl, openssl, and PostgreSQL first"
  log "installing PostgreSQL and download prerequisites"
  export DEBIAN_FRONTEND=noninteractive
  apt-get update
  apt-get install -y ca-certificates curl openssl postgresql
}

detect_external_url() {
  local host port public_ip
  host="${LISTEN_ADDRESS%:*}"
  port="${LISTEN_ADDRESS##*:}"
  case "$host" in
    ""|"0.0.0.0"|"[::]")
      public_ip=$(curl --fail --silent --show-error --max-time 5 https://api.ipify.org 2>/dev/null || true)
      if [ -z "$public_ip" ]; then
        public_ip=$(hostname -I 2>/dev/null | awk '{print $1}')
      fi
      host="${public_ip:-127.0.0.1}"
      ;;
    "localhost"|"[::1]") host="127.0.0.1" ;;
  esac
  printf 'http://%s:%s' "$host" "$port"
}

download_verified() {
  local repository="$1"
  local filename="$2"
  local destination="$3"
  local base_url="https://github.com/${repository}/releases/download/${VERSION}"
  log "downloading ${repository} ${filename}"
  curl --fail --location --silent --show-error --retry 5 "${base_url}/${filename}" -o "${TMP_DIR}/${filename}"
  curl --fail --location --silent --show-error --retry 5 "${base_url}/${filename}.sha256" -o "${TMP_DIR}/${filename}.sha256"
  (cd "$TMP_DIR" && sha256sum --check --status "${filename}.sha256") || fail "checksum verification failed for ${filename}"
  install -m 755 "${TMP_DIR}/${filename}" "$destination"
}

download_agent_artifacts() {
  mkdir -p "$AGENT_ARTIFACT_DIR"
  local filename
  for filename in corade-linux-amd64 corade-linux-arm64; do
    local base_url="https://github.com/${AGENT_REPOSITORY}/releases/download/${VERSION}"
    log "downloading Agent artifact ${filename}"
    if ! curl --fail --location --silent --show-error --retry 5 "${base_url}/${filename}" -o "${TMP_DIR}/${filename}" ||
       ! curl --fail --location --silent --show-error --retry 5 "${base_url}/${filename}.sha256" -o "${TMP_DIR}/${filename}.sha256"; then
      log "Agent artifact ${filename} is not published yet; Agents will download it directly from GitHub"
      continue
    fi
    if ! (cd "$TMP_DIR" && sha256sum --check --status "${filename}.sha256"); then
      log "Agent artifact ${filename} failed checksum verification and was not cached"
      continue
    fi
    install -m 644 "${TMP_DIR}/${filename}" "${AGENT_ARTIFACT_DIR}/${filename}"
    install -m 644 "${TMP_DIR}/${filename}.sha256" "${AGENT_ARTIFACT_DIR}/${filename}.sha256"
  done
  chown -R cpanel:cpanel "$AGENT_ARTIFACT_DIR"
}

write_service() {
  cat >"${TMP_DIR}/${SERVICE_NAME}" <<EOF
[Unit]
Description=CPanel device management platform
After=network-online.target postgresql.service
Wants=network-online.target

[Service]
Type=simple
User=cpanel
Group=cpanel
WorkingDirectory=${DATA_DIR}
EnvironmentFile=${ENV_FILE}
ExecStart=${BINARY_PATH} serve
Restart=always
RestartSec=5
NoNewPrivileges=true
PrivateTmp=true
ProtectHome=true
ProtectSystem=strict
ReadWritePaths=${DATA_DIR}
LimitNOFILE=1048576

[Install]
WantedBy=multi-user.target
EOF
  install -m 644 "${TMP_DIR}/${SERVICE_NAME}" "$SERVICE_FILE"
  systemctl daemon-reload
}

wait_until_ready() {
  local health_url="http://127.0.0.1${LISTEN_ADDRESS}/ca/jk/jx"
  if [[ "$LISTEN_ADDRESS" != :* ]]; then
    local port="${LISTEN_ADDRESS##*:}"
    health_url="http://127.0.0.1:${port}/ca/jk/jx"
  fi
  for _ in $(seq 1 30); do
    if systemctl is-active --quiet "$SERVICE_NAME" && curl --fail --silent "$health_url" >/dev/null 2>&1; then
      return
    fi
    sleep 1
  done
  journalctl -u "$SERVICE_NAME" -n 50 --no-pager >&2 || true
  fail "CPanel did not become ready"
}

install_prerequisites
command -v sha256sum >/dev/null 2>&1 || fail "sha256sum is required"

if [ "$ACTION" = "upgrade" ]; then
  [ -f "$ENV_FILE" ] || fail "CPanel is not installed; run install first"
  systemctl stop "$SERVICE_NAME" >/dev/null 2>&1 || true
  download_verified "$REPOSITORY" "cpanel-linux-${ARCH}" "$BINARY_PATH"
  download_agent_artifacts
  write_service
  systemctl enable "$SERVICE_NAME" >/dev/null
  systemctl restart "$SERVICE_NAME"
  LISTEN_ADDRESS=$(sed -n 's/^CPANEL_ADDR=//p' "$ENV_FILE" | tail -n 1)
  LISTEN_ADDRESS=${LISTEN_ADDRESS:-127.0.0.1:8256}
  wait_until_ready
  log "upgrade completed"
  exit 0
fi

[ ! -e "$ENV_FILE" ] || fail "CPanel is already installed; run this script with upgrade"
[[ "$LISTEN_ADDRESS" != *[[:space:]]* ]] || fail "listen address contains whitespace"
[[ "$ADMIN_EMAIL" == *@* ]] || fail "administrator email is invalid"

if [ -z "$EXTERNAL_URL" ]; then
  EXTERNAL_URL=$(detect_external_url)
fi
[[ "$EXTERNAL_URL" =~ ^https?://[^[:space:]]+$ ]] || fail "external URL must start with http:// or https:// and contain no whitespace"

systemctl enable --now postgresql >/dev/null
DB_PASSWORD=$(openssl rand -hex 24)
ENCRYPTION_KEY=$(openssl rand -base64 32 | tr -d '\n')
GENERATED_ADMIN_PASSWORD=false
if [ -z "$ADMIN_PASSWORD" ]; then
  ADMIN_PASSWORD=$(openssl rand -hex 12)
  GENERATED_ADMIN_PASSWORD=true
fi
[ "${#ADMIN_PASSWORD}" -ge 8 ] || fail "CPANEL_ADMIN_PASSWORD must contain at least 8 characters"

if ! runuser -u postgres -- psql -tAc "SELECT 1 FROM pg_roles WHERE rolname='cpanel'" | grep -q 1; then
  runuser -u postgres -- psql -v ON_ERROR_STOP=1 -c "CREATE ROLE cpanel LOGIN PASSWORD '${DB_PASSWORD}'" >/dev/null
else
  runuser -u postgres -- psql -v ON_ERROR_STOP=1 -c "ALTER ROLE cpanel WITH LOGIN PASSWORD '${DB_PASSWORD}'" >/dev/null
fi
if ! runuser -u postgres -- psql -tAc "SELECT 1 FROM pg_database WHERE datname='cpanel'" | grep -q 1; then
  runuser -u postgres -- createdb --owner=cpanel cpanel
fi

id cpanel >/dev/null 2>&1 || useradd --system --home-dir "$DATA_DIR" --shell /usr/sbin/nologin cpanel
install -d -m 750 -o root -g cpanel "$CONFIG_DIR"
install -d -m 750 -o cpanel -g cpanel "$DATA_DIR" "$AGENT_ARTIFACT_DIR"

COOKIE_SECURE=false
[[ "$EXTERNAL_URL" == https://* ]] && COOKIE_SECURE=true
cat >"${TMP_DIR}/cpanel.env" <<EOF
CPANEL_ADDR=${LISTEN_ADDRESS}
CPANEL_DATABASE_URL=postgres://cpanel:${DB_PASSWORD}@127.0.0.1:5432/cpanel?sslmode=disable
CPANEL_EXTERNAL_URL=${EXTERNAL_URL%/}
CPANEL_ENCRYPTION_KEY=${ENCRYPTION_KEY}
CPANEL_COOKIE_SECURE=${COOKIE_SECURE}
CPANEL_SESSION_TTL=24h
CPANEL_AGENT_HEARTBEAT_INTERVAL=60s
CPANEL_AGENT_TELEMETRY_INTERVAL=60s
CPANEL_AGENT_FALLBACK_PULL_INTERVAL=60s
CPANEL_AGENT_ARTIFACT_DIR=${AGENT_ARTIFACT_DIR}
EOF
install -m 640 -o root -g cpanel "${TMP_DIR}/cpanel.env" "$ENV_FILE"

download_verified "$REPOSITORY" "cpanel-linux-${ARCH}" "$BINARY_PATH"
download_agent_artifacts
write_service

export CPANEL_ADDR="$LISTEN_ADDRESS"
export CPANEL_DATABASE_URL="postgres://cpanel:${DB_PASSWORD}@127.0.0.1:5432/cpanel?sslmode=disable"
export CPANEL_EXTERNAL_URL="${EXTERNAL_URL%/}"
export CPANEL_ENCRYPTION_KEY="$ENCRYPTION_KEY"
export CPANEL_COOKIE_SECURE="$COOKIE_SECURE"
export CPANEL_AGENT_ARTIFACT_DIR="$AGENT_ARTIFACT_DIR"
export CPANEL_ADMIN_PASSWORD="$ADMIN_PASSWORD"
"$BINARY_PATH" admin create --email "$ADMIN_EMAIL" --name "$ADMIN_NAME" --password-env CPANEL_ADMIN_PASSWORD
unset CPANEL_ADMIN_PASSWORD

systemctl enable "$SERVICE_NAME" >/dev/null
systemctl restart "$SERVICE_NAME"
wait_until_ready

log "installation completed"
log "panel: ${EXTERNAL_URL%/}"
log "administrator: ${ADMIN_EMAIL}"
if [ "$GENERATED_ADMIN_PASSWORD" = true ]; then
  log "generated password (shown once): ${ADMIN_PASSWORD}"
fi
log "environment: ${ENV_FILE}"
log "service logs: journalctl -u ${SERVICE_NAME} -f"
