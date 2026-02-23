#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 <name> <workdir> <pid_file> <log_file> <service_url> <health_url> <timeout_seconds> -- <command...>" >&2
}

if [ "$#" -lt 9 ]; then
  usage
  exit 64
fi

name="$1"
workdir="$2"
pid_file="$3"
log_file="$4"
service_url="$5"
health_url="$6"
timeout_seconds="$7"
stop_timeout_seconds="${SERVICE_STOP_TIMEOUT:-10}"
shift 7

if [ "$1" != "--" ]; then
  usage
  exit 64
fi
shift

if [ "$#" -eq 0 ]; then
  echo "error: missing command for $name" >&2
  usage
  exit 64
fi

if ! [[ "$timeout_seconds" =~ ^[0-9]+$ ]] || [ "$timeout_seconds" -lt 1 ]; then
  echo "error: timeout_seconds must be a positive integer, got '$timeout_seconds'" >&2
  exit 64
fi

if ! [[ "$stop_timeout_seconds" =~ ^[0-9]+$ ]] || [ "$stop_timeout_seconds" -lt 1 ]; then
  echo "error: SERVICE_STOP_TIMEOUT must be a positive integer, got '$stop_timeout_seconds'" >&2
  exit 64
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
# shellcheck source=scripts/path-utils.sh
source "$script_dir/path-utils.sh"

pid_file_path="$(resolve_repo_path "$repo_root" "$pid_file")"
log_file_path="$(resolve_repo_path "$repo_root" "$log_file")"

case "$workdir" in
  /*) workdir_path="$workdir" ;;
  *) workdir_path="$repo_root/$workdir" ;;
esac

if [ ! -d "$workdir_path" ]; then
  echo "error: working directory for $name does not exist: $workdir_path" >&2
  exit 1
fi

mkdir -p "$(dirname "$pid_file_path")"
mkdir -p "$(dirname "$log_file_path")"

if [ -f "$pid_file_path" ]; then
  existing_pid="$(tr -d '[:space:]' <"$pid_file_path")"
  if [[ "$existing_pid" =~ ^[0-9]+$ ]] && kill -0 "$existing_pid" 2>/dev/null; then
    echo "$name already running (pid $existing_pid)"
    echo "url: $service_url"
    exit 0
  fi
  echo "removing stale pid file: $pid_file_path"
  rm -f "$pid_file_path"
fi

echo "starting $name"
(
  cd "$workdir_path"
  nohup "$@" > "$log_file_path" 2>&1 < /dev/null &
  echo "$!" > "$pid_file_path"
)

if [ ! -s "$pid_file_path" ]; then
  echo "error: failed to record pid for $name in $pid_file_path" >&2
  exit 1
fi

pid="$(tr -d '[:space:]' <"$pid_file_path")"
if ! [[ "$pid" =~ ^[0-9]+$ ]]; then
  echo "error: invalid pid recorded for $name: '$pid'" >&2
  rm -f "$pid_file_path"
  exit 1
fi

if ! bash "$script_dir/wait-for-service.sh" "$health_url" "$timeout_seconds" "$pid"; then
  echo "error: $name did not become ready"
  if kill -0 "$pid" 2>/dev/null; then
    kill -TERM "$pid" 2>/dev/null || true
    bash "$script_dir/wait-for-exit.sh" "$pid" "$stop_timeout_seconds" || true
  fi
  rm -f "$pid_file_path"
  exit 1
fi

echo "$name ready"
echo "url: $service_url"
echo "log: $log_file_path"
