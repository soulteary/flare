#!/usr/bin/env bash
# run-report.sh runs the gtr CLI, captures its semantic exit code
# WITHOUT failing this step, extracts machine outputs from the deterministic
# JSON report, and writes them to GITHUB_OUTPUT. The saved exit code is written
# to a file so the later "Enforce result" step can fail the job.
#
# All user inputs are passed as explicit CLI arguments (never via eval/bash -c).
set -uo pipefail

BIN="${GTR_BIN:?GTR_BIN must point to the gtr binary}"
GITHUB_OUTPUT="${GITHUB_OUTPUT:-/dev/stdout}"
EXITCODE_FILE="${GTR_EXITCODE_FILE:-${RUNNER_TEMP:-/tmp}/gtr.exit}"

# Required paths / values (validated & resolved earlier by validate-paths).
DIRECTORY="${INPUT_DIRECTORY:-.}"
PACKAGES="${INPUT_PACKAGES:-./...}"
RACE="${INPUT_RACE:-false}"
COVER_MODE="${INPUT_COVER_MODE:-atomic}"
COVER_PKG="${INPUT_COVER_PKG:-}"
TIMEOUT="${INPUT_TIMEOUT:-10m}"
TEST_ARGS="${INPUT_TEST_ARGS:-}"
COVERAGE_THRESHOLD="${INPUT_COVERAGE_THRESHOLD:-0}"
PACKAGE_THRESHOLD="${INPUT_PACKAGE_THRESHOLD:-0}"
REPORT_OUTPUT="${INPUT_REPORT_OUTPUT:-.github/go-test-report.md}"
BADGE_OUTPUT="${INPUT_BADGE_OUTPUT:-.github/coverage.svg}"
JSON_OUTPUT="${INPUT_JSON_OUTPUT:-.github/go-test-report.json}"
RAW_OUTPUT="${INPUT_RAW_OUTPUT:-.github/test-results}"

log() { printf '%s\n' "$*" >&2; }

# Build the argument array. Each user value is a distinct array element, so the
# shell never re-splits or evaluates it.
args=(run
  -directory "$DIRECTORY"
  -packages "$PACKAGES"
  -cover-mode "$COVER_MODE"
  -timeout "$TIMEOUT"
  -coverage-threshold "$COVERAGE_THRESHOLD"
  -package-threshold "$PACKAGE_THRESHOLD"
  -json-output "$JSON_OUTPUT"
  -markdown-output "$REPORT_OUTPUT"
  -svg-output "$BADGE_OUTPUT"
  -raw-output-dir "$RAW_OUTPUT"
  -summary-output "${RUNNER_TEMP:-/tmp}/gtr-summary.md"
)

if [ "$RACE" = "true" ]; then
  args+=(-race)
fi
if [ -n "$COVER_PKG" ]; then
  args+=(-cover-pkg "$COVER_PKG")
fi
if [ -n "$TEST_ARGS" ]; then
  args+=(-test-args "$TEST_ARGS")
fi

# exclude: one regexp per line -> a repeatable -exclude flag.
if [ -n "${INPUT_EXCLUDE:-}" ]; then
  while IFS= read -r line; do
    [ -z "$line" ] && continue
    args+=(-exclude "$line")
  done <<< "${INPUT_EXCLUDE}"
fi

# Job Summary metadata (dynamic; never written into the stable reports).
args+=(-sha "${GITHUB_SHA:-}" -branch "${GITHUB_REF_NAME:-}" -runner "${RUNNER_OS:-}")

log "running: $BIN ${args[*]}"
set +e
"$BIN" "${args[@]}"
code=$?
set -e 2>/dev/null || true

printf '%s' "$code" > "$EXITCODE_FILE"
log "gtr exit code: $code (saved to $EXITCODE_FILE)"

# Map the semantic exit code to a status string.
status="error"
case "$code" in
  0) status="passed" ;;
  10) status="test_failed" ;;
  11 | 12) status="coverage_failed" ;;
  20 | 21) status="error" ;;
esac

# Extract fields from the deterministic JSON report using the CLI-independent
# jq if present, else a small Go-free fallback via grep/sed on the stable schema.
json_get_num() {
  local key="$1" file="$2"
  if command -v jq >/dev/null 2>&1; then
    jq -r "$3" "$file" 2>/dev/null
  else
    grep -E "\"$key\"" "$file" | head -n1 | sed -E 's/.*: *([0-9.]+).*/\1/'
  fi
}

coverage="" tests="" passed="" failed="" skipped=""
if [ -f "$JSON_OUTPUT" ]; then
  coverage="$(json_get_num percentage "$JSON_OUTPUT" '.coverage.percentage')"
  tests="$(json_get_num total "$JSON_OUTPUT" '.tests.total')"
  passed="$(json_get_num passed "$JSON_OUTPUT" '.tests.passed')"
  failed="$(json_get_num failed "$JSON_OUTPUT" '.tests.failed')"
  skipped="$(json_get_num skipped "$JSON_OUTPUT" '.tests.skipped')"
fi

{
  printf 'status=%s\n' "$status"
  printf 'coverage=%s\n' "$coverage"
  printf 'tests=%s\n' "$tests"
  printf 'passed=%s\n' "$passed"
  printf 'failed=%s\n' "$failed"
  printf 'skipped=%s\n' "$skipped"
  printf 'report=%s\n' "$REPORT_OUTPUT"
  printf 'badge=%s\n' "$BADGE_OUTPUT"
  printf 'json=%s\n' "$JSON_OUTPUT"
  printf 'coverage_profile=%s\n' "${RAW_OUTPUT}/coverage.out"
  printf 'exit_code=%s\n' "$code"
} >> "$GITHUB_OUTPUT"

# This step must succeed regardless of the semantic exit code.
exit 0
