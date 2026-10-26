# Flare 0.6.6 release preflight

Preparation only, recorded 2026-10-26 (Asia/Shanghai). No tag, release, push or
registry publication is authorized by this preflight. Current stable is 0.6.5.

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

Results and candidate SHA are recorded below after execution.
