#!/usr/bin/env bash
set -euo pipefail

MAKE_BIN="${MAKE:-make}"
TMP_FILE="$(mktemp)"

cleanup() {
  rm -f "$TMP_FILE"
}
trap cleanup EXIT

if ! "$MAKE_BIN" --warn-undefined-variables --dry-run check >"$TMP_FILE" 2>&1; then
  cat "$TMP_FILE"
  exit 1
fi

cat "$TMP_FILE"

if grep -q "warning: undefined variable" "$TMP_FILE"; then
  echo "error: GNU Make reported undefined variable warnings during dry run"
  exit 1
fi
