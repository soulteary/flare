#!/usr/bin/env python3
"""Generate the dependency-free, offline report viewer from summary.json."""
import argparse
import json
from pathlib import Path

ROOT = Path(__file__).resolve().parent
STYLE = """
:root { color-scheme: light dark; --background: light-dark(#fff,#181a1d); --foreground: light-dark(#202328,#e5e7eb); --muted: light-dark(#e8eaed,#30343a); --muted-foreground: light-dark(#626972,#aeb5be); --viz-series-1: light-dark(#2576ce,#77b6fc); --border: light-dark(#d8dce1,#454b54); }
* { box-sizing: border-box; }
body { margin: 0 auto; max-width: 1000px; padding: 24px; color: var(--foreground); background: var(--background); font: 15px/1.6 system-ui,sans-serif; }
h2 { margin: 0 0 20px; font-size: 22px; font-weight: 600; }
.viz-controls { display: flex; flex-wrap: wrap; gap: 16px; margin: 16px 0; }
.form-label { display: flex; flex: 1 1 220px; flex-direction: column; gap: 6px; }
.form-select { width: 100%; padding: 10px; border: 1px solid var(--border); border-radius: 6px; font: inherit; color: inherit; background: var(--background); }
.table-responsive { overflow-x: auto; }
.table { border-collapse: collapse; }
.table th, .table td { padding: 12px 8px; border-bottom: 1px solid var(--border); text-align: left; }
.table th { font-weight: 600; }
.table .text-end { text-align: right; }
.tabular-nums { font-variant-numeric: tabular-nums; }
.text-small { font-size: 12px; }
.text-muted { color: var(--muted-foreground); }
a { color: var(--viz-series-1); }
@media (max-width: 500px) { body { padding: 16px; } }
"""


def document():
    summary = json.loads((ROOT / "summary.json").read_text())
    fields = ("qps", "p95_ms", "server_cpu_us", "rss_peak_mib")
    rows = [{**{k: row[k] for k in ("size", "scene", "version", "phase")},
             "median": {k: row["median"][k] for k in fields},
             "range": {k: row["range"][k] for k in fields}} for row in summary["groups"]]
    fragment = (ROOT / "view.html").read_text().replace(
        "__COMPACT_DATA__", json.dumps(rows, separators=(",", ":"), ensure_ascii=False).replace("<", "\\u003c"))
    return ("<!doctype html>\n<html lang=\"zh-CN\"><head><meta charset=\"utf-8\">"
            "<meta name=\"viewport\" content=\"width=device-width,initial-scale=1\">"
            "<title>Flare 0.4.0 / 0.5.0 / 0.6.9 性能对照</title><style>" + STYLE +
            "</style></head><body>" + fragment +
            '<p class="text-small"><a href="report.md">完整报告</a> · <a href="README.md">方法与复核</a></p>'
            "</body></html>\n")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--check", action="store_true", help="fail if the saved viewer is stale")
    args = parser.parse_args()
    expected = document()
    destination = ROOT / "index.html"
    if args.check:
        if destination.read_text() != expected:
            raise SystemExit("index.html differs; run render.py")
        print("Offline viewer matches summary.json and view.html.")
    else:
        destination.write_text(expected)
        print(destination)


if __name__ == "__main__":
    main()
