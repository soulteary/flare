#!/usr/bin/env bash
#
# Runs the goreportcard CLI against the target project, writes a self-contained
# SVG badge (and optionally a Markdown report), and exports the
# grade/score/badge/report values as GitHub Action step outputs.

set -euo pipefail

DIRECTORY="${INPUT_DIRECTORY:-.}"
OUTPUT="${INPUT_OUTPUT:-.github/goreportcard.svg}"
THRESHOLD="${INPUT_THRESHOLD:-0}"
REPORT="${INPUT_REPORT:-true}"
REPORT_OUTPUT="${INPUT_REPORT_OUTPUT:-.github/goreportcard-report.md}"

# The output paths are relative to the repository (GITHUB_WORKSPACE) that
# consumes this action; fall back to the current directory when run locally.
WORKSPACE="${GITHUB_WORKSPACE:-$(pwd)}"
BADGE_PATH="${WORKSPACE}/${OUTPUT}"
REPORT_PATH="${WORKSPACE}/${REPORT_OUTPUT}"

mkdir -p "$(dirname "${BADGE_PATH}")"

echo "Grading Go project in '${DIRECTORY}'..."

# Capture the JSON report so we can surface the grade/score.
REPORT_JSON="$(goreportcard-cli -d "${DIRECTORY}" -j)"

# Build the CLI arguments: always the badge, plus the report when requested.
# Note: we intentionally do NOT pass the threshold here. The badge and report
# should always be generated (and committed) reflecting the real grade; the
# threshold check happens at the end so a failing grade doesn't prevent the
# artifacts from being written and committed.
cli_args=(-d "${DIRECTORY}" -svg "${BADGE_PATH}")
if [ "${REPORT}" = "true" ]; then
  mkdir -p "$(dirname "${REPORT_PATH}")"
  cli_args+=(-report "${REPORT_PATH}")
fi

# Write the badge (and report).
goreportcard-cli "${cli_args[@]}"

# Extract grade and score without requiring jq.
GRADE="$(printf '%s' "${REPORT_JSON}" | sed -n 's/.*"GradeFromPercentage":"\([^"]*\)".*/\1/p')"
AVERAGE="$(printf '%s' "${REPORT_JSON}" | sed -n 's/.*"average":\([0-9.]*\).*/\1/p')"
SCORE="$(awk "BEGIN { printf \"%.1f\", ${AVERAGE:-0} * 100 }")"

echo "Grade: ${GRADE:-unknown} (${SCORE}%)"
echo "Badge written to ${BADGE_PATH}"

{
  echo "grade=${GRADE}"
  echo "score=${SCORE}"
  echo "badge=${OUTPUT}"
} >> "${GITHUB_OUTPUT:-/dev/stdout}"

if [ "${REPORT}" = "true" ]; then
  echo "Report written to ${REPORT_PATH}"
  echo "report=${REPORT_OUTPUT}" >> "${GITHUB_OUTPUT:-/dev/stdout}"
fi

# Export whether the score meets the threshold. The actual enforcement happens
# in a later step (after the badge/report are committed) so a failing grade
# doesn't prevent the artifacts from being written and committed.
below="$(awk "BEGIN { print (${SCORE:-0} < ${THRESHOLD}) ? 1 : 0 }")"
if [ "${below}" = "1" ]; then
  echo "below_threshold=true" >> "${GITHUB_OUTPUT:-/dev/stdout}"
  echo "Score ${SCORE}% is below the required threshold of ${THRESHOLD}%."
else
  echo "below_threshold=false" >> "${GITHUB_OUTPUT:-/dev/stdout}"
fi
