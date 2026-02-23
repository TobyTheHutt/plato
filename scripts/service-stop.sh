#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 <name> <pid_file> <timeout_seconds>" >&2
}

if [ "$#" -ne 3 ]; then
  usage
  exit 64
fi

name="$1"
pid_file="$2"
timeout_seconds="$3"

if ! [[ "$timeout_seconds" =~ ^[0-9]+$ ]] || [ "$timeout_seconds" -lt 1 ]; then
  echo "error: timeout_seconds must be a positive integer, got '$timeout_seconds'" >&2
  exit 64
fi

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
service_port="${SERVICE_PORT:-}"
service_process_pattern="${SERVICE_PROCESS_PATTERN:-}"
# shellcheck source=scripts/path-utils.sh
source "$script_dir/path-utils.sh"

pid_file_path="$(resolve_repo_path "$repo_root" "$pid_file")"

listener_pids() {
  local port="$1"
  if command -v lsof >/dev/null 2>&1; then
    lsof -nP -tiTCP:"$port" -sTCP:LISTEN 2>/dev/null | sort -u
    return 0
  fi
  if command -v ss >/dev/null 2>&1; then
    ss -ltnp "sport = :$port" 2>/dev/null | awk '
      {
        while (match($0, /pid[=:][0-9]+/)) {
          pid = substr($0, RSTART, RLENGTH)
          sub(/pid[=:]/, "", pid)
          print pid
          $0 = substr($0, RSTART + RLENGTH)
        }
      }
    ' | sort -u
    return 0
  fi
  echo "warning: neither lsof nor ss is available to check port $port" >&2
  return 2
}

is_valid_regex() {
  local pattern="$1"
  if [ -z "$pattern" ]; then
    return 0
  fi

  if printf 'validate-pattern\n' | grep -Eq -- "$pattern" 2>/dev/null; then
    return 0
  fi

  if [ "$?" -eq 2 ]; then
    return 1
  fi

  return 0
}

stop_lingering_port_listener() {
  if [ -z "$service_port" ]; then
    return 0
  fi

  if ! is_valid_regex "$service_process_pattern"; then
    echo "error: invalid SERVICE_PROCESS_PATTERN regex: $service_process_pattern" >&2
    return 1
  fi

  if ! port_pids="$(listener_pids "$service_port")"; then
    echo "warning: cannot verify whether port $service_port is closed" >&2
    return 0
  fi
  if [ -z "$port_pids" ]; then
    echo "$name port $service_port is closed"
    return 0
  fi

  for port_pid in $port_pids; do
    if [ -z "$port_pid" ]; then
      continue
    fi
    cmd="$(ps -p "$port_pid" -o args= 2>/dev/null || true)"
    if [ -n "$service_process_pattern" ]; then
      if printf '%s\n' "$cmd" | grep -Eq -- "$service_process_pattern"; then
        :
      else
        if [ "$?" -eq 2 ]; then
          echo "error: invalid SERVICE_PROCESS_PATTERN regex during match: $service_process_pattern" >&2
          return 1
        fi
        echo "warning: port $service_port has unexpected listener pid $port_pid: $cmd" >&2
        continue
      fi
    fi
    echo "stopping lingering $name listener (pid $port_pid) on port $service_port"
    kill -TERM "$port_pid" 2>/dev/null || true
    bash "$script_dir/wait-for-exit.sh" "$port_pid" "$timeout_seconds" || true
  done

  if ! remaining_port_pids="$(listener_pids "$service_port")"; then
    echo "warning: cannot verify final listener state for port $service_port" >&2
    return 0
  fi
  if [ -n "$remaining_port_pids" ]; then
    echo "error: port $service_port is still in use by pid(s): $remaining_port_pids" >&2
    return 1
  fi

  echo "$name port $service_port is closed"
}

if [ ! -f "$pid_file_path" ]; then
  echo "$name already stopped"
else
  pid="$(tr -d '[:space:]' <"$pid_file_path")"
  if ! [[ "$pid" =~ ^[0-9]+$ ]]; then
    echo "removing invalid pid file: $pid_file_path"
    rm -f "$pid_file_path"
  else
    if kill -0 "$pid" 2>/dev/null; then
      echo "stopping $name (pid $pid)"
      kill -TERM "$pid" 2>/dev/null || true
      if ! bash "$script_dir/wait-for-exit.sh" "$pid" "$timeout_seconds"; then
        echo "warning: timed out while waiting for $name pid $pid to exit cleanly" >&2
      fi
    else
      echo "$name process not running (stale pid $pid)"
    fi
    rm -f "$pid_file_path"
  fi
fi

stop_lingering_port_listener
echo "$name stopped"
