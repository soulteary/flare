# Development release readiness after 0.6.4

> This preparation checklist is retained from the work leading to
> [0.6.5](https://github.com/soulteary/flare/releases/tag/0.6.5). Its unchecked
> boxes are not a live release-status dashboard. Physical-device validation
> remains unverified; see the release notes for the published limitations.

This page is a checklist, not a release announcement. No next version or new
image is implied. The release owner records the final source SHA, image digest,
checks and browser evidence before publishing.

- [ ] Merge reviewed correctness fixes and verify CI at the merged commit.
- [ ] Rebuild production assets; verify list/forms/batch, icons and old/new cache behavior.
- [ ] Exercise concurrent public/private navigation and dynamic redirects.
- [ ] Verify save errors, stale versions, draft restore and recovery instructions.
- [ ] Measure CPU, allocations, retained heap, idle/peak RSS and P95 under the same fixtures.
- [ ] Run search, clock, category/layout combinations and browser acceptance.
- [ ] Record physical Safari/Via/Android/iPad checks as PASS, FAIL or NOT TESTED.
- [ ] Verify release archives and multi-platform images; distinguish cross-build from device execution.
- [ ] Replace development availability labels with the actual release version.
- [ ] Update deployment README, screenshots and issue resolution links only for shipped scope.

The [device matrix](mobile-device-acceptance.md) currently has no physical results.
Unavailable devices remain NOT TESTED with follow-up ownership; automation must not
convert Chromium touch emulation into a Safari/Via pass. Follow the existing
release policy for deciding whether remaining evidence permits publication.
