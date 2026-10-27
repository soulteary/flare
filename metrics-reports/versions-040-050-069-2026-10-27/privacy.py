#!/usr/bin/env python3
"""Reject private host metadata in both visible files and archived evidence."""
import json
from pathlib import Path
import re
import tarfile

ROOT = Path(__file__).resolve().parent
FORBIDDEN = [
    re.compile(r"\x2f(?:Users|home|private|tmp)\x2f"),
    re.compile(r"\x2fvar\x2f(?:lib\x2fcontainers|folders)\x2f"),
    re.compile(r"[A-Za-z]:\\(?:Users|Documents and Settings)\\"),
    re.compile(r"libpod-[0-9a-f]{12,}"),
    re.compile(r"\b[0-9A-Fa-f]{2}(?::[0-9A-Fa-f]{2}){5}\b"),
]
PRIVATE_KEYS = {"recorded_at", "cgroup", "self_cgroup", "pid1_cmdline", "Hostname", "Domainname",
                "GraphDriver", "ContainerConfig", "LowerDir", "UpperDir", "WorkDir", "At"}
TOKEN = re.compile(r'''<input[^>]*name=(?:"csrf-token"|'csrf-token'|csrf-token)[^>]*value="([^"]*)"''')


def check_text(label, text):
    for pattern in FORBIDDEN:
        if pattern.search(text):
            raise ValueError("private path or host identifier in " + label)
    for match in TOKEN.finditer(text):
        if match[1] and set(match[1]) != {"X"}:
            raise ValueError("unredacted response token in " + label)


def check_json(label, value):
    if isinstance(value, dict):
        if PRIVATE_KEYS & value.keys():
            raise ValueError("private metadata key in " + label)
        for key, child in value.items():
            if key == "Headers" and isinstance(child, dict):
                if {k.lower() for k in child} & {"cookie", "set-cookie", "date", "authorization", "proxy-authorization"}:
                    raise ValueError("private response header in " + label)
            check_json(label, child)
    elif isinstance(value, list):
        for child in value:
            check_json(label, child)


def main():
    count = 0
    for path in sorted(ROOT.rglob("*")):
        if not path.is_file() or "__pycache__" in path.parts:
            continue
        label = str(path.relative_to(ROOT))
        if path.name.endswith(".tar.gz"):
            header = path.read_bytes()[:10]
            if header[3] or int.from_bytes(header[4:8], "little"):
                raise ValueError("non-anonymous gzip header: " + label)
            with tarfile.open(path, "r:gz") as packed:
                for member in packed:
                    if member.uid or member.gid or member.uname or member.gname or member.mtime or member.pax_headers:
                        raise ValueError("non-anonymous tar header: " + label)
                    if not member.isfile():
                        raise ValueError("unexpected archive entry: " + label)
                    text = packed.extractfile(member).read().decode()
                    check_text(label + ":" + member.name, text)
                    if member.name.endswith(".json"):
                        check_json(label + ":" + member.name, json.loads(text))
                    count += 1
        else:
            text = path.read_text()
            check_text(label, text)
            if path.suffix == ".json":
                check_json(label, json.loads(text))
            count += 1
    print("Privacy checks passed for", count, "files and archive members.")


if __name__ == "__main__":
    main()
