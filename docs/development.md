# Build and test

Use the Go toolchain declared in `go.mod`. Run from the repository root:

```sh
go run build/build.go
go build -o /tmp/flare .
go test ./... -count=1
go test -race ./...
go vet ./...
```

Resource generation is required after a clean checkout and after editing files
under `embed/`. It prepares vendor editor/guide assets, templates, styles, favicon,
and icon resources consumed by `go:embed`. A plain `go build` alone can compile
with placeholder vendor directories and leave optional browser features missing.
GoReleaser runs the generation hook automatically. Do not run generation and
compilation concurrently against the same checkout.

## Browser checks

```sh
cd tests/clock
npm ci --ignore-scripts
npx playwright install chromium
cd ../..
FLARE_BINARY=/tmp/flare node tests/editor/browser.cjs
FLARE_BINARY=/tmp/flare node tests/mobile/browser.cjs
FLARE_BINARY=/tmp/flare node tests/mobile/category-navigation.browser.cjs
FLARE_BINARY=/tmp/flare NODE_PATH="$PWD/tests/clock/node_modules" node tests/search/hybrid.cjs
```

Use `CHROMIUM_EXECUTABLE` to select an existing compatible Chromium. The CI
workflow lists the complete acceptance matrix. Scale measurements in
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
