#!/usr/bin/env python3
"""Demonstrate whether Linux RUSAGE MaxRSS inherits launcher memory across exec."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import subprocess
import sys

sys.dont_write_bytecode = True
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--target', type=Path, required=True, help='existing baseline executable')
parser.add_argument('--output', type=Path, required=True)
args = parser.parse_args()
if not sys.platform.startswith('linux'):
    parser.error('this diagnostic requires Linux /proc')
spec = importlib.util.spec_from_file_location('comparison', Path(__file__).resolve().with_name('compare-http-stacks.py'))
comparison = importlib.util.module_from_spec(spec)
spec.loader.exec_module(comparison)
observations = []
for size in (0, 80 * 1024 * 1024):
    allocation = bytearray(size)
    process = subprocess.Popen([str(args.target.resolve()), '-serve'], stdout=subprocess.PIPE,
                               stderr=subprocess.PIPE, text=True, env=dict(os.environ, GOMAXPROCS='4'))
    try:
        info = comparison.ready(process)
        observations.append({'parent_allocation_bytes': size,
            'target_rss_bytes': comparison.rss(process.pid),
            'target_VmHWM_bytes': comparison.proc_high_water_rss(process.pid),
            'target_RUSAGE_MaxRSS_bytes': comparison.stats(info['control'])['PeakRSSBytes']})
    finally:
        process.terminate()
        process.wait(timeout=8)
        process.stdout.close()
        process.stderr.close()
    del allocation
with args.output.open('x') as destination:
    json.dump(observations, destination, indent=2)
    destination.write('\n')
print(json.dumps(observations, indent=2))
