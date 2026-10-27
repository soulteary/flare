# Navigation snapshots and HTML fragment cache

> Parsed data caches shipped in stable releases; request settings snapshots and
> the HTML fragment cache below describe current main development after 0.6.7.
> See the [recorded measurements](../metrics-reports/html-fragment-cache-2026-10-27/report.md).

## Request settings and rendered fragments

Homepage/help rendering captures settings once per request, including failures.
Weather rendering consumes that same read-only snapshot instead of rereading
configuration. A new request still reads current `config.yml` bytes for revision
and downgrade checks; this is not an indefinite global settings cache.

Default, unfiltered navigation HTML is cached in process before cloning its
authorized collection. Immutable navigation views have distinct numeric identities
for public and administrator snapshots. Both collections are still selected as a
pair, and tentative authenticated reads still check backing files before looking
up HTML. File/storage errors and final authorization failures never fall back to
retained private HTML. The existing response cache policy remains `private, no-store`.

Fragment keys include content identity, configuration revision, relevant rendering
options, route, and theme generation. Successful in-process saves replace content
snapshots; observed configuration changes replace the revision. Old builds can
only publish under their old key, never overwrite a newer version. A theme change
during rendering (including change-and-revert) prevents retaining that build.
In-flight requests retain the existing immutable-snapshot semantics; there is no
new promise that a content edit cancels already-started responses.

Search queries, selected categories, encoded links, extension links and dynamic
destination placeholders bypass this first-stage cache. Encoded links depend on
the source URI even without placeholders. CSRF tokens, cookies, weather and clocks
are never retained in a navigation fragment. Rendering and cache construction do
not bypass or reduce durable security checks.

The FIFO cache retains at most 64 entries, 16 MiB of HTML, and 1 MiB per entry.
Larger fragments render normally without retention. Per-key builds are coalesced;
at most 64 distinct builds are tracked. Entries retain numeric identities rather
than whole navigation snapshots. Old generations become unreachable by new
requests and are reclaimed by bounded eviction; the byte budget does not include
transient rendering allocations or all process heap memory.

Cold/hot fragment and native loopback HTTP benchmarks are available:

```sh
go test ./internal/pages/home -run '^$' \
  -bench '^BenchmarkNavigationFragmentCache$' -benchmem -benchtime=500ms -count=3
go test ./internal/pages/home -run '^$' \
  -bench '^BenchmarkNativeHomepageFragmentCache$' -benchmem -benchtime=2s -count=3
```

Both warm parsed YAML first. “Cold” means the HTML cache is cleared for every
request, not a cold filesystem/process. Native HTTP uses sequential keep-alive
TCP requests with 1,000 public entries per collection plus private sentinels,
with login disabled, anonymous, and authenticated identities. Client and server
run in the same process; allocation counts include both. Reported p50/p95 use at
most the first 10,000 timed requests per round. Cold-cache clearing contributes to
ns/op and allocation totals but is outside the individual latency sample.
The 10,000-entry application microbenchmark exceeds the single-entry budget and
is explicitly labelled `oversize-bypass`; it does not demonstrate a cache hit.
These measurements do not establish browser DOM cost, peak RSS, cold startup,
multi-client saturation, or remote network latency.

## Parsed data and indexes

Display reads retain immutable parsed snapshots of `config.yml`, `apps.yml` and
`bookmarks.yml`. An unchanged snapshot avoids repeated YAML decoding. On 0.6.5,
all three use the existing byte cache. The 0.6.6 rereads `config.yml`
bytes for revision/version checks and reuses its parsed snapshot only when those
bytes are unchanged; navigation collections retain their byte cache.
Public configuration/collection APIs return independent copies of nested settings
and slices, so mutating a returned model never changes another request's data.
Concurrent bookmark readers share a read lock; first-file initialization and
writers still take the exclusive persistence lock.

Every existing in-process save/import/editor commit invalidates the bytes, parsed
snapshot and its lazy indexes together. A deleted file cannot reuse the previous
snapshot. It is regenerated only when no retained editor recovery copies exist;
otherwise normal reads, target lookups, search and import previews leave it missing
and require manual recovery instead of creating example data. Editor revisions and import previews continue reading
exact on-disk bytes and strict-decoding them; the display cache never replaces
those external-edit/conflict checks. Restart after externally editing `apps.yml`
or `bookmarks.yml`. On 0.6.5 this also applies to `config.yml`; the 0.6.6
observes external `config.yml` changes on the next configuration read. Stop writers
before making external edits to avoid racing in-process saves.

Redirect lookup builds a static URL set and a separate list of dynamic templates
on first use. Request-expanded URLs are never cached globally. The route keeps
its source validation, protocol checks and exact expanded-target allowlist match.
Search builds lowercase matching fields and category membership on first use,
keeps stored ordering and module scope, and expands dynamic URLs per request.
Both indexes are private to an immutable snapshot and disappear on invalidation.

These caches trade some retained model/index memory for much less decoding and
allocation churn. Indexes are lazy, and returned collection copies still scale
with the entry count. Reduced allocated bytes per operation do not establish a
reduction in idle or peak process RSS. Do not change GC limits based on these
microbenchmarks alone.

Reproduce server-side data-loading and lookup measurements with:

```sh
go test ./config/data -run '^$' \
  -bench 'BenchmarkNavigation.*Scale|BenchmarkLoadFavoriteBookmarks' \
  -benchmem -benchtime=200ms -count=3
```

The benchmark process does not include an HTTP client or browser. Its scope is
warm collection reads and index queries, not complete HTTP requests, latency under
network load, startup, or steady-state resident memory. Preserve environment and
raw rounds when comparing versions.
