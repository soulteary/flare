#!/usr/bin/env python3
"""Compare two existing Flare baseline binaries with a fixed, isolated client."""
import argparse
from datetime import datetime
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import platform
import statistics
import sys
from zoneinfo import ZoneInfo

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('http_comparison', ROOT / 'scripts/compare-http-stacks.py')
comparison = importlib.util.module_from_spec(spec)
spec.loader.exec_module(comparison)


def binary_metadata(path, revision, env):
    return {'path': str(path), 'sha256': hashlib.sha256(path.read_bytes()).hexdigest(),
            'declared_revision': revision,
            'go_build_info': comparison.run(['go', 'version', '-m', path], env).strip()}


def describe_semantics(rows):
    """Strictly retain every normalized mismatch; never relabel it as equality."""
    result = {}
    for scenario in comparison.SCENARIOS:
        variants = {}
        for label in ('before', 'after'):
            values = [{key: value for key, value in row['preflight'].items() if key != 'raw_sha256'}
                      for row in rows if row['target'] == label and row['Scenario'] == scenario]
            variants[label] = list({json.dumps(value, sort_keys=True): value for value in values}.values())
        equal = len(variants['before']) == len(variants['after']) == 1 and variants['before'] == variants['after']
        result[scenario] = {'equal': equal, 'responses': variants,
                            'interpretation': 'Equivalent normalized response' if equal else
                            'Response differs: inspect recorded bodies before interpreting a speedup'}
    return result


