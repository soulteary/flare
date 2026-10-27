# Report packaging validation

Validated 2026-10-27. These checks verify the archived evidence and this PR's
packaging; they do not replace or extend the historical performance run.

- `python3 verify.py`: archive and member SHA-256 checks; 144 distinct windows,
  8,871,300 requests, zero errors/drops/throttling; sanitized and normalized response
  checks; navigation projections and redirect destinations; complete statistical
  recomputation exactly matches `summary.json`. The 24-probe preflight archive
  is verified separately.
- `python3 render.py --check`: saved offline viewer matches its source and data.
- `replay.py ./probe-run --probe-only`, run with Python 3.13:
  rebuilds the three fixed versions and passes all 24 size/route/version probes
  on the local Linux ARM64 Podman engine. This is a functional replay, not an
  additional timed performance experiment.
- Headless browser acceptance: loads `index.html` without HTTP(S) requests;
  checks three version rows, size and scenario selection, fixed-rate mode and
  360px viewport; zero runtime errors and no page-width overflow.
- Repository Markdown link checker: zero local destination failures.
- `go run build/build.go` and `go build`: passed.
- Full repository `go test -race -count=1 -coverpkg=./... -covermode=atomic`:
  passed; total statement coverage 91.1%.
- `go vet ./...`: passed.
- `golangci-lint v2.15.0 run`: 0 issues.
- `gofmt -s -l .` and `git diff --check`: passed.

Go source under `capture/` is deliberately archived as `.go.txt`, preserving
the collector algorithm used for measurement (host paths and test credentials are removed) without adding a production package.

- Privacy cleanup: visible files and both archives scanned; archive headers anonymized;
  sanitized responses and numerical summary revalidated. See `PRIVACY.md`.
