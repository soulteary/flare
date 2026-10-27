# Flare 0.6.5 release verification

> Archived historical record. For current instructions use the [documentation index](../../README.md).
> Original candidate facts and measurements below are retained; they do not certify current main.

Recorded on 2026-10-26 (Asia/Shanghai). The published stable release is
[0.6.5](https://github.com/soulteary/flare/releases/tag/0.6.5), built from
`a2ed02e0085df283fd1a6a685b97fad3b6377b78`.

The tag-triggered [Release workflow](https://github.com/soulteary/flare/actions/runs/40242139232)
completed successfully, including GoReleaser, Docker Hub publication, GHCR copying
and release-report dispatch. Later documentation/report commits are not the
source of these published binaries.

## Published archives

All seven archives match the published SHA-256 checksum file and contain `flare`,
`LICENSE` and `README.md`. Binary formats/architectures and embedded version/commit
metadata were inspected for Darwin amd64/arm64 and Linux 386/amd64/arm64/armv6/armv7.
Only Linux amd64 was executed; inspection is not cross-platform runtime testing.
See the [archive evidence](../../../metrics-reports/release-0.6.5/archives-verification.json).

The published Linux amd64 binary passed HTTP checks for homepage, SVG/XML, repeat
GET, HEAD, 304, Range 206, invalid Range 416/no-store, missing-resource 404/no-store,
theme-specific icon changes and failed asset request logging. Its version and
commit banner also matched the release. See the
[HTTP evidence](../../../metrics-reports/release-0.6.5/binary-http-smoke.json).

## Published images

Docker Hub and GHCR version `0.6.5` and `latest` all resolved to the same manifest
digest at verification time:

```text
sha256:44948e8c1c3d4c4743cd1db907993feec89362391f69ea87ed9d75a0710b60bf
```

Independent read-only anonymous registry checks verified the four platforms
`linux/amd64`, `linux/arm64`, `linux/arm/v7` and `linux/arm/v6`, their image-config
architectures and version/source/commit labels. Mutable tags were rechecked after
reading the configs. See the [registry evidence](../../../metrics-reports/release-0.6.5/registry-verification.json).

The actual published amd64 `/bin/flare` extracted from each registry was byte
identical, SHA-256 `6885d07b3269d435f9e18a9f41a8829a0d1b1aa3d3d72702523bb75d74e87360`.
The extracted image executable passed the same host HTTP smoke checks. This is
**not** Docker-engine container execution or a container mount/network test.
See the [binary comparison](../../../metrics-reports/release-0.6.5/container-binary-verification.json)
and [extracted-binary HTTP evidence](../../../metrics-reports/release-0.6.5/container-binary-http-smoke.json).

## Upgrade, restart and rollback scope

An isolated disposable copy of repository sample YAML was run with the actual
0.6.4 and 0.6.5 Linux amd64 binaries. Private anonymous access remained protected,
invalid login was rejected, authenticated home/applications/bookmarks/editor
routes worked, and the existing session survived upgrade and restart. Data-file
and cookie-key hashes remained unchanged. Restoring the complete pre-upgrade
backup permitted the 0.6.4 rollback run. See the
[upgrade evidence](../../../metrics-reports/release-0.6.5/upgrade-verification.json).

This exercise used sample data, not a production user's data, and did not mutate
editor content. Editor save/conflict coverage is provided separately by CI; this
exercise does not establish mutation compatibility or rollback of new settings.
Continue to back up the complete data directory and external configuration before
upgrading, and verify restoration in a disposable deployment.

## Remaining validation limits

Physical Safari, Via, Android, iPad and Raspberry Pi execution remain **NOT
VERIFIED**. Chromium acceptance, architecture inspection and Linux amd64 host
execution do not replace those checks. The [device matrix](../../mobile-device-acceptance.md)
retains the unverified environments.

The [performance report](../../../metrics-reports/audit-resource-comparison-2026-10-26.md)
compares pre-audit main, not release 0.6.4. CPU/allocation gains do not establish
universally lower RSS or ARM performance. This published-artifact record
supersedes the preparation draft and checklist; the original preparation material
remains available in Git history.
