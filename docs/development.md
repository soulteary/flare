# Build and test

Use the Go toolchain declared in `go.mod`. Browser acceptance uses Node.js 24
and the locked dependencies in `tests/clock/package-lock.json`. Run from the
repository root:

```sh
go run build/build.go
go build -o /tmp/flare .
go test -race -count=1 -coverpkg=./... -coverprofile=/tmp/flare-coverage.out -covermode=atomic ./...
go tool cover -func=/tmp/flare-coverage.out
go vet ./...
```

Resource generation is required after a clean checkout and after editing files
under `embed/`. It prepares vendor editor/guide assets, templates, styles, favicon,
and icon resources consumed by `go:embed`. A plain `go build` alone can compile
with placeholder vendor directories and leave optional browser features missing.
GoReleaser runs the generation hook automatically. Do not run generation and
compilation concurrently against the same checkout.

The main CI requires at least 85% all-package statement coverage. Formatting
and `golangci-lint` v2 checks are also required; see [contributing](../CONTRIBUTING.md).
The separate release/manual quality-report workflow uses a different reporting
threshold; see [report scopes](../.github/GO-REPORTS.md).

## Browser checks

```sh
cd tests/clock
npm ci --ignore-scripts
npx playwright install chromium
cd ../..
node --test tests/clock/clock.test.cjs tests/editor/state.test.cjs tests/editor/mobile-ui.test.cjs
node tests/mobile/bookmark-columns.browser.cjs
node tests/mobile/desktop-columns.browser.cjs
node tests/editor/scale.browser.cjs
FLARE_BINARY=/tmp/flare node tests/clock/browser.cjs
FLARE_BINARY=/tmp/flare node tests/editor/browser.cjs
FLARE_BINARY=/tmp/flare node tests/editor/legacy.browser.cjs
FLARE_BINARY=/tmp/flare node tests/editor/reliability.browser.cjs
node tests/editor/icon-picker.browser.cjs
FLARE_BINARY=/tmp/flare node tests/mobile/browser.cjs
FLARE_BINARY=/tmp/flare node tests/mobile/layout.browser.cjs
FLARE_BINARY=/tmp/flare node tests/mobile/category-navigation.browser.cjs
FLARE_BINARY=/tmp/flare node tests/mobile/readability.browser.cjs
FLARE_BINARY=/tmp/flare NODE_PATH="$PWD/tests/clock/node_modules" node tests/search/hybrid.cjs
FLARE_WEATHER_BROWSER=1 go test ./internal/settings/weather -run '^TestWeatherBrowser$' -count=1 -v
```

Use `CHROMIUM_EXECUTABLE` to select an existing compatible Chromium. The
[Clock acceptance workflow](../.github/workflows/clock.yml) also includes the
scratch-container timezone check. These browser commands use local fixtures;
[live weather verification](weather.md) and
[release archive/container preflight](release-0.6.6-preflight.md) are separate.

Scale measurements in
`tests/editor/scale.browser.cjs` report browser construction/layout timings; they
are not server CPU/RSS measurements and do not certify physical Safari or Via.

## Debug mode

Published release binaries deliberately disable development asset mode.
`FLARE_DEBUG=on` independently enables application DEBUG logs, including in release
builds; it does not select filesystem templates or development CSS.
Development builds recognize
`--debug`/`-D` through development flag detection; inspect the actual build/version
before expecting filesystem templates or development CSS routes. Use
`FLARE_ACCESS_LOG=all` for ordinary request diagnostics on stable builds.

## Measurement

Use [the baseline methods](baseline-metrics.md) for repeatable CPU/allocation and
RSS measurements. Record the source SHA, generation/build commands, binary hash,
platform, Go version, data size, enabled modules, concurrency, connection reuse,
logging, and GC settings. Never compare client-inclusive allocations with pure
server allocations or treat short-lived allocation bytes as retained heap.
