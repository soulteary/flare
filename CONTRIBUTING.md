# Contributing to Flare

Start with the [development guide](docs/development.md) for resource generation,
builds and browser acceptance. Keep pull requests focused and describe the
observable behavior, relevant checks and any remaining limitations. Use synthetic
data in tests and screenshots. Follow the [security policy](SECURITY.md) for
vulnerabilities.

## Required checks

Use the Go version in `go.mod`. From the repository root:

```sh
go run build/build.go
go build -o /tmp/flare .
go test -race -count=1 -coverpkg=./... -coverprofile=/tmp/flare-coverage.out -covermode=atomic ./...
go tool cover -func=/tmp/flare-coverage.out
go vet ./...
golangci-lint run
```

The main CI requires at least 85% all-package statement coverage. Use a
`golangci-lint` v2 release compatible with the project's Go toolchain and the
checked-in `.golangci.yml`. Format changed Go files with `gofmt -w` (use `-s`
for simplification); CI rejects formatting differences.

Go unit tests use `go test`. Browser checks additionally require Node.js, the
locked Playwright dependency under `tests/clock`, and Chromium; follow the
[development guide](docs/development.md#browser-checks) and run the suites relevant
to the change. Do not run resource generation and compilation concurrently.

## Code and documentation

Follow [Effective Go](https://go.dev/doc/effective_go) and the
[Go code review comments](https://go.dev/wiki/CodeReviewComments).
Explain invariants and non-obvious decisions in comments; document exported APIs.
Choose names and receiver types to suit ownership and concurrency requirements.
See the [additional conventions](CONVENTIONS-OF-CODE.md) for performance work.

Update affected guides with behavior changes. Distinguish published features from
unpublished development changes, keep commands reproducible, and link validation
to the tested commit. Historical benchmark results must retain their original
scope and limitations rather than being presented as current measurements.
