# Parsed navigation snapshots

Display reads retain immutable parsed snapshots of `config.yml`, `apps.yml` and
`bookmarks.yml` alongside the existing byte cache. A warm read avoids YAML decoding.
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
those external-edit/conflict checks. Display caches still require a restart after
manual edits to existing YAML files, as before.

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
