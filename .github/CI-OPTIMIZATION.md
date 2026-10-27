# CI execution and cache ownership

## Go caches

`actions/setup-go` is invoked once per build/test job with its automatic cache
disabled. The local `actions/setup-go` composite owns two separate caches:

- Module downloads: OS, runner architecture, Go version and go.mod/go.sum hash.
  `go mod download` populates the complete module graph before a successful job
  saves this immutable cache. Formatting and archive aggregation never save it.
- Compiled packages: the same toolchain/dependency identity plus a task profile
  and source SHA. A miss restores the latest compatible task snapshot, and a
  successful job saves a new snapshot. Race, browser, CodeQL, release and each
  cross-build target have distinct profiles. No other step restores these paths.

The v2 namespace avoids the old tiny/overlapping cache entries. Caches accelerate
compilation; they never replace test execution. The race jobs retain `-count=1`,
all-package coverage and the 85% PR/main gate. Profile snapshots are subject to
GitHub's normal cache quota/eviction; compare restore/save time and cache size
with compile time after deployment, including both cold and warm runs.

Report wrappers use the previously pinned GTR v1.1.2 and GoReportCard v1.1.3
release CLIs, with mandatory checksum verification. They use the project's Go
toolchain, and GTR path validation reuses that installed CLI. Upstream execution
scripts, licensing and local changes are recorded in `actions/NOTICE.md`.

## Release preflight

The dependency graph is:

1. Prepare a committed source snapshot; generate production resources once;
   compare every tracked file to the checkout and bundle ignored generated
   editor/guide assets too. Record the source SHA, candidate version and date.
2. Build all seven targets with a maximum of four concurrent jobs. The four
   Linux container jobs consume the exact binary built for their archive.
   Container jobs start independently of the other archive targets; the longer
   Linux targets are listed first. There are seven production builds instead of
   the previous seven archive builds plus four separate image builds.
3. Verify the complete set of archives, checksums, binary hashes, target
   metadata and source identities. Extract the amd64 browser candidate from
   its archive and run the original editor, reliability and legacy suites.

Intermediate artifacts are scoped to this workflow run and source SHA and
retained for seven days, matching the final verified archives. Producer-owned
artifact names use overwrite on retry so full or partial reruns do not collide. Artifact transfer disables redundant ZIP compression for tar.gz.
These artifacts are never used by the publication workflow.

Snapshot generation and target builds use `-trimpath` so random extraction
directories do not invalidate otherwise reusable compiled packages. Candidate
executables embed the source commit date, while evidence separately records the
actual build time. This makes same-source executables reproducible and allows
BuildKit to reuse their compression layer. Formal GoReleaser release dates are
unchanged.

The existing `Seven candidate archives` and `Local image linux/...` check names
are preserved. The archive aggregate runs after failed dependencies too and
explicitly fails unless all target jobs succeeded, preventing a failed build
from turning a required check into a successful skip.

`workflow_dispatch` accepts a candidate version, defaulting to 0.6.6. Local
`scripts/release-preflight.py` still supports building all seven targets, with
two workers by default; `--target`, `--source-archive`, `--prepare-only` and
`--collect` support the CI graph. Python 3.12+ is required, as on ubuntu-latest.

## Docker and browser preparation

The ARM recipes run UPX in a builder pinned to `$BUILDPLATFORM`, avoiding target
CPU emulation for compression. The amd64 recipe keeps its existing x86 builder
because that Alpine release does not ship the same UPX package on arm64. UPX versions and `-9` remain unchanged. Install UPX before
copying the binary; install runtime timezone data before copying packed output.
Preflight imports/exports platform-specific BuildKit GHA caches with mode=max.
QEMU is installed only for foreign Linux container execution; startup, timezone
and restart checks still execute for all four platforms.

Both browser jobs use `npm ci` with the existing lockfile download cache and
install Chromium's headless shell plus system dependencies. No acceptance
scripts were removed, and public/private search modes remain separate.

## Trigger and publication safeguards

Main pushes use CI; Release runs only for stable `0.x.y` tags. The release job
validates the tag and peeled commit before using registry credentials. Tests,
GoReleaser checks/publication, GHCR digest/provenance/platform verification and
report dispatch/reuse stay in place. Manual Release must select a matching tag.

Only superseded PR runs are actively cancelled. Release, GHCR serialization and
report writeback retain their existing coordination. gosec retains its Docker
Action and SARIF behavior; unused host Go setup/download steps are removed.
Push filtering includes dependencies, config, generators and embedded assets.

Actual GitHub cache hit rate and end-to-end improvement require post-deployment
CI measurements. Local compilation and container checks do not measure GHA
cache transfer, hosted runner startup, or matrix scheduling.

## Hosted-runner measurements

The controlled same-commit cold/warm comparison is recorded in
[the 2026-10-27 measurement report](../metrics-reports/ci-cache-comparison-2026-10-27.md).
Both complete runs passed ARM v6/v7 acceptance. Warm Go module and compiler
cache restores were exact hits in all 13 consuming jobs; all four UPX layers
were cached, with identical executable hashes for all seven targets.
