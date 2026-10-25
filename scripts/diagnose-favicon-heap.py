#!/usr/bin/env python3
"""Measure exact post-GC heap change across one favicon request, in fresh targets."""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import sys
sys.dont_write_bytecode = True

parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument('--helper', type=Path, default=Path(__file__).resolve().with_name('compare-http-stacks.py'))
parser.add_argument('--target', action='append', required=True, help='label=baseline-binary')
parser.add_argument('--output', type=Path, required=True)
args = parser.parse_args()
spec = importlib.util.spec_from_file_location('comparison', args.helper)
comparison = importlib.util.module_from_spec(spec)
spec.loader.exec_module(comparison)
env = dict(os.environ, GOMAXPROCS='4')
rows = []
for target in args.target:
    label, separator, binary = target.partition('=')
    if not separator:
        parser.error('--target requires label=binary')
    binary = str(Path(binary).resolve())
    process = subprocess.Popen([binary, '-serve'], stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, env=env)
    try:
        info = comparison.ready(process)
        def live():
            with comparison.OPENER.open(info['control'] + '/debug/pprof/heap?gc=1&debug=1') as response:
                profile = response.read().decode()
            return int(re.search(r'^# HeapAlloc = (\d+)$', profile, re.M).group(1))
        before = live()
        with comparison.OPENER.open(info['url'] + '/favicon.ico') as response:
            content = response.read()
            status = response.status
        after = live()
        rows.append({'target': label, 'binary_sha256': hashlib.sha256(Path(binary).read_bytes()).hexdigest(),
            'before_favicon_live_heap_bytes': before, 'after_favicon_live_heap_bytes': after,
            'delta_bytes': after-before, 'favicon_status': status, 'favicon_bytes': len(content),
            'favicon_sha256': hashlib.sha256(content).hexdigest()})
    finally:
        process.terminate()
        process.wait(timeout=8)
        process.stdout.close()
        process.stderr.close()
args.output.write_text(json.dumps(rows, indent=2) + '\n')
print(json.dumps(rows, indent=2))