def render_report(data, destination):
    meta = data['metadata']
    lines = ['# Isolated process resource comparison', '',
             f"Generated: {meta['generated_at']}", '',
             f"Before: `{meta['binaries']['before']['declared_revision']}`", '',
             f"After: `{meta['binaries']['after']['declared_revision']}`", '',
             f"Fixed client SHA-256: `{meta['client']['sha256']}`", '',
             f"Environment: {meta['host']}; {meta['go_version']}; GOMAXPROCS={meta['parameters']['gomaxprocs']}", '',
             f"Workload: {meta['parameters']['resource_requests']} requests/scene/target/round; "
             f"{meta['parameters']['resource_warmup']} warmup; concurrency {meta['parameters']['concurrency']}; "
             f"pool {meta['parameters']['keepalive_connections']}; {meta['parameters']['rounds']} alternating rounds.", '',
             '## Response validation', '',
             '| Scene | Normalized response equality | Before / after bytes |',
             '| --- | --- | --- |']
    for scenario, value in data['semantics'].items():
        sizes = ['/'.join(str(row['bytes']) for row in value['responses'][label]) for label in ('before', 'after')]
        lines.append(f"| {scenario} | {'equal' if value['equal'] else '**DIFFERENT — inspect raw bodies**'} | {' / '.join(sizes)} |")
    columns = [('server_cpu_ns_per_request', 1000, 'CPU µs/request'),
               ('server_bytes_per_request', 1, 'allocated B/request'),
               ('server_allocs_per_request', 1, 'allocations/request'),
               ('idle_rss_bytes', 2**20, 'warmed idle RSS MiB'),
               ('sampled_peak_rss_bytes', 2**20, 'sampled load peak RSS MiB'),
               ('lifetime_peak_rss_bytes', 2**20, 'process lifecycle peak RSS MiB'),
               ('server_live_heap_sampled_bytes', 2**20, 'sampled live heap MiB after forced GC'),
               ('P95', 10**6, 'client-observed P95 ms')]
    for key, scale, title in columns:
        lines += ['', f'## {title}', '', '| Scene | Before median | After median | Change |', '| --- | ---: | ---: | ---: |']
        for scenario in comparison.SCENARIOS:
            medians = [statistics.median(row[key] for row in data['isolated'] if row['target'] == label and row['Scenario'] == scenario) / scale for label in ('before', 'after')]
            change = (medians[1] / medians[0] - 1) * 100 if medians[0] else None
            delta = f'{change:+.1f}%' if change is not None else 'n/a'
            lines.append(f'| {scenario} | {medians[0]:.3f} | {medians[1]:.3f} | {delta} |')
    lines += ['', '## Scope and limitations', '',
              '- Load CPU, allocation and GC deltas cover the isolated server process only. Client measurements remain in raw rows and must not be confused with server allocation cost.',
              '- P95 is measured by the fixed client over loopback and includes scheduling and network latency. Four rounds do not establish confidence intervals.',
              '- On Linux, RSS is read from a /proc child verified by both parent PID and namespace-local NSpid. This avoids sampling an unrelated host process in mixed PID-namespace environments.',
              '- Warmed idle RSS is measured after warmup. Sampled peak RSS is sampled every ~50 ms during load; lifecycle peak RSS is retained separately in raw observations.',
              '- Live heap is a sampled `inuse_space` estimate from the server heap profile after forced GC, collected after load counters. It is not exact MemStats HeapAlloc and is not RSS. Default Go heap profiling sampling can make small live-heap differences noisy.',
              '- Both binaries use offline mode, disabled login/request logs, and default fixtures. This does not measure production authentication, weather, browser CPU, ARM hardware, or large bookmark datasets.',
              '- Raw response bodies, normalized fingerprints, binary hashes, build metadata, heap profiles, all per-round observations and execution orders are retained. Only clock-time text is normalized by the existing preflight helper.',
              '- A response mismatch remains a mismatch. Explain it before interpreting resource differences; do not claim a like-for-like improvement if functionality was removed.', '']
    destination.write_text('\n'.join(lines))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--before', required=True, type=Path)
    parser.add_argument('--after', required=True, type=Path)
    parser.add_argument('--client', required=True, type=Path)
    parser.add_argument('--before-revision', required=True)
    parser.add_argument('--after-revision', required=True)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--rounds', type=int, default=4)
    parser.add_argument('--resource-requests', type=int, default=40000)
    parser.add_argument('--resource-warmup', type=int, default=1000)
    parser.add_argument('--concurrency', type=int, default=8)
    parser.add_argument('--keepalive-connections', type=int, default=8)
    parser.add_argument('--gomaxprocs', type=int, default=4)
    args = parser.parse_args()
    if min(args.rounds, args.resource_requests, args.resource_warmup, args.concurrency, args.gomaxprocs) < 1 or args.keepalive_connections < 0:
        parser.error('workloads must be positive and pool non-negative')
    targets = {'before': args.before.resolve(), 'after': args.after.resolve()}
    args.client = args.client.resolve()
    args.output = args.output.resolve()
    args.output.mkdir(parents=True, exist_ok=True)
    if (args.output / 'results.json').exists():
        parser.error('output already contains observations; choose a fresh directory')
    args.capture_live_heap = True
    env = dict(os.environ, GOMAXPROCS=str(args.gomaxprocs))
    parameters = {name: getattr(args, name) for name in ('rounds', 'resource_requests', 'resource_warmup', 'concurrency', 'keepalive_connections', 'gomaxprocs')}
    data = {'metadata': {'generated_at': datetime.now(ZoneInfo('Asia/Shanghai')).isoformat(timespec='seconds'),
            'host': platform.platform(), 'machine': platform.machine(), 'logical_cpus': os.cpu_count(),
            'affinity_cpus': len(os.sched_getaffinity(0)) if hasattr(os, 'sched_getaffinity') else None,
            'go_version': comparison.run(['go', 'version'], env).strip(),
            'harness_revision': comparison.run(['git', 'rev-parse', 'HEAD'], env).strip(),
            'harness_sha256': hashlib.sha256(Path(__file__).read_bytes()).hexdigest(),
            'isolation_helper_sha256': hashlib.sha256((ROOT / 'scripts/compare-http-stacks.py').read_bytes()).hexdigest(),
            'cpu_model': next((line.split(':', 1)[1].strip() for line in Path('/proc/cpuinfo').read_text().splitlines() if line.startswith('model name')), platform.processor()) if Path('/proc/cpuinfo').exists() else platform.processor(),
            'cgroup_cpu_max': Path('/sys/fs/cgroup/cpu.max').read_text().strip() if Path('/sys/fs/cgroup/cpu.max').exists() else None,
            'rss_collector': 'Linux /proc VmRSS with verified host PPid and namespace-local NSpid' if sys.platform.startswith('linux') else 'ps rss',
            'parameters': parameters, 'runtime_env': {key: env.get(key, '(unset)') for key in ('GOGC', 'GOMEMLIMIT', 'GODEBUG', 'TZ')},
            'binaries': {label: binary_metadata(binary, getattr(args, label + '_revision'), env) for label, binary in targets.items()},
            'client': binary_metadata(args.client, args.before_revision, env), 'orders': []}, 'isolated': []}
    for round_number in range(args.rounds):
        order = ['before', 'after'] if round_number % 2 == 0 else ['after', 'before']
        data['metadata']['orders'].append({'round': round_number + 1, 'targets': order})
        for scenario in comparison.SCENARIOS:
            for label in order:
                stem = f'{label}-{scenario}-round-{round_number + 1}'
                print(f'round={round_number + 1}/{args.rounds} scene={scenario} target={label}', flush=True)
                args.capture_response_path = args.output / (stem + '.body')
                row = comparison.isolated(targets[label], args.client, scenario, args, env, args.output / (stem + '.stderr.log'))
                row.update(target=label, round=round_number + 1)
                data['isolated'].append(row)
                (args.output / 'results.json').write_text(json.dumps(data, indent=2) + '\n')
    data['semantics'] = describe_semantics(data['isolated'])
    (args.output / 'results.json').write_text(json.dumps(data, indent=2) + '\n')
    render_report(data, args.output / 'report.md')
    if any(not value['equal'] for value in data['semantics'].values()):
        raise RuntimeError('Responses differ; retained raw data and report. Inspect before accepting resource comparisons.')
    print(f"Results: {args.output / 'results.json'}", flush=True)


if __name__ == '__main__':
    main()
