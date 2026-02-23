#!/usr/bin/env bash
set -euo pipefail

if ! command -v curl >/dev/null 2>&1; then
  echo "error: curl is required for status health checks" >&2
  exit 127
fi

pid_dir="${PID_DIR:-.make}"
backend_pid_file="${BACKEND_PID_FILE:-$pid_dir/backend.pid}"
frontend_pid_file="${FRONTEND_PID_FILE:-$pid_dir/frontend.pid}"
backend_url="${BACKEND_URL:-http://127.0.0.1:8070}"
backend_health_url="${BACKEND_HEALTH_URL:-$backend_url/healthz}"
frontend_url="${FRONTEND_URL:-http://127.0.0.1:5173}"
frontend_health_url="${FRONTEND_HEALTH_URL:-$frontend_url}"
cleanup_stale="${CLEANUP_STALE_PID_FILES:-true}"
script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
# shellcheck source=scripts/path-utils.sh
source "$script_dir/path-utils.sh"

is_healthy() {
  local health_url="$1"
  local max_attempts=3
  local attempt=1

  while [ "$attempt" -le "$max_attempts" ]; do
    if curl --silent --fail --max-time 2 --output /dev/null "$health_url"; then
      return 0
    fi
    sleep 1
    attempt=$((attempt + 1))
  done

  return 1
}

print_status() {
  local name="$1"
  local pid_file="$2"
  local service_url="$3"
  local health_url="$4"
  local pid_file_path

  pid_file_path="$(resolve_repo_path "$repo_root" "$pid_file")"

  if [ ! -f "$pid_file_path" ]; then
    echo "$name: stopped"
    echo "   PID file: $pid_file_path (missing)"
    return
  fi

  local pid
  pid="$(tr -d '[:space:]' <"$pid_file_path")"
  if ! [[ "$pid" =~ ^[0-9]+$ ]]; then
    echo "$name: invalid pid file"
    echo "   PID file: $pid_file_path"
    if [ "$cleanup_stale" = "true" ]; then
      rm -f "$pid_file_path"
      echo "   Removed invalid PID file"
    fi
    return
  fi

  if ! kill -0 "$pid" 2>/dev/null; then
    echo "$name: not running (stale pid $pid)"
    echo "   PID file: $pid_file_path"
    if [ "$cleanup_stale" = "true" ]; then
      rm -f "$pid_file_path"
      echo "   Removed stale PID file"
    fi
    return
  fi

  if is_healthy "$health_url"; then
    echo "$name: running and healthy"
  else
    echo "$name: running but health check failed"
  fi
  echo "   PID: $pid"
  echo "   URL: $service_url"
  echo "   Health: $health_url"
}

echo "Service status"
print_status "Backend" "$backend_pid_file" "$backend_url" "$backend_health_url"
print_status "Frontend" "$frontend_pid_file" "$frontend_url" "$frontend_health_url"
