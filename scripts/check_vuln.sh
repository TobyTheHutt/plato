#!/usr/bin/env bash
set -euo pipefail

if ! command -v govulncheck >/dev/null 2>&1; then
  echo "error: govulncheck not found in PATH"
  echo "install it with: go install golang.org/x/vuln/cmd/govulncheck@v1.1.4"
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
TMP_FILE="$(mktemp)"

cleanup() {
  rm -f "$TMP_FILE"
}
trap cleanup EXIT

pushd "$ROOT_DIR/backend" >/dev/null
set +e
govulncheck -json ./... >"$TMP_FILE"
SCAN_EXIT=$?
set -e
popd >/dev/null

if [ "$SCAN_EXIT" -ne 0 ] && [ "$SCAN_EXIT" -ne 3 ]; then
  echo "error: govulncheck failed with exit code $SCAN_EXIT"
  exit "$SCAN_EXIT"
fi

cd "$ROOT_DIR/backend"
go run -tags tools ./cmd/vulnpolicy \
  -input "$TMP_FILE" \
  -overrides "$ROOT_DIR/docs/security-vulnerability-overrides.json"
