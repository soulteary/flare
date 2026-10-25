# Navigation rendering: category buckets and hidden modules

The categorized renderer now buckets matching entries once, then renders each
category's entries. The old path scanned every entry for every category and
validated URLs/rendered icons before checking membership. Default placement still
uses the first category, item/category order is unchanged, and orphan category
references remain absent from the homepage. Filtering and dynamic URL expansion
share one pass rather than two full result slices.

Homepage, application and bookmark handlers now skip disabled modules before
loading their backing YAML. Malformed visible data produces a generic alert and
an actionable server log instead of an unexplained blank module; local filenames
and YAML error details are not included in that alert.

## Measurement

Linux amd64, AMD EPYC 9V74, Go 1.27.3, default GOMAXPROCS=8. Baseline source is
`02151c05579065bbac0e6ff3be1e971d6dc165cb` plus the same benchmark fixture. The
comparison includes the raw-byte cache, YAML decode, request-independent URL
handling, grouping, escaping and HTML construction; it does not include HTTP,
client/browser work, server CPU profiling or RSS measurement. Fixtures use no
icons and no dynamic URL placeholders. There is no parsed-document cache in this
comparison. Other work was running on the host, so wall-clock timings are noisy.
The additional 20-entry/4-category check used the same fixture code on both
revisions and was run with concurrent host activity; its allocation counts are
more useful than its short wall-clock timings.

Run on each revision with the same fixture:

```sh
go test ./internal/pages/home -run '^$' -bench BenchmarkCategorizedNavigation \
  -benchmem -benchtime=200ms -count=3
```

Medians from three observations (decimal MB for allocated bytes):

| Entries | Categories | Before ms/op | After ms/op | Before MB/op | After MB/op | Before allocs/op | After allocs/op |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 20 | 4 | 0.141 | 0.122 | 0.085 | 0.074 | 1,068 | 964 |
| 1000 | 10 | 11.98 | 5.71 | 5.52 | 3.17 | 56,461 | 38,544 |
| 1000 | 100 | 57.86 | 5.68 | 31.46 | 3.25 | 239,194 | 41,695 |
| 10000 | 10 | 135.97 | 56.25 | 56.01 | 33.12 | 560,553 | 380,673 |
| 10000 | 100 | 497.79 | 55.12 | 315.68 | 33.01 | 2,363,908 | 384,705 |

[Raw baseline](navigation-render-before-2026-10-26.txt) and
[raw optimized runs](navigation-render-after-2026-10-26.txt) retain all observations,
including the noisy 10,000-entry baseline. No confidence interval was calculated.
Allocated bytes are cumulative temporary allocations per operation, not a claim
about retained heap, peak process memory, or a production CPU percentage.
