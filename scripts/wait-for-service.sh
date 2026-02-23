#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 <url> <timeout_seconds> [pid]" >&2
}

if [ "$#" -lt 2 ] || [ "$#" -gt 3 ]; then
  usage
  exit 64
fi

if ! command -v curl >/dev/null 2>&1; then
  echo "error: curl is required for health checks" >&2
  exit 127
fi

url="$1"
timeout_seconds="$2"
pid="${3:-}"

if ! [[ "$timeout_seconds" =~ ^[0-9]+$ ]] || [ "$timeout_seconds" -lt 1 ]; then
  echo "error: timeout_seconds must be a positive integer, got '$timeout_seconds'" >&2
  exit 64
fi

if [ -n "$pid" ] && ! [[ "$pid" =~ ^[0-9]+$ ]]; then
  echo "error: pid must be numeric when provided, got '$pid'" >&2
  exit 64
fi

echo "waiting for service at $url for up to ${timeout_seconds}s"
deadline=$((SECONDS + timeout_seconds))

while true; do
  if curl --silent --fail --max-time 2 --output /dev/null "$url"; then
    echo "service reachable: $url"
    exit 0
  fi

  if [ -n "$pid" ] && ! kill -0 "$pid" 2>/dev/null; then
    echo "error: service process exited before becoming healthy (pid $pid)" >&2
    exit 1
  fi

  if [ "$SECONDS" -ge "$deadline" ]; then
    echo "error: timed out waiting for service: $url" >&2
    echo "check logs and ensure the configured port is free" >&2
    exit 1
  fi

  sleep 1
done
