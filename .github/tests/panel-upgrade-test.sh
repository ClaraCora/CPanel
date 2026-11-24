#!/usr/bin/env bash
set -Eeuo pipefail
installer=${1:-./install.sh}
test_root=$(mktemp -d)
trap 'rm -rf -- "$test_root"' EXIT

# Use the installer functions verbatim, with local fixture files and simulated
# service state. The independent signature suite exercises the real downloads.
awk '
  /^(wait_until_ready|upgrade_panel|cache_agent_file|cleanup|log|fail)\(\) \{/ { printing = 1 }
  printing { print }
  /^\}/ || /^(log|fail)\(\) / { printing = 0 }
' "$installer" >"$test_root/functions.sh"
. "$test_root/functions.sh"

run_upgrade() (
  set -Eeuo pipefail
  local mode=$1
  local root="$test_root/$mode"
  BINARY_PATH="$root/cpanel"
  ENV_FILE="$root/cpanel.env"
  SERVICE_FILE="$root/cpanel.service"
  SERVICE_NAME=cpanel.service
  VERSION=latest ARCH=amd64
  TMP_DIR="$root/tmp" UPGRADE_STAGE="" KEEP_BACKUP=false
  trap cleanup EXIT

  mkdir -p "$TMP_DIR" "$SERVICE_FILE.d"
  printf 'old\n' >"$BINARY_PATH"
  local listen='[::]:9187' probe='[::1]:9187' expected_source=::1
  if [[ "$mode" = vendor ]]; then
    listen=192.0.2.10:9187
    probe=$listen
    expected_source=127.0.0.1
  fi
  printf 'CPANEL_ADDR=%s\n' "$listen" >"$ENV_FILE"
  printf 'custom\n' >"$SERVICE_FILE"
  printf 'override\n' >"$SERVICE_FILE.d/settings.conf"
  [[ "$mode" != vendor ]] || rm "$SERVICE_FILE"

  download_verified() {
    echo download >>"$root/events"
    [[ "$mode" != download_failure ]] || fail "simulated failure"
    printf 'new\n' >"$4"
  }
  download_agent_artifacts() {
    echo cache >>"$root/events"
    [[ "$(cat "$BINARY_PATH")" = old ]]
  }
  write_service() { fail "overwrote a custom or vendor unit"; }
  systemctl() {
    case "$1" in
      cat) [[ "$mode" = vendor ]] ;;
      show) echo 0 ;;
      is-active) return 0 ;;
      restart)
        local installed
        installed=$(cat "$BINARY_PATH")
        echo "restart-$installed" >>"$root/events"
        [[ "$mode" != restart_failure || "$installed" = old ]]
        ;;
      reset-failed) : ;;
      *) fail "unexpected systemctl $1" ;;
    esac
  }
  curl() {
    [[ " $* " = *' --max-time 2 '* && " $* " = *' --connect-timeout 1 '* && "${!#}" = "http://${probe}/ca/jk/jx" ]] || return 1
    [[ " $* " = *" --interface $expected_source "* && " $* " = *' --header Host: localhost '* ]] || return 1
    if [[ "$mode" = both_failed || ( "$mode" = probe_failure && "$(cat "$BINARY_PATH")" = new ) ]]; then echo 200; else echo 204; fi
  }
  sleep() { :; }
  journalctl() { :; }
  upgrade_panel
)

for mode in success vendor download_failure restart_failure probe_failure both_failed; do
  root="$test_root/$mode"
  rc=0
  # Keep errexit active inside the installer even when a failure is expected.
  set +e
  run_upgrade "$mode" >"$test_root/$mode.log" 2>&1
  rc=$?
  set -e
  case "$mode" in
    success|vendor)
      [[ "$rc" = 0 && "$(cat "$root/cpanel")" = new ]]
      expected=$'download\ncache\nrestart-new' ;;
    download_failure)
      [[ "$rc" != 0 && "$(cat "$root/cpanel")" = old ]]
      expected=download ;;
    *)
      [[ "$rc" != 0 && "$(cat "$root/cpanel")" = old ]]
      expected=$'download\ncache\nrestart-new\nrestart-old' ;;
  esac
  [[ "$(cat "$root/events")" = "$expected" ]]
  [[ "$mode" = vendor || "$(cat "$root/cpanel.service")" = custom ]]
  [[ "$(cat "$root/cpanel.service.d/settings.conf")" = override ]]
  shopt -s nullglob
  retained=("$root"/cpanel.upgrade.*/)
  if [[ "$mode" = both_failed ]]; then
    [[ ${#retained[@]} = 1 && "$(cat "${retained[0]}previous")" = old ]]
  else
    [[ ${#retained[@]} = 0 ]]
  fi
done

# A request which opened the old cache file keeps reading the complete old
# contents while a new request observes the complete replacement.
runuser() {
  [[ "$1" = -u && "$2" = cpanel && "$3" = -- ]] || return 1
  shift 3
  "$@"
}
printf 'old cached artifact' >"$test_root/artifact"
printf 'new cached artifact' >"$test_root/download"
exec 3<"$test_root/artifact"
cache_agent_file "$test_root/download" "$test_root/artifact"
[[ "$(cat <&3)" = 'old cached artifact' && "$(cat "$test_root/artifact")" = 'new cached artifact' ]]
exec 3<&-
echo "Panel upgrade tests passed"
