# Post-audit isolated process resources — 2026-10-26

**Server CPU and allocation traffic are substantially lower; process memory is
mixed, not uniformly lower.** In the final five-route workload, CPU/request fell
59.6–81.1%, allocated bytes/request fell 55.2–94.5%, and client-observed loopback
P95 fell 66.4–77.9%. HTML warmed RSS ranged from −0.8% to +3.3%; sampled load-peak
RSS remained 3.0–4.2% higher. Redirect warmed RSS fell 8.3%. These observations do
not establish a generally smaller resident process or a production CPU percentage.

The source baseline is **pre-audit main `39df281`, not the published 0.6.4 tag**.
The final captured binary is clean reviewed head `94d2c0a`, merged as `a116b716`.
Both commits have the identical full Git tree
`ec7ff79fb19a652404f62b2ebeb93387c4a0e820`; the measured binary was not rebuilt or
relabelled after merge. The fixed client is the original baseline binary.

## Initial memory regression: retained and corrected

The first formal capture compared `39df281` with combined candidate `7b76abb`.
It also used four alternating rounds and 40,000 requests per route/target/round.
It found lower CPU, but **HTML warmed RSS increased 5.1–9.0%, sampled load-peak RSS
increased 5.7–15.0%, and post-GC live heap increased 15.1–22.1%**. Those negative
observations were not overwritten or replaced by the favorable rerun:

- [Initial raw rounds, hashes and fingerprints](audit-resource-comparison-2026-10-26-initial.json).
- [Initial response bodies, heap profiles, capture manifest and diagnostics](audit-resource-comparison-2026-10-26-initial.tar.gz).

The added favicon metadata path called `mime.TypeByExtension` before overriding
its result with the already-known ICO type. Initial profiles include
`mime.initMimeUnix`, `mime.loadMimeGlobsFile` and `mime.setExtensionType`; the
baseline profiles do not. A separate five-round fresh-process diagnostic found
median favicon post-GC HeapAlloc changes of 77,848 B (baseline), 563,600 B
(regressed) and 72,376 B (fixed). Every request returned the same 2,816-byte icon.
The bounded MIME switch/content-sniffing fix removes this avoidable, host-dependent
registry load. See [the diagnostic, source and raw rounds](favicon-mime-init-2026-10-26.md).

The final comparison below is a **new, independently captured four-round series**
after that fix. Initial and final medians are not pooled. Persistent snapshots,
indexes and metadata intentionally retain some data to reduce decoding/GC work;
the experiment does not attribute the remaining retained-memory differences to
individual components. Lower allocation traffic alone is not lower RSS.

## What remained the same, and what intentionally changed

All 40 final runs matched the captured selected fields: valid-request HTTP status,
Content-Type, CSP and Location; normalized document text; navigation destinations,
targets and order; selected control fields; and ordered actual MDI/favicon bytes.
**This is selected-field validation, not complete DOM or browser-behavior equivalence.**
The live projection omits form `rel` and several control attributes, collapses text
whitespace, and does not model CSS visibility/layout or execute scripts.

