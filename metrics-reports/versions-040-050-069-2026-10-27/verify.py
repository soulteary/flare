#!/usr/bin/env python3
"""Verify the frozen archive and recompute its summary without changing files."""
import hashlib
from html.parser import HTMLParser
import importlib.machinery
import importlib.util
import json
import os
from pathlib import Path
import re
import tarfile
import tempfile

from privacy import main as privacy_check

ROOT = Path(__file__).resolve().parent
CSRF = re.compile(r'''(<input[^>]*name=(?:"csrf-token"|'csrf-token'|csrf-token)[^>]*value=")[^"]*(")''')



class NavigationParser(HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.current = None
        self.items = []

    def handle_starttag(self, tag, attrs):
        if tag == "a":
            self.current = [dict(attrs), []]

    def handle_data(self, value):
        if self.current:
            self.current[1].append(value)

    def handle_endtag(self, tag):
        if tag != "a" or not self.current:
            return
        attrs, text = self.current
        href = attrs.get("href", "")
        if href.startswith("https://benchmark.invalid/"):
            self.items.append({"href": href, "target": attrs.get("target"),
                               "rel": attrs.get("rel"), "text": "".join("".join(text).split())})
        self.current = None


def digest(value):
    return hashlib.sha256(value).hexdigest()


def check(condition, message):
    if not condition:
        raise ValueError(message)


def main():
    privacy_check()
    metadata = json.loads((ROOT / "environment.json").read_text())
    manifest = json.loads((ROOT / "raw-manifest.json").read_text())
    probes = json.loads((ROOT / "preflight-manifest.json").read_text())
    probe_archive = ROOT / "preflight.tar.gz"
    check(digest(probe_archive.read_bytes()) == probes["archive_sha256"], "preflight archive digest mismatch")
    with tarfile.open(probe_archive, "r:gz") as packed:
        seen_probes = set()
        for member in packed:
            check(member.isfile() and member.name in probes["members"] and member.name not in seen_probes,
                  "invalid preflight member")
            check(digest(packed.extractfile(member).read()) == probes["members"][member.name],
                  "preflight member digest mismatch")
            seen_probes.add(member.name)
        check(seen_probes == set(probes["members"]), "missing preflight members")
    archive = ROOT / "raw-windows.tar.gz"
    check(digest(archive.read_bytes()) == metadata["raw_archive_sha256"], "archive digest mismatch")
    check(len(manifest) == metadata["raw_archive_members"], "manifest count mismatch")
    with tempfile.TemporaryDirectory(prefix="flare-report-verify-") as directory:
        target = Path(directory)
        rows = []
        seen = set()
        with tarfile.open(archive, "r:gz") as packed:
            for member in packed:
                check(member.isfile() and member.name in manifest and member.name not in seen,
                      "unexpected or duplicate archive member: " + member.name)
                check(Path(member.name).parts[0] == "windows" and ".." not in Path(member.name).parts,
                      "invalid archive path")
                content = packed.extractfile(member).read()
                check(digest(content) == manifest[member.name], "member digest mismatch: " + member.name)
                destination = target / member.name
                destination.parent.mkdir(parents=True, exist_ok=True)
                destination.write_bytes(content)
                seen.add(member.name)
                if member.name.endswith(".json"):
                    rows.append(json.loads(content))
        check(seen == set(manifest), "missing archive members")
        check(len(rows) == 144, "expected 144 windows")
        check(len({(r["size"], r["scenario"], r["version"], r["round"]) for r in rows}) == 144,
              "duplicate experiment window")
        timed_requests = 0
        for row in rows:
            check(row["WarmErrors"] == 0, "warmup errors")
            check(row["Environment"].get("separate_cgroups") is True,
                  "client and server cgroup must differ")
            if row["scenario"] != "redirect-last":
                parser = NavigationParser()
                parser.feed(row["Pre"]["Body"])
                check(parser.items == row["navigation_projection"], "navigation projection mismatch")
                expected_count = 1 if row["scenario"] == "search-last" else row["size"] + (8 if row["scenario"] == "home" else 0)
                check(len(parser.items) == expected_count, "navigation count mismatch")
            else:
                expected = "https://benchmark.invalid/bookmarks/" + format(row["size"] - 1, "06d")
                for key in ("Pre", "Post"):
                    check(row[key]["Headers"].get("Location") == [expected], "redirect destination mismatch")
            for key in ("Pre", "Post"):
                probe = row[key]
                body = probe["Body"].encode()
                check(len(body) == probe["Bytes"] and digest(body) == probe["Hash"], "raw response mismatch")
                normalized = CSRF.sub(r"\1CSRF_TOKEN\2", probe["Body"]).encode()
                check(digest(normalized) == probe["NormalizedHash"], "normalized response mismatch")
                check(probe["Status"] == (302 if row["scenario"] == "redirect-last" else 200),
                      "unexpected response status")
            check(row["Pre"]["NormalizedHash"] == row["Post"]["NormalizedHash"], "response changed")
            for key in ("Load", "Fixed"):
                phase = row.get(key)
                if not phase:
                    continue
                timed_requests += phase["Requests"]
                check(phase["Errors"] == phase["Dropped"] == 0, "request errors or drops")
                check(set(phase["BodyHashes"]) <= {row["Pre"]["NormalizedHash"]}, "sampled response changed")
                check(phase["After"]["CPU"]["usage_usec"] >= phase["Before"]["CPU"]["usage_usec"],
                      "non-monotonic CPU counter")
                check(phase["After"]["CPU"]["nr_throttled"] == phase["Before"]["CPU"]["nr_throttled"],
                      "CPU quota throttling")
                if key == "Fixed":
                    check(phase["Requests"] == (200 if row["size"] == 20 else 50), "fixed-rate completion mismatch")
        check(timed_requests == 8871300, "request count mismatch")
        source = ROOT / "capture" / "analyze.py.txt"
        loader = importlib.machinery.SourceFileLoader("captured_analysis", str(source))
        spec = importlib.util.spec_from_loader(loader.name, loader)
        module = importlib.util.module_from_spec(spec)
        previous = os.environ.get("FLARE_BENCH_OUTPUT")
        os.environ["FLARE_BENCH_OUTPUT"] = str(target)
        try:
            loader.exec_module(module)
        finally:
            if previous is None:
                os.environ.pop("FLARE_BENCH_OUTPUT", None)
            else:
                os.environ["FLARE_BENCH_OUTPUT"] = previous
        module.OUT = target
        module.collect(complete=True)
        actual = json.loads((target / "summary.json").read_text())
        expected = json.loads((ROOT / "summary.json").read_text())
        check(actual == expected, "recomputed summary differs")
    print("Verified 408 archive members, 144 windows, 8,871,300 requests and the complete summary.")


if __name__ == "__main__":
    main()
