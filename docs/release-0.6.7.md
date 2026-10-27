# Flare 0.6.7 release guide

Stable 0.6.7 was published on 2026-10-27 at 00:34 Asia/Shanghai.
Source: [published release notes and verification records](https://github.com/soulteary/flare/releases/tag/0.6.7)
(release metadata as of 2026-10-27).
The acceptance evidence below applies to the published release and its recorded
builds and environments.

## Included features

- Chinese and English translations across settings, editor, transfer and errors.
- [Columns within bookmark categories](desktop-columns-and-scale.md#columns-within-bookmark-categories-since-067).
- Better editor icon contrast, and MDI cheat-sheet copying with Flare icon names.
- Rejected write sources distinguished from expired sessions, with escaped
  navigation error text.
- Static-analysis and benchmark-profile output safeguards and restored status badges.

Persistent accounts, revocable server-side sessions and enforced per-entry/category
privacy are current-main development features, not part of this release. Stable
0.6.7 private deployments still need `FLARE_DISABLE_LOGIN=false` together with
`FLARE_VISIBILITY=PRIVATE`.

## Install and upgrade

```sh
docker pull soulteary/flare:0.6.7
docker pull ghcr.io/soulteary/flare:0.6.7
```

Stop writers and back up the full data directory and external deployment settings
before upgrading. Recreate containers after changing images/environment and refresh
open settings/editor pages. Use [tagged documentation](https://github.com/soulteary/flare/tree/0.6.7/docs)
for stable behavior; read the [development upgrade guide](login-private-upgrade.md)
only when intentionally moving to current-source security features.

## Published evidence and limits

The release records [CI](https://github.com/soulteary/flare/actions/runs/40358586814),
[browser acceptance](https://github.com/soulteary/flare/actions/runs/40358587694),
[CodeQL](https://github.com/soulteary/flare/actions/runs/40358586795),
[candidate preflight](https://github.com/soulteary/flare/actions/runs/40358785700) and
[publication](https://github.com/soulteary/flare/actions/runs/40359528854).
The candidate preflight used the workflow's older 0.6.6 test label; published
0.6.7 artifacts were built by the tag workflow. Refer to the release for recorded
archive, registry and ARM64 VM verification. Mutable `latest` tags require fresh
registry checks at deployment time; historical digests are not perpetual guarantees.
Physical Safari/Via/Android/iPad and Raspberry Pi testing remains unclaimed.
