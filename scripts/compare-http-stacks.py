#!/usr/bin/env python3
"""Compare optional native Fiber and default Echo with identical workloads."""
import argparse
import hashlib
from datetime import datetime
from zoneinfo import ZoneInfo
import json
import os
import platform
import re
import selectors
import statistics
import subprocess
import shutil
import sys
import tarfile
import tempfile
import time
import urllib.error
import urllib.request
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
# System proxy settings must never route loopback measurements off-host.
class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None

OPENER = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
SCENARIOS = ('home', 'home-search', 'bookmarks', 'applications', 'redir-url')
PATHS = {'home': '/', 'home-search': '/', 'bookmarks': '/bookmarks',
         'applications': '/applications', 'redir-url': '/redir/url?go=aHR0cHM6Ly9saW5rLmV4YW1wbGUuY29t'}


def run(args, env, **kwargs):
    return subprocess.check_output([str(x) for x in args], cwd=ROOT, env=env, text=True, **kwargs)


def metrics(output):
    for line in output.splitlines():
        if line.startswith('[{'):
            return json.loads(line)
    raise RuntimeError('No metrics in output: ' + output[-2000:])


def stats(url):
    with OPENER.open(url + '/stats', timeout=10) as response:
        return json.load(response)


_PROC_CHILDREN = {}


def rss(pid):
    # A process namespace can expose host /proc PIDs while Popen returns local
    # PIDs. Verify both ancestry and NSpid; never sample an unrelated /proc/PID.
    if sys.platform.startswith('linux'):
        own = Path('/proc/self/status').read_text()
        parent = int(re.search(r'^Pid:\s*(\d+)', own, re.M).group(1))
        candidates = [_PROC_CHILDREN.get(pid), Path('/proc') / str(pid)]
        candidates += [path for path in Path('/proc').iterdir() if path.name.isdigit()]
        for path in candidates:
            if path is None:
                continue
            try:
                status = (path / 'status').read_text()
            except (FileNotFoundError, ProcessLookupError, PermissionError):
                continue
            ancestor = re.search(r'^PPid:\s*(\d+)', status, re.M)
            namespace = re.search(r'^NSpid:\s*([\d\t ]+)', status, re.M)
            actual = re.search(r'^Pid:\s*(\d+)', status, re.M)
            local = int(namespace.group(1).split()[-1]) if namespace else int(actual.group(1))
            if ancestor and int(ancestor.group(1)) == parent and local == pid:
                value = re.search(r'^VmRSS:\s*(\d+)\s+kB', status, re.M)
                if value:
                    _PROC_CHILDREN[pid] = path
                    return int(value.group(1)) * 1024
        raise RuntimeError(f'Cannot identify live child process {pid} in /proc')
    return int(subprocess.check_output(['ps', '-o', 'rss=', '-p', str(pid)], text=True).strip()) * 1024


def ready(process):
    selector = selectors.DefaultSelector()
    selector.register(process.stdout, selectors.EVENT_READ)
    try:
        until = time.monotonic() + 20
        while time.monotonic() < until:
            if process.poll() is not None:
                raise RuntimeError('Target exited before startup')
            if selector.select(timeout=1):
                line = process.stdout.readline()
                if line.startswith('{'):
                    info = json.loads(line)
                    if 'control' in info:
                        stats(info['control'])
                        return info
        raise TimeoutError('Target did not become ready')
    finally:
        selector.close()


def client_command(binary, info, scenario, requests, concurrency, keepalive_connections):
    return [str(binary), '-url', info['url'], '-scenario', scenario,
            '-requests', str(requests), '-concurrency', str(concurrency), '-warmup', '0',
            '-keepalive-connections',str(keepalive_connections), '-json']


