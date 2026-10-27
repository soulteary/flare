# Flare 0.4.0 / 0.5.0 / 0.6.9 performance evidence

Measured on 2026-10-27. These are frozen source-version measurements, not a
benchmark of the current branch or original compressed release binaries.

- [Chinese report](report.md): conclusions, measurements and limitations.
- [Redaction policy](PRIVACY.md): published records contain no private host paths or identifiers.
- [Packaging validation](CHECKS.md): evidence verification, replay and repository checks.
- [Offline interactive viewer](index.html): download/open locally, or serve this
  directory with `python3 -m http.server 8000` and visit
  `http://localhost:8000/index.html`. GitHub displays HTML source rather than
  executing it; no GitHub Pages deployment is required or implied.
- [Summary](summary.json), [validation](validation.json), [environment](environment.json)
  and [build identities](builds.json).
- [Raw windows](raw-windows.tar.gz) and [per-file SHA-256 manifest](raw-manifest.json).
  The archive contains 144 JSON windows and their sanitized pre/post response bodies.
- [Preflight archive](preflight.tar.gz): 24 separate functional probes, excluded from timing;
  [manifest](preflight-manifest.json) records the sanitized member hashes.
- `capture/`: sanitized historical collector/analysis/generator source snapshots.
  Working directories are supplied through environment variables and are stored as `.txt`, so the old Go
  collector does not become a production package or affect coverage.

## Verify without a server or container engine

Python 3.9+ and its standard library are sufficient for offline verification. From the repository root:

```sh
python3 metrics-reports/versions-040-050-069-2026-10-27/privacy.py
python3 metrics-reports/versions-040-050-069-2026-10-27/verify.py
python3 metrics-reports/versions-040-050-069-2026-10-27/render.py --check
```

Verification checks the archive and every member digest, sanitized/normalized response
hashes, request/error/drop/throttle counts and fixed-rate completion counts. It
then recomputes all medians, ranges and paired bootstrap results in a temporary
directory and compares the full result with `summary.json`. Recorded files are
not changed. To edit the view, change `view.html` and run `render.py`; all chart
values are generated from `summary.json`. The viewer has no network dependencies.

## Repeat the experiment

Install Go, Python 3.12+, Git and a native Linux ARM64 Docker-compatible engine
with cgroup v2. The historical environment was Podman 5.8.3, with five visible
CPUs; this is not an emulation benchmark. Tags must resolve to the exact commits
in `builds.json`; fetch missing tags before running. Network access is needed for
Go 1.27.3, version-specific dependencies and the pinned Alpine base image.

```sh
# Choose a NEW directory outside this evidence folder.
python3 metrics-reports/versions-040-050-069-2026-10-27/replay.py ./replay-probe --probe-only
python3 metrics-reports/versions-040-050-069-2026-10-27/replay.py ./replay-full
```

The helper builds all three frozen sources with the same toolchain, generates
fixtures, and reuses the captured collector and workload. Path/image/container
names are adapted; container cleanup is performed in dependency order. Outputs
and image names are unique, and an existing destination is rejected. The
`--probe-only` run checks 24 version/size/route combinations but does not establish
new performance results. A full replay creates new observations and cannot be
expected to reproduce the original timings or binary hashes exactly.

The full workload takes approximately 20 minutes on the original machine. It
uses no CPU affinity or resource limits. It does not stop unrelated workloads.
Keep the machine otherwise quiet; record any environment difference. Generated
benchmark images remain available under the printed unique prefix for inspection
or deliberate removal. Do not merge a new replay into this frozen dataset.
