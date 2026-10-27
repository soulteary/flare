import importlib.util
import pathlib
import threading
import time
import unittest

spec = importlib.util.spec_from_file_location("resource_analysis", pathlib.Path(__file__).with_name("resource_analysis.py"))
mod = importlib.util.module_from_spec(spec)
spec.loader.exec_module(mod)


class ResourceAnalysisTest(unittest.TestCase):
    def test_cpu_time_units(self):
        self.assertEqual(mod.cpu_seconds("2:03.50"), 123.5)
        self.assertEqual(mod.cpu_seconds("1:02:03.50"), 3723.5)
        self.assertEqual(mod.cpu_seconds("1-1:02:03.50"), 90123.5)
        with self.assertRaises(ValueError):
            mod.cpu_seconds("123")

    def test_open_loop_has_no_waiting_backlog(self):
        lock = threading.Lock()
        active, peak, completed = 0, 0, 0

        def slow(index):
            nonlocal active, peak, completed
            with lock:
                active += 1
                peak = max(peak, active)
            time.sleep(.04)
            with lock:
                active -= 1
                completed += 1

        result = mod.bounded_offered_load(.1, 100, 1, slow)
        self.assertEqual(result["offered"], 10)
        self.assertEqual(result["admitted"] + len(result["drops"]), 10)
        self.assertEqual(result["admitted"], completed)
        self.assertEqual(peak, 1)
        self.assertTrue(result["drops"])
        self.assertEqual(result["waiting_queue_capacity"], 0)
        self.assertGreaterEqual(result["actual_elapsed_seconds"], .1)

    def test_worker_failure_is_counted_and_releases_capacity(self):
        def broken(index):
            raise RuntimeError("expected")
        result = mod.bounded_offered_load(.05, 40, 1, broken)
        self.assertEqual(result["offered"], 2)
        self.assertGreaterEqual(result["admitted"], 1)
        self.assertEqual(len(result["operation_errors"]), result["admitted"])
        self.assertFalse(any(row["reason"] == "inflight-cap" for row in result["drops"]))


if __name__ == "__main__":
    unittest.main()