def preflight(info, scenario):
    body = b'search=example' if scenario == 'home-search' else None
    request = urllib.request.Request(info['url'] + PATHS[scenario], data=body)
    if body is not None:
        request.add_header('Content-Type', 'application/x-www-form-urlencoded')
    try:
        response = OPENER.open(request, timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        content = response.read()
        if info.get('_capture_response_path'):
            Path(info['_capture_response_path']).write_bytes(content)
        expected = 302 if scenario == 'redir-url' else 200
        if response.status != expected:
            raise RuntimeError(f'{scenario} returned {response.status}, expected {expected}')
        return {'status': response.status, 'bytes': len(content),
                'raw_sha256': hashlib.sha256(content).hexdigest(),
                'sha256': hashlib.sha256(re.sub(rb'(<span id=(?:"clock-time"|clock-time)>)[^<]*(</span>)', rb'\1CLOCK\2', content)).hexdigest(),
                'content_type': (response.headers.get('Content-Type') or '').lower(),
                'csp': response.headers.get('Content-Security-Policy'),
                'location': response.headers.get('Location')}


def isolated(binary, client, scenario, args, env, logfile):
    with logfile.open('w') as errors:
        process = subprocess.Popen([str(binary), '-serve'], cwd=ROOT, env=env,
                                   stdout=subprocess.PIPE, stderr=errors, text=True)
        try:
            info = ready(process)
            if getattr(args, 'capture_response_path', None):
                info['_capture_response_path'] = str(args.capture_response_path)
            validation = preflight(info, scenario)
            run(client_command(client, info, scenario, args.resource_warmup, args.concurrency, args.keepalive_connections), env)
            idle_rss = rss(process.pid)
            before = stats(info['control'])
            command = client_command(client, info, scenario, args.resource_requests, args.concurrency, args.keepalive_connections)
            load = subprocess.Popen(command, cwd=ROOT, env=env, text=True,
                                    stdout=subprocess.PIPE, stderr=subprocess.PIPE)
            sampled_peak = idle_rss
            try:
                while True:
                    try:
                        stdout, stderr = load.communicate(timeout=0.05)
                        break
                    except subprocess.TimeoutExpired:
                        sampled_peak = max(sampled_peak, rss(process.pid))
            finally:
                if load.poll() is None:
                    load.kill()
                    load.wait()
            if load.returncode:
                raise RuntimeError(stderr)
            after = stats(info['control'])
            sampled_peak = max(sampled_peak, rss(process.pid))
            n = args.resource_requests
            before_mem, after_mem = before['Mem'], after['Mem']
            result = metrics(stdout)[0]
            result.update({
                'server_cpu_ns_per_request': ((after['UserCPUSeconds'] + after['SystemCPUSeconds']) -
                                              (before['UserCPUSeconds'] + before['SystemCPUSeconds'])) * 1e9 / n,
                'server_allocs_per_request': (after_mem['Mallocs'] - before_mem['Mallocs']) / n,
                'server_bytes_per_request': (after_mem['TotalAlloc'] - before_mem['TotalAlloc']) / n,
                'server_gc_cycles': after_mem['NumGC'] - before_mem['NumGC'],
                'server_gc_pause_ns': after_mem['PauseTotalNs'] - before_mem['PauseTotalNs'],
                'server_heap_after_bytes': after_mem['HeapAlloc'],
                'idle_rss_bytes': idle_rss,
                'sampled_peak_rss_bytes': sampled_peak,
                'lifetime_peak_rss_bytes': after['PeakRSSBytes'],
                'preflight': validation,
                'read_buffer_bytes': info.get('read_buffer_bytes'),
            })
            if getattr(args, 'capture_live_heap', False):
                # Run only after both measured resource snapshots: forced GC and
                # profile encoding are diagnostics, never load CPU/allocation cost.
                profile_path = logfile.with_suffix('.heap.pprof')
                with OPENER.open(info['control'] + '/debug/pprof/heap?gc=1', timeout=30) as response:
                    profile_path.write_bytes(response.read())
                raw_profile = run(['go', 'tool', 'pprof', '-raw', binary, profile_path], env)
                expected_samples = 'alloc_objects/count alloc_space/bytes inuse_objects/count inuse_space/bytes'
                if expected_samples not in raw_profile:
                    raise RuntimeError('Unexpected heap profile sample order; refuse to mislabel live heap')
                samples = re.findall(r'^\s*(-?\d+)\s+(-?\d+)\s+(-?\d+)\s+(-?\d+):', raw_profile, re.M)
                if not samples:
                    raise RuntimeError('No allocation samples in forced-GC heap profile')
                period = re.search(r'^Period:\s*(\d+)', raw_profile, re.M)
                result['heap_profile_sampling_period_bytes'] = int(period.group(1)) if period else None
                result['server_live_heap_sampled_bytes'] = sum(int(sample[3]) for sample in samples)
                result['server_live_objects_sampled'] = sum(int(sample[2]) for sample in samples)
                result['heap_profile_sha256'] = hashlib.sha256(profile_path.read_bytes()).hexdigest()
                result['heap_profile_file'] = profile_path.name
            return result
        finally:
            if process.poll() is None:
                process.terminate()
            try:
                process.wait(timeout=8)
            except subprocess.TimeoutExpired:
                process.kill()
                process.wait()
            process.stdout.close()


def median(rows, key):
    return statistics.median(row[key] for row in rows)


def report(data, output):
    meta = data['metadata']
    read_buffer = meta.get('read_buffer_bytes', {}).get('fiber')
    scope = "frozen historical experiment" if meta.get("historical_reference") else "plus the experimental changes in this workspace"
    lines = ['# Flare Echo / Fiber performance comparison', '',
             f"- Generated: {meta['generated_at']} (Asia/Shanghai)",
             f"- Source: `{meta['revision']}` ({scope})",
             f"- Host: {meta['host']} · {meta['go_version']} · GOMAXPROCS={meta['gomaxprocs']}",
             f"- CPU: {meta['cpu']} · logical CPUs: {meta['logical_cpus']}",
             f"- Client idle connections per host: {meta.get('keepalive_connections',0) or 2} ({'explicit pool' if meta.get('keepalive_connections',0) else 'original Go default'})",
             f"- Frameworks: Echo v5.4.1; Fiber v3.6.1 / fasthttp v1.75.0",
             f"- Effective Fiber read buffer: {read_buffer} bytes (reported by the native target)", '',
             '## Method', '',
             'Both builds use identical offline fixtures, disabled login and request logging, the same Go toolchain, concurrency, and client. '
             'The default configuration still enables the guide and editor. Fiber serves requests on its native fasthttp listener; '
             'the net/http compatibility adapter is used only by existing HTTP integration tests. '
             'Fiber enables Immutable and explicitly copies retained bound form strings. Its effective read buffer is recorded above; buffer references are built from the same source. Request lines and headers exceeding that buffer can receive HTTP 431, so this candidate no longer preserves Echo’s larger header capacity. '
             'Static favicon/SVG responses retain a ServeContent adapter and are outside these five scenarios.', '',
             f"Existing baseline: {meta['rounds']} alternating rounds, {meta['requests']} requests and {meta['warmup']} warmup requests per scenario, concurrency {meta['concurrency']}. "
             'Each stack runs in its own process, with its client and server in that same process as in the original harness. '
             'Allocation values in this section include the client. Cells show the median across rounds.', '',
             '| Scenario | Echo QPS | Fiber QPS | QPS change | Echo P95 (ms) | Fiber P95 (ms) |',
             '| --- | ---: | ---: | ---: | ---: | ---: |']
    for scenario in SCENARIOS:
        rows = {stack: [r for r in data['baseline'] if r['stack'] == stack and r['Scenario'] == scenario] for stack in ('echo', 'fiber')}
        eq, fq = (median(rows[s], 'QPS') for s in ('echo', 'fiber'))
        ep, fp = (median(rows[s], 'P95')/1e6 for s in ('echo', 'fiber'))
        lines.append(f'| {scenario} | {eq:.0f} | {fq:.0f} | {(fq/eq-1)*100:+.1f}% | {ep:.3f} | {fp:.3f} |')
    lines += ['', '## Isolated server resources', '',
              f"{meta['resource_rounds']} alternating rounds; fresh target process per scenario; {meta['resource_warmup']} warmup requests followed by {meta['resource_requests']} measured requests, concurrency {meta['concurrency']}. "
              'The load generator is a separate process and always uses the Echo-built client binary for all targets. '
              'CPU is target-process user+system time from getrusage; allocations and GC are target-process runtime counters. '
              'Snapshots are served on a separate loopback control listener, whose small fixed overhead is amortized into the counters. '
              'RSS is the target process sampled with ps every 50 ms; it includes executable pages, runtime and heap. '
              'RSS after warmup and the lifetime high-water mark are retained in the raw data. '
              'Neither RSS nor retained heap is equivalent to bytes allocated per request.', '',
              '| Scenario | Echo QPS | Fiber QPS | QPS change | Echo CPU (µs/req) | Fiber CPU (µs/req) | CPU change | Echo allocs/req | Fiber allocs/req | Echo KiB/req | Fiber KiB/req |',
              '| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |']
    for scenario in SCENARIOS:
        rows = {stack: [r for r in data['isolated'] if r['stack'] == stack and r['Scenario'] == scenario] for stack in ('echo', 'fiber')}
        eq, fq = (median(rows[s], 'QPS') for s in ('echo', 'fiber'))
        ec, fc = (median(rows[s], 'server_cpu_ns_per_request')/1e3 for s in ('echo', 'fiber'))
        ea, fa = (median(rows[s], 'server_allocs_per_request') for s in ('echo', 'fiber'))
        eb, fb = (median(rows[s], 'server_bytes_per_request')/1024 for s in ('echo', 'fiber'))
        lines.append(f'| {scenario} | {eq:.0f} | {fq:.0f} | {(fq/eq-1)*100:+.1f}% | {ec:.1f} | {fc:.1f} | {(fc/ec-1)*100:+.1f}% | {ea:.1f} | {fa:.1f} | {eb:.1f} | {fb:.1f} |')
    lines += ['', '| Scenario | Echo P95 (ms) | Fiber P95 (ms) | Echo idle RSS (MiB) | Fiber idle RSS (MiB) | Echo sampled peak RSS (MiB) | Fiber sampled peak RSS (MiB) | Echo GC cycles | Fiber GC cycles |',
              '| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |']
    for scenario in SCENARIOS:
        rows = {stack: [r for r in data['isolated'] if r['stack'] == stack and r['Scenario'] == scenario] for stack in ('echo', 'fiber')}
        vals = []
        for key, scale in [('P95',1e6), ('idle_rss_bytes',2**20), ('sampled_peak_rss_bytes',2**20), ('server_gc_cycles',1)]:
            vals.extend(median(rows[s], key)/scale for s in ('echo', 'fiber'))
        lines.append('| '+scenario+' | '+' | '.join(f'{v:.2f}' for v in vals)+' |')
    references = [s for s in ('fiber_8kib', 'fiber_1mib') if s in meta['baseline_sha256']]
    if references:
        sizes = meta['read_buffer_bytes']
        variants = sorted(['fiber', *references], key=lambda s: sizes[s])
        labels = {s: (f"{sizes[s] // (1 << 20)} MiB" if sizes[s] >= 1 << 20 else f"{sizes[s] // 1024} KiB") for s in variants}
        lines += ['', '## Same-session read buffer comparison', '',
                  'All Fiber variants use the same current source; Go overlays only replace ReadBufferSize. '
                  'Each native target reports its effective size. All variants and Echo are rotated in the same session. '
                  'Tables show per-stack medians; their ratios are not medians of paired per-round ratios. '
                  'Small gains remain sensitive to host noise.', '',
                  '| Scenario | Echo baseline QPS | ' + ' | '.join(labels[s] + ' baseline QPS' for s in variants) + ' | Candidate vs 8 KiB | Candidate throughput retained vs 1 MiB |',
                  '| --- | ---: | ' + ' | '.join('---:' for s in variants) + ' | ---: | ---: |']
        for scenario in SCENARIOS:
            rows = {stack: [r for r in data['baseline'] if r['stack'] == stack and r['Scenario'] == scenario] for stack in ('echo', *variants)}
            qps = {s:median(rows[s], 'QPS') for s in rows}
            vs8 = f"{(qps['fiber']/qps['fiber_8kib']-1)*100:+.1f}%" if 'fiber_8kib' in qps else 'n/a'
            retained = f"{qps['fiber']/qps['fiber_1mib']*100:.1f}%" if 'fiber_1mib' in qps else 'n/a'
            lines.append('| '+scenario+f" | {qps['echo']:.0f} | "+' | '.join(f'{qps[s]:.0f}' for s in variants)+f' | {vs8} | {retained} |')
        lines += ['', '| Scenario | ' + ' | '.join(labels[s] + ' baseline P95 (ms)' for s in variants) + ' |',
                  '| --- | ' + ' | '.join('---:' for s in variants) + ' |']
        for scenario in SCENARIOS:
            rows = {stack:[r for r in data['baseline'] if r['stack']==stack and r['Scenario']==scenario] for stack in variants}
            lines.append('| '+scenario+' | '+' | '.join(f"{median(rows[s], 'P95')/1e6:.3f}" for s in variants)+' |')
        lines += ['', '| Scenario | ' + ' | '.join(labels[s] + ' peak RSS (MiB)' for s in variants) + ' | Candidate RSS change vs 8 KiB | Candidate RSS reduction vs 1 MiB |',
                  '| --- | ' + ' | '.join('---:' for s in variants) + ' | ---: | ---: |']
        for scenario in SCENARIOS:
            rows = {stack:[r for r in data['isolated'] if r['stack']==stack and r['Scenario']==scenario] for stack in variants}
            rss_values = {s:median(rows[s], 'sampled_peak_rss_bytes')/2**20 for s in rows}
            vs8 = f"{rss_values['fiber']-rss_values['fiber_8kib']:+.2f} MiB" if 'fiber_8kib' in rss_values else 'n/a'
            reduction = f"{(1-rss_values['fiber']/rss_values['fiber_1mib'])*100:.1f}%" if 'fiber_1mib' in rss_values else 'n/a'
            lines.append('| '+scenario+' | '+' | '.join(f'{rss_values[s]:.2f}' for s in variants)+f' | {vs8} | {reduction} |')
        lines += ['', '| Scenario | ' + ' | '.join(labels[s] + ' CPU (µs/req)' for s in variants) + ' | ' + ' | '.join(labels[s] + ' isolated QPS' for s in variants) + ' |',
                  '| --- | ' + ' | '.join('---:' for _ in range(len(variants)*2)) + ' |']
        for scenario in SCENARIOS:
            rows = {stack:[r for r in data['isolated'] if r['stack']==stack and r['Scenario']==scenario] for stack in variants}
            values = [median(rows[s], 'server_cpu_ns_per_request')/1e3 for s in variants]
            values += [median(rows[s], 'QPS') for s in variants]
            lines.append('| '+scenario+' | '+' | '.join(f'{v:.1f}' for v in values)+' |')
    lines += ['', '## Limits and interpretation', '',
              '- These are local macOS/ARM64 loopback tests, not NAS/Linux measurements or production capacity estimates.',
              '- The original client keeps at most 2 idle connections per host, so concurrency 8 can cause connection churn. An explicit --keepalive-connections setting is recorded above; results include the costs of that connection pattern.',
              '- Closed-loop throughput uses a fixed concurrency; the machine also runs the client. Alternating order reduces but does not eliminate host noise and thermal effects.',
              '- The five original baseline scenarios use default fixtures. `redir-url` validates the existing default-fixture redirect to https://link.example.com and expects HTTP 302.',
              '- Offline mode excludes weather network requests. Login, uploads, large collections and static assets are validated separately and are not performance claims from this run.',
              '- Every isolated run validates the expected HTTP status (200 or 302), response body hash/size, content type, CSP and redirect destination. Content type casing and the visible clock time are normalized for cross-stack comparisons; raw preflight results are stored for cross-stack comparison.',
              '- The compatibility-oriented candidate keeps Immutable enabled and standard-library multipart parsing; results do not represent an aggressively tuned Fiber microbenchmark.',
              '- Medians are descriptive. Check raw per-round data before treating small differences as improvements.', '',
              '## Binary size', '',
              f"- Echo application: {meta['binary_bytes']['echo']/2**20:.2f} MiB; Fiber application: {meta['binary_bytes']['fiber']/2**20:.2f} MiB (same default build flags, no stripping).", '',
              '## Reproduce', '', '```sh',
              f"python3 scripts/compare-http-stacks.py --output .benchmark/http-comparison{'-keepalive-8' if meta.get('keepalive_connections') == 8 else ''} --requests {meta['requests']} --warmup {meta['warmup']} --rounds {meta['rounds']} --resource-requests {meta['resource_requests']} --resource-warmup {meta['resource_warmup']} --resource-rounds {meta['resource_rounds']} --concurrency {meta['concurrency']} --gomaxprocs {meta['gomaxprocs']} --keepalive-connections {meta.get('keepalive_connections',0)}{' --compare-read-buffers' if meta.get('compare_read_buffers') else ''}",
              '```', '', 'Full per-round metrics and metadata: `comparison.json` in the output directory.', '']
    output.write_text('\n'.join(lines))


# This frozen experiment contains the two implementations, dependencies and
# original harness. Production no longer keeps an Echo implementation or tag.
REFERENCE_REVISION = '92a9c47771b1083ec5c46e1ea6104ea061c14d42'


def main():
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument('--reference-revision', default=REFERENCE_REVISION)
    parser.add_argument('--skip-build', action='store_true')
    args, forwarded = parser.parse_known_args()
    if args.skip_build:
        raise SystemExit("Historical replay always rebuilds binaries; --skip-build cannot verify their source.")
    revision = subprocess.check_output(
        ['git', 'rev-parse', '--verify', args.reference_revision + '^{commit}'],
        cwd=ROOT, text=True).strip()
    # Resolve outputs in the caller's project, before entering the snapshot.
    if '--output' in forwarded:
        index = forwarded.index('--output') + 1
        if index == len(forwarded):
            raise SystemExit('--output requires a directory')
        forwarded[index] = str(Path(forwarded[index]).resolve())
    elif any(value.startswith('--output=') for value in forwarded):
        forwarded = ['--output=' + str(Path(value.split('=', 1)[1]).resolve())
                     if value.startswith('--output=') else value for value in forwarded]
    else:
        forwarded += ['--output', str(ROOT / '.benchmark/http-comparison-historical')]
    scratch = ROOT / '.benchmark'
    scratch.mkdir(exist_ok=True)
    print(f'Replaying the frozen Echo/Fiber experiment at {revision}. '
          'The current production build uses Fiber only.', flush=True)
    with tempfile.TemporaryDirectory(prefix='http-reference-', dir=scratch) as directory:
        snapshot = Path(directory)
        archive = snapshot / 'source.tar'
        subprocess.run(['git', 'archive', '--format=tar', '--output', str(archive), revision],
                       cwd=ROOT, check=True)
        # Extract ordinary source files only, without trusting archive paths or links.
        with tarfile.open(archive) as source:
            for member in source:
                target = snapshot / member.name
                if not target.resolve().is_relative_to(snapshot.resolve()):
                    raise RuntimeError('Reference archive contains an unsafe path')
                if member.isdir():
                    target.mkdir(parents=True, exist_ok=True)
                elif member.isfile():
                    target.parent.mkdir(parents=True, exist_ok=True)
                    with source.extractfile(member) as stream, target.open('wb') as output:
                        shutil.copyfileobj(stream, output)
                    target.chmod(member.mode & 0o777)
                else:
                    raise RuntimeError('Reference archive contains an unsupported entry')
        archive.unlink()
        router = snapshot / 'internal/server/router.go'
        fiber_router = snapshot / 'internal/server/router_fiber.go'
        runner = snapshot / 'scripts/compare-http-stacks.py'
        marker = "'revision':run(['git','rev-parse','HEAD'],env).strip(),"
        if (not fiber_router.is_file() or not router.is_file() or not runner.is_file()
                or 'github.com/labstack/echo/v5' not in router.read_text()
                or 'github.com/labstack/echo/v5' not in (snapshot / 'go.mod').read_text()
                or 'REFERENCE_REVISION =' in runner.read_text()
                or runner.read_text().count(marker) != 1):
            raise SystemExit('The reference must contain the original dual-stack experiment, not the replay wrapper.')
        # Stamp provenance before the first partial result is saved, including
        # when a later workload fails. Only Python metadata is changed, not Go.
        runner.write_text(runner.read_text().replace(marker,
            f"'revision':{revision!r}, 'historical_reference':True,"))
        subprocess.run([sys.executable, str(runner), *forwarded], cwd=snapshot, check=True)
        output_index = forwarded.index('--output') + 1 if '--output' in forwarded else None
        output_path = Path(forwarded[output_index]) if output_index else Path(next(
            value.split('=', 1)[1] for value in forwarded if value.startswith('--output=')))
        result = output_path / 'comparison.json'
        if result.exists() and not any(value in ('-h', '--help') for value in forwarded):
            data = json.loads(result.read_text())
            data['metadata']['revision'] = revision
            data['metadata']['historical_reference'] = True
            result.write_text(json.dumps(data, indent=2) + '\n')
            report(data, output_path / 'comparison.md')


if __name__ == '__main__':
    main()
