import importlib.util
import pathlib
import tempfile
import unittest
import time
from unittest.mock import patch

spec=importlib.util.spec_from_file_location('acceptance',pathlib.Path(__file__).with_name('acceptance.py'))
mod=importlib.util.module_from_spec(spec); spec.loader.exec_module(mod)

class FixtureTest(unittest.TestCase):
    def test_exact_release_fixture_and_reproducible_bytes(self):
        with tempfile.TemporaryDirectory() as directory:
            first=mod.fixture(directory); raw=(pathlib.Path(directory)/'bookmarks.yml').read_bytes()
            self.assertEqual(raw.count(b'  - name:'),20000)
            self.assertEqual(raw.count(b'private: true'),10000)
            self.assertEqual(raw.count(b'private: false'),10000)
            self.assertEqual(raw.count(b'  - id:'),100)
            self.assertEqual(first,mod.fixture(directory))
            self.assertEqual(raw,(pathlib.Path(directory)/'bookmarks.yml').read_bytes())
    def test_statuses_and_latency_are_not_success_only(self):
        out=mod.summary([(303,1),(429,2),('exception',3)])
        self.assertEqual(out['requests'],3)
        self.assertEqual(out['statuses'],{'303':1,'429':1,'exception':1})
        self.assertEqual(out['p50_ms'],2)
        self.assertEqual(out['p95_ms'],3)

class AdministratorPressureTest(unittest.TestCase):
    def test_search_query_echo_is_not_a_visible_private_card(self):
        self.assertFalse(mod.contains_private_card(b'<input value="PRIVATE Entry 19999">'))
        self.assertTrue(mod.contains_private_card(b'<a><span>PRIVATE Entry 19999</span></a>'))
    def test_http200_without_private_content_is_a_failed_assertion(self):
        class MissingPrivateClient:
            def __init__(self, port): self.cookies={}
            def login(self): return 303,b"",1
            def request(self,path="/",body=None,headers=None):
                time.sleep(.001)
                if path=="/editor/csrf": return 200,b'{"csrfToken":"test"}',1
                if path=="/logout": return 303,b"",1
                return 200,b"PUBLIC Entry 19998",1
        with patch.object(mod,"Client",MissingPrivateClient):
            result=mod.administrator_pressure(1,.025)
        self.assertTrue(result["privacy_violations"])
        self.assertGreater(result["administrator_home"]["requests"],0)
        self.assertTrue(any(row["kind"]=="home" for row in result["privacy_violations"]))
        self.assertGreaterEqual(result["actual_pressure_elapsed_seconds"],.025)

if __name__=='__main__': unittest.main()