An additional offline audit of the retained bodies checked all four rounds. Tag
counts/order and raw text/style data match after replacing only the visible clock
time. Attribute differences are limited to favicon references, MDI image sources,
and the search form's intentional `rel="noopener noreferrer"` → `rel="noopener"`
change. The audit is included in the final archive. Browser Origin/referrer behavior
is separately covered by [#152](https://github.com/soulteary/flare/pull/152) and its
browser acceptance; it is not inferred from this route load generator.

Literal fingerprints differ on all four HTML routes and remain recorded. Favicon
and MDI references gain content-version queries, some MDI filenames become
canonical lowercase, and the search form changes as above. Ordered icons still
return HTTP 200 with matching content hashes. The larger HTML responses are also
retained in the byte-count table; no page content was removed to obtain the gains.

Resource policy changed from `public, max-age=31536000` to
`public, max-age=0, must-revalidate`. Version queries bypass stale old entries but
do not make the new resources a zero-network immutable cache. Timed load fetches
HTML/redirect routes only; icon/favicon requests occur in preflight before warmup.
This does not measure real-browser request mixes, conditional-request counts or
whole-page latency. The [separate resource microbenchmarks](resources-and-url-validation-2026-10-26.md)
measure conditional/full resource paths with different scope and baseline.

## Artifacts and measurement boundaries

- [Final raw observations](audit-resource-comparison-2026-10-26.json).
- [Final raw bodies, text heap profiles, capture manifest, original harness snapshots and offline audit](audit-resource-comparison-2026-10-26.tar.gz).
- Reproduction harness: [`compare-release-resources.py`](../scripts/compare-release-resources.py), using the established isolated target/client helper.
- Linux fork/exec peak diagnostic: [`diagnose-process-rss.py`](../scripts/diagnose-process-rss.py).
- Fresh-process favicon diagnostic: [`diagnose-favicon-heap.py`](../scripts/diagnose-favicon-heap.py).

Each capture contains 1.6 million timed requests, excluding warmup and diagnostics.
The host is Linux amd64 / AMD EPYC 9V74 with nine visible logical CPUs and an
eight-CPU cgroup quota (`800000 100000`). Client and target each use GOMAXPROCS=4;
concurrency and idle connection pool are eight. Go 1.27.3, default data, offline
mode, disabled login/request logs, and unset GOGC/GOMEMLIMIT/GODEBUG were used.
`TZ=Asia/Tokyo` is the recorded host test environment; report timestamps use +08:00.
No other benchmark or full test suite ran during either formal capture.

CPU/allocation/GC counters cover the server process, with small control-endpoint
overhead; the client is a separate fixed binary. Live heap uses exact HeapAlloc
from `heap?gc=1&debug=1`: Go reads MemStats after forced GC and before allocating
profile-encoding data. It includes small control-request overhead and is not RSS.
Default sampled `inuse_space` estimates are retained separately, never substituted
for exact HeapAlloc.

Linux RUSAGE MaxRSS is excluded from comparisons. A controlled experiment changed
only the launcher's allocation by 80 MiB: the unchanged target reported MaxRSS
about 18 → 97 MiB while verified target VmRSS/VmHWM stayed about 18.5 MiB. Raw
MaxRSS remains available, but includes pre-exec launcher residency on this host.
The final series therefore uses verified namespace/parent-matched `/proc` VmRSS,
load-time samples, and post-exec VmHWM. These separate Linux counter observations
do not strictly order in every row and are not silently clamped. See the
[method warning](../docs/baseline-metrics.md#linux-rss-与-pid-命名空间).

All percentages below are ratios of per-version medians, not medians of paired
ratios. Four rounds do not establish confidence intervals. Small retained-memory
or RSS differences must not be advertised as a universal capacity improvement.

## Final per-route observations

Generated: 2026-10-26T01:08:46+08:00

Before: `39df281349fccfa2d51ccc8b1a749be7c54af62d`

After: `94d2c0a93660553ba21262da701595b402950eaf`

Fixed client SHA-256: `7ba9e0bf47267ce447c69750bdae5fc3e9c5bdc18a4667bd5a77aa2bb6b5363d`

Environment: Linux-6.18.55-x86_64-with-glibc2.39; go version go1.27.3 linux/amd64; GOMAXPROCS=4

Workload: 40000 requests/scene/target/round; 1000 warmup; concurrency 8; pool 8; 4 alternating rounds.

## Response validation

| Scene | Literal normalized response | Selected DOM / asset / HTTP fields | Before / after bytes |
| --- | --- | --- | --- |
| home | **DIFFERENT — retained** | captured fields match | 29047 / 30980 |
| home-search | **DIFFERENT — retained** | captured fields match | 29110 / 31043 |
| bookmarks | **DIFFERENT — retained** | captured fields match | 25060 / 26470 |
| applications | **DIFFERENT — retained** | captured fields match | 23455 / 24061 |
| redir-url | equal | captured fields match | 0 / 0 |

## CPU µs/request

| Scene | Before median | After median | Change |
| --- | ---: | ---: | ---: |
| home | 666.233 | 237.819 | -64.3% |
| home-search | 732.742 | 257.837 | -64.8% |
| bookmarks | 508.808 | 160.623 | -68.4% |
| applications | 260.740 | 105.365 | -59.6% |
| redir-url | 118.634 | 22.381 | -81.1% |

## allocated B/request

| Scene | Before median | After median | Change |
| --- | ---: | ---: | ---: |
| home | 169981.479 | 76194.069 | -55.2% |
| home-search | 187866.896 | 77645.583 | -58.7% |
| bookmarks | 129973.106 | 57295.766 | -55.9% |
| applications | 53753.861 | 20055.581 | -62.7% |
| redir-url | 19109.981 | 1053.022 | -94.5% |

## allocations/request

| Scene | Before median | After median | Change |
| --- | ---: | ---: | ---: |
| home | 2645.190 | 512.213 | -80.6% |
| home-search | 2876.422 | 540.241 | -81.2% |
| bookmarks | 2044.380 | 299.858 | -85.3% |
| applications | 809.041 | 234.343 | -71.0% |
| redir-url | 350.163 | 17.010 | -95.1% |

## warmed idle RSS MiB

| Scene | Before median | After median | Change |
| --- | ---: | ---: | ---: |
| home | 27.357 | 27.857 | +1.8% |
| home-search | 27.492 | 28.391 | +3.3% |
| bookmarks | 27.184 | 27.332 | +0.5% |
| applications | 26.031 | 25.828 | -0.8% |
| redir-url | 23.742 | 21.775 | -8.3% |

## sampled load peak RSS MiB

| Scene | Before median | After median | Change |
| --- | ---: | ---: | ---: |
| home | 29.762 | 30.881 | +3.8% |
| home-search | 29.809 | 30.693 | +3.0% |
| bookmarks | 29.580 | 30.832 | +4.2% |
| applications | 27.967 | 29.002 | +3.7% |
| redir-url | 24.752 | 24.115 | -2.6% |

## post-exec process high-water RSS MiB

| Scene | Before median | After median | Change |
| --- | ---: | ---: | ---: |
| home | 30.139 | 32.090 | +6.5% |
| home-search | 30.381 | 33.174 | +9.2% |
| bookmarks | 29.438 | 32.576 | +10.7% |
| applications | 27.879 | 32.121 | +15.2% |
| redir-url | 24.447 | 23.959 | -2.0% |

## live runtime heap MiB after forced GC

| Scene | Before median | After median | Change |
| --- | ---: | ---: | ---: |
| home | 2.709 | 2.702 | -0.3% |
| home-search | 2.658 | 2.820 | +6.1% |
| bookmarks | 2.527 | 2.691 | +6.5% |
| applications | 2.565 | 2.707 | +5.6% |
| redir-url | 2.007 | 1.965 | -2.1% |

## client-observed P95 ms

| Scene | Before median | After median | Change |
| --- | ---: | ---: | ---: |
| home | 4.256 | 1.195 | -71.9% |
| home-search | 4.371 | 1.250 | -71.4% |
| bookmarks | 2.704 | 0.858 | -68.3% |
| applications | 1.925 | 0.646 | -66.4% |
| redir-url | 1.208 | 0.268 | -77.9% |

## Scope and limitations

- Load CPU, allocation and GC deltas cover the isolated server process only. Client measurements remain in raw rows and must not be confused with server allocation cost.
- P95 is measured by the fixed client over loopback and includes scheduling and network latency. Four rounds do not establish confidence intervals.
- On Linux, RSS is read from a /proc child verified by both parent PID and namespace-local NSpid. This avoids sampling an unrelated host process in mixed PID-namespace environments.
- Warmed idle RSS is measured after warmup. Sampled peak RSS is sampled every ~50 ms during load. Linux /proc VmHWM is the post-exec process high-water mark; raw RUSAGE MaxRSS is retained but excluded from comparisons because it can inherit launcher residency before exec.
- Live heap is exact runtime HeapAlloc recorded by Go’s debug=1 heap handler after forced GC and before profile encoding, collected after all load counters. It includes the control request’s small instrumentation overhead and is not RSS. Sampled inuse_space/profile-period estimates remain separately recorded; do not substitute them for exact HeapAlloc.
- Timed load covers HTML/redirect routes only. Icons/favicon are fetched for semantic preflight before warmup; browser cache/request mixes and full-page latency are not measured. Asset Cache-Control differences are retained and can change real-browser revalidation traffic.
- Both binaries use offline mode, disabled login/request logs, and default fixtures. This does not measure production authentication, weather, browser CPU, ARM hardware, or large bookmark datasets.
- Raw response bodies, literal fingerprints, parsed semantic projections, icon response statuses/content hashes, binary hashes, build metadata, heap profiles and per-round observations/orders are retained. Only clock-time text is normalized.
- Selected-field checks compare HTTP status/type/CSP/Location, whitespace-collapsed document text (excluding script/style), navigation destinations/targets/order, selected form/control fields, and ordered icon/favicon content hashes. Icon URL/cache-key changes remain recorded. The anchor-rel guard permits only noopener/noreferrer additions; it does not cover form rel.
- This is not complete DOM, form, script, computed-style, layout or browser-behavior equivalence. Form rel and several control attributes are omitted from the live projection; raw bodies retain them for offline review. Browser Origin/referrer acceptance is separate.
- A response mismatch remains a mismatch. Explain it before interpreting resource differences; do not claim a like-for-like improvement if functionality was removed.

## Reproduce a fresh comparison

Use Linux amd64 and Go 1.27.3. Rebuild the exact source revisions in separate clean
checkouts; keep one common client. New binaries can have different hashes because
checkout paths/build environments differ, so retain the new hashes and build info
rather than overwriting these historical observations.

```sh
git worktree add --detach /tmp/flare-bench-before 39df281349fccfa2d51ccc8b1a749be7c54af62d
git worktree add --detach /tmp/flare-bench-after 94d2c0a93660553ba21262da701595b402950eaf
(cd /tmp/flare-bench-before && go run build/build.go && go build -o /tmp/flare-bench-before-bin ./tools/baseline)
(cd /tmp/flare-bench-after && go run build/build.go && go build -o /tmp/flare-bench-after-bin ./tools/baseline)
TZ=Asia/Tokyo python3 scripts/compare-release-resources.py \
  --before /tmp/flare-bench-before-bin --after /tmp/flare-bench-after-bin \
  --client /tmp/flare-bench-before-bin \
  --before-revision 39df281349fccfa2d51ccc8b1a749be7c54af62d \
  --after-revision 94d2c0a93660553ba21262da701595b402950eaf \
  --output .benchmark/fresh-resource-comparison \
  --rounds 4 --resource-requests 40000 --resource-warmup 1000 \
  --concurrency 8 --keepalive-connections 8 --gomaxprocs 4
```

Use a fresh output directory; the runner refuses to replace existing observations.
It records literal mismatches separately and fails when its selected-field checks
mismatch. Inspect raw responses and validate intentional policy differences before
interpreting results. The archive preserves the exact harness sources and hashes
used for each original capture, including historical wording before peer review
clarified the selected-field projection's scope.
