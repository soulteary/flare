#!/usr/bin/env bash
# commit-report.sh commits the three stable report files back to the repository,
# but ONLY when every safety condition is met:
#   - commit == true
#   - event is a push to the default branch, or a workflow_dispatch
#   - not a fork pull request
#   - tests passed OR commit_on_failure == true
#   - at least one of the three stable files actually changed
#
# It only ever `git add`s the three explicit files. It never uses `git add .`
# or `git add -A`, and never auto-rebases.
set -euo pipefail

log() { printf '%s\n' "$*" >&2; }
warn() { printf '::warning::%s\n' "$*"; }

COMMIT="${INPUT_COMMIT:-false}"
COMMIT_ON_FAILURE="${INPUT_COMMIT_ON_FAILURE:-false}"
COMMIT_MESSAGE="${INPUT_COMMIT_MESSAGE:-chore: update Go test report [skip ci]}"
REPORT="${INPUT_REPORT_OUTPUT:-.github/go-test-report.md}"
BADGE="${INPUT_BADGE_OUTPUT:-.github/coverage.svg}"
JSON="${INPUT_JSON_OUTPUT:-.github/go-test-report.json}"

EXITCODE_FILE="${GTR_EXITCODE_FILE:-${RUNNER_TEMP:-/tmp}/gtr.exit}"
GITHUB_OUTPUT="${GITHUB_OUTPUT:-/dev/stdout}"

emit() { printf '%s=%s\n' "$1" "$2" >> "$GITHUB_OUTPUT"; }

# Default outputs.
emit committed "false"
emit commit_sha ""

if [ "$COMMIT" != "true" ]; then
  log "commit disabled; skipping"
  exit 0
fi

EVENT="${GITHUB_EVENT_NAME:-}"
REF_NAME="${GITHUB_REF_NAME:-}"
DEFAULT_BRANCH="${GTR_DEFAULT_BRANCH:-${GITHUB_DEFAULT_BRANCH:-}}"

# Fork PR detection: on pull_request events from a fork, the head repo differs
# from the base repo. Never attempt to commit in that case.
if [ "$EVENT" = "pull_request" ] || [ "$EVENT" = "pull_request_target" ]; then
  warn "commit:true is set on a pull_request event; refusing to write back (use a default-branch workflow instead)"
  exit 0
fi

# Only push to default branch, or an explicit workflow_dispatch.
allowed="false"
if [ "$EVENT" = "push" ]; then
  if [ -n "$DEFAULT_BRANCH" ] && [ "$REF_NAME" = "$DEFAULT_BRANCH" ]; then
    allowed="true"
  elif [ -z "$DEFAULT_BRANCH" ]; then
    # If we cannot determine the default branch, be conservative and require
    # workflow_dispatch instead.
    warn "default branch unknown; refusing push-triggered commit"
  else
    log "push is on '$REF_NAME', not default branch '$DEFAULT_BRANCH'; skipping commit"
  fi
elif [ "$EVENT" = "workflow_dispatch" ]; then
  allowed="true"
else
  log "event '$EVENT' is not eligible for write-back; skipping"
fi

if [ "$allowed" != "true" ]; then
  exit 0
fi

# Respect test outcome unless commit_on_failure is set.
code="0"
if [ -f "$EXITCODE_FILE" ]; then
  code="$(cat "$EXITCODE_FILE")"
fi
if [ "$code" != "0" ] && [ "$COMMIT_ON_FAILURE" != "true" ]; then
  log "result exit code $code != 0 and commit_on_failure=false; skipping commit"
  exit 0
fi

# Stage ONLY the three explicit files (any that exist).
to_add=()
for f in "$REPORT" "$BADGE" "$JSON"; do
  if [ -f "$f" ]; then
    to_add+=("$f")
  fi
done
if [ "${#to_add[@]}" -eq 0 ]; then
  log "no stable report files present; nothing to commit"
  exit 0
fi

git config user.name "github-actions[bot]"
git config user.email "41898282+github-actions[bot]@users.noreply.github.com"

git add -- "${to_add[@]}"

# No staged changes => nothing to do.
if git diff --cached --quiet; then
  log "no changes in stable report files; skipping commit"
  exit 0
fi

git commit -m "$COMMIT_MESSAGE"
new_sha="$(git rev-parse HEAD)"

# Push without auto-rebase. A non-fast-forward is a hard failure.
if ! git push; then
  log "git push failed (possibly non-fast-forward); not rebasing"
  exit 1
fi

emit committed "true"
emit commit_sha "$new_sha"
log "committed stable reports as $new_sha"
