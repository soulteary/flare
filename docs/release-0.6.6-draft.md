# Flare 0.6.6 — unpublished release draft

This is preparation material. No 0.6.6 tag, release or registry image is implied.
Current published stable remains 0.6.5.

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

See [the 0.6.6 preflight record](release-0.6.6-preflight.md) for the exact candidate,
automated checks, packaging evidence and unresolved publication gates. Physical
Safari/Via/Android/iPad and ARM-device execution must not be inferred from desktop
Chromium emulation or cross-compilation. Published-artifact verification remains
a separate post-publication step.
