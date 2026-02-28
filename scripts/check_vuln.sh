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
BINARY_GOVULN_INPUT="${PLATO_VULN_GOVULNCHECK_BINARY_INPUT:-}"
NVD_SNAPSHOT="${PLATO_VULN_NVD_SNAPSHOT:-}"
NVD_API_BASE_URL="${PLATO_VULN_NVD_API_BASE_URL:-}"
GHSA_API_BASE_URL="${PLATO_VULN_GHSA_API_BASE_URL:-}"
GHSA_TOKEN_FILE="${PLATO_VULN_GHSA_TOKEN_FILE:-${GHSA_TOKEN_FILE:-}}"
NVD_API_KEY_FILE="${PLATO_VULN_NVD_API_KEY_FILE:-${NVD_API_KEY_FILE:-}}"
REPORT_DIR="${PLATO_VULN_REPORT_DIR:-}"
REPORT_DIR_ABS=""
BINARY_ARTIFACT_DIR="${PLATO_VULN_BINARY_ARTIFACT_DIR:-$CACHE_DIR/artifacts}"
BINARY_ARTIFACT="$BINARY_ARTIFACT_DIR/plato-backend"

to_abs_path() {
  local candidate="$1"
  case "$candidate" in
    /*)
      printf '%s' "$candidate"
      ;;
    *)
      printf '%s' "$ROOT_DIR/$candidate"
      ;;
  esac
}

if [ -n "$GOVULN_INPUT" ]; then
  GOVULN_INPUT="$(to_abs_path "$GOVULN_INPUT")"
fi

if [ -n "$BINARY_GOVULN_INPUT" ]; then
  BINARY_GOVULN_INPUT="$(to_abs_path "$BINARY_GOVULN_INPUT")"
fi

if [ -n "$NVD_SNAPSHOT" ]; then
  NVD_SNAPSHOT="$(to_abs_path "$NVD_SNAPSHOT")"
fi

if [ -n "$GHSA_TOKEN_FILE" ]; then
  GHSA_TOKEN_FILE="$(to_abs_path "$GHSA_TOKEN_FILE")"
fi

if [ -n "$NVD_API_KEY_FILE" ]; then
  NVD_API_KEY_FILE="$(to_abs_path "$NVD_API_KEY_FILE")"
fi

mkdir -p "$CACHE_DIR"

# CI normally sets PLATO_VULN_REPORT_DIR explicitly.
# Keep this fallback so reports still generate if that env wiring is removed.
if [ -n "${CI:-}" ] && [ -z "${PLATO_VULN_REPORT_DIR:-}" ]; then
  REPORT_DIR="$CACHE_DIR/reports"
fi

if [ -n "$REPORT_DIR" ]; then
  case "$REPORT_DIR" in
    /*)
      REPORT_DIR_ABS="$REPORT_DIR"
      ;;
    *)
      REPORT_DIR_ABS="$ROOT_DIR/$REPORT_DIR"
      ;;
  esac
fi

if [ -f "$ROOT_DIR/backend/go.sum" ]; then
  SOURCE_SCAN_FINGERPRINT="$(
    { cat "$ROOT_DIR/backend/go.mod"; cat "$ROOT_DIR/backend/go.sum"; cat "$ROOT_DIR/docs/security-vulnerability-overrides.json"; govulncheck -version 2>/dev/null || true; } \
      | sha256sum
  )"
else
  SOURCE_SCAN_FINGERPRINT="$(
    { cat "$ROOT_DIR/backend/go.mod"; cat "$ROOT_DIR/docs/security-vulnerability-overrides.json"; govulncheck -version 2>/dev/null || true; } \
      | sha256sum
  )"
fi
SOURCE_SCAN_FINGERPRINT="${SOURCE_SCAN_FINGERPRINT%% *}"
CACHED_SOURCE_GOVULN_FILE="$CACHE_DIR/govulncheck-source-$SOURCE_SCAN_FINGERPRINT.json"
TMP_SOURCE_FILE="$(mktemp)"
TMP_BINARY_FILE="$(mktemp)"

trap 'rm -f "$TMP_SOURCE_FILE" "$TMP_BINARY_FILE"' EXIT

if [ "$SCAN_MODE" != "live" ] && [ "$SCAN_MODE" != "prefer-cache" ] && [ "$SCAN_MODE" != "snapshot" ]; then
  echo "error: unsupported PLATO_VULN_SCAN_MODE '$SCAN_MODE'"
  echo "valid values: live, prefer-cache, snapshot"
  exit 1
fi

run_source_govulncheck() {
  pushd "$ROOT_DIR/backend" >/dev/null
  set +e
  govulncheck -json ./... >"$TMP_SOURCE_FILE"
  SCAN_EXIT=$?
  set -e
  popd >/dev/null

  if [ "$SCAN_EXIT" -ne 0 ] && [ "$SCAN_EXIT" -ne 3 ]; then
    echo "error: govulncheck failed with exit code $SCAN_EXIT"
    exit "$SCAN_EXIT"
  fi
}

build_binary_artifact() {
  mkdir -p "$BINARY_ARTIFACT_DIR"
  pushd "$ROOT_DIR/backend" >/dev/null
  go build -o "$BINARY_ARTIFACT" ./cmd/plato
  popd >/dev/null
}

run_binary_govulncheck() {
  set +e
  govulncheck -mode=binary -json "$BINARY_ARTIFACT" >"$TMP_BINARY_FILE"
  SCAN_EXIT=$?
  set -e

  if [ "$SCAN_EXIT" -ne 0 ] && [ "$SCAN_EXIT" -ne 3 ]; then
    echo "error: govulncheck binary scan failed with exit code $SCAN_EXIT"
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

run_policy() {
  local scan_mode="$1"
  local input_file="$2"
  shift 2

  local -a vulnpolicy_args=(
    -input "$input_file"
    -overrides "$ROOT_DIR/docs/security-vulnerability-overrides.json"
    -scan-mode "$scan_mode"
  )

  if [ $# -gt 0 ]; then
    vulnpolicy_args+=( "$@" )
  fi

  if [ -n "$NVD_API_KEY_FILE" ]; then
    vulnpolicy_args+=( -nvd-api-key-file "$NVD_API_KEY_FILE" )
  fi

  if [ -n "$GHSA_TOKEN_FILE" ]; then
    vulnpolicy_args+=( -ghsa-token-file "$GHSA_TOKEN_FILE" )
  fi

  if [ -n "$NVD_API_BASE_URL" ]; then
    vulnpolicy_args+=( -nvd-api-base-url "$NVD_API_BASE_URL" )
  fi

  if [ -n "$GHSA_API_BASE_URL" ]; then
    vulnpolicy_args+=( -ghsa-api-base-url "$GHSA_API_BASE_URL" )
  fi

  if [ -n "$NVD_SNAPSHOT" ]; then
    vulnpolicy_args+=( -severity-snapshot "$NVD_SNAPSHOT" )
  fi

  if [ "$SCAN_MODE" = "snapshot" ] || [ "${PLATO_VULN_OFFLINE:-0}" = "1" ]; then
    vulnpolicy_args+=( -offline )
  fi

  if [ -n "$REPORT_DIR_ABS" ]; then
    if ! mkdir -p "$REPORT_DIR_ABS"; then
      echo "error: failed to create vulnerability report directory '$REPORT_DIR_ABS'"
      return 1
    fi

    local report_file="$REPORT_DIR_ABS/vulnpolicy-$scan_mode-report.json"
    rm -f "$report_file"
    vulnpolicy_args+=( -report-file "$report_file" )
  fi

  pushd "$ROOT_DIR/backend" >/dev/null
  go run -tags tools ./cmd/vulnpolicy "${vulnpolicy_args[@]}"
  popd >/dev/null
}

verify_report_file() {
  local scan_mode="$1"

  if [ -z "$REPORT_DIR_ABS" ]; then
    return 0
  fi

  local report_file="$REPORT_DIR_ABS/vulnpolicy-$scan_mode-report.json"

  if [ ! -f "$report_file" ]; then
    echo "error: expected vulnerability report file was not created: $report_file"
    return 1
  fi

  if [ ! -s "$report_file" ]; then
    echo "error: vulnerability report file is empty: $report_file"
    return 1
  fi
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
  if [ -z "$BINARY_GOVULN_INPUT" ]; then
    echo "error: snapshot mode requires PLATO_VULN_GOVULNCHECK_BINARY_INPUT"
    exit 1
  fi
  if [ ! -f "$BINARY_GOVULN_INPUT" ]; then
    echo "error: snapshot binary govulncheck input not found at '$BINARY_GOVULN_INPUT'"
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

  cp "$GOVULN_INPUT" "$TMP_SOURCE_FILE"
  cp "$BINARY_GOVULN_INPUT" "$TMP_BINARY_FILE"
elif [ "$SCAN_MODE" = "prefer-cache" ] && [ -f "$CACHED_SOURCE_GOVULN_FILE" ]; then
  cp "$CACHED_SOURCE_GOVULN_FILE" "$TMP_SOURCE_FILE"
  build_binary_artifact
  run_binary_govulncheck
else
  run_source_govulncheck
  cp "$TMP_SOURCE_FILE" "$CACHED_SOURCE_GOVULN_FILE"
  build_binary_artifact
  run_binary_govulncheck
fi

prune_cache_files

overall_status=0

echo "== Vulnerability scan: source mode =="
if ! run_policy source "$TMP_SOURCE_FILE"; then
  overall_status=1
fi
if ! verify_report_file source; then
  overall_status=1
fi

echo ""
echo "== Vulnerability scan: binary mode (deduplicated against source reachable findings) =="
if ! run_policy binary "$TMP_BINARY_FILE" -exclude-input "$TMP_SOURCE_FILE"; then
  overall_status=1
fi
if ! verify_report_file binary; then
  overall_status=1
fi

exit "$overall_status"
