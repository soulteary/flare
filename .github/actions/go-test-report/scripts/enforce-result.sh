#!/usr/bin/env bash
# enforce-result.sh is the final step. It reads the semantic exit code saved by
# run-report.sh and fails the job accordingly, AFTER reports were generated,
# the summary was written, and any allowed commit was made.
set -euo pipefail

EXITCODE_FILE="${GTR_EXITCODE_FILE:-${RUNNER_TEMP:-/tmp}/gtr.exit}"

if [ ! -f "$EXITCODE_FILE" ]; then
  echo "::error::gtr exit code file not found: $EXITCODE_FILE"
  exit 1
fi

code="$(cat "$EXITCODE_FILE")"

case "$code" in
  0)
    echo "All tests passed and coverage gates satisfied."
    exit 0
    ;;
  10)
    echo "::error::Go tests or compilation failed."
    ;;
  11)
    echo "::error::Total coverage is below the configured threshold."
    ;;
  12)
    echo "::error::At least one package is below the per-package coverage threshold."
    ;;
  20)
    echo "::error::Input or configuration error."
    ;;
  21)
    echo "::error::Go toolchain or internal execution error."
    ;;
  *)
    echo "::error::gtr failed with exit code ${code}."
    ;;
esac

exit "$code"
