# Editor client observations, 2026-10-26

Reference source: `b668cfcd926e371ff7a0d10f56218b60978e8b48`. Both sources use the same current harness. The optimized source is identified by the app/state/harness SHA-256 hashes in [the raw report](editor-client-2026-10-26.json).

Linux x64, AMD EPYC 9V74, Node 24.22.1, Chromium 153.0.8010.0, 1440x1080. Three rounds alternate before/after execution order. Values below are medians; timings are rounded milliseconds. This is a local observation without confidence intervals. Concurrent machine load, warm-up, browser scheduling and collection timing can affect individual values.

| Entries | Initial accept + render, before / after ms | Unchanged render ms | Filter work ms | 100 unchanged dirty checks ms | Post-GC JS heap MiB | Event listeners |
| ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 1,000 | 22 / 16 | 20 / 4 | 3 / 3 | 88 / <1 | 2.40 / 2.27 | 3066 / 67 |
| 5,000 | 80 / 84 | 76 / 19 | 18 / 20 | 505 / <1 | 4.48 / 3.53 | 15066 / 67 |
| 10,000 | 189 / 201 | 161 / 31 | 35 / 28 | 844 / <1 | 5.83 / 5.68 | 30066 / 67 |

The durable changes are row identity reuse, one delegated entry listener, and no repeated full-document serialization for unchanged dirty checks. Initial rendering and individual filtering cases are not uniformly faster. Text input deliberately waits 100 ms to coalesce a typing burst; the filter-work column measures explicit render work and excludes that waiting period.

Heap values are Chromium Runtime.getHeapUsage after HeapProfiler.collectGarbage, not whole-browser RSS, DOM native allocations or Go service memory. Heap variation is substantial at these small sizes; treat it as diagnostic evidence, not a memory guarantee. DOM counts remain proportional to entry count: this change does not add pagination or virtualization. Hidden rows are not retained in an extra detached cache.

The fixture uses mocked HTTP and production editor assets. It excludes network, YAML parsing, physical-device behavior, full paint and batch-table scale. Functional assertions cover stored data counts, dirty state, row identity, offscreen focus, Tab order and filter restoration; they do not enforce unstable timing thresholds.

Reproduction:

```sh
CHROMIUM_EXECUTABLE=/path/to/chromium EDITOR_SOURCE_ROOT=/path/to/reference EXPECT_ROW_REUSE=0 node tests/editor/scale.browser.cjs
CHROMIUM_EXECUTABLE=/path/to/chromium node tests/editor/scale.browser.cjs
```
