# Phase-one scale acceptance

Run from the repository with Python 3 on macOS/Linux. Use an unused disposable directory: the harness **replaces `apps.yml` and `bookmarks.yml`** there. It listens only through the tested Flare server and connects to `127.0.0.1`; no external URL is fetched. OS permission to bind a local port and inspect the child process is required.

```sh
python3 -m unittest discover -s tools/security-load -p 'test_*.py'
python3 tools/security-load/acceptance.py --binary /absolute/path/to/current-flare --directory /tmp/flare-scale-login-new --port 15180 --duration 1800 --output /tmp/flare-scale-login.json
python3 tools/security-load/acceptance.py --binary /absolute/path/to/old-0.6.7 --directory /tmp/flare-scale-baseline-new --port 15181 --baseline --output /tmp/flare-scale-baseline.json
python3 tools/security-load/acceptance.py --binary /absolute/path/to/current-flare --directory /tmp/flare-scale-current-new --port 15182 --output /tmp/flare-scale-current.json
python3 tools/security-load/acceptance.py --binary /absolute/path/to/current-flare --directory /tmp/flare-scale-import-new --port 15183 --import-only --import-repeats 5 --output /tmp/flare-scale-import-5.json
```

Do not run the last two commands concurrently, or while sustained pressure/heavy builds are running: latency comparisons need the same otherwise idle machine. The pressure test intentionally runs for 1,800 actual monotonic seconds, then waits for its bounded in-flight requests and probe. A shorter smoke run can debug the harness but never substitutes for acceptance.

All fixtures contain exactly 20,000 entries, 100 categories and 10,000 private entries. JSON includes fixture/binary hashes, start/end times, cold homepage/startup latency, warm homepage and local-search p50/p95/throughput, real HTTP import preview/commit time, statuses and resource samples. Pressure uses four fixed correct-password browser workers, each replacing its previous session via normal login, and a homepage probe every two seconds. This tests repeated password verification and session persistence without intentionally filling the separate 4,096-session limit. It does not exercise password failure rate limits or invent a low-end ARM/NAS result.

RSS time-series values are sampled every two seconds. For short measurements, `kernel_children_peak_rss_kib` additionally records the kernel child high-water mark; its children are the server and tiny `ps` probes. CPU is the host `ps %cpu` observation, not a container CPU entitlement or a portable CPU-seconds measure. The freeze and final interpretation are in [scale acceptance](../../docs/requirements/scale-acceptance.md).

The old baseline runs with its explicit trusted/no-login mode so the old import endpoint can be measured. The current build runs with management authentication and a fresh CSRF-protected session for import. In addition, old 0.6.7 renders all 20,000 entries and does not enforce private flags, while the candidate anonymous homepage renders the 10,000 allowed entries. These measurements are useful cost/regression evidence and **are not authorization-equivalent comparisons**.

Import-only mode repeats five real preview/commit pairs with the original byte-identical upload regenerated each time. It records nearest-rank p50/p95 (five samples are a small sample, not a stable tail estimate), exact item/category counts, preservation of a target unknown-field sentinel, and the 10,000 persisted private markers. Each target is reset between samples; this is acceptance in a disposable directory, not an upgrade procedure.

For matched administrator-read optimization tests, use the same script and fixture with `--admin-pressure --duration 180` on each immutable binary, in sequence on an otherwise idle host. Four readers alternate full administrator homepage and private-last-item search; a separate worker logs in, reads private content, logs out with CSRF, and replays the old cookie. Privacy checks match the actual card span, because the search input echoes a query even for anonymous users. The probe latency statistic covers its login POST; private-read/logout/replay outcomes are separate assertions. See [the paired optimization report](../../docs/requirements/admin-performance-optimization.md).

For equal offered-rate resource analysis, use `resource_analysis.py` sequentially on an idle host. Explicitly set `--rate 3 --duration 180`: the original 6/s calibration did not admit every read on the oldest build and is excluded from causal comparisons. The harness has four read slots, one probe slot and no waiting queue; require 540 admitted/completed reads, 90 completed login/revocation probes and no drops/errors before comparing. CPU/read and allocation/read include that identical mixed probe workload.

