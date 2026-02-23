#!/usr/bin/env bash
set -euo pipefail

usage() {
  echo "Usage: $0 <pid> <timeout_seconds>" >&2
}

if [ "$#" -ne 2 ]; then
  usage
  exit 64
fi

pid="$1"
timeout_seconds="$2"

if ! [[ "$pid" =~ ^[0-9]+$ ]]; then
  echo "error: pid must be numeric, got '$pid'" >&2
  exit 64
fi

if ! [[ "$timeout_seconds" =~ ^[0-9]+$ ]] || [ "$timeout_seconds" -lt 1 ]; then
  echo "error: timeout_seconds must be a positive integer, got '$timeout_seconds'" >&2
  exit 64
fi

if ! kill -0 "$pid" 2>/dev/null; then
  echo "process $pid is already stopped"
  exit 0
fi

echo "waiting up to ${timeout_seconds}s for pid $pid to exit"
deadline=$((SECONDS + timeout_seconds))

while kill -0 "$pid" 2>/dev/null; do
  if [ "$SECONDS" -ge "$deadline" ]; then
    echo "pid $pid did not exit in time, sending SIGTERM before SIGKILL"
    kill -TERM "$pid" 2>/dev/null || true
    sleep 1
    if kill -0 "$pid" 2>/dev/null; then
      echo "pid $pid is still running, sending SIGKILL"
      kill -KILL "$pid" 2>/dev/null || true
    fi
    break
  fi
  sleep 1
done

kill_deadline=$((SECONDS + 5))
while kill -0 "$pid" 2>/dev/null; do
  if [ "$SECONDS" -ge "$kill_deadline" ]; then
    echo "error: pid $pid is still running after SIGKILL attempt" >&2
    exit 1
  fi
  sleep 1
done

echo "process $pid stopped"
