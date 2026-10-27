import importlib.util
import pathlib
import tempfile
import unittest


def load(name):
    spec=importlib.util.spec_from_file_location(name,pathlib.Path(__file__).with_name(name+'.py'))
    module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module);return module


class TransactionDiagnosticTests(unittest.TestCase):
    def test_counter_delta_rejects_reset_or_changed_schema(self):
        counts=load('transaction_counts')
        self.assertEqual(counts.delta({'commit':4},{'commit':6}),{'commit':2})
        with self.assertRaises(ValueError):counts.delta({'commit':4},{'commit':1})
        with self.assertRaises(ValueError):counts.delta({'commit':4},{'other':4})

    def test_overlay_rejects_real_repository_before_any_write(self):
        overlay=load('transaction_overlay')
        repository=pathlib.Path(__file__).resolve().parents[2]
        store=repository/'internal/security/store.go';original=store.read_bytes()
        with self.assertRaises(ValueError):overlay.instrument(repository)
        self.assertEqual(store.read_bytes(),original)

    def test_overlay_rejects_git_file_and_directory_before_any_write(self):
        overlay=load('transaction_overlay')
        for is_directory in [False,True]:
            with tempfile.TemporaryDirectory() as directory:
                root=pathlib.Path(directory)
                if is_directory:(root/'.git').mkdir()
                else:(root/'.git').write_text('gitdir: /example')
                with self.assertRaises(ValueError):overlay.instrument(root)
                self.assertFalse((root/'internal').exists())

    def test_overlay_rejects_missing_boundary_without_modifying_source(self):
        overlay=load('transaction_overlay')
        with tempfile.TemporaryDirectory() as directory:
            root=pathlib.Path(directory);(root/'internal/security').mkdir(parents=True)
            store=root/'internal/security/store.go';store.write_text('package security\n')
            with self.assertRaises(ValueError):overlay.instrument(root)
            self.assertEqual(store.read_text(),'package security\n')
            self.assertFalse((root/'internal/security/transaction_diagnostic.go').exists())

    def test_overlay_instruments_all_store_sync_calls_in_disposable_copy(self):
        overlay=load('transaction_overlay')
        repository=pathlib.Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory() as directory:
            root=pathlib.Path(directory);(root/'internal/security').mkdir(parents=True)
            source=(repository/'internal/security/store.go').read_text()
            (root/'internal/security/store.go').write_text(source)
            (root/'main_profile_test.go').write_text((repository/'main_profile_test.go').read_text())
            report=overlay.instrument(root)
            changed=(root/'internal/security/store.go').read_text()
            self.assertGreaterEqual(report['sync_callsites_wrapped'],3)
            self.assertNotIn('f.Sync()',changed);self.assertNotIn('d.Sync()',changed)
            self.assertEqual((repository/'internal/security/store.go').read_text(),source)
            self.assertIn('/counters',(root/'main_profile_test.go').read_text())


if __name__=='__main__':unittest.main()
