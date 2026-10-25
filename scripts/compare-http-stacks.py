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


def rss(pid):
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
    lines = ['# Flare Echo / Fiber performance comparison', '',
             f"- Generated: {meta['generated_at']} (Asia/Shanghai)",
             f"- Source: `{meta['revision']}` plus the experimental changes in this workspace",
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


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, default=ROOT / '.benchmark/http-comparison')
    for name, default in [('requests',2000), ('warmup',100), ('rounds',5), ('concurrency',8),
                          ('resource-requests',20000), ('resource-warmup',1000), ('resource-rounds',3), ('gomaxprocs',4)]:
        parser.add_argument('--'+name, type=int, default=default)
    parser.add_argument('--skip-build', action='store_true')
    parser.add_argument('--compare-read-buffers', action='store_true', help='also build same-source 8 KiB and 1 MiB Fiber references with Go overlays')
    parser.add_argument('--keepalive-connections', type=int, default=0, help='0 preserves the original default transport; set to concurrency for a stable idle pool')
    args = parser.parse_args()
    if args.keepalive_connections < 0:
        parser.error('--keepalive-connections must be non-negative')
    if min(args.requests, args.warmup, args.rounds, args.concurrency, args.resource_requests,
           args.resource_warmup, args.resource_rounds, args.gomaxprocs) < 1:
        parser.error('all workload parameters must be positive')
    output = args.output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    env = dict(os.environ, GOMAXPROCS=str(args.gomaxprocs))
    binaries = {stack: output / (stack+'-baseline') for stack in ('echo','fiber')}
    if not args.skip_build:
        for stack in binaries:
            tags = ['-tags','fiber'] if stack == 'fiber' else []
            subprocess.run(['go','build',*tags,'-o',str(binaries[stack]),'./tools/baseline'], cwd=ROOT, env=env, check=True)
            subprocess.run(['go','build',*tags,'-o',str(output/(stack+'-app')),'.'], cwd=ROOT, env=env, check=True)
    reference_sources = {}
    expected_buffers = {}
    if args.compare_read_buffers:
        source = ROOT / 'internal/server/router_fiber.go'
        original_source = source.read_text()
        field = re.compile(r'(?m)^(\s*ReadBufferSize:\s*)[^,\n]+,')
        if len(field.findall(original_source)) > 1:
            raise RuntimeError('Expected at most one ReadBufferSize override')
        for stack, size in [('fiber_8kib', 8192), ('fiber_1mib', 1 << 20)]:
            if field.search(original_source):
                replacement_source = field.sub(lambda match: match.group(1)+str(size)+',', original_source, count=1)
            else:
                replacement_source = original_source.replace('fiber.New(fiber.Config{', f'fiber.New(fiber.Config{{\n ReadBufferSize: {size},', 1)
            if replacement_source == original_source:
                raise RuntimeError('Reference did not alter ReadBufferSize; choose a different candidate')
            reference_sources[stack] = replacement_source
            expected_buffers[stack] = size
            replacement = output / (stack + '_router.go')
            replacement.write_text(replacement_source)
            overlay = output / (stack + '_overlay.json')
            overlay.write_text(json.dumps({'Replace':{str(source):str(replacement)}})+'\n')
            binaries[stack] = output / (stack+'-baseline')
            if not args.skip_build:
                subprocess.run(['go','build','-overlay',str(overlay),'-tags','fiber','-o',str(binaries[stack]),'./tools/baseline'],cwd=ROOT,env=env,check=True)
    cpu = run(['sysctl','-n','machdep.cpu.brand_string'], env).strip() if platform.system() == 'Darwin' else platform.processor()
    metadata = {k:v for k,v in vars(args).items() if k not in ('output','skip_build')}
    metadata.update({'generated_at':datetime.now(ZoneInfo('Asia/Shanghai')).isoformat(timespec='seconds'),
                     'host':platform.platform(), 'cpu':cpu, 'logical_cpus':os.cpu_count(),
                     'go_version':run(['go','version'],env).strip(),
                     'revision':run(['git','rev-parse','HEAD'],env).strip(),
                     'runtime_env':{k:env.get(k,'(unset)') for k in ('GOGC','GOMEMLIMIT','GODEBUG','TZ')},
                     'baseline_sha256':{s:hashlib.sha256(binaries[s].read_bytes()).hexdigest() for s in binaries},
                     'application_sha256':{s:hashlib.sha256((output/(s+'-app')).read_bytes()).hexdigest() for s in ('echo','fiber')},
                     'binary_bytes':{s:(output/(s+'-app')).stat().st_size for s in ('echo','fiber')},
                     'read_buffer_bytes':{},
                     'baseline_orders':[], 'isolated_orders':[],
                     'buffer_reference_source_sha256':{s:hashlib.sha256(src.encode()).hexdigest() for s,src in reference_sources.items()},
                     'buffer_candidate_source_sha256':hashlib.sha256((ROOT/'internal/server/router_fiber.go').read_bytes()).hexdigest()})
    data = {'metadata':metadata,'baseline':[],'isolated':[]}
    destination = output / 'comparison.json'
    def save():
        destination.write_text(json.dumps(data, indent=2)+'\n')
    for round_number in range(args.rounds):
        order = list(binaries)
        order = order[round_number % len(order):] + order[:round_number % len(order)]
        data['metadata']['baseline_orders'].append(order.copy())
        for stack in order:
            print(f'baseline round {round_number+1}/{args.rounds}: {stack}', flush=True)
            rows = metrics(run([binaries[stack],'-requests',args.requests,'-warmup',args.warmup,
                                '-concurrency',args.concurrency,'-keepalive-connections',args.keepalive_connections,'-json'],env))
            for row in rows:
                row.update(stack=stack, round=round_number+1)
                data['baseline'].append(row)
            save()
    for round_number in range(args.resource_rounds):
        order = list(reversed(binaries))
        order = order[round_number % len(order):] + order[:round_number % len(order)]
        data['metadata']['isolated_orders'].append(order.copy())
        for scenario in SCENARIOS:
            for stack in order:
                print(f'isolated round {round_number+1}/{args.resource_rounds}: {scenario} {stack}', flush=True)
                row = isolated(binaries[stack],binaries['echo'],scenario,args,env,
                               output/f'{stack}-{scenario}-{round_number+1}.stderr.log')
                row.update(stack=stack, round=round_number+1)
                data['metadata']['read_buffer_bytes'][stack] = row['read_buffer_bytes']
                if stack in expected_buffers and row['read_buffer_bytes'] != expected_buffers[stack]:
                    raise RuntimeError(f'Reference {stack} reported an unexpected buffer size')
                data['isolated'].append(row)
                save()
    validation = {scenario:{stack:{json.dumps({k:v for k,v in r['preflight'].items() if k != 'raw_sha256'},sort_keys=True) for r in data['isolated']
                                      if r['stack']==stack and r['Scenario']==scenario}
                            for stack in binaries} for scenario in SCENARIOS}
    mismatches = [s for s, results in validation.items() if any(values != results['echo'] for values in results.values())]
    data['metadata']['preflight_mismatches'] = mismatches
    save()
    if mismatches:
        raise RuntimeError('Cross-stack response validation differs; inspect before interpreting: '+', '.join(mismatches))
    report(data,output/'comparison.md')
    print(f'Results: {output / "comparison.md"}',flush=True)


if __name__ == '__main__':
    main()
