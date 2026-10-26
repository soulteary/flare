# Flare 0.6.6 published artifact verification

Verified on 2026-10-26 (Asia/Shanghai). [Flare 0.6.6](https://github.com/soulteary/flare/releases/tag/0.6.6)
is a published stable release from `26596ab40ba9960bdcc8803719e211b45e2c26c8`.
The signed numeric tag resolves to that commit. Later documentation/report
commits are not the source of the released binaries.

The [tag Release workflow](https://github.com/soulteary/flare/actions/runs/40287730564)
succeeded, including GoReleaser, Docker Hub, the four-platform GHCR mirror and
release-report dispatch. Before tagging, all checks passed on both the
[fix PR #179](https://github.com/soulteary/flare/pull/179) and its merged main
commit. The PR's [seven-archive/four-platform preflight](https://github.com/soulteary/flare/actions/runs/40286841454)
also passed; all-package race coverage was 86.4%, above the 85% gate.

Tag-specific [quality reporting](https://github.com/soulteary/flare/actions/runs/40287730597)
also succeeded and published report commit `2136742`: 308 passing tests, zero
failures and one opt-in test skipped. Its separate default coverage instrumentation
records 73.29%; this is not the CI gate's `-coverpkg=./...` measurement and must
not be substituted for the 86.4% acceptance result.

## Public archives

Downloaded all seven standard Darwin/Linux archives plus the checksum file.
Every archive matches both the published SHA-256 and GitHub asset digest, contains
`flare`, `LICENSE` and `README.md`, and has the expected binary architecture,
GOARM setting where applicable, linked version `0.6.6` and full source commit.
Darwin ARM64 was executed; inspecting the other six is not native-device execution.
See [archive and registry evidence](../metrics-reports/release-0.6.6/published-verification.json).

All binaries report `vcs.modified=true`. The actual value is retained in the
evidence. An isolated checkout of the exact release commit regenerated assets
without tracked changes; a clean build stamped false, and adding only untracked
`dist/metadata.json` stamped true. Official logs place Git validation before
metadata generation and compilation. This explains the generated-output stamp;
it must not be relabeled as false. See [provenance reproduction](../metrics-reports/release-0.6.6/vcs-provenance.json).

## Public images

Independent anonymous registry reads verified both version `0.6.6` and `latest`
at Docker Hub and GHCR share this manifest digest:

```text
sha256:4a33de00c2fa4f729c18011ad62a4779a3e0e834e9a2dd6fcdf9175e32f48a25
```

Both contain exactly `linux/amd64`, `linux/arm64`, `linux/arm/v7` and `linux/arm/v6`.
Each platform config matches the expected architecture, source URL, version and
full release revision. Mutable tags were checked again after reading configs.

Using an empty client authentication configuration, the actual ARM64 images were
pulled anonymously from both registries and run on the local ARM64 Linux VM.
Each passed HTTP, server/custom timezone and restart checks, plus all 19 legacy
MDI icon previews before and after restart. Test containers were removed.
See [published container evidence](../metrics-reports/release-0.6.6/container-smoke.json).

## Compatibility, upgrade and rollback

The actual published Darwin ARM64 executable returned all 19 retired MDI SVG
paths correctly. Themed favicon GET, HEAD, conditional 304, byte range 206 and
theme-switch content URLs passed. See [native HTTP evidence](../metrics-reports/release-0.6.6/native-http-smoke.json).

A disposable synthetic legacy data directory was run using the actual published
0.6.5 and 0.6.6 binaries. Read-only startup preserved every data byte. Search writes
preserved custom fields and the original backup; editor writes and stale-conflict
rejection worked. Sessions, navigation and settings survived restart. Restoring
the complete pre-upgrade backup allowed 0.6.5 rollback with original data and
session continuity. See [published-binary upgrade evidence](../metrics-reports/release-0.6.6/upgrade-verification.json).
This is synthetic-data acceptance, not an exercise on a production user's data.

## Disclosed limits

Physical Safari/Via/Android/iPad and Raspberry Pi execution remain NOT TESTED.
The release owner authorized publication with this limitation retained; follow-up
owner is repository maintainer soulteary. No physical-device status is inferred
from Chromium, an ARM64 VM or QEMU. Continue to back up the complete data directory
and external configuration before upgrade and restore the original backup on
rollback. See [final acceptance](release-0.6.6-final-preflight.md).
