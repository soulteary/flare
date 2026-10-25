# Warm parsed snapshots and navigation indexes

Measured locally on Linux amd64, AMD EPYC 9V74, Go 1.27.3. Each benchmark uses
`-benchmem -benchtime=200ms -count=3`; the reported timings below are medians.
Before: `39df281349fccfa2d51ccc8b1a749be7c54af62d` plus the scale benchmark added
in this change. After: the optimized implementation and benchmark in this PR.
These are server-side data functions, without an HTTP client or browser. Processes
run sequentially, but shared-host load was not controlled and timing varied.
No confidence intervals, whole-request throughput, CPU profiles or RSS comparison
were collected. Use allocated bytes/op separately from live heap and resident RAM.

## Public collection reads

Public reads still copy slices so callers cannot mutate shared snapshots.

| Entries | Before median | After median | Before B/op | After B/op | Before allocs/op | After allocs/op |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| Default 8 apps | 32.7 µs | 1.14 µs | 18,992 | 1,000 | 346 | 4 |
| 1,000 | 4.96 ms | 27.8 µs | 1,970,493 | 90,385 | 45,086 | 5 |
| 5,000 | 22.9 ms | 387 µs | 9,884,016 | 442,644 | 225,091 | 5 |
| 10,000 | 52.2 ms | 838 µs | 19,762,096 | 885,011 | 450,094 | 5 |

The stable allocation reduction comes from avoiding repeat YAML decoding. Timing
changes are illustrative observations from this environment, not production
speed guarantees. The 10,000-entry public-read copy still allocates about 0.84 MiB.

## Warm indexes

The static redirect target is the last entry. The search is a single last-entry
match. Target lookup checks both collections; search enables only bookmarks.

| Entries | Static target median | Static B/op / allocs | Search median | Search B/op / allocs |
| ---: | ---: | ---: | ---: | ---: |
| 1,000 | 1.19 µs | 472 / 6 | 25.7 µs | 1,280 / 5 |
| 5,000 | 1.29 µs | 472 / 6 | 94.3 µs | 1,280 / 5 |
| 10,000 | 3.21 µs | 472 / 6 | 183 µs | 1,280 / 5 |

Static lookup uses a map; dynamic templates must still be expanded and compared
per request. Search still scans entries, but uses pre-normalized fields and
constant-time category membership without copying/decoding the whole collection.
Indexes are lazy and add retained memory. No lower-idle-RSS claim follows from
these allocation measurements.

## Reproduction and raw rounds

```sh
go test ./config/data -run '^$' \
  -bench 'BenchmarkNavigation.*Scale|BenchmarkLoadFavoriteBookmarks' \
  -benchmem -benchtime=200ms -count=3
```

- [Before rounds](navigation-snapshots-2026-10-26/before.txt)
- [Optimized scale/index rounds](navigation-snapshots-2026-10-26/after.txt)
- [Optimized default/supplementary rounds](navigation-snapshots-2026-10-26/after-default.txt)
- [Cache semantics and limitations](../docs/navigation-cache.md)
