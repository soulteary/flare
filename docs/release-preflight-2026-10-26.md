# Release preflight — 2026-10-26 (Asia/Shanghai)

> Archived preparation record. Flare 0.6.5 has since been published; see the
> [final release notes](https://github.com/soulteary/flare/releases/tag/0.6.5).
> The original candidate-time statements below are retained as historical evidence.
> Published artifact checks and remaining limits: [verification record](release-0.6.5-verification.md).

Status: preparation only; not a release announcement. Suggested next tag: `0.6.5`.
Latest published stable release checked: `0.6.4`. The `0.6.5` tag does not exist.
Candidate source: `1c5f4d611f7df49a64635d84dac8a249b64ff021`.
Any subsequent source or release-documentation commit requires recording the new
candidate SHA and checking its CI before tagging.

## Verified evidence

- No open source-repository PRs at the check.
- Candidate main CI, CodeQL and Clock acceptance completed successfully.
- Candidate Release run [40217502098](https://github.com/soulteary/flare/actions/runs/40217502098)
  passed generation/tests and Docker Hub login. GoReleaser, GHCR publication and
  release-report dispatch were **skipped** on this branch run. This is not proof
  that candidate release archives or images have been built or published.
- Security Scan passed on final report PR head
  `fc2d21f24e154041432bf08bcdb5d50f8d11ab7e` and preceding implementation merge
  `a116b71613ccfff353bfd86affa741c51d8c339c`; no scan run at candidate main was
  returned by the latest scan-workflow query. Do not label it an exact-SHA pass.
- The audit's 15 PRs were merged after final clean review/checks. Release scope
  also includes earlier editor/search/layout work since 0.6.4: the complete local
  tag-to-candidate comparison has 144 commits and changes 195 files.
- CPU/heap/RSS results and limitations are recorded in
  [the comparison report](../metrics-reports/audit-resource-comparison-2026-10-26.md).
  Its baseline is pre-audit main `39df281`, **not** release 0.6.4.

## Open release work

| Item | Status / required result |
| --- | --- |
| Physical mobile acceptance | All five environments in [the matrix](mobile-device-acceptance.md) remain unverified: two iPhone/Safari classes, Android/Chrome, Android/Via and iPad/Safari. The document explicitly calls this a release gate. Complete evidence or record an explicit release-owner exception and follow-up ownership; do not silently waive it. |
| Release checklist | [Readiness checklist](release-readiness.md) still has all boxes unchecked; reconcile each with actual evidence. Existing automated checks and benchmarks must not be presented as missing implementation work. |
| Version and availability documentation | README, docs index/configuration and Chinese deployment docs still accurately describe stable 0.6.4 versus development. Prepare a coordinated version update and only describe the next version as published when artifacts exist. |
| Release notes | English draft is in [release-0.6.5-draft.md](release-0.6.5-draft.md); include earlier features since the tag, not only the audit PRs. |
| Candidate artifact preflight | Generate production assets, cross-build the seven configured archive targets, inspect archive contents/checksums/version metadata and smoke-test executable targets. A config check alone is insufficient. |
| Upgrade and rollback evidence | Exercise a disposable copy of real 0.6.4 data; verify authentication, navigation, search, editor save/conflict/draft recovery and persistence across restart. Preserve the original entire data directory and externally configured secrets. Restore original data on rollback; do not assume an older binary understands every new setting. |
| Tag publication | Not performed. Use the final reviewed SHA and stable numeric tag without `v`; watch the tag-triggered Release workflow. |
| Published artifact verification | After publication: seven archives plus checksums; Docker Hub and GHCR four platforms (`amd64`, `arm64`, `arm/v7`, `arm/v6`); commit/version labels; public anonymous pulls; matching version manifest digests; correct `latest`; tag-specific quality reports. Record digests and smoke-test pulled images. |

The deployment repository also has two older open documentation PRs (#153 and
#69). They are not part of the audit batch and have not been reviewed here as
release blockers. Reconcile overlapping documentation before separately merging.

## Follow-up work that does not by itself block this release

- RSS is not uniformly lower: several default-data HTML cases remain 0.5–3.3%
  above the measured pre-audit baseline. Preserve the disclosed tradeoff rather
  than claiming a universal memory reduction.
- Larger data, real workload mixes and ARM/Raspberry Pi runtime measurements
  remain useful; cross-compilation does not establish device execution.
- Custom uploaded icons, multiuser access, LAN/WAN dual addresses, browser capture
  API and full subpath deployment remain separate product work, not promised scope.
