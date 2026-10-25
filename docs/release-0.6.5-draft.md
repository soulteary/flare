# Flare 0.6.5 — draft release notes

> Archived preparation record. Flare 0.6.5 has since been published; see the
> [final release notes](https://github.com/soulteary/flare/releases/tag/0.6.5).
> The original candidate-time statements below are retained as historical evidence.
> Published artifact checks and remaining limits: [verification record](release-0.6.5-verification.md).

Unpublished draft. Candidate scope is changes since 0.6.4; version and final
commit must be confirmed by the release process.

## Features

- Add the navigation editor with list, form and batch views, category management,
  icon selection, unsaved-change protection, version-conflict handling and draft
  export/restore. Improve Chinese and English editor localization.
- Add configurable search engines and hybrid search, category navigation and
  mobile/desktop layout controls.

## Correctness and reliability

- Escape navigation text and attributes and enforce supported URL/image schemes.
- Keep dynamic URL metadata request-local, preserve encoded redirects and source
  context, and fix IPv6 and Base64 handling.
- Preserve authentication settings when unrelated environment values are invalid;
  repair deprecated boolean CLI aliases and validate unsafe request sources.
- Preserve failed editor/settings submissions and guard recovery data against
  unintended default-file initialization.
- Refresh weather asynchronously with coalescing, stale snapshots and failure
  backoff; avoid showing fabricated values before data is available.
- Make static resource caching safe across upgrades with content-derived ETags
  and versioned resource URLs.

## Performance

- Reuse immutable parsed configuration/navigation snapshots, reduce category
  rendering work and avoid processing hidden modules or unmatched icons.
- Reduce editor filtering, repeated row construction, listeners and dirty checks.
- Cache immutable resource metadata and avoid loading the host MIME registry for
  bundled assets.
- In the documented default-data Linux benchmark, CPU per request fell
  59.6–81.1% and allocated bytes per request fell 55.2–94.5% versus pre-audit main
  `39df281` (not 0.6.4). RSS was mixed; some HTML cases increased 0.5–3.3%.
  See [the full report](../metrics-reports/audit-resource-comparison-2026-10-26.md).

## Upgrade notes and limitations

- Back up the complete data directory, Compose/environment settings and external
  secrets before upgrading; test restoration separately.
- Unsupported URL protocols remain in stored data but render as `#`. Icons
  accept bundled identifiers or absolute HTTP(S) image URLs. Administrator Footer
  HTML remains trusted administrator content.
- Per-entry private flags do not enforce per-entry access control. Protect a
  private deployment with whole-site authentication.
- Reverse proxies must preserve the request context needed by protected writes.
  Full subpath hosting is not implemented.
- Physical Safari/Via/Android/iPad acceptance remains unverified at draft time;
  update this statement only from actual device evidence.
- CPU/allocation improvements do not establish universally lower retained memory,
  browser latency or ARM performance.

Documentation now covers configuration precedence, backup/recovery, proxy limits,
troubleshooting, development builds and measurement methods.
