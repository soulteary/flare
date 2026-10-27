#!/usr/bin/env bash
# validate-inputs.sh checks that the project directory and all output paths stay
# inside GITHUB_WORKSPACE. The actual path-escape logic lives in the Go CLI
# (validate-paths subcommand) so there is no shell path arithmetic. Because the
# pinned release CLI is installed before this step, reuse it directly.
set -euo pipefail

WS="${GITHUB_WORKSPACE:?GITHUB_WORKSPACE is required}"
VBIN="${GTR_BIN:?GTR_BIN must point to the verified gtr release}"

log() { printf '%s\n' "$*" >&2; }

args=(validate-paths -workspace "$WS")
for p in "${INPUT_DIRECTORY:-.}" "${INPUT_REPORT_OUTPUT:-.github/go-test-report.md}" "${INPUT_BADGE_OUTPUT:-.github/coverage.svg}" "${INPUT_JSON_OUTPUT:-.github/go-test-report.json}" "${INPUT_RAW_OUTPUT:-.github/test-results}"; do
  [ -z "$p" ] && continue
  args+=(-path "$p")
done

log "validating paths against workspace: $WS"
"$VBIN" "${args[@]}"
log "all paths validated"
