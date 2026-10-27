"""Recalculate paired summaries and verify sanitized benchmark evidence."""
import csv
import gzip
import hashlib
import io
import math
import json
from pathlib import Path
import re
import statistics as stats
import sys
import tarfile

ROOT = Path(__file__).resolve().parent
TOKEN_VALUE = re.compile(r'''(<input[^>]*name=(?:"csrf-token"|'csrf-token'|csrf-token)[^>]*value=")[^"]*(")''')
TOKEN = re.compile(r'''<input[^>]*name=(?:"csrf-token"|'csrf-token'|csrf-token)[^>]*>''')

def metrics(row, phase):
    s = row[phase]
    cpu = s['After']['CPU']['usage_usec'] - s['Before']['CPU']['usage_usec']
    snapshots = [s['Before'], *s['Samples'], s['After']]
    return {
        'qps': s['QPS'], 'p50_ms': s['P50'], 'p95_ms': s['P95'], 'p99_ms': s['P99'],
        'cpu_us_per_request': cpu / s['Requests'], 'cpu_cores': cpu / 1e6 / s['Elapsed'],
        'rss_median_mib': stats.median(x['RSS'] for x in snapshots) / 2**20,
        'rss_peak_mib': max(x['RSS'] for x in snapshots) / 2**20,
        'hwm_mib': s['After']['HWM'] / 2**20,
        'cgroup_memory_after_mib': s['After']['MemoryCurrent'] / 2**20,
        'client_cpu_cores': (s['ClientCPUAfter']['usage_usec'] - s['ClientCPUBefore']['usage_usec']) / 1e6 / s['Elapsed'],
        'schedule_lag_p95_ms': s['ScheduleLagP95'],
    }

def analyze(rows):
    assert len(rows) == 88, len(rows)
    lookup = {(r['cookie_jar'],r['size'],r['scenario'],r['round'],r['version']):r for r in rows}
    assert len(lookup) == len(rows)
    total = 0
    for r in rows:
        assert r['Environment']['separate_cgroups'] == 'true'
        assert r['Environment']['cookie_jar'] == str(r['cookie_jar']).lower()
        assert r['WarmRequests'] == 200 and r['WarmErrors'] == 0
        expected_count = int(r['version'] == 'before')
        for p in ('Pre','Post'):
            x = r[p]
            assert x['Status'] == 200 and x['SetCookieCount'] == expected_count
            assert len(TOKEN.findall(x['Body'])) == expected_count
            assert hashlib.sha256(x['Body'].encode()).hexdigest() == x['Hash']
            assert len(x['Body'].encode()) == x['Bytes']
            normalized = TOKEN_VALUE.sub(r'\g<1>CSRF_TOKEN\g<2>', x['Body'])
            assert hashlib.sha256(normalized.encode()).hexdigest() == x['NormalizedHash']
        assert r['Pre']['NormalizedHash'] == r['Post']['NormalizedHash']
        assert hashlib.sha256(json.dumps(r['navigation_projection'],sort_keys=True).encode()).hexdigest() == r['projection_sha256']
        for phase in ('Load','Fixed'):
            s = r[phase]
            assert s['Errors'] == 0
            assert s['Elapsed'] > 0
            assert math.isclose(s['QPS'], s['Requests']/s['Elapsed'], rel_tol=1e-12)
            assert s['Bytes'] == s['Requests'] * r['Pre']['Bytes']
            assert 0 <= s['P50'] <= s['P95'] <= s['P99']
            assert sum(s['BodyHashes'].values()) == s['Requests']//256
            assert s['Requests'] > 0 and len(s['Samples']) > 0
            for snap in [s['Before'],*s['Samples'],s['After']]:
                assert not snap['Error'] and snap['RSS'] > 0
            assert s['After']['CPU']['usage_usec'] > s['Before']['CPU']['usage_usec']
            assert set(s['BodyHashes']) <= {r['Pre']['NormalizedHash']}
            if s['Requests'] >= 256:
                assert len(s['BodyHashes']) == 1
            if phase == 'Fixed':
                assert s['Requests'] + s['Dropped'] == (1000 if r['size'] == 20 else 200)
            total += s['Requests']
    groups = []
    for jar,size,scene in sorted(set(k[:3] for k in lookup)):
        n = 6 if jar else 4
        canonical = {TOKEN.sub('',lookup[jar,size,scene,i,v]['Pre']['Body']) for i in range(n) for v in ('before','after')}
        assert len(canonical) == 1
        for roundno in range(n):
            before,after = [lookup[jar,size,scene,roundno,v] for v in ('before','after')]
            assert before['projection_sha256'] == after['projection_sha256']
            for p in ('Pre','Post'):
                assert TOKEN.sub('',before[p]['Body']) == TOKEN.sub('',after[p]['Body'])
                assert before[p]['Bytes'] - after[p]['Bytes'] == 87
        for phase in ('Fixed','Load'):
            by_version = {v:[metrics(lookup[jar,size,scene,i,v],phase) for i in range(n)] for v in ('before','after')}
            summary = {}
            for metric in by_version['before'][0]:
                b = [x[metric] for x in by_version['before']]
                a = [x[metric] for x in by_version['after']]
                deltas = [(y/x-1)*100 if x else 0 for x,y in zip(b,a)]
                summary[metric] = {'before_median':stats.median(b),'after_median':stats.median(a),'paired_change_median_pct':stats.median(deltas),'paired_change_min_pct':min(deltas),'paired_change_max_pct':max(deltas),'paired_changes_pct':deltas}
            groups.append({'cookie_jar':jar,'size':size,'scenario':scene,'phase':phase,'pairs':n,'metrics':summary})
    traffic = [{'cookie_jar':jar,
                'planned':sum(r['Fixed']['Requests']+r['Fixed']['Dropped'] for r in rows if r['cookie_jar']==jar),
                'completed':sum(r['Fixed']['Requests'] for r in rows if r['cookie_jar']==jar),
                'dropped':sum(r['Fixed']['Dropped'] for r in rows if r['cookie_jar']==jar)} for jar in (True,False)]
    return {'fixed_traffic':traffic,'windows':len(rows),'measured_requests':total,'fixed_dropped_requests':sum(r['Fixed']['Dropped'] for r in rows),'fixed_windows_with_drops':sum(r['Fixed']['Dropped']>0 for r in rows),'fixed_schedule_lag_p95_max_ms':max(r['Fixed']['ScheduleLagP95'] for r in rows),'groups':groups}

