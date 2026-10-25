# Avoidable favicon MIME initialization — 2026-10-26

The first formal resource capture found a memory regression after HTML/resource
preflight. Those original negative observations and profiles remain unchanged.
Profiles of `7b76abb` include `mime.initMimeUnix`, `mime.loadMimeGlobsFile` and
`mime.setExtensionType`; the earlier `39df281` profiles do not show those stacks.

The new embedded-file metadata loader called `mime.TypeByExtension` before its
explicit `.ico` override. Its first favicon request therefore initialized the
process-wide operating-system MIME registry even though the final type was
already known. The fix uses a bounded switch for bundled formats and
`http.DetectContentType` for other content. It avoids host-dependent lookups for
scripts/styles/fonts too, rather than merely moving the `.ico` special case.

## Isolated diagnostic

Each row uses a fresh native baseline target with `GOMAXPROCS=4`: capture the
control heap profile with `gc=1`, request `/favicon.ico` once, then capture the
control heap profile with `gc=1` again. The table reports the difference in the
profile's `HeapAlloc` value, including the same control instrumentation in every
case. Five rounds rotated process order; these are not throughput measurements.

| Source | Median HeapAlloc change | Minimum | Maximum |
| --- | ---: | ---: | ---: |
| 39df281 (before) | 77,848 B | 71,648 B | 88,936 B |
| 7b76abb (regression) | 563,600 B | 549,600 B | 565,920 B |
| 48e432f (fixed) | 72,376 B | 61,048 B | 83,768 B |

All 15 requests returned HTTP 200 and the same 2,816-byte favicon with SHA-256
`72404fa6497d55d874607c408c30b20a6ead3f9d5cc2538735e030c9b4ebcd9d`.
The fixed diagnostic binary was built from clean commit `48e432f` with
`vcs.modified=false`. [Per-round results and binary hashes](favicon-mime-init-2026-10-26.json).

This Linux host has freedesktop/system MIME mappings. The retained registry cost
is host-dependent: a minimal Alpine or scratch image may have far fewer or no
system MIME entries. These diagnostic heap changes do not establish a universal
RSS reduction, production capacity, or CPU percentage. A separate formal native
CPU/RSS rerun is needed for the final combined source; the first capture is not
rewritten as though it never regressed.

## Behavior and verification

Bundled favicon, JavaScript, CSS, HTML, SVG, font and JSON types stay explicit and
case-insensitive. Unknown extensions are sniffed from their bytes without loading
an OS database. HTTP tests cover actual Content-Type, bodies, HEAD length,
conditional ETag responses, Range and If-Range behavior; existing resource cache,
same-size upgrade and native request regressions continue to run.
