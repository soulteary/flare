#!/usr/bin/env bash
#
# Commits the generated SVG badge (and Markdown report, when enabled) back into
# the repository, but only when something actually changed. Runs inside the
# consumer repository's workspace.

set -euo pipefail

OUTPUT="${INPUT_OUTPUT:-.github/goreportcard.svg}"
REPORT="${INPUT_REPORT:-true}"
REPORT_OUTPUT="${INPUT_REPORT_OUTPUT:-.github/goreportcard-report.md}"
COMMIT_MESSAGE="${INPUT_COMMIT_MESSAGE:-chore: update Go Report Card badge [skip ci]}"
USER_NAME="${INPUT_COMMIT_USER_NAME:-github-actions[bot]}"
USER_EMAIL="${INPUT_COMMIT_USER_EMAIL:-github-actions[bot]@users.noreply.github.com}"

WORKSPACE="${GITHUB_WORKSPACE:-$(pwd)}"
cd "${WORKSPACE}"

# Collect the files to commit.
paths=()
if [ -f "${OUTPUT}" ]; then
  paths+=("${OUTPUT}")
fi
if [ "${REPORT}" = "true" ] && [ -f "${REPORT_OUTPUT}" ]; then
  paths+=("${REPORT_OUTPUT}")
fi

if [ "${#paths[@]}" -eq 0 ]; then
  echo "No badge or report files found, nothing to commit."
  exit 0
fi

git config user.name "${USER_NAME}"
git config user.email "${USER_EMAIL}"

git add "${paths[@]}"

if git diff --cached --quiet; then
  echo "Badge and report are unchanged, skipping commit."
  exit 0
fi

git commit -m "${COMMIT_MESSAGE}"
git push
echo "Committed and pushed: ${paths[*]}"