def csv_text(rows):
    output = io.StringIO(newline='')
    writer = csv.writer(output)
    names = list(metrics(rows[0], 'Load'))
    writer.writerow(['cookie_jar','bookmarks','scenario','round','version','phase',*names])
    for r in sorted(rows,key=lambda r:(r['cookie_jar'],r['size'],r['scenario'],r['round'],r['version'])):
        for phase in ('Fixed','Load'):
            m=metrics(r,phase)
            writer.writerow([r['cookie_jar'],r['size'],r['scenario'],r['round'],r['version'],phase,*[m[k] for k in names]])
    return output.getvalue()

def main():
    if len(sys.argv)>1:
        rows = [json.loads(p.read_text()) for p in Path(sys.argv[1]).glob('*.json')]
        with (ROOT/'raw.tar.gz').open('wb') as out:
            with gzip.GzipFile(filename='',mode='wb',fileobj=out,mtime=0) as gz:
                with tarfile.open(fileobj=gz,mode='w',format=tarfile.USTAR_FORMAT) as archive:
                    for row in sorted(rows,key=lambda r:(r['cookie_jar'],r['size'],r['scenario'],r['round'],r['version'])):
                        data = (json.dumps(row,ensure_ascii=False,indent=2)+'\n').encode()
                        name = f"{row['size']}-{row['scenario']}-{row['round']}-{row['version']}-{row['cookie_jar']}.json"
                        info=tarfile.TarInfo(name);info.size=len(data);info.mode=0o644
                        archive.addfile(info,io.BytesIO(data))
    else:
        with tarfile.open(ROOT/'raw.tar.gz') as archive:
            rows=[json.load(archive.extractfile(member)) for member in archive]
    result=analyze(rows)
    encoded=json.dumps(result,ensure_ascii=False,indent=2)+'\n'
    if len(sys.argv)>1:
        (ROOT/'summary.json').write_text(encoded)
        (ROOT/'windows.csv').write_bytes(csv_text(rows).encode())
    else:
        assert json.loads((ROOT/'summary.json').read_text())==result
        assert (ROOT/'windows.csv').read_bytes() == csv_text(rows).encode(), 'windows.csv differs from raw evidence'
    print(f"Verified {len(rows)} windows, {result['measured_requests']} measured requests, paired HTML and summaries.")

if __name__=='__main__':main()