```sh
python3 tools/security-load/resource_analysis.py --binary /absolute/path/to/flare --directory /tmp/flare-resource-new --port 15380 --rate 3 --duration 180 --output /tmp/flare-resource.json
```

The optional test-only profiling wrapper provides runtime/pprof on a separate localhost control port; production binaries expose no new profiling route. Use the runner's `--help` for profiling and heap-diagnostic arguments. Capture allocation profiles before and after the timed window and subtract the start profile (`go tool pprof -base start.pb.gz binary end.pb.gz`) to exclude startup parsing. Force GC only outside timing. A single post-GC heap snapshot is sensitive to `sync.Pool` victim retention: the separate `heap_retention_diagnostic.py` captures one/two/three GC phases and is diagnostic, not a 180-second acceptance substitute. Native CPU/RSS and instrumented runtime measurements must be compared within their respective matched builds. See [resource cause analysis and complete evidence](../../docs/requirements/admin-resource-analysis.md).

For transaction I/O diagnostics, first copy/archive the intended source into a disposable directory, then run `transaction_overlay.py COPIED_SOURCE_DIRECTORY` and compile that copy with `go test -c`. The generated overlay counts Store commit/atomic-write calls and each actual Go `os.File.Sync` invocation in `internal/security/store.go`; it does not count kernel syscalls or content YAML persistence. It adds `/counters` only to the existing test-only localhost control server. Never generate this instrumentation in the production checkout, and never use its binary for native CPU/RSS comparisons.

```sh
python3 tools/security-load/transaction_overlay.py /tmp/flare-copied-source
# Build the copied source with go test -c, then:
python3 tools/security-load/transaction_counts.py --binary /tmp/flare-count.test --directory /tmp/flare-count-data --port 15600 --control-port 15601 --output /tmp/flare-count.json
```

The controlled sequence checks authenticated/anonymous GET and HEAD across offline home/search, applications, bookmarks, category and missing-category responses; snapshots counters around whole HTTP requests and verifies status/body/authorization. Login, logout and revoked-cookie replay are separate recorded sequences. Its added `Stat` accounting and counter overhead are diagnostic only. See [transaction I/O acceptance](../../docs/requirements/transaction-io-optimization.md).

The overlay generator rejects the tool's own repository root and any target containing a `.git` file or directory before writing. Use a plain archive/copy rather than a checkout or worktree.

## Matched native HTTP search/category comparison

`navigation_http.py` compares two immutable production binaries with login enabled
in both. It uses fresh disposable directories, 10,000 applications plus 10,000
bookmarks, 20 mixed-visibility categories, and actual anonymous/authenticated
cookies. It never uses the older harness's trusted/no-login baseline mode.

```sh
python3 tools/security-load/navigation_http.py \
  --baseline /absolute/path/to/baseline-flare \
  --candidate /absolute/path/to/candidate-flare \
  --output /tmp/flare-navigation-http-new \
  --duration 30 --rounds 2 --anonymous-rate 20 --authenticated-rate 4
```

The output directory must not exist. Each scenario starts a fresh server, warms
20 reads, then runs a fixed-rate bounded workload with four in-flight slots and
no waiting queue. Versions run sequentially and exchange order in the second
round. Every response checks exact rendered card membership, including private
content. Authenticated runs additionally check logout and old-cookie replay
outside timing. Any drop, operation error or sampler failure rejects the window;
exclude it and recalibrate both versions at the same rate rather than comparing
different completion counts.

Latency covers the complete HTTP GET through reading the body over loopback with
per-worker keep-alive connections. CPU uses the native server's cumulative
`ps time` delta (10 ms precision on the measured macOS host); it excludes client
CPU. RSS is sampled every second; a separate `wait4` high-water mark covers only
the native server child over its whole lifetime, including startup/login/probes.
Neither RSS metric is equivalent to live Go heap size. These short steady-load
comparisons are not the 180-second or 1,800-second scale acceptance above.
The PR #210 report is at
[complete HTTP comparison](../../metrics-reports/navigation-read-index-2026-10-27/http-report.md).
