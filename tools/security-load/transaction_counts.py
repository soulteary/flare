#!/usr/bin/env python3
"""Whole HTTP request Store I/O invocation counts using diagnostic-only overlay."""
import argparse
import hashlib
import http.client
import importlib.util
import json
import os
import pathlib
import subprocess
import time

_spec=importlib.util.spec_from_file_location('acceptance',pathlib.Path(__file__).with_name('acceptance.py'))
acceptance=importlib.util.module_from_spec(_spec);_spec.loader.exec_module(acceptance)


def counters(port):
    c=http.client.HTTPConnection('127.0.0.1',port,timeout=15)
    try:
        c.request('GET','/counters');r=c.getresponse();raw=r.read();assert r.status==200
        return json.loads(raw)
    finally:c.close()


def delta(before,after):
    if before.keys()!=after.keys():raise ValueError('counter schema changed')
    result={k:after[k]-before[k] for k in before}
    if min(result.values())<0:raise ValueError('counter reset during request')
    return result


def run(args):
    root=pathlib.Path(args.directory).resolve();root.mkdir(parents=True,exist_ok=True)
    fixture=acceptance.fixture(root)
    env=dict(os.environ,FLARE_PORT=str(args.port),FLARE_USER='scale-admin',FLARE_PASS='scale-test-password-2026',FLARE_DISABLE_LOGIN='false',FLARE_VISIBILITY='DEFAULT',FLARE_OFFLINE='true',FLARE_EDITOR='true',FLARE_PROFILE_TEST_ENABLE='1',FLARE_PROFILE_TEST_PORT=str(args.control_port))
    result={'started_utc':acceptance.utc_now(),'binary_sha256':hashlib.sha256(pathlib.Path(args.binary).read_bytes()).hexdigest(),'fixture':fixture,'scope':'Go commit/atomicWrite/os.File.Sync invocation counts in security Store only; not kernel syscalls; timing excluded','requests':[]}
    log=open(root/'server.log','wb');p=subprocess.Popen([args.binary],cwd=root,env=env,stdout=log,stderr=subprocess.STDOUT)
    try:
        for _ in range(300):
            try:
                if acceptance.Client(args.port).request()[0]==200:counters(args.control_port);break
            except OSError:pass
            if p.poll() is not None:raise RuntimeError('diagnostic server exited')
            time.sleep(.1)
        else:raise RuntimeError('readiness timeout')
        admin=acceptance.Client(args.port);before=counters(args.control_port);status,_,_=admin.login();assert status==303
        result['login_sequence_counts']=delta(before,counters(args.control_port))
        paths=['/','/?search=PRIVATE%20Entry%2019999','/applications','/bookmarks','/bookmarks?category=c099','/bookmarks?category=nonexistent']
        for authenticated in [True,False]:
            client=admin if authenticated else acceptance.Client(args.port)
            for path in paths:
                for method in ['GET','HEAD']:
                    before=counters(args.control_port)
                    c=http.client.HTTPConnection('127.0.0.1',args.port,timeout=15)
                    try:
                        c.request(method,path,headers={'Cookie':'; '.join(f'{k}={v}' for k,v in client.cookies.items())});r=c.getresponse();body=r.read();status=r.status
                    finally:c.close()
                    counts=delta(before,counters(args.control_port))
                    expected=404 if 'nonexistent' in path else 200
                    assert status==expected,(path,method,status)
                    assert method!='HEAD' or body==b''
                    private=acceptance.contains_private_card(body)
                    if method=='GET' and path in ['/','/?search=PRIVATE%20Entry%2019999','/bookmarks','/bookmarks?category=c099']:
                        assert private==authenticated,(path,authenticated,private)
                    result['requests'].append({'authenticated':authenticated,'method':method,'path':path,'status':status,'body_bytes':len(body),'body_sha256':hashlib.sha256(body).hexdigest(),'private_card_present':private,'counts':counts})
        # Same explicit current-session revocation and stale-cookie replay as load probes.
        code,raw,_=admin.request('/editor/csrf');assert code==200
        csrf=json.loads(raw)['csrfToken'];replay=acceptance.Client(args.port);replay.cookies=admin.cookies.copy()
        before=counters(args.control_port)
        status,_,_=admin.request('/logout',acceptance.urllib.parse.urlencode({'csrf-token':csrf}),{'Content-Type':'application/x-www-form-urlencoded','Origin':f'http://127.0.0.1:{args.port}'})
        result['logout_counts']=delta(before,counters(args.control_port));assert status==303
        before=counters(args.control_port);status,body,_=replay.request('/?search=PRIVATE%20Entry%2019999');assert status==200 and not acceptance.contains_private_card(body)
        result['revoked_replay_counts']=delta(before,counters(args.control_port));result['all_assertions_pass']=True
    finally:
        p.terminate()
        try:p.wait(timeout=15)
        except subprocess.TimeoutExpired:p.kill();p.wait()
        log.close();result['completed_utc']=acceptance.utc_now();pathlib.Path(args.output).write_text(json.dumps(result,indent=2)+'\n')

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--binary',required=True);parser.add_argument('--directory',required=True);parser.add_argument('--output',required=True)
    parser.add_argument('--port',type=int,default=15600);parser.add_argument('--control-port',type=int,default=15601)
    run(parser.parse_args())
