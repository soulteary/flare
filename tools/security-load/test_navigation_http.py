import collections
from pathlib import Path
import tempfile
import unittest

import navigation_http as benchmark


class NavigationHTTPTests(unittest.TestCase):
    def test_fixture_has_private_and_public_entries_in_each_category(self):
        with tempfile.TemporaryDirectory() as directory:
            records, hashes = benchmark.fixture(Path(directory), 40)
            self.assertEqual(len(hashes), 3)
            for category in range(20):
                self.assertEqual({private for _, private, group in records if group == category}, {False, True})
            self.assertEqual(sum(benchmark.expected_cards(records, "anonymous", "category").values()), 1)
            self.assertEqual(sum(benchmark.expected_cards(records, "authenticated", "category").values()), 2)

    def test_search_checks_both_collections_and_authorization(self):
        records = [("PUBLIC Entry 09900", False, 0), ("PRIVATE Entry 09920", True, 0)]
        self.assertEqual(benchmark.expected_cards(records, "anonymous", "search"), collections.Counter({"PUBLIC Entry 09900": 2}))
        self.assertEqual(sum(benchmark.expected_cards(records, "authenticated", "search").values()), 4)

    def test_echoed_private_query_cannot_satisfy_card_assertion(self):
        body = b'<input value="PRIVATE Entry 09920"><span>PUBLIC Entry 09900</span><p class="app-title">PUBLIC Entry 09900</p>'
        expected = collections.Counter({"PUBLIC Entry 09900": 2})
        benchmark.check_response(200, body, expected)
        with self.assertRaises(RuntimeError):
            benchmark.check_response(200, body, collections.Counter({"PRIVATE Entry 09920": 1}))
        with self.assertRaises(RuntimeError):
            benchmark.check_response(503, body, expected)

    def test_percentiles_use_nearest_rank(self):
        self.assertEqual(benchmark.percentile([4, 1, 3, 2], .5), 2)
        self.assertEqual(benchmark.percentile([4, 1, 3, 2], .95), 4)


if __name__ == "__main__":
    unittest.main()
