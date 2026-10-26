#!/usr/bin/env python3
"""Build and inspect local release-candidate archives. Never publishes or tags.

Run after production asset generation, from a committed candidate checkout:
  python3 scripts/release-preflight.py --version 0.6.6 --output /tmp/flare-066
These are local preflight archives, not GoReleaser/Docker release artifacts.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import io
import json
import os
from pathlib import Path
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
TARGETS = [('darwin', 'amd64', ''), ('darwin', 'arm64', ''),
           ('linux', '386', ''), ('linux', 'amd64', ''),
           ('linux', 'arm64', ''), ('linux', 'arm', '6'), ('linux', 'arm', '7')]


def run(args, **kwargs):
    return subprocess.check_output(args, cwd=ROOT, text=True, **kwargs).strip()


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', required=True)
    parser.add_argument('--output', required=True, type=Path)
    args = parser.parse_args()
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=False)
    commit = run(['git', 'rev-parse', 'HEAD'])
    if run(['git', 'status', '--porcelain', '--untracked-files=no']):
        raise RuntimeError('Commit candidate changes before building archives')
    date = datetime.now(timezone.utc).isoformat()
    evidence = {'version': args.version, 'commit': commit, 'built_at': date,
                'scope': 'Local cross-build and packaging inspection; no publication, container or foreign-architecture execution.',
                'go_version': run(['go', 'version']), 'archives': []}
    ldflags = f'-w -s -X github.com/soulteary/version-kit/v4.Version={args.version} -X github.com/soulteary/version-kit/v4.Commit={commit} -X github.com/soulteary/version-kit/v4.BuildDate={date}'
    checksums = []
    # Build the committed tree, excluding unrelated output files and local data.
    source_tree = tempfile.TemporaryDirectory(prefix='flare-release-source-')
    with tarfile.open(fileobj=io.BytesIO(subprocess.check_output(['git', 'archive', commit], cwd=ROOT))) as tar:
        tar.extractall(source_tree.name, filter='data')
    # Some embedded editor/guide assets are generated and intentionally ignored
    # by Git. Generate them inside the snapshot, not only in the working tree.
    generation_env = dict(os.environ)
    for key in ('GOOS', 'GOARCH', 'GOARM'):
        generation_env.pop(key, None)
    subprocess.run(['go', 'run', 'build/build.go'], cwd=source_tree.name, env=generation_env, check=True)
    evidence['production_assets_generated_in_snapshot'] = True
    for goos, arch, arm in TARGETS:
        name = f'{goos}_{arch}' + (f'v{arm}' if arm else '')
        directory = output / name
        directory.mkdir()
        binary = directory / 'flare'
        env = {**os.environ, 'CGO_ENABLED': '0', 'GOOS': goos, 'GOARCH': arch, 'GOARM': arm}
        subprocess.run(['go', 'build', '-buildvcs=false', '-ldflags', ldflags, '-o', str(binary), '.'], cwd=source_tree.name, env=env, check=True)
        info = run(['go', 'version', '-m', str(binary)])
        for expected in (f'GOOS={goos}', f'GOARCH={arch}', 'CGO_ENABLED=0', commit, args.version):
            if expected not in info:
                raise RuntimeError(f'{name}: missing metadata {expected}')
        if arm and f'GOARM={arm}' not in info:
            raise RuntimeError(f'{name}: wrong GOARM')
        archive = output / f'flare_{args.version}_{name}.tar.gz'
        with tarfile.open(archive, 'w:gz') as tar:
            for source in (binary, ROOT / 'LICENSE', ROOT / 'README.md'):
                tar.add(source, arcname=source.name)
        with tarfile.open(archive) as tar:
            if sorted(tar.getnames()) != ['LICENSE', 'README.md', 'flare']:
                raise RuntimeError(f'{name}: wrong archive contents')
            if hashlib.sha256(tar.extractfile('flare').read()).hexdigest() != sha(binary):
                raise RuntimeError(f'{name}: archived binary mismatch')
        digest = sha(archive)
        checksums.append(f'{digest}  {archive.name}\n')
        evidence['archives'].append({'target': name, 'archive': archive.name, 'sha256': digest,
                                     'binary_sha256': sha(binary), 'build_info': info,
                                     'file': run(['file', str(binary)])})
        print(f'PASS {name}', flush=True)
    (output / f'flare_{args.version}_checksums.txt').write_text(''.join(checksums))
    (output / 'archives-verification.json').write_text(json.dumps(evidence, indent=2) + '\n')
    source_tree.cleanup()


if __name__ == '__main__':
    main()
