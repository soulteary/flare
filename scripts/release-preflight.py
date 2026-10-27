#!/usr/bin/env python3
"""Build and inspect release candidates without publication or tagging.

A prepared source bundle generates production resources once. Matrix consumers
build one target each; Linux containers use the exact binary in their archive.
The default local invocation still builds and verifies all seven targets.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timezone
import hashlib
import io
import json
import os
from pathlib import Path
import re
import subprocess
import tarfile
import tempfile

ROOT = Path(__file__).resolve().parents[1]
TARGETS = [('darwin', 'amd64', ''), ('darwin', 'arm64', ''),
           ('linux', '386', ''), ('linux', 'amd64', ''),
           ('linux', 'arm64', ''), ('linux', 'arm', '6'), ('linux', 'arm', '7')]


def target_name(target):
    goos, arch, arm = target
    return f'{goos}_{arch}' + (f'v{arm}' if arm else '')


TARGET_MAP = {target_name(target): target for target in TARGETS}


def run(args, **kwargs):
    return subprocess.check_output(args, cwd=ROOT, text=True, **kwargs).strip()


def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def prepare_source(bundle, version, commit):
    """Generate once in a committed snapshot, checking every tracked output."""
    if run(['git', 'status', '--porcelain', '--untracked-files=no']):
        raise RuntimeError('Commit candidate changes before building archives')
    with tempfile.TemporaryDirectory(prefix='flare-release-source-') as directory:
        source = Path(directory)
        with tarfile.open(fileobj=io.BytesIO(subprocess.check_output(['git', 'archive', commit], cwd=ROOT))) as tar:
            tar.extractall(source, filter='data')
        generation_env = dict(os.environ)
        for key in ('GOOS', 'GOARCH', 'GOARM'):
            generation_env.pop(key, None)
        subprocess.run(['go', 'run', '-trimpath', 'build/build.go'], cwd=source, env=generation_env, check=True)
        # git diff cannot see ignored generated assets, which are included below.
        tracked = subprocess.check_output(['git', 'ls-files', '-z'], cwd=ROOT).decode().split('\0')
        for name in filter(None, tracked):
            original, generated = ROOT / name, source / name
            if original.is_file() and (not generated.is_file() or original.read_bytes() != generated.read_bytes()):
                raise RuntimeError(f'Generated tracked asset is stale: {name}')
        metadata = {'version': version, 'commit': commit,
                    'built_at': datetime.now(timezone.utc).isoformat(),
                    'build_date': run(['git', 'show', '-s', '--format=%cI', commit]),
                    'production_assets_generated_in_snapshot': True}
        (source / '.flare-preflight-source.json').write_text(json.dumps(metadata) + '\n')
        bundle.parent.mkdir(parents=True, exist_ok=True)
        with bundle.open('xb') as stream, tarfile.open(fileobj=stream, mode='w:gz') as tar:
            for path in sorted(source.iterdir()):
                tar.add(path, arcname=path.name)


def verify_archive(archive, entry, version, commit):
    target = entry['target']
    if target not in TARGET_MAP or archive.name != f'flare_{version}_{target}.tar.gz':
        raise RuntimeError(f'Unexpected archive target/name: {archive.name}')
    if sha(archive) != entry['sha256']:
        raise RuntimeError(f'{target}: archive checksum mismatch')
    with tarfile.open(archive) as tar:
        members = tar.getmembers()
        if sorted(member.name for member in members) != ['LICENSE', 'README.md', 'flare'] or not all(member.isfile() for member in members):
            raise RuntimeError(f'{target}: wrong archive contents')
        binary = tar.extractfile('flare').read()
        if hashlib.sha256(binary).hexdigest() != entry['binary_sha256']:
            raise RuntimeError(f'{target}: archived binary mismatch')
    info = entry['build_info']
    goos, arch, arm = TARGET_MAP[target]
    expected = [f'GOOS={goos}', f'GOARCH={arch}', 'CGO_ENABLED=0', commit, version]
    if arm:
        expected.append(f'GOARM={arm}')
    if not all(value in info for value in expected):
        raise RuntimeError(f'{target}: missing/wrong build metadata')
    return binary


def build_target(target, source, output, metadata):
    goos, arch, arm = TARGET_MAP[target]
    directory = output / target
    directory.mkdir()
    binary = directory / 'flare'
    version, commit = metadata['version'], metadata['commit']
    # Repeated checks of one commit must produce the same executable for layer reuse.
    date = metadata.get('build_date', metadata['built_at'])
    ldflags = f'-w -s -X github.com/soulteary/version-kit/v4.Version={version} -X github.com/soulteary/version-kit/v4.Commit={commit} -X github.com/soulteary/version-kit/v4.BuildDate={date}'
    env = {**os.environ, 'CGO_ENABLED': '0', 'GOOS': goos, 'GOARCH': arch, 'GOARM': arm}
    subprocess.run(['go', 'build', '-trimpath', '-buildvcs=false', '-ldflags', ldflags, '-o', str(binary), '.'], cwd=source, env=env, check=True)
    archive = output / f'flare_{version}_{target}.tar.gz'
    with tarfile.open(archive, 'w:gz') as tar:
        for path in (binary, source / 'LICENSE', source / 'README.md'):
            tar.add(path, arcname=path.name)
    entry = {'target': target, 'archive': archive.name, 'sha256': sha(archive),
             'binary_sha256': sha(binary), 'build_info': run(['go', 'version', '-m', str(binary)]),
             'file': run(['file', str(binary)])}
    verify_archive(archive, entry, version, commit)
    evidence = {**metadata, 'archives': [entry]}
    (output / f'{target}-verification.json').write_text(json.dumps(evidence, indent=2) + '\n')
    print(f'PASS {target}', flush=True)
    return entry


def collect_archives(output, version, commit):
    entries = []
    for target in TARGET_MAP:
        evidence = json.loads((output / f'{target}-verification.json').read_text())
        if evidence['version'] != version or evidence['commit'] != commit or evidence.get('production_assets_generated_in_snapshot') is not True:
            raise RuntimeError(f'{target}: source provenance mismatch')
        if len(evidence['archives']) != 1 or evidence['archives'][0]['target'] != target:
            raise RuntimeError(f'{target}: invalid target evidence')
        entry = evidence['archives'][0]
        binary = verify_archive(output / entry['archive'], entry, version, commit)
        # Run browser acceptance against bytes extracted from the verified archive.
        if target == 'linux_amd64':
            directory = output / target
            directory.mkdir(exist_ok=True)
            (directory / 'flare').write_bytes(binary)
            (directory / 'flare').chmod(0o755)
        entries.append(entry)
    write_summary(output, {'version': version, 'commit': commit,
                          'production_assets_generated_in_snapshot': True}, entries)


def write_summary(output, metadata, entries):
    (output / f"flare_{metadata['version']}_checksums.txt").write_text(''.join(
        f"{entry['sha256']}  {entry['archive']}\n" for entry in entries))
    (output / 'archives-verification.json').write_text(json.dumps(
        {**metadata, 'scope': 'Cross-build and archive inspection; no publication.',
         'go_version': run(['go', 'version']), 'archives': entries}, indent=2) + '\n')


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--version', required=True)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--target', choices=TARGET_MAP, action='append')
    parser.add_argument('--jobs', type=int, default=2)
    parser.add_argument('--source-archive', type=Path)
    parser.add_argument('--prepare-only', action='store_true')
    parser.add_argument('--collect', action='store_true')
    args = parser.parse_args()
    if not re.fullmatch(r'[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?', args.version):
        parser.error('--version must be a semantic version without a v prefix')
    if args.jobs < 1:
        parser.error('--jobs must be positive')
    output = args.output.resolve()
    commit = run(['git', 'rev-parse', 'HEAD'])
    if args.collect:
        collect_archives(output, args.version, commit)
        return
    if args.prepare_only:
        prepare_source(output, args.version, commit)
        return
    output.mkdir(parents=True, exist_ok=False)
    with tempfile.TemporaryDirectory(prefix='flare-release-build-') as directory:
        bundle = args.source_archive
        if bundle is None:
            bundle = Path(directory) / 'source.tar.gz'
            prepare_source(bundle, args.version, commit)
        source = Path(directory) / 'source'
        with tarfile.open(bundle) as tar:
            tar.extractall(source, filter='data')
        metadata = json.loads((source / '.flare-preflight-source.json').read_text())
        if metadata['version'] != args.version or metadata['commit'] != commit or metadata.get('production_assets_generated_in_snapshot') is not True:
            raise RuntimeError('Prepared source does not match this candidate version/commit')
        targets = args.target or list(TARGET_MAP)
        if len(set(targets)) != len(targets):
            parser.error('Duplicate targets')
        with ThreadPoolExecutor(max_workers=min(args.jobs, len(targets))) as pool:
            entries = list(pool.map(lambda target: build_target(target, source, output, metadata), targets))
        write_summary(output, metadata, entries)


if __name__ == '__main__':
    main()
