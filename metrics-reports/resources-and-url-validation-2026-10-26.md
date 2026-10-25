# Embedded resource and navigation microbenchmarks — 2026-10-26

The candidate reuses immutable embedded-file validators, opens seekable files only
when a body is needed, reuses canonical icon markup, and avoids duplicate URL
parsing. It does not replace the HTTP framework or relax URL validation.

## Source and method

- Baseline: `722d17d` (batch-1 cache correctness plus the administrative theme snapshot changes).
- Candidate code: `d09e939`; the report itself is a subsequent documentation-only commit.
- Linux amd64, AMD EPYC 9V74, Go 1.27.3, `GOMAXPROCS=4`; `GOGC` and `GOMEMLIMIT` unset.
- Three one-second rounds per case, all baseline rounds followed by all candidate rounds.
- Static resource: 256 KiB JavaScript fixture, warmed metadata, full or conditional GET.
- The resource benchmark calls the native fasthttp handler directly. It includes
  server-side HTTP adaptation and buffered response work, but no client/load
  generator allocations, socket I/O, or network latency. Icon/URL cases are function microbenchmarks.
- Parallel icon results are aggregate Go benchmark time per operation across four workers, not single-request latency.
- Shared-host scheduling/background work can affect times. No confidence interval
  was computed; inline-icon timing does not show a stable improvement.

[Machine-readable per-round records and medians](resources-and-url-validation-2026-10-26.json).

## Median results

| Case | Before ns/op | After ns/op | Before B/op | After B/op | Before allocs/op | After allocs/op |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Resource / full | 626,339.00 | 318,189.00 | 1,433,578 | 1,133,305 | 43 | 32 |
| Resource / conditional | 299,565.00 | 9,272.00 | 273,129 | 2,386 | 29 | 20 |
| Icon / external/serial | 341.90 | 94.45 | 192 | 16 | 4 | 1 |
| Icon / external/parallel | 420.60 | 52.48 | 192 | 16 | 4 | 1 |
| Icon / inline/serial | 90.09 | 87.27 | 16 | 16 | 1 | 1 |
| Icon / inline/parallel | 76.01 | 46.11 | 16 | 16 | 1 | 1 |
| URL / href | 421.60 | 273.40 | 288 | 144 | 2 | 1 |
| URL / remote-image | 436.70 | 249.60 | 288 | 144 | 2 | 1 |
| URL / builtin-icon | 307.60 | 6.74 | 288 | 0 | 2 | 0 |

## What the numbers establish

Warm embedded conditional requests no longer copy/hash the full 256 KiB fixture.
A counted filesystem regression verifies zero opens/reads for warm 304 and HEAD;
range/full bodies still open a reader. Only content-derived ETag, MIME type and
length are retained, not another permanent copy of embedded file data.

External MDI hits reuse markup with canonical case-insensitive keys and a read
lock. Inline MDI allocation behavior is unchanged. Small per-used-icon markup is
retained to avoid repeated concatenation; canonical naming avoids duplicate SVG
files for spelling variants. Mutable generated SVGs continue reading/hashing the
actual current bytes, rather than trusting timestamp/size-only validators.

Favicon and generated MDI references carry content versions to bypass old
one-year cache entries. Fixed URLs and older mixed-case MDI names remain usable;
version queries do not turn mutable resources into an immutable cache promise.

Navigation checks parse web destinations once. Built-in icon names are rejected
as remote image URLs before parsing; allowed schemes and escaping stay unchanged.

**These are time/allocation results, not process CPU-time, live-heap, or RSS
measurements.** Lower allocation traffic can reduce GC work, but no whole-process
CPU percentage or lower RSS is claimed. Static metadata and cached markup are
small long-lived tradeoffs. The template buffer pool is unchanged: choosing a
size cap needs large-page/idle RSS and repeated-render CPU measurements.

## Reproduce

Use the same benchmark files from the candidate for both source trees:

```sh
GOMAXPROCS=4 go test ./internal/resources/assets ./internal/resources/mdi ./internal/pages/home \
  -run '^$' -bench 'BenchmarkStaticResourceDispatch|BenchmarkIconCacheHit|BenchmarkNavigationURLValidation' \
  -benchmem -benchtime=1s -count=3
```

The baseline needs copies of `static_benchmark_test.go`, `mdi_benchmark_test.go`
and `navigation_perf_test.go` from the candidate; only benchmarks are run.
For a production decision, repeat with isolated server/client processes and
realistic bookmark/icon data, including warmed/idle/peak RSS, before asserting a
CPU or memory capacity change.
