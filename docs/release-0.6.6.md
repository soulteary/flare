# Flare 0.6.6

Published on 2026-10-26 (Asia/Shanghai) from
`26596ab40ba9960bdcc8803719e211b45e2c26c8`.
See [the GitHub release](https://github.com/soulteary/flare/releases/tag/0.6.6) and
[published artifact verification](release-0.6.6-verification.md).

## Changes since 0.6.5

- Add explicit hybrid, local-first and selector search modes with safe legacy
  defaults. Preserve local intent without automatically enabling external engines.
- Protect settings upgrades with version checks, revision conflicts, retained
  original/latest backups and atomic configuration replacement. Preserve unknown
  YAML fields and submitted settings on failure. Enforce 85% all-package coverage.
- Upgrade funny-china-weather to 0.4.0. Add authenticated location lookup and an
  explicit candidate picker using exact provider province/city/county names.
  Revalidate selected candidates on save; retain form input on errors. Uniquely
  resolvable existing locations can migrate without overwriting concurrent choices.
- Improve phone/tablet bookmark columns and weather settings layout.
- Unify YAML parsing and serialization on YAML v3, with strict bookmark/import
  validation and coverage for legacy boolean and numeric-looking string values.
  Remove unused legacy CSV editor code and editor/guide memory filesystems; old
  CSV submissions continue to receive 409 rather than bypass revision protection.
- Upgrade Material Design Icons to 7.4.47. Preserve all 19 retired identifiers
  from the 0.6.5 bundle, including Discord, DesktopMac and TabletAndroid, using
  their original SVG paths. They remain available in previews, navigation and
  the editor picker without rewriting stored bookmarks.
- Match the favicon to the darkest theme palette color. Generate content-hashed
  icons at build time and retain correct conditional/range HTTP behavior.
- Update CI actions to Node.js 24-compatible releases and verify seven archives
  plus four container platforms without publication during preflight.

## Behavior changes and upgrade instructions

- Search engine `!keyword` shortcuts are removed. All input is ordinary search
  text; choose an engine using the target controls. Old `EnableKeywords` and
  `Keyword` configuration fields are ignored by search.
- Empty weather locations require explicit selection in Settings. IP detection
  is a user-triggered suggestion. Ambiguous/unresolved old locations need an
  explicit selection; no administrative suffixes or county are guessed.
- Refresh any settings form opened before restart/upgrade before saving.
- Before upgrading, stop the application and back up the complete data directory,
  Compose/environment settings and external authentication secrets. Exercise the
  upgrade on a disposable copy. Managed search backups are not a substitute for a
  complete deployment backup. To roll back, stop the candidate and restore the
  entire pre-upgrade data directory and external configuration with the old binary.
  Do not rely on old binaries understanding newly written settings.

## Validation

The release owner authorized publication after the icon compatibility and release
documentation fixes, retaining the explicitly disclosed physical-device testing
limitation. Safari/Via/Android/iPad remain NOT TESTED; follow-up owner is repository
maintainer soulteary. Chromium emulation and QEMU execution are not physical-device
certification. No device result or completion date is implied by this exception.

See [final acceptance](release-0.6.6-final-preflight.md) and
[published verification](release-0.6.6-verification.md) for source, checks and artifact evidence. Physical
Safari/Via/Android/iPad and ARM-device execution must not be inferred from desktop
Chromium emulation or cross-compilation. Public archive and registry checks are recorded separately from preflight.
