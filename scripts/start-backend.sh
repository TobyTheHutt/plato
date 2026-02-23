#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
# shellcheck source=scripts/path-utils.sh
source "$script_dir/path-utils.sh"

pid_dir="${PID_DIR:-.make}"
backend_bin="${BACKEND_BIN:-$pid_dir/plato-backend}"
backend_pid="${BACKEND_PID:-$pid_dir/backend.pid}"
backend_log="${BACKEND_LOG:-$pid_dir/backend.log}"
backend_addr="${BACKEND_ADDR:-127.0.0.1:8070}"
backend_url="${BACKEND_URL:-http://127.0.0.1:8070}"
backend_health_url="${BACKEND_HEALTH_URL:-$backend_url/healthz}"
backend_dev_mode="${BACKEND_DEV_MODE:-true}"
backend_production_mode="${BACKEND_PRODUCTION_MODE:-false}"
service_start_timeout="${SERVICE_START_TIMEOUT:-30}"

backend_bin_path="$(resolve_repo_path "$repo_root" "$backend_bin")"
mkdir -p "$(resolve_repo_path "$repo_root" "$pid_dir")"

(
  cd "$repo_root/backend"
  go build -o "$backend_bin_path" ./cmd/plato
)

(
  cd "$repo_root"
  DEV_MODE="$backend_dev_mode" \
  PRODUCTION_MODE="$backend_production_mode" \
  PLATO_ADDR="$backend_addr" \
  bash "$script_dir/service-start.sh" \
    backend \
    backend \
    "$backend_pid" \
    "$backend_log" \
    "$backend_url" \
    "$backend_health_url" \
    "$service_start_timeout" \
    -- \
    "$backend_bin_path"
)
