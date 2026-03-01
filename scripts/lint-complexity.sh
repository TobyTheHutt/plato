#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"

CYCLOP_MAX_COMPLEXITY="${CYCLOP_MAX_COMPLEXITY:-25}"
CYCLOP_PACKAGE_AVERAGE="${CYCLOP_PACKAGE_AVERAGE:-12}"
GOCYCLO_MIN_COMPLEXITY="${GOCYCLO_MIN_COMPLEXITY:-20}"
GOCOGNIT_MIN_COMPLEXITY="${GOCOGNIT_MIN_COMPLEXITY:-25}"
RUN_TESTS="${RUN_TESTS:-false}"

FAIL_ON_ISSUES=1
TARGETS=()

usage() {
  cat <<'EOF'
Usage: scripts/lint-complexity.sh [--no-fail] [target...]

Run complexity-focused golangci-lint checks with sortable output.

Options:
  --no-fail    Report violations without returning a failing exit code
  -h, --help   Show this help

Targets:
  Package patterns passed to golangci-lint from backend directory.
  Examples:
    scripts/lint-complexity.sh
    scripts/lint-complexity.sh ./backend/...
    scripts/lint-complexity.sh ./backend/internal/httpapi/...

Environment overrides:
  CYCLOP_MAX_COMPLEXITY      default: 25
  CYCLOP_PACKAGE_AVERAGE     default: 12
  GOCYCLO_MIN_COMPLEXITY     default: 20
  GOCOGNIT_MIN_COMPLEXITY    default: 25
  RUN_TESTS                  default: false
EOF
}

normalize_target() {
  local target="$1"
  case "$target" in
    ./backend)
      printf '.\n'
      ;;
    ./backend/*)
      printf '.%s\n' "${target#./backend}"
      ;;
    backend)
      printf '.\n'
      ;;
    backend/*)
      printf './%s\n' "${target#backend/}"
      ;;
    *)
      printf '%s\n' "$target"
      ;;
  esac
}

if ! command -v golangci-lint >/dev/null 2>&1; then
  echo "error: golangci-lint not found in PATH"
  exit 1
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "error: jq not found in PATH"
  exit 1
fi

while [ "$#" -gt 0 ]; do
  case "$1" in
    --no-fail)
      FAIL_ON_ISSUES=0
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    --)
      shift
      while [ "$#" -gt 0 ]; do
        TARGETS+=( "$1" )
        shift
      done
      break
      ;;
    *)
      TARGETS+=( "$1" )
      ;;
  esac
  shift
done

if [ "${#TARGETS[@]}" -eq 0 ]; then
  TARGETS=( "./backend/..." )
fi

NORMALIZED_TARGETS=()
for target in "${TARGETS[@]}"; do
  NORMALIZED_TARGETS+=( "$(normalize_target "$target")" )
done

TMP_CFG="$(mktemp /tmp/plato-complexity-lint-XXXXXX.yml)"
TMP_JSON="$(mktemp /tmp/plato-complexity-lint-XXXXXX.json)"
TMP_SORTED="$(mktemp /tmp/plato-complexity-lint-XXXXXX.tsv)"
trap 'rm -f "$TMP_CFG" "$TMP_JSON" "$TMP_SORTED"' EXIT

cat >"$TMP_CFG" <<EOF
run:
  timeout: 5m
  tests: ${RUN_TESTS}
  modules-download-mode: readonly
  issues-exit-code: 0
linters:
  disable-all: true
  enable:
    - cyclop
    - gocyclo
    - gocognit
linters-settings:
  cyclop:
    max-complexity: ${CYCLOP_MAX_COMPLEXITY}
    package-average: ${CYCLOP_PACKAGE_AVERAGE}
  gocyclo:
    min-complexity: ${GOCYCLO_MIN_COMPLEXITY}
  gocognit:
    min-complexity: ${GOCOGNIT_MIN_COMPLEXITY}
issues:
  max-issues-per-linter: 0
  max-same-issues: 0
  uniq-by-line: false
EOF

pushd "$BACKEND_DIR" >/dev/null
set +e
golangci-lint run -c "$TMP_CFG" --out-format json "${NORMALIZED_TARGETS[@]}" >"$TMP_JSON"
LINT_EXIT=$?
set -e
popd >/dev/null

if [ "$LINT_EXIT" -ne 0 ]; then
  echo "error: golangci-lint failed with exit code $LINT_EXIT"
  exit "$LINT_EXIT"
fi

ISSUE_COUNT="$(jq '.Issues | length' "$TMP_JSON")"

printf 'thresholds: cyclop max=%s package-average=%s, gocyclo min=%s, gocognit min=%s\n' \
  "$CYCLOP_MAX_COMPLEXITY" "$CYCLOP_PACKAGE_AVERAGE" "$GOCYCLO_MIN_COMPLEXITY" "$GOCOGNIT_MIN_COMPLEXITY"
printf 'targets: %s\n' "${TARGETS[*]}"
printf 'violations: %s\n' "$ISSUE_COUNT"

if [ "$ISSUE_COUNT" -eq 0 ]; then
  exit 0
fi

jq -r \
  --argjson cyclop "$CYCLOP_MAX_COMPLEXITY" \
  --argjson gocyclo "$GOCYCLO_MIN_COMPLEXITY" \
  --argjson gocognit "$GOCOGNIT_MIN_COMPLEXITY" '
    .Issues[]
    | . as $issue
    | (try (.Text | match("[0-9]+(\\.[0-9]+)?").string | tonumber) catch 0) as $score
    | (if $issue.FromLinter == "cyclop" then $cyclop
       elif $issue.FromLinter == "gocyclo" then $gocyclo
       elif $issue.FromLinter == "gocognit" then $gocognit
       else 0
       end) as $threshold
    | ($score - $threshold) as $delta
    | (if $delta >= 20 then "critical"
       elif $delta >= 10 then "high"
       elif $delta >= 5 then "medium"
       else "low"
       end) as $severity
    | (if $severity == "critical" then 4
       elif $severity == "high" then 3
       elif $severity == "medium" then 2
       else 1
       end) as $rank
    | [
        $rank,
        $score,
        $severity,
        $issue.FromLinter,
        ($issue.Pos.Filename + ":" + ($issue.Pos.Line | tostring)),
        $issue.Text
      ]
    | @tsv
  ' "$TMP_JSON" \
  | sort -t $'\t' -k1,1nr -k2,2gr -k4,4 -k5,5 >"$TMP_SORTED"

printf '\n%-8s %-7s %-10s %-56s %s\n' "SEVERITY" "SCORE" "LINTER" "LOCATION" "MESSAGE"

while IFS=$'\t' read -r _ score severity linter location message; do
  printf '%-8s %-7s %-10s %-56s %s\n' "$severity" "$score" "$linter" "$location" "$message"
done <"$TMP_SORTED"

if [ "$FAIL_ON_ISSUES" -eq 1 ]; then
  exit 1
fi
