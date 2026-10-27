"""Ensure the published audit rejects modified derived data without altering it."""
import contextlib
import importlib.util
import io
import json
from pathlib import Path
import shutil
import subprocess
import sys
import tarfile
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parent
spec = importlib.util.spec_from_file_location('csrf_report_analyze', ROOT / 'analyze.py')
audit = importlib.util.module_from_spec(spec)
spec.loader.exec_module(audit)


class EvidenceIntegrity(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        with tarfile.open(ROOT / 'raw.tar.gz') as archive:
            cls.rows = [json.load(archive.extractfile(member)) for member in archive]

    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.root = Path(self.directory.name)
        for name in ('raw.tar.gz', 'summary.json', 'windows.csv', 'render.py', 'report.md'):
            shutil.copyfile(ROOT / name, self.root / name)

    def verify(self):
        with patch.object(audit, 'ROOT', self.root), patch.object(sys, 'argv', ['analyze.py']):
            with contextlib.redirect_stdout(io.StringIO()):
                audit.main()

    def render(self, check=True):
        command = [sys.executable, str(self.root / 'render.py')]
        if check:
            command.append('--check')
        return subprocess.run(command, capture_output=True, text=True)

    def test_published_evidence_and_tables(self):
        self.verify()
        before = (self.root / 'report.md').read_bytes()
        self.assertEqual(self.render().returncode, 0)
        self.assertEqual((self.root / 'report.md').read_bytes(), before)

    def test_rejects_modified_csv(self):
        with (self.root / 'windows.csv').open('ab') as out:
            out.write(b'corrupted,row\n')
        with self.assertRaisesRegex(AssertionError, 'windows.csv'):
            self.verify()

    def test_rejects_modified_summary(self):
        path = self.root / 'summary.json'
        data = json.loads(path.read_text())
        data['measured_requests'] += 1
        path.write_text(json.dumps(data))
        with self.assertRaises(AssertionError):
            self.verify()

    def test_rejects_inconsistent_throughput(self):
        rows = list(self.rows)
        rows[0] = dict(rows[0], Load=dict(rows[0]['Load']))
        rows[0]['Load']['QPS'] += 1
        with self.assertRaises(AssertionError):
            audit.analyze(rows)

    def test_rejects_inconsistent_sample_count(self):
        rows = list(self.rows)
        load = dict(rows[0]['Load'])
        load['BodyHashes'] = dict(load['BodyHashes'])
        key = next(iter(load['BodyHashes']))
        load['BodyHashes'][key] += 1
        rows[0] = dict(rows[0], Load=load)
        with self.assertRaises(AssertionError):
            audit.analyze(rows)

    def test_rejects_modified_report_table(self):
        path = self.root / 'report.md'
        original = path.read_text()
        self.assertIn('20821 → 23772', original)
        path.write_text(original.replace('20821 → 23772', '20821 → 23773'))
        self.assertNotEqual(self.render().returncode, 0)

    def test_drop_counts_are_rendered_from_data(self):
        path = self.root / 'summary.json'
        data = json.loads(path.read_text())
        group = next(x for x in data['fixed_traffic'] if x['cookie_jar'])
        group['dropped'] = 7
        group['planned'] = 43204
        data['fixed_dropped_requests'] += 4
        path.write_text(json.dumps(data))
        self.assertEqual(self.render(False).returncode, 0)
        self.assertIn('7/43,204', (self.root / 'report.md').read_text())
        self.assertEqual(self.render().returncode, 0)


if __name__ == '__main__':
    unittest.main()
