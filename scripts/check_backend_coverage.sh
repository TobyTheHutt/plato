#!/usr/bin/env bash
set -euo pipefail

THRESHOLD="${1:-90}"

if ! [[ "$THRESHOLD" =~ ^[0-9]+([.][0-9]+)?$ ]]; then
  echo "error: coverage threshold must be a number"
  echo "received: $THRESHOLD"
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
COVERAGE_FILE="$BACKEND_DIR/coverage.out"

cd "$BACKEND_DIR"
go test -count=1 -tags tools ./... -coverprofile="$COVERAGE_FILE"

TOTAL_LINE="$(go tool cover -func="$COVERAGE_FILE" | tail -n 1)"
if [ -z "$TOTAL_LINE" ]; then
  echo "error: failed to parse total coverage from $COVERAGE_FILE"
  exit 1
fi

echo "$TOTAL_LINE"

TOTAL_PERCENT="$(grep -oE '[0-9]+([.][0-9]+)?%' <<<"$TOTAL_LINE" | tr -d '%' | head -n 1)"
if [ -z "$TOTAL_PERCENT" ]; then
  echo "error: failed to extract total coverage percentage"
  exit 1
fi

if awk -v total="$TOTAL_PERCENT" -v threshold="$THRESHOLD" 'BEGIN { exit(total + 0 >= threshold + 0 ? 0 : 1) }'; then
  echo "ok: backend coverage ${TOTAL_PERCENT}% meets threshold ${THRESHOLD}%"
else
  echo "error: backend coverage ${TOTAL_PERCENT}% is below threshold ${THRESHOLD}%"
  exit 1
fi
