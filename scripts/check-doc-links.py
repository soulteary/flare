#!/usr/bin/env python3
"""Check repository Markdown destinations; optionally probe remote HTTP links.

Uses only the Python standard library. Remote anchors are not validated: a 200
response establishes reachability, not whether a remote heading still exists.
"""

import argparse
from concurrent.futures import ThreadPoolExecutor
from html import unescape
import json
from pathlib import Path
import re
import subprocess
import sys
from urllib.error import HTTPError, URLError
from urllib.parse import unquote, urldefrag, urlsplit
from urllib.request import Request, urlopen

ROOT = Path(__file__).resolve().parents[1]


def without_fences(text):
    lines = []
    fence = None
    for line in text.splitlines():
        match = re.match(r"^\s*(`{3,}|~{3,})", line)
        if match:
            mark = match.group(1)
            if fence is None:
                fence = mark
            elif mark[0] == fence[0] and len(mark) >= len(fence):
                fence = None
            lines.append("")
        else:
            lines.append(line if fence is None else "")
    return "\n".join(lines)


def headings(text):
    ids, counts = set(), {}
    lines = without_fences(text).splitlines()
    for index, line in enumerate(lines):
        match = re.match(r"^\s{0,3}#{1,6}\s+(.+?)\s*#*\s*$", line)
        heading = match.group(1) if match else None
        if index + 1 < len(lines) and line.strip() and re.match(
            r"^\s{0,3}(?:=+|-+)\s*$", lines[index + 1]
        ):
            heading = line.strip()
        if heading:
            heading = re.sub(r"\[([^\]]+)\]\([^)]*\)", r"\1", heading)
            heading = re.sub(r"<[^>]+>", "", heading)
            slug = re.sub(r"[^\w\-\s]", "", unescape(heading).lower())
            slug = re.sub(r"\s", "-", slug)
            count = counts.get(slug, 0)
            counts[slug] = count + 1
            ids.add(slug + (f"-{count}" if count else ""))
    ids.update(re.findall(r'''\b(?:id|name)=["']([^"']+)["']''', text))
    return ids


def links(text):
    text = without_fences(text)
    references = {}
    for match in re.finditer(r"^\s{0,3}\[([^\]]+)\]:\s*<?([^\s>]+)>?", text, re.M):
        references[match.group(1).casefold()] = match.group(2)
    patterns = [
        r"!?\[[^\]\n]*\]\(\s*(<[^>]+>|[^\s)]+)(?:\s+[\"'][^\n]*?[\"'])?\s*\)",
        r"<(https?://[^>\s]+)>",
        r'''<(?:a|img)\b[^>]*\b(?:href|src)=["']([^"']+)["']''',
    ]
    for pattern in patterns:
        for match in re.finditer(pattern, text, re.I):
            yield text.count("\n", 0, match.start()) + 1, unescape(match.group(1).strip("<>"))
    for match in re.finditer(r"!?\[([^\]\n]+)\]\[([^\]\n]*)\]", text):
        label = (match.group(2) or match.group(1)).casefold()
        yield text.count("\n", 0, match.start()) + 1, references.get(label)
    for match in re.finditer(r"(?<![!\]])\[([^\]\n]+)\](?![\[(:])", text):
        if match.group(1).casefold() in references:
            yield text.count("\n", 0, match.start()) + 1, references[match.group(1).casefold()]
    for match in re.finditer(r"^\s{0,3}\[[^\]]+\]:\s*<?([^\s>]+)>?", text, re.M):
        yield text.count("\n", 0, match.start()) + 1, match.group(1)


def probe(url):
    try:
        request = Request(url, headers={"User-Agent": "Flare-documentation-link-check/1.0"}, method="HEAD")
        try:
            response = urlopen(request, timeout=12)
        except HTTPError as error:
            if error.code != 405:
                raise
            response = urlopen(Request(url, headers={"User-Agent": "Flare-documentation-link-check/1.0"}), timeout=12)
        with response:
            return {"url": url, "status": "reachable", "http": response.status, "final_url": response.url}
    except HTTPError as error:
        return {"url": url, "status": "missing" if error.code in (404, 410) else "unverified", "http": error.code}
    except (URLError, TimeoutError, OSError, ValueError) as error:
        return {"url": url, "status": "unverified", "reason": str(error)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--external", action="store_true", help="Probe all HTTP(S) destinations (remote fragments require manual review)")
    parser.add_argument("--report", type=Path, help="Write machine-readable results")
    args = parser.parse_args()
    # Include new documentation before it is committed, but exclude ignored local
    # dependencies and benchmark output. Never modify Git's index.
    output = subprocess.check_output(
        ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"], cwd=ROOT
    )
    files = sorted({ROOT / name for name in output.decode().split("\0") if name.endswith(".md")})
    documents = {path: path.read_text(encoding="utf-8") for path in files if path.is_file()}
    ids = {path: headings(text) for path, text in documents.items()}
    failures, remote, local_count = [], {}, 0
    for path, text in documents.items():
        for line, target in links(text):
            source = f"{path.relative_to(ROOT)}:{line}"
            if target is None:
                failures.append({"source": source, "error": "undefined reference link"})
                continue
            split = urlsplit(target)
            if split.scheme in ("http", "https"):
                remote.setdefault(urldefrag(target)[0], []).append(source)
                continue
            if split.scheme or split.netloc:
                continue
            local_count += 1
            dest = (ROOT / unquote(split.path).lstrip("/") if split.path.startswith("/") else path.parent / unquote(split.path)) if split.path else path
            dest = dest.resolve()
            if not dest.exists():
                failures.append({"source": source, "target": target, "error": "missing local target"})
            elif split.fragment and dest.suffix.lower() == ".md":
                valid = ids.get(dest)
                if valid is None:
                    valid = headings(dest.read_text(encoding="utf-8"))
                if unquote(split.fragment) not in valid:
                    failures.append({"source": source, "target": target, "error": "missing heading anchor"})
    external = []
    if args.external:
        with ThreadPoolExecutor(max_workers=6) as pool:
            external = list(pool.map(probe, sorted(remote)))
        for result in external:
            result["sources"] = remote[result["url"]]
    result = {"markdown_files": len(documents), "local_links": local_count, "local_failures": failures,
              "unique_external_urls": len(remote), "external": external}
    print(f"Checked {len(documents)} Markdown files and {local_count} local destinations; {len(failures)} local failures; {len(remote)} unique HTTP(S) URLs.")
    for failure in failures:
        print(json.dumps(failure, ensure_ascii=False))
    for entry in external:
        if entry["status"] != "reachable":
            print(json.dumps(entry, ensure_ascii=False))
    if external:
        counts = {status: sum(row["status"] == status for row in external) for status in ("reachable", "missing", "unverified")}
        print("External results: " + json.dumps(counts))
        print("Remote anchors were not validated; blocked/authenticated/network responses remain unverified.")
    if args.report:
        args.report.write_text(json.dumps(result, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    return 1 if failures or any(row["status"] == "missing" for row in external) else 2 if any(row["status"] == "unverified" for row in external) else 0


if __name__ == "__main__":
    sys.exit(main())
