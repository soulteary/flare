# Go report workflow

The PR job runs both owned Actions with `commit: false` and released CLIs
(`go-test-report-action v1.1.2`, `goreportcard-action v1.1.3`, pinned wrapper SHAs).
It replaces the primary Linux Go test/coverage invocation and reuses its profile
for existing coverage artifacts/gates. Release verification, integration,
benchmark and fuzz checks retain their distinct scopes.

Reports run on pushed tags, published releases and `workflow_dispatch`. They do
not generate bot commits on branch pushes. A separate job may commit the five stable reports in
one report commit, only if the tested commit still equals the default branch
head. Older release tags and non-default manual refs retain downloadable reports
without overwriting the current branch. PRs never enter that write job. Protected
branches can reject writeback; no PAT, bypass, or force-push is used.

Test directory: `.`. Packages: `./...`.
Race detector: `true`. Additional args: `-count=1`.

The PR test job in `ci.yml` enforces **85%** all-package statement coverage
with `-coverpkg=./...`. The separate `go-quality-reports.yml` reporting workflow
uses a **0%** coverage floor to record release/manual baselines; it does not
replace the PR acceptance gate. Neither workflow sets a per-package coverage
floor. The quality-score floor is `0%`.
Raw profiles/test logs are artifacts, never committed.

Published releases and automated release dispatches reuse only completed successful tag-push report runs for the tested SHA that still have an unexpired stable report artifact. An artifact from an earlier attempt of the same run is valid after a partial retry (`artifact_attempt <= run_attempt`). Active, failed, cancelled, missing, or expired runs do not suppress fresh reports. API errors fail the gate rather than silently assume evidence exists.

The producing reports job outputs its artifact name. Writeback uses that output even when only failed jobs are rerun; it never substitutes the consuming attempt number. The five report files restore the `.github` artifact root.

Report concurrency includes the source ref and unique run ID, so pending reports cannot replace each other. Writeback concurrency includes the tested SHA, so historical releases cannot replace the current source's pending writer. Only the unchanged default-branch source can publish; identical-source writers serialize and a successful report commit makes later writers stale. Artifacts remain available when writeback is skipped or blocked by branch protection.

Cleanup runs even after preceding steps fail. Stable and raw evidence is uploaded after gate failures, and failed report jobs cannot publish repository changes. Report commits have no CI skip annotation: the normal workflow token prevents recursive push triggers. Production Actions and CLI versions remain pinned to formal releases; PR report calls remain read-only.

Successful token-published releases explicitly dispatch the current default-branch report workflow for their released tag, since token-generated release events do not start another workflow. Branch snapshots and PR release checks do not dispatch report publishers. Historical tags can also be reported manually from the default-branch workflow using `source_ref`.

## Reading report evidence

Use [PR/main CI runs](https://github.com/soulteary/flare/actions/workflows/ci.yml)
and [release/manual report runs](https://github.com/soulteary/flare/actions/workflows/go-quality-reports.yml)
to select the tested commit, run and attempt before interpreting a report.
Check the generation settings and coverage instrumentation in that workflow.
Reports checked into Git are snapshots; their numbers do not certify a later
commit. The README links to workflows instead of displaying undated report badges.

The historical 0.6.5 report snapshot was committed by
[`c3106b0`](https://github.com/soulteary/flare/commit/c3106b0fbe2fa18fc54fae1c0cb1bc94efb5087c)
for source [`a2ed02e`](https://github.com/soulteary/flare/commit/a2ed02e0085df283fd1a6a685b97fad3b6377b78)
(0.6.5). Its report commit is dated 2026-10-26 in Asia/Shanghai.
The JSON and SVG files in that report commit belong to the same snapshot; they
are not current-main results. New report snapshots must be interpreted against their
producing workflow run and source, not these historical numbers.
