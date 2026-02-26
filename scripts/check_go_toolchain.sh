#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_MOD_FILE="$ROOT_DIR/backend/go.mod"

if [ ! -f "$GO_MOD_FILE" ]; then
  echo "error: expected Go module file at $GO_MOD_FILE"
  exit 1
fi

REQUIRED_GO_VERSION="$(awk '/^go[[:space:]]+[0-9]+\.[0-9]+(\.[0-9]+)?$/ {print $2; exit}' "$GO_MOD_FILE")"
if [ -z "$REQUIRED_GO_VERSION" ]; then
  echo "error: failed to parse required Go version from $GO_MOD_FILE"
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "error: go toolchain not found in PATH"
  echo "install Go $REQUIRED_GO_VERSION or run in a base image pinned to golang:$REQUIRED_GO_VERSION"
  exit 1
fi

CURRENT_GO_VERSION_RAW="$(go env GOVERSION 2>/dev/null || true)"
CURRENT_GO_VERSION="${CURRENT_GO_VERSION_RAW#go}"

if [ -z "$CURRENT_GO_VERSION" ]; then
  echo "error: unable to detect local Go version via 'go env GOVERSION'"
  echo "ensure your Go installation is healthy and matches $REQUIRED_GO_VERSION"
  exit 1
fi

if [ "$CURRENT_GO_VERSION" != "$REQUIRED_GO_VERSION" ]; then
  echo "error: Go toolchain mismatch"
  echo "backend/go.mod requires Go $REQUIRED_GO_VERSION"
  echo "detected local Go toolchain: $CURRENT_GO_VERSION"
  echo "install Go $REQUIRED_GO_VERSION or run using golang:$REQUIRED_GO_VERSION"
  echo "CI is pinned to the same version for reproducible checks"
  exit 1
fi
