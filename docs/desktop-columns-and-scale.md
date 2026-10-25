# Desktop columns and editor scale measurements

> Availability: development branch after 0.6.4; not included in the published 0.6.4 image.

Homepage appearance settings retain the existing layout by default. The optional
Automatic mode fits readable columns to the available width. Explicit 4/5/6/8
choices are maximum counts, not a promise to squeeze that many columns into every
window. Columns use a minimum 11.25rem width and reduce their count as space runs
out. Above 768px the setting applies to both application cards and bookmark groups;
mobile single/compact layouts remain independent. The same appearance classes
apply on the homepage, /bookmarks and /applications, including letter-case
preferences. The category navigation spans
the full grid width. CSS Grid preserves stored order and aligns group tops; it is
not a desktop masonry layout.

Run the reproducible, mock-HTTP list-renderer measurement with:

```sh
node tests/editor/scale.browser.cjs
node tests/mobile/desktop-columns.browser.cjs
```

Both use the project's Playwright dependency and optional CHROMIUM_EXECUTABLE.
The scale measurement loads production editor markup, state, script and CSS. It
reports DOM construction, a forced synchronous layout, and search/filter time at
1000, 5000 and 10000 entries. It verifies that display filtering never truncates
stored data, changing filters does not mark the document dirty, and all entries
return when filters clear. It also focuses the initially offscreen final row,
checks native scrolling and Tab navigation, searches that final row, and restores
the complete list without changing data. Physical assistive-technology behavior
remains a manual check. It does not measure network, YAML loading, paint,
physical-device responsiveness or batch-table performance. Timings are environment
specific and deliberately do not impose a flaky CI time limit.

A local Chromium 1440x1080 measurement showed the following rounded milliseconds:

| Entries | DOM before | Layout before | DOM after | Layout after |
| --- | ---: | ---: | ---: | ---: |
| 1000 | 17 | 56 | 20 | 10 |
| 5000 | 73 | 292 | 67 | 44 |
| 10000 | 159 | 526 | 158 | 88 |

The measured layout cost justified an opt-in-browser CSS optimization:
`content-visibility:auto` skips offscreen list row layout, with an intrinsic size
estimate. All rows remain in the DOM and shared working document, keyboard/search
semantics are retained, and unsupported browsers use normal layout. No pagination
or JavaScript virtualization was added. CI runs functional checks and emits fresh
measurements; the table is a single local observation, not a speed guarantee.

## Incremental editor updates

List search input is debounced by 100 ms. Location changes and explicit rendering
remain immediate; a later immediate render cancels a pending search render.
Existing row nodes are reused on unchanged renders and compatible updates. One
list-level click listener handles entry actions instead of three listeners per
row. Hidden rows are not kept in a detached-node cache.

The editor observes working-document mutations and caches the exact dirty result
until another mutation. Display filtering does not repeatedly canonicalize and
serialize both full documents. The first check after a real data change still
compares complete data, so reverting values, undoing a deletion and discarding
changes retain their exact semantics. Display views contain frozen flat bookmark
copies; writes use the observed working document or explicit store operations.

The scale harness now also records unchanged rerender time, 100 unchanged dirty
checks, post-GC JavaScript heap, DOM nodes and listeners. Measurements use Chromium
DevTools counters and do not represent whole-browser RSS or service memory.
Compare another source checkout with the same harness:

```sh
EDITOR_SOURCE_ROOT=/path/to/reference EXPECT_ROW_REUSE=0 node tests/editor/scale.browser.cjs
node tests/editor/scale.browser.cjs
```

See [the client measurement report](../metrics-reports/editor-client-2026-10-26.md)
for alternating-order observations and their limits. No timing threshold is used
as a CI assertion; functional ownership, dirty-state, data-count, focus and node
reuse checks remain mandatory.
