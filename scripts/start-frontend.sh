#!/usr/bin/env bash
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
# shellcheck source=scripts/path-utils.sh
source "$script_dir/path-utils.sh"

pid_dir="${PID_DIR:-.make}"
frontend_pid="${FRONTEND_PID:-$pid_dir/frontend.pid}"
frontend_log="${FRONTEND_LOG:-$pid_dir/frontend.log}"
frontend_url="${FRONTEND_URL:-http://127.0.0.1:5173}"
frontend_health_url="${FRONTEND_HEALTH_URL:-$frontend_url}"
frontend_host="${FRONTEND_HOST:-127.0.0.1}"
frontend_port="${FRONTEND_PORT:-5173}"
frontend_vite_bin="${FRONTEND_VITE_BIN:-frontend/node_modules/.bin/vite}"
service_start_timeout="${SERVICE_START_TIMEOUT:-30}"

vite_bin_path="$(resolve_repo_path "$repo_root" "$frontend_vite_bin")"
if [ ! -x "$vite_bin_path" ]; then
  echo "error: frontend vite binary not found at $vite_bin_path" >&2
  echo "run 'cd frontend && npm install' first" >&2
  exit 1
fi

mkdir -p "$(resolve_repo_path "$repo_root" "$pid_dir")"

(
  cd "$repo_root"
  bash "$script_dir/service-start.sh" \
    frontend \
    frontend \
    "$frontend_pid" \
    "$frontend_log" \
    "$frontend_url" \
    "$frontend_health_url" \
    "$service_start_timeout" \
    -- \
    "$vite_bin_path" --host "$frontend_host" --port "$frontend_port" --strictPort
)
