#!/usr/bin/env bash
set -euo pipefail

if ! command -v govulncheck >/dev/null 2>&1; then
  echo "error: govulncheck not found in PATH"
  echo "install it with: go install golang.org/x/vuln/cmd/govulncheck@v1.1.4"
  exit 1
fi

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
CACHE_DIR="${PLATO_VULN_CACHE_DIR:-$ROOT_DIR/.cache/vuln}"
MAX_CACHE_FILES=10
SCAN_MODE="${PLATO_VULN_SCAN_MODE:-live}"
GOVULN_INPUT="${PLATO_VULN_GOVULNCHECK_INPUT:-}"
NVD_SNAPSHOT="${PLATO_VULN_NVD_SNAPSHOT:-}"
NVD_API_BASE_URL="${PLATO_VULN_NVD_API_BASE_URL:-}"

mkdir -p "$CACHE_DIR"

if [ -f "$ROOT_DIR/backend/go.sum" ]; then
  SCAN_FINGERPRINT="$(
    { cat "$ROOT_DIR/backend/go.mod"; cat "$ROOT_DIR/backend/go.sum"; cat "$ROOT_DIR/docs/security-vulnerability-overrides.json"; govulncheck -version 2>/dev/null || true; } \
      | sha256sum
  )"
else
  SCAN_FINGERPRINT="$(
    { cat "$ROOT_DIR/backend/go.mod"; cat "$ROOT_DIR/docs/security-vulnerability-overrides.json"; govulncheck -version 2>/dev/null || true; } \
      | sha256sum
  )"
fi
SCAN_FINGERPRINT="${SCAN_FINGERPRINT%% *}"
CACHED_GOVULN_FILE="$CACHE_DIR/govulncheck-$SCAN_FINGERPRINT.json"
TMP_FILE="$(mktemp)"

cleanup() {
  rm -f "$TMP_FILE"
}
trap cleanup EXIT

if [ "$SCAN_MODE" != "live" ] && [ "$SCAN_MODE" != "prefer-cache" ] && [ "$SCAN_MODE" != "snapshot" ]; then
  echo "error: unsupported PLATO_VULN_SCAN_MODE '$SCAN_MODE'"
  echo "valid values: live, prefer-cache, snapshot"
  exit 1
fi

run_govulncheck() {
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
}

prune_cache_files() {
  mapfile -t cached_files < <(find "$CACHE_DIR" -maxdepth 1 -type f -name 'govulncheck-*.json' -printf '%T@ %p\n' | sort -nr | awk '{print $2}')

  if [ "${#cached_files[@]}" -le "$MAX_CACHE_FILES" ]; then
    return
  fi

  for ((index = MAX_CACHE_FILES; index < ${#cached_files[@]}; index++)); do
    rm -f "${cached_files[$index]}"
  done
}

if [ "$SCAN_MODE" = "snapshot" ]; then
  if [ -z "$GOVULN_INPUT" ]; then
    echo "error: snapshot mode requires PLATO_VULN_GOVULNCHECK_INPUT"
    exit 1
  fi
  if [ ! -f "$GOVULN_INPUT" ]; then
    echo "error: snapshot govulncheck input not found at '$GOVULN_INPUT'"
    exit 1
  fi
  if [ -z "$NVD_SNAPSHOT" ]; then
    echo "error: snapshot mode requires PLATO_VULN_NVD_SNAPSHOT"
    exit 1
  fi
  if [ ! -f "$NVD_SNAPSHOT" ]; then
    echo "error: NVD severity snapshot not found at '$NVD_SNAPSHOT'"
    exit 1
  fi

  cp "$GOVULN_INPUT" "$TMP_FILE"
elif [ "$SCAN_MODE" = "prefer-cache" ] && [ -f "$CACHED_GOVULN_FILE" ]; then
  cp "$CACHED_GOVULN_FILE" "$TMP_FILE"
else
  run_govulncheck
  cp "$TMP_FILE" "$CACHED_GOVULN_FILE"
fi

prune_cache_files

VULNPOLICY_ARGS=(
  -input "$TMP_FILE"
  -overrides "$ROOT_DIR/docs/security-vulnerability-overrides.json"
)

if [ -n "${NVD_API_KEY_FILE:-}" ]; then
  VULNPOLICY_ARGS+=( -nvd-api-key-file "$NVD_API_KEY_FILE" )
fi

if [ -n "$NVD_API_BASE_URL" ]; then
  VULNPOLICY_ARGS+=( -nvd-api-base-url "$NVD_API_BASE_URL" )
fi

if [ -n "$NVD_SNAPSHOT" ]; then
  VULNPOLICY_ARGS+=( -severity-snapshot "$NVD_SNAPSHOT" )
fi

if [ "$SCAN_MODE" = "snapshot" ] || [ "${PLATO_VULN_OFFLINE:-0}" = "1" ]; then
  VULNPOLICY_ARGS+=( -offline )
fi

cd "$ROOT_DIR/backend"
go run -tags tools ./cmd/vulnpolicy "${VULNPOLICY_ARGS[@]}"
