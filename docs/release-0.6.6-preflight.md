# Flare 0.6.6 release preflight

Preparation only, recorded 2026-10-26 (Asia/Shanghai). No tag, release or registry
publication is authorized by this preflight. A preparation branch/draft PR may be
pushed for read-only CI acceptance. Current stable is 0.6.5.

The candidate includes search/configuration safety improvements after 0.6.5,
weather 0.4.0 with explicit location selection, retirement of search keyword
shortcuts and adaptive bookmark columns. Generated production assets are part of
the candidate. Unrelated `output/imagegen` assets are excluded.

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
- Review the [unpublished release notes](release-0.6.6-draft.md), especially the
  keyword and weather selection changes.
- Physical mobile environments in [the matrix](mobile-device-acceptance.md)
  remain NOT TESTED until actual evidence is supplied. Any release-owner exception
  must explicitly record the limitation and follow-up ownership.
- Local archive checks do not prove GoReleaser release or registry publication.
  After a separately authorized release, verify seven archives, checksums, both
  registries' four platforms, version/commit labels, anonymous pulls and `latest`.

## Candidate and decision

Candidate source: `995666d34f430d6d8e8e00a26f88f89bfb1eafb5` on local branch
`codex/release-0.6.6-preflight`. Later evidence/documentation commits do not change
the tested application. Status: **local automated acceptance passed, held for
review; not cleared for publication**. No tag, GitHub release or registry
publication was performed. Existing `output/imagegen` content remains untracked
and outside the candidate.

The full application race/coverage suite and browser suite ran on `6ffcafa`;
only the new packaging/upgrade harness was corrected afterward. A comparison of
`config`, `internal`, `embed`, `.github`, `go.mod` and `go.sum` confirms identical
application contents at `995666d`. Final archives and a second upgrade/rollback
exercise use `995666d` explicitly. A future application change invalidates these
results and requires relevant checks again.

## Completed checks

- Full Go race suite passed with `-coverpkg=./...`: **86.5%** statement coverage,
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

Machine-readable results and logs: [checks](../metrics-reports/release-0.6.6-preflight/checks.json),
[archive verification](../metrics-reports/release-0.6.6-preflight/archives-verification.json).
Candidate archives remain local at `/tmp/flare-066-final`; reproduce them with
the committed preflight script rather than committing binaries to the repository.

## Upgrade, mutation, restart and rollback

Downloaded the actual published 0.6.5 Darwin ARM64 archive and verified its
published SHA-256:
`6f12c9dae60ebb8beaea1dbb635316a65ede5591d511d9f6d16b1313d13c48a7`.

The [repeatable upgrade exercise](../tests/release/upgrade.cjs) used an isolated
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
is not certified. See [upgrade evidence](../metrics-reports/release-0.6.6-preflight/upgrade-verification.json).

## Containers and security

The configured AMD64 and ARM64 Dockerfiles built successfully on the local
container engine. Both images passed actual HTTP, custom Kathmandu/server
New York timezone and container restart checks. Local image labels use
`0.6.6-preflight` and the full candidate SHA; embedded binary version is `0.6.6`.
AMD64 runs under the local engine's emulation; ARM64 runs on the ARM64 VM.
See [container evidence](../metrics-reports/release-0.6.6-preflight/container-verification.json).

ARMv6 and ARMv7 image builds stopped at `/bin/sh` with `Exec format error` before
running Flare. Their Go archives built successfully, but this engine cannot
execute the ARM32 build stages. Do not infer image/device acceptance from that.
Complete both builds and runtime smoke checks in a QEMU-capable environment;
the existing Release workflow sets up QEMU, but its future success is unverified.

Clean-source gosec scans of both 0.6.5 and the candidate reported **16 findings**.
Comparing rule, relative file and description produced no new groups. Existing
alerts include trusted administrator/generated HTML, generated icon constants,
builder paths/permissions, configuration file permissions and a benchmark tool
error check. This comparison is not a zero-findings assertion or a dependency
vulnerability audit. See [security comparison](../metrics-reports/release-0.6.6-preflight/security-comparison.json).

## Still required before a separately authorized release

1. Build and smoke-test ARMv6/ARMv7 images in a QEMU-capable environment.
2. Record physical Safari/Via/Android/iPad acceptance, or an explicit release-owner
   exception with named follow-up ownership. No exception is granted here.
3. Review the candidate/notes and require remote CI and CodeQL on the exact final
   reviewed commit. The draft PR runs the new `Release preflight (no publication)`
   workflow: seven archive checks and four actual release Dockerfile builds and
   HTTP/timezone/restart checks with QEMU. It has read-only repository permissions,
   performs no registry login and contains no push or release step. Remote results
   are recorded separately after execution.
4. Only after explicit publication authorization, use the reviewed source and
   numeric tag `0.6.6`, then verify public artifacts and registry manifests.
