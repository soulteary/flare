"""Exercise archive/provenance and tool-install boundaries without publishing."""
import hashlib
import importlib.util
import io
import json
from pathlib import Path
import subprocess
import tarfile
import tempfile
import unittest
from unittest import mock

ROOT = Path(__file__).resolve().parents[2]


def load_module(name, path):
    spec = importlib.util.spec_from_file_location(name, path)
    module = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(module)
    return module


preflight = load_module('preflight', ROOT / 'scripts/release-preflight.py')
installer = load_module('installer', Path(__file__).with_name('install-report-tool.py'))


def tar_bytes(files):
    stream = io.BytesIO()
    with tarfile.open(fileobj=stream, mode='w:gz') as tar:
        for name, data, mode in files:
            info = tarfile.TarInfo(name)
            info.size, info.mode = len(data), mode
            tar.addfile(info, io.BytesIO(data))
    return stream.getvalue()


class InstallerTests(unittest.TestCase):
    def install(self, archive, checksum=None, tool='gtr', version='v1.1.2'):
        arch = 'amd64'
        prefix = 'gtr' if tool == 'gtr' else 'goreportcard'
        asset = f'{prefix}_{version}_linux_{arch}.tar.gz'
        checksums = (checksum or hashlib.sha256(archive).hexdigest()) + '  ' + asset + '\n'
        def response(url, **kwargs):
            return io.BytesIO(checksums.encode() if url.endswith('/checksums.txt') else archive)
        with tempfile.TemporaryDirectory() as directory:
            destination = Path(directory) / 'bin'
            with mock.patch.object(installer.platform, 'system', return_value='Linux'), mock.patch.object(installer.platform, 'machine', return_value='x86_64'), mock.patch.object(installer.urllib.request, 'urlopen', side_effect=response):
                installer.install(tool, version, destination)
            return {file.name: file.read_bytes() for file in destination.iterdir()}

    def test_verified_release_binary_and_bundled_linters_are_installed(self):
        archive = tar_bytes([('release/goreportcard-cli', b'cli', 0o755),
                             ('release/misspell', b'linter', 0o755),
                             ('README', b'docs', 0o644)])
        self.assertEqual(self.install(archive, tool='goreportcard', version='v1.1.3'),
                         {'goreportcard-cli': b'cli', 'misspell': b'linter'})

    def test_checksum_failure_is_fatal(self):
        with self.assertRaisesRegex(ValueError, 'Checksum mismatch'):
            self.install(tar_bytes([('gtr', b'cli', 0o755)]), '0' * 64)

    def test_missing_binary_or_ambiguous_names_are_rejected(self):
        for files in [[('README', b'docs', 0o644)],
                      [('a/gtr', b'a', 0o755), ('b/gtr', b'b', 0o755)]]:
            with self.subTest(files=files), self.assertRaises(ValueError):
                self.install(tar_bytes(files))

    def test_unreviewed_release_never_reaches_network(self):
        with mock.patch.object(installer.urllib.request, 'urlopen') as network:
            with self.assertRaises(ValueError):
                installer.install('gtr', 'latest', Path('/unused'))
            network.assert_not_called()


class ArchiveTests(unittest.TestCase):
    version = '0.6.6'
    commit = 'a' * 40

    def create_candidates(self, directory):
        for target, (goos, arch, arm) in preflight.TARGET_MAP.items():
            binary = f'binary-{target}'.encode()
            archive = directory / f'flare_{self.version}_{target}.tar.gz'
            archive.write_bytes(tar_bytes([('flare', binary, 0o755),
                                          ('LICENSE', b'license', 0o644),
                                          ('README.md', b'readme', 0o644)]))
            entry = {'target': target, 'archive': archive.name,
                     'sha256': preflight.sha(archive),
                     'binary_sha256': hashlib.sha256(binary).hexdigest(),
                     'build_info': f'GOOS={goos} GOARCH={arch} GOARM={arm} CGO_ENABLED=0 {self.commit} {self.version}'}
            (directory / f'{target}-verification.json').write_text(json.dumps(
                {'version': self.version, 'commit': self.commit,
                 'production_assets_generated_in_snapshot': True, 'archives': [entry]}))

    def test_collect_verifies_seven_targets_and_extracts_the_archived_binary(self):
        with tempfile.TemporaryDirectory() as directory, mock.patch.object(preflight, 'run', return_value='go fixture'):
            directory = Path(directory)
            self.create_candidates(directory)
            preflight.collect_archives(directory, self.version, self.commit)
            self.assertEqual((directory / 'linux_amd64/flare').read_bytes(), b'binary-linux_amd64')
            self.assertTrue((directory / 'linux_amd64/flare').stat().st_mode & 0o111)
            self.assertEqual(len(json.loads((directory / 'archives-verification.json').read_text())['archives']), 7)
            self.assertEqual(len((directory / f'flare_{self.version}_checksums.txt').read_text().splitlines()), 7)

    def test_moved_source_missing_target_and_corrupted_archive_fail_closed(self):
        for problem in ('commit', 'missing', 'archive', 'binary', 'platform'):
            with self.subTest(problem=problem), tempfile.TemporaryDirectory() as directory:
                directory = Path(directory)
                self.create_candidates(directory)
                evidence = directory / 'linux_armv6-verification.json'
                data = json.loads(evidence.read_text())
                if problem == 'commit':
                    data['commit'] = 'b' * 40
                elif problem == 'binary':
                    data['archives'][0]['binary_sha256'] = '0' * 64
                elif problem == 'platform':
                    data['archives'][0]['build_info'] = data['archives'][0]['build_info'].replace('GOARM=6', 'GOARM=7')
                elif problem == 'archive':
                    (directory / data['archives'][0]['archive']).write_bytes(b'corrupt')
                evidence.write_text(json.dumps(data))
                if problem == 'missing':
                    evidence.unlink()
                with self.assertRaises((RuntimeError, FileNotFoundError)):
                    preflight.collect_archives(directory, self.version, self.commit)

    def test_generation_rejects_stale_tracked_assets_and_bundles_ignored_assets(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'repo'
            root.mkdir()
            (root / 'tracked.txt').write_text('original')
            subprocess.run(['git', 'init', '-q', str(root)], check=True)
            subprocess.run(['git', '-C', str(root), 'add', '.'], check=True)
            subprocess.run(['git', '-C', str(root), '-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '--no-gpg-sign', '-qm', 'fixture'], check=True)
            commit = subprocess.check_output(['git', '-C', str(root), 'rev-parse', 'HEAD'], text=True).strip()
            real_run = subprocess.run
            def generate(args, **kwargs):
                if args[:2] == ['go', 'run']:
                    snapshot = Path(kwargs['cwd'])
                    (snapshot / 'ignored.txt').write_text('generated')
                    if stale:
                        (snapshot / 'tracked.txt').write_text('different')
                    return subprocess.CompletedProcess(args, 0)
                return real_run(args, **kwargs)
            with mock.patch.object(preflight, 'ROOT', root), mock.patch.object(preflight.subprocess, 'run', side_effect=generate):
                stale = True
                with self.assertRaisesRegex(RuntimeError, 'stale'):
                    preflight.prepare_source(Path(directory) / 'bad.tar.gz', self.version, commit)
                stale = False
                bundle = Path(directory) / 'source.tar.gz'
                preflight.prepare_source(bundle, self.version, commit)
                with tarfile.open(bundle) as tar:
                    self.assertEqual(tar.extractfile('ignored.txt').read(), b'generated')
                    self.assertEqual(json.load(tar.extractfile('.flare-preflight-source.json'))['commit'], commit)


if __name__ == '__main__':
    unittest.main()
