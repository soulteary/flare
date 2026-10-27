# Flare 0.6.6 release preflight

> Archived historical record. For current instructions use the [documentation index](../../README.md).
> Original candidate facts and measurements below are retained; they do not certify current main.

> Historical candidate record. Version 0.6.6 is now published; see
> [final acceptance](release-0.6.6-final-preflight.md) and
> [published verification](release-0.6.6-verification.md). Candidate-time statements
> below are preserved as historical evidence.

Preparation only, recorded 2026-10-26 (Asia/Shanghai). No tag, release or registry
publication is implied by this preflight. The preparation was merged into `main`
through [PR #172](https://github.com/soulteary/flare/pull/172) at
`db57b9d22cc81304d11a9b1a14d307f3ae1cb1c3`. The stable release at candidate review was 0.6.5.
Evidence below identifies its tested candidate; it does not certify subsequent
changes or publication readiness of the current branch.

The candidate includes search/configuration safety improvements after 0.6.5,
weather 0.4.0 with explicit location selection, retirement of search keyword
shortcuts and adaptive bookmark columns. Generated production assets are part of
the candidate.

## Reproduction

1. Generate resources with `go run build/build.go`; commit the candidate and check
   that a second generation leaves tracked files unchanged.
2. Run the full race/coverage suite with the same `-coverpkg=./...` instrumentation
   as CI; require at least 85%. Run vet, formatting and workflow safeguard tests.
3. Run the browser acceptance steps from `.github/workflows/clock.yml`, including
   `FLARE_WEATHER_BROWSER=1 go test ./internal/settings/weather -run '^TestWeatherBrowser$' -count=1 -v`.
4. Run `python3 scripts/release-preflight.py --version 0.6.6 --output /tmp/flare-066-preflight`.
   This builds seven local archives and inspects checksums, contents and embedded
   metadata. It does not run GoReleaser or publish containers.
5. Download the published 0.6.5 host archive and verify its published SHA-256.
   Run `tests/release/upgrade.cjs` with `FLARE_OLD_BINARY`, `FLARE_BINARY` and
   `FLARE_RELEASE_EVIDENCE` pointing to isolated binaries and an output JSON path.
   The exercise creates and deletes only disposable synthetic data.

## Publication gates

- All automated evidence must identify the fixed candidate commit.
- Review the [release notes](release-0.6.6.md), especially the
  keyword and weather selection changes.
- Physical mobile environments in [the matrix](../../mobile-device-acceptance.md)
  remain NOT TESTED until actual evidence is supplied. A release exception
  requires a recorded limitation and follow-up owner.
- Local archive checks do not prove GoReleaser release or registry publication.
  After publication, verify seven archives, checksums, both
  registries' four platforms, version/commit labels, anonymous pulls and `latest`.

## Candidate and decision

Candidate source: `6296c81c5e4e19c3d2c6a8ccf2e34950ad84ce96` on branch
`codex/release-0.6.6-preflight`, reviewed through [PR #172](https://github.com/soulteary/flare/pull/172)
and subsequently merged into `main`. Later evidence/documentation commits in that
PR did not change the tested application. Status: **candidate automated acceptance
passed and preparation merged; publication gates remain open**. No tag, GitHub
release or registry publication was performed by this preflight.

After synchronizing main, the full application race/coverage suite ran on
`f2f9429`. Application contents (`config`, `internal`, `embed`, `go.mod`, `go.sum`)
are identical at `6296c81`; subsequent changes fix the preflight harness. Final
archives and the upgrade/rollback exercise use `6296c81` explicitly. Browser
acceptance runs directly against that archive's host binary. A future application
change invalidates these results and requires relevant checks again.

## Completed checks

- Full Go race suite passed with `-coverpkg=./...`: **86.4%** statement coverage,
  exceeding the 85% CI gate. Vet, Go formatting and golangci-lint 2.15.0 passed.
- All 28 clock/editor state/mobile focus tests passed. All 12 existing production
  browser suites passed: adaptive columns, compact layout, category navigation,
  readability, desktop columns, editor scale, clock, mobile editor, navigation
  editor, hybrid search, legacy bookmarks and icon picker. The editor suite also
  exercises conflicts, failed saves and draft recovery.
- Weather picker browser regression passed at desktop and mobile viewports.
  Live-provider tests passed for Beijing, the old 北京市 spelling, ambiguous 朝阳
  candidates and 津市; migration preserves ambiguous/unresolved inputs. Full
  authenticated production-router save/query tests passed. Live desktop/mobile
  browser selection, persistence and weather query also passed. These are Chrome
  tests, not physical Safari/Via certification.
- Rebuilding production resources twice produced no additional tracked diff.
  Workflow safeguard tests passed **19/19**; actionlint 1.7.12 and GoReleaser
  2.19.0 configuration checks passed. GoReleaser reports the existing
  `dockers`/`docker_manifests` deprecation; it is not a config failure.
- Seven local candidate archives passed contents, binary architecture,
  version/commit metadata and SHA-256 inspection. Each contains `flare`,
  `LICENSE` and `README.md`. Cross-build is not target-device execution.
- Actual GoReleaser `--snapshot --skip=docker` passed generation, seven builds,
  archives and checksum generation in an isolated synthetic Git checkout. Its
  snapshot version is `0.6.6-preflight`; its synthetic Git SHA is not the candidate
  SHA. The separately built archives below carry the authoritative candidate
  metadata. Neither path invokes a publisher.

Machine-readable results and logs: [checks](../../../metrics-reports/release-0.6.6-preflight/checks.json),
[archive verification](../../../metrics-reports/release-0.6.6-preflight/archives-verification.json).
Candidate archives remain local at `/tmp/flare-066-complete`; reproduce them with
the committed preflight script rather than committing binaries to the repository.

## Upgrade, mutation, restart and rollback

Downloaded the actual published 0.6.5 Darwin ARM64 archive and verified its
published SHA-256:
`6f12c9dae60ebb8beaea1dbb635316a65ede5591d511d9f6d16b1313d13c48a7`.

The [repeatable upgrade exercise](../../../tests/release/upgrade.cjs) used an isolated
synthetic legacy configuration with custom YAML extension fields, old keyword
settings, a private application, category IDs, descriptions and an exact URL.
The candidate retained all bytes on read-only startup. Saving search settings
updated the version/mode, preserved custom fields and retained the original
configuration in the permanent first-upgrade backup. Editor changes persisted;
a stale second document was rejected with 409. Navigation, configuration and
authentication survived restart. Restoring the complete pre-upgrade directory
allowed the actual old binary to run with the original document and session.

The exercise uses offline weather to keep backup checks deterministic. The
separate live-provider tests above cover weather migration and selection. No
production user data was used. Rollback without restoring the original directory
is not certified. See [upgrade evidence](../../../metrics-reports/release-0.6.6-preflight/upgrade-verification.json).

## Containers and security

The configured AMD64 and ARM64 Dockerfiles built successfully on the local
container engine. Both images passed actual HTTP, custom Kathmandu/server
New York timezone and container restart checks. Local image labels use
`0.6.6-preflight` and the full candidate SHA; embedded binary version is `0.6.6`.
AMD64 runs under the local engine's emulation; ARM64 runs on the ARM64 VM.
See [container evidence](../../../metrics-reports/release-0.6.6-preflight/container-verification.json).

Local ARMv6 and ARMv7 image builds stopped at `/bin/sh` with `Exec format error` before
running Flare. Their Go archives built successfully, but this engine cannot
execute the ARM32 build stages. Do not infer image/device acceptance from that.
This gap was subsequently resolved by the no-publication QEMU CI preflight.
All four image jobs passed in [run 40278421194](https://github.com/soulteary/flare/actions/runs/40278421194)
at PR head `dcf2394`, including actual ARMv6/ARMv7 builds and HTTP/timezone/restart
checks. The initial run's four restart failures were a harness error: Docker
reallocated random host ports; rereading the port after restart fixed all four.
This is emulated CI execution, not Raspberry Pi/device validation.

The first local archive script also omitted ignored generated editor/guide
assets when building from a clean Git snapshot. Direct archived-binary browser
checks exposed the missing batch editor code. The script now generates production
assets inside the snapshot; the archive CI job runs both editor and legacy batch
acceptance against the packaged binary. Earlier incomplete local archives are
superseded by the `6296c81` archive evidence.

Clean-source gosec scans of both 0.6.5 and the candidate reported **16 findings**.
Comparing rule, relative file and description produced no new groups. Existing
alerts include trusted administrator/generated HTML, generated icon constants,
builder paths/permissions, configuration file permissions and a benchmark tool
error check. This comparison is not a zero-findings assertion or a dependency
vulnerability audit. See [security comparison](../../../metrics-reports/release-0.6.6-preflight/security-comparison.json).

## Remaining publication gates at candidate review

1. Record physical Safari/Via/Android/iPad acceptance, or an explicit release-owner
   exception with named follow-up ownership. This candidate record contains no
   approved exception.
2. Review the candidate/notes and require remote CI and CodeQL on the exact final
   reviewed commit. PR #172 ran the `Release preflight (no publication)`
   workflow: seven archive checks and four actual release Dockerfile builds and
   HTTP/timezone/restart checks with QEMU. It has read-only repository permissions,
   performs no registry login and contains no push or release step. At PR head
   `6296c81`, CI, CodeQL, Security Scan, Clock acceptance and Release preflight
   all passed. Seven archive and all four image jobs passed, including editor
   browser tests against the packaged Linux binary. See [remote CI evidence](../../../metrics-reports/release-0.6.6-preflight/remote-ci.json).
   Before release, check CI for the exact final source being tagged, including
   any merge or subsequent changes; candidate-head evidence alone does not certify
   a later commit.
3. After release approval, tag the reviewed source as `0.6.6`, publish it, and
   verify public artifacts and registry manifests.
