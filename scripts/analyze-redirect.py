#!/usr/bin/env python3
"""Profile or compare isolated redirect targets using the existing workload."""
import argparse
from concurrent.futures import ThreadPoolExecutor
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import platform
import subprocess
import sys
import time

sys.dont_write_bytecode = True
ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('http_comparison', ROOT / 'scripts/compare-http-stacks.py')
comparison = importlib.util.module_from_spec(spec)
spec.loader.exec_module(comparison)


def download(url, path, timeout=30):
    with comparison.OPENER.open(url, timeout=timeout) as response:
        path.write_bytes(response.read())


def profile(label, binary, client, pool, args, env):
    directory = args.output / f'{label}-pool-{pool}-profiles'
    directory.mkdir(exist_ok=True)
    with (directory / 'server.stderr.log').open('w') as errors:
        process = subprocess.Popen([str(binary), '-serve'], cwd=ROOT, env=env,
                                   stdout=subprocess.PIPE, stderr=errors, text=True)
        try:
            info = comparison.ready(process)
            validation = comparison.preflight(info, 'redir-url')
            comparison.run(comparison.client_command(client, info, 'redir-url', args.resource_warmup,
                                                      args.concurrency, pool), env)
            download(info['control'] + '/debug/pprof/heap?gc=1', directory / 'before.heap')
            # The profile HTTP response completes after its sampling window.
            # No client process is part of this target-process CPU profile.
            with ThreadPoolExecutor(max_workers=1) as executor:
                cpu = executor.submit(download, info['control'] + f'/debug/pprof/profile?seconds={args.profile_seconds}',
                                      directory / 'cpu.pprof', args.profile_seconds + 120)
                time.sleep(0.25)
                load = comparison.metrics(comparison.run(comparison.client_command(
                    client, info, 'redir-url', args.profile_requests, args.concurrency, pool), env))[0]
                cpu.result()
            download(info['control'] + '/debug/pprof/heap?gc=1', directory / 'after.heap')
        finally:
            if process.poll() is None:
                process.terminate()
            try:
                process.wait(timeout=8)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
            process.stdout.close()
    for name, options, filename in [
            ('cpu-flat', [], 'cpu.pprof'), ('cpu-cumulative', ['-cum'], 'cpu.pprof'),
            ('alloc-space', ['-alloc_space', '-base', str(directory / 'before.heap')], 'after.heap'),
            ('alloc-objects', ['-alloc_objects', '-base', str(directory / 'before.heap')], 'after.heap')]:
        summary = comparison.run(['go', 'tool', 'pprof', '-top', '-nodecount=40', *options,
                                  binary, directory / filename], env)
        (directory / (name + '.txt')).write_text(summary)
    return {'target': label, 'keepalive_connections': pool, 'read_buffer_bytes': info.get('read_buffer_bytes'),
            'preflight': validation, 'diagnostic_load': load, 'directory': str(directory)}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--target', action='append', required=True, help='unique label=existing baseline binary')
    parser.add_argument('--client', type=Path, required=True, help='one common existing client binary')
    parser.add_argument('--output', type=Path, required=True)
    parser.add_argument('--pools', type=int, nargs='+', default=[0, 8])
    parser.add_argument('--rounds', type=int, default=4)
    parser.add_argument('--resource-requests', type=int, default=100000)
    parser.add_argument('--resource-warmup', type=int, default=1000)
    parser.add_argument('--concurrency', type=int, default=8)
    parser.add_argument('--gomaxprocs', type=int, default=4)
    parser.add_argument('--profiles', action='store_true')
    parser.add_argument('--profiles-only', action='store_true')
    parser.add_argument('--profile-seconds', type=int, default=15)
    parser.add_argument('--profile-requests', type=int, default=200000)
    args = parser.parse_args()
    targets = {}
    for target in args.target:
        label, separator, path = target.partition('=')
        if not separator or not label or any(c not in 'abcdefghijklmnopqrstuvwxyz0123456789_-' for c in label) or label in targets:
            parser.error('--target needs a unique simple label=path')
        targets[label] = Path(path).resolve()
    if any(pool < 0 for pool in args.pools) or min(args.rounds, args.resource_requests, args.resource_warmup,
            args.concurrency, args.gomaxprocs, args.profile_seconds, args.profile_requests) < 1:
        parser.error('workloads must be positive and pools non-negative')
    args.output = args.output.resolve()
    args.output.mkdir(parents=True, exist_ok=True)
    args.client = args.client.resolve()
    env = dict(os.environ, GOMAXPROCS=str(args.gomaxprocs))
    data = {'metadata': {'host': platform.platform(), 'go_version': comparison.run(['go', 'version'], env).strip(),
             'generated_at': comparison.datetime.now(comparison.ZoneInfo('Asia/Shanghai')).isoformat(timespec='seconds'),
             'revision': comparison.run(['git', 'rev-parse', 'HEAD'], env).strip(),
             'targets': {label: {'path': str(path), 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()} for label, path in targets.items()},
             'client': str(args.client), 'client_sha256': hashlib.sha256(args.client.read_bytes()).hexdigest(),
             'parameters': {key: value for key, value in vars(args).items() if key not in ('output', 'target', 'client')},
             'runtime_env': {key: env.get(key, '(unset)') for key in ('GOGC', 'GOMEMLIMIT', 'GODEBUG', 'TZ')},
             'orders': []}, 'isolated': [], 'profiles': []}
    destination = args.output / 'results.json'
    def save():
        destination.write_text(json.dumps(data, indent=2) + '\n')
    if not args.profiles_only:
        for pool in args.pools:
            args.keepalive_connections = pool
            for round_number in range(args.rounds):
                order = list(targets)
                offset = round_number % len(order)
                order = order[offset:] + order[:offset]
                data['metadata']['orders'].append({'pool': pool, 'round': round_number + 1, 'targets': order})
                for label in order:
                    print(f'pool={pool} round={round_number + 1}/{args.rounds}: {label}', flush=True)
                    row = comparison.isolated(targets[label], args.client, 'redir-url', args, env,
                                               args.output / f'{label}-pool-{pool}-round-{round_number + 1}.stderr.log')
                    row.update(target=label, round=round_number + 1, keepalive_connections=pool)
                    data['isolated'].append(row)
                    save()
        validations = {json.dumps({k: v for k, v in row['preflight'].items() if k != 'raw_sha256'}, sort_keys=True)
                       for row in data['isolated']}
        if len(validations) != 1:
            raise RuntimeError('Redirect responses differ; inspect before interpreting results')
    if args.profiles or args.profiles_only:
        for pool in args.pools:
            for label, binary in targets.items():
                print(f'profiling pool={pool}: {label}', flush=True)
                data['profiles'].append(profile(label, binary, args.client, pool, args, env))
                save()
    save()
    print(f'Results: {destination}', flush=True)


if __name__ == '__main__':
    main()
