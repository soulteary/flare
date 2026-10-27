#!/usr/bin/env python3
"""Install the pinned report CLIs without changing Go or restoring caches."""
import argparse
import hashlib
from pathlib import Path
import platform
import re
import tarfile
import tempfile
import urllib.request

TOOLS = {
    'gtr': ('soulteary/go-test-report-action', 'v1.1.2', 'gtr'),
    'goreportcard': ('soulteary/goreportcard-action', 'v1.1.3', 'goreportcard'),
}


def install(tool, version, destination):
    repo, pinned, prefix = TOOLS[tool]
    if version != pinned:
        raise ValueError(f'{tool}: expected reviewed release {pinned}, got {version!r}')
    system = platform.system().lower()
    arch = {'x86_64': 'amd64', 'arm64': 'arm64', 'aarch64': 'arm64'}.get(platform.machine())
    if system not in ('linux', 'darwin') or arch is None:
        raise ValueError('Report tools require a supported Linux or macOS runner')
    asset = f'{prefix}_{version}_{system}_{arch}.tar.gz'
    base = f'https://github.com/{repo}/releases/download/{version}'
    with tempfile.TemporaryDirectory(prefix='flare-report-install-') as directory:
        archive = Path(directory) / asset
        with urllib.request.urlopen(f'{base}/{asset}', timeout=60) as response:
            archive.write_bytes(response.read())
        with urllib.request.urlopen(f'{base}/checksums.txt', timeout=60) as response:
            checksums = response.read().decode()
        entries = [line.split() for line in checksums.splitlines()]
        matches = [parts[0] for parts in entries if len(parts) == 2 and parts[1].lstrip('*') == asset]
        if len(matches) != 1 or not re.fullmatch(r'[0-9a-f]{64}', matches[0]):
            raise ValueError(f'Missing or ambiguous SHA256 for {asset}')
        if hashlib.sha256(archive.read_bytes()).hexdigest() != matches[0]:
            raise ValueError(f'Checksum mismatch: {asset}')
        # Copy only regular executable files, never extract release paths/links.
        destination.mkdir(parents=True, exist_ok=True)
        binaries = set()
        with tarfile.open(archive) as tar:
            for member in tar.getmembers():
                if not member.isfile() or not member.mode & 0o111:
                    continue
                name = Path(member.name).name
                if name in binaries:
                    raise ValueError(f'Duplicate executable {name}')
                binaries.add(name)
                target = destination / name
                if target.is_symlink():
                    raise ValueError(f'Refusing symlink {target}')
                target.write_bytes(tar.extractfile(member).read())
                target.chmod(0o755)
        required = 'gtr' if tool == 'gtr' else 'goreportcard-cli'
        if required not in binaries:
            raise ValueError(f'{asset} is missing {required}')
    print(f'Installed and verified {tool} {version}: {destination}')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('tool', choices=TOOLS)
    parser.add_argument('version')
    parser.add_argument('destination', type=Path)
    args = parser.parse_args()
    install(args.tool, args.version, args.destination)
