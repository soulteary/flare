#!/usr/bin/env python3
"""Disposable Docker + HTTPS proxy acceptance. Never operates on pre-existing resources.
Usage: python3 acceptance.py --binary /absolute/path/to/linux/flare [--image alpine:3.24]
Requires Docker-compatible daemon, Python 3, OpenSSL; uses only named resources it creates.
"""
import argparse, hashlib, http.client, http.server, json, os, pathlib, re, ssl, subprocess, tempfile, threading, time, urllib.parse, uuid

parser = argparse.ArgumentParser()
parser.add_argument('--binary', required=True)
parser.add_argument('--image', default='alpine:3.24')
parser.add_argument('--use-image-binary', action='store_true', help='Run /bin/flare already in candidate image instead of copying --binary')
parser.add_argument('--old-image', default='ghcr.io/soulteary/flare:0.6.7')
args = parser.parse_args()
prefix = 'flare-accept-' + uuid.uuid4().hex[:10]
containers, volumes, results = [], [], []
password = 'acceptance-only-password'

def docker(*argv, check=True):
    result = subprocess.run(['docker', *argv], text=True, capture_output=True)
    if check and result.returncode:
        raise RuntimeError('docker ' + argv[0] + ': ' + result.stderr.strip())
    return (result.stdout + (result.stderr if argv[0] == 'logs' else '')).strip()

def record(name):
    results.append(name)
    print('PASS ' + name, flush=True)

def volume(suffix):
    name = prefix + '-' + suffix
    docker('volume', 'create', name)
    volumes.append(name)
    return name

def container(suffix, mounts, env=(), image=None, binary=True, extra=()):
    name = prefix + '-' + suffix
    cmd = ['create', '--name', name, '-p', '127.0.0.1::5005', '-w', '/app', *extra]
    for mount in mounts:
        cmd += ['-v', mount]
    for value in env:
        cmd += ['-e', value]
    cmd += [image or args.image, '/bin/flare']
    docker(*cmd)
    containers.append(name)
    if binary and not args.use_image_binary:
        docker('cp', str(pathlib.Path(args.binary).resolve()), name + ':/bin/flare')
    return name

def start(name, ready=True):
    docker('start', name)
    if not ready:
        time.sleep(1)
        return
    port = json.loads(docker('inspect', name))[0]['NetworkSettings']['Ports']['5005/tcp'][0]['HostPort']
    for _ in range(100):
        try:
            response = request(int(port), '/', headers={})
            if response[0] < 500:
                return int(port)
        except OSError:
            pass
        time.sleep(.1)
    raise RuntimeError(name + ' did not become ready; ' + docker('logs', name)[-1000:])

def request(port, path, method='GET', fields=None, headers=None, tls=False):
    connection = (http.client.HTTPSConnection('127.0.0.1', port, context=ssl._create_unverified_context()) if tls else http.client.HTTPConnection('127.0.0.1', port))
    header = dict(headers or {})
    data = urllib.parse.urlencode(fields) if fields is not None else None
    if data is not None:
        header['Content-Type'] = 'application/x-www-form-urlencoded'
    connection.request(method, path, data, header)
    response = connection.getresponse()
    result = response.status, response.getheaders(), response.read().decode('utf-8', 'replace')
    connection.close()
    return result

class Browser:
    def __init__(self, port, tls=False, host=None):
        self.port, self.tls, self.host, self.cookies = port, tls, host, {}
    def request(self, path, method='GET', fields=None, headers=None):
        header = dict(headers or {})
        if self.host:
            header['Host'] = self.host
        header['Cookie'] = '; '.join(k+'='+v for k,v in self.cookies.items())
        status, response, body = request(self.port, path, method, fields, header, self.tls)
        for key,value in response:
            if key.lower() == 'set-cookie':
                name, value = value.split(';', 1)[0].split('=', 1)
                self.cookies[name] = value
        return status, response, body
    def token(self):
        status, _, body = self.request('/settings/application')
        assert status == 200, status
        match = re.search(r'name="csrf-token" value="([^"]+)"', body)
        assert match, 'missing CSRF token'
        return match[1]
    def login(self):
        status, headers, _ = self.request('/login', 'POST', {'username':'admin', 'password':password, 'csrf-token':self.token()})
        assert status == 303, status
        return headers

def seed(name, folder):
    docker('cp', str(folder) + '/.', name + ':/app')

try:
    docker('image', 'inspect', args.image)
    with tempfile.TemporaryDirectory(prefix=prefix) as directory:
        work = pathlib.Path(directory)
        fixture = work / 'fixture'
        fixture.mkdir()
        (fixture/'apps.yml').write_text('links:\n- name: PUBLIC-ACCEPTANCE\n  link: https://public.example/\n- name: PRIVATE-ACCEPTANCE\n  link: https://private.example/\n  private: true\n')
        (fixture/'bookmarks.yml').write_text('links: []\n')
        (fixture/'config.yml').write_text('Title: Acceptance\nShowApps: true\nShowBookmarks: true\n')
        expected = {p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in fixture.iterdir()}
        data = volume('data')
        env = ['FLARE_DISABLE_LOGIN=false','FLARE_USER=admin','FLARE_PASS='+password,'FLARE_OFFLINE=true','FLARE_GUIDE=false','FLARE_VISIBILITY=DEFAULT']
        # The old release creates its original cookie key and uses the same content volume.
        old = container('old', [data+':/app'], env, args.old_image, False)
        seed(old, fixture)
        old_port = start(old)
        assert request(old_port, '/')[0] in (200,302)
        docker('stop', old)
        record('0.6.7 runs before upgrade on unchanged YAML')
        first = container('directory', [data+':/app'], env)
        port = start(first)
        assert 'PUBLIC-ACCEPTANCE' in request(port, '/')[2]
        assert 'PRIVATE-ACCEPTANCE' not in request(port, '/')[2]
        browser = Browser(port)
        browser.login()
        assert 'PRIVATE-ACCEPTANCE' in browser.request('/')[2]
        cookie = dict(browser.cookies)
        docker('stop', first)
        second = container('recreated', [data+':/app'], env)
        port = start(second)
        browser.port = port
        assert 'PRIVATE-ACCEPTANCE' in browser.request('/')[2]
        record('automatic old-login migration, anonymous filtering and container-recreate session persistence')
        # Copy data out without relying on daemon host sharing.
        out = work/'copied'
        docker('cp', second+':/app', str(out))
        for name,digest in expected.items():
            assert hashlib.sha256((out/name).read_bytes()).hexdigest() == digest
        record('upgrade preserves all three YAML files byte-for-byte')
        docker('stop', second)
        # Bind individual read-only files from this test's named volume, plus a separate security volume.
        mountpoint = json.loads(docker('volume', 'inspect', data))[0]['Mountpoint']
        security = volume('security')
        individual = [mountpoint+'/'+name+':/app/'+name+':ro' for name in expected]
        individual += [security+':/app/.flare-security']
        files = container('individual', individual, env)
        port = start(files)
        Browser(port).login()
        assert 'PRIVATE-ACCEPTANCE' not in request(port, '/')[2]
        docker('stop', files)
        again = container('individual-again', individual, env)
        port = start(again)
        ro_browser = Browser(port)
        ro_browser.login()
        _, _, page = ro_browser.request('/settings/theme')
        token = re.search(r'name="?csrf-token"? value="?([A-Za-z0-9_.-]+)',page)[1]
        status, _, body = ro_browser.request('/settings/theme','POST',{'csrf-token':token,'theme':'blackboard'})
        assert status >= 400, 'read-only settings write unexpectedly accepted: '+str(status)
        record('individual read-only YAML binds plus separate persistent security volume recreate')
        docker('stop', again)
        # Secret file path is readable in container, password env absent.
        (fixture/'pass').write_text(password+'\n')
        secret_data = volume('secret-data')
        secret = container('secret', [secret_data+':/app'], [e for e in env if not e.startswith('FLARE_PASS=')]+['FLARE_PASS_FILE=/app/pass'])
        seed(secret, fixture)
        secret_mountpoint = json.loads(docker('volume','inspect',secret_data))[0]['Mountpoint']
        secret_reader = container('secret-reader',[secret_data+':/app',secret_mountpoint+'/pass:/run/flare-pass:ro'],[e for e in env if not e.startswith('FLARE_PASS=')]+['FLARE_PASS_FILE=/run/flare-pass'])
        port = start(secret_reader)
        Browser(port).login()
        record('read-only deployment Secret file input')
        docker('stop', secret_reader)
        conflict = container('conflict', [volume('conflict-data')+':/app'], ['FLARE_DISABLE_LOGIN=true','FLARE_OFFLINE=true'])
        seed(conflict, fixture)
        start(conflict, False)
        # A paused instance may expose health but never content.
        state = json.loads(docker('inspect', conflict))[0]['State']
        if state['Running']:
            port = int(json.loads(docker('inspect', conflict))[0]['NetworkSettings']['Ports']['5005/tcp'][0]['HostPort'])
            status,_,body = request(port, '/')
            assert status >= 400 and 'PRIVATE-ACCEPTANCE' not in body and 'PUBLIC-ACCEPTANCE' not in body
        else:
            assert state['ExitCode'] != 0
        record('trusted-mode/private conflict fails closed')
        # Real filesystem faults in the Linux kernel, not injected Go errors.
        readonly = container('readonly-state',[data+':/app',security+':/app/.flare-security:ro'],env)
        start(readonly,False)
        readonly_state = json.loads(docker('inspect',readonly))[0]['State']
        if readonly_state['Running']:
            fault_port = int(json.loads(docker('inspect',readonly))[0]['NetworkSettings']['Ports']['5005/tcp'][0]['HostPort'])
            assert request(fault_port,'/')[0] == 503
        else:
            assert readonly_state['ExitCode'] != 0
        assert any(m['Destination']=='/app/.flare-security' and not m['RW'] for m in json.loads(docker('inspect',readonly))[0]['Mounts'])
        record('read-only persisted security directory refuses startup')
        diskfull = container('diskfull',[volume('diskfull-data')+':/app'],env,extra=['--tmpfs','/app/.flare-security:rw,size=1024'])
        seed(diskfull,fixture)
        start(diskfull,False)
        disk_state = json.loads(docker('inspect',diskfull))[0]['State']
        if disk_state['Running']:
            fault_port = int(json.loads(docker('inspect',diskfull))[0]['NetworkSettings']['Ports']['5005/tcp'][0]['HostPort'])
            assert request(fault_port,'/')[0] == 503
        else:
            assert disk_state['ExitCode'] != 0
        docker('exec','-d',diskfull,'sh','-c','dd if=/dev/zero of=/app/.flare-security/space-probe bs=4096 count=2 2>/app/enospc.txt')
        time.sleep(.3)
        docker('cp',diskfull+':/app/enospc.txt',str(work/'enospc.txt'))
        assert 'no space left' in (work/'enospc.txt').read_text().lower()
        record('real Linux tmpfs ENOSPC refuses migration without exposing content')
        # Real HTTPS termination with Host preserved, canonical and alias authorities.
        cert, key = work/'cert.pem', work/'key.pem'
        subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-keyout',str(key),'-out',str(cert),'-days','1','-subj','/CN=flare.acceptance.test'],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
        backend = {'port':None}
        class Proxy(http.server.BaseHTTPRequestHandler):
            def do_GET(self): self.forward()
            def do_POST(self): self.forward()
            def log_message(self,*args): pass
            def forward(self):
                headers = {k:v for k,v in self.headers.items() if k.lower() not in ('connection','x-forwarded-host','x-forwarded-proto','x-forwarded-for','forwarded')}
                # Sanitize client forwarding inputs; application is configured with fixed HTTPS origin.
                conn = http.client.HTTPConnection('127.0.0.1',backend['port'])
                data = self.rfile.read(int(self.headers.get('Content-Length',0)))
                conn.request(self.command,self.path,data,headers)
                response = conn.getresponse()
                self.send_response(response.status)
                for k,v in response.getheaders():
                    if k.lower() not in ('connection','transfer-encoding'):
                        self.send_header(k,v)
                self.end_headers()
                self.wfile.write(response.read())
                conn.close()
        proxy = http.server.ThreadingHTTPServer(('127.0.0.1',0),Proxy)
        context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        context.load_cert_chain(cert,key)
        proxy.socket = context.wrap_socket(proxy.socket,server_side=True)
        public_port = proxy.server_port
        canonical = 'flare.acceptance.test:'+str(public_port)
        alias = 'alias.acceptance.test:'+str(public_port)
        proxy_env = env+['FLARE_EXTERNAL_URL=https://'+canonical,'FLARE_ALLOWED_HOSTS='+alias]
        target = container('proxy-backend',[volume('proxy-data')+':/app'],proxy_env)
        seed(target, fixture)
        backend['port'] = start(target,False)
        backend['port'] = int(json.loads(docker('inspect',target))[0]['NetworkSettings']['Ports']['5005/tcp'][0]['HostPort'])
        threading.Thread(target=proxy.serve_forever,daemon=True).start()
        for authority in (canonical,alias):
            b = Browser(public_port,True,authority)
            headers = b.login()
            assert any(k.lower()=='set-cookie' and 'secure' in v.lower() and 'httponly' in v.lower() for k,v in headers)
            assert 'PRIVATE-ACCEPTANCE' in b.request('/')[2]
            token = b.token()
            assert b.request('/logout','POST',{'csrf-token':token},{'Origin':'https://evil.example'})[0] == 403
            assert b.request('/logout','POST',{})[0] == 403
            assert b.request('/logout','POST',{'csrf-token':token},{'Origin':'https://'+authority})[0] == 303
            assert 'PRIVATE-ACCEPTANCE' not in b.request('/')[2]
        record('HTTPS external port, canonical/alias Host, Secure HttpOnly cookie, CSRF and logout')
        assert request(backend['port'],'/',headers={'Host':'evil.example','X-Forwarded-Host':canonical,'X-Forwarded-Proto':'https'})[0] == 400
        assert request(backend['port'],'/',headers={'Host':canonical,'X-Forwarded-Host':'evil.example','X-Forwarded-Proto':'http'})[0] == 200
        record('untrusted direct forwarding headers cannot override canonical authority or downgrade HTTPS')
        # Isolated trusted-peer path: only this throwaway backend trusts all peers.
        trusted = container('trusted-proxy',[volume('trusted-proxy-data')+':/app'],proxy_env+['FLARE_TRUSTED_PROXIES=0.0.0.0/0,::/0'])
        seed(trusted,fixture)
        start(trusted,False)
        trust_port = int(json.loads(docker('inspect',trusted))[0]['NetworkSettings']['Ports']['5005/tcp'][0]['HostPort'])
        forwarded = {'Host':'internal-backend:5005','X-Forwarded-Host':canonical,'X-Forwarded-Proto':'https','X-Forwarded-For':'203.0.113.17'}
        assert request(trust_port,'/',headers=forwarded)[0] == 200
        assert request(trust_port,'/',headers={**forwarded,'X-Forwarded-Proto':'http'})[0] == 400
        assert request(trust_port,'/',headers={**forwarded,'X-Forwarded-Host':'evil.example'})[0] == 400
        assert request(trust_port,'/',headers={**forwarded,'X-Forwarded-For':'not-an-ip'})[0] == 400
        assert request(trust_port,'/',headers={**forwarded,'X-Forwarded-Proto':'https,http'})[0] == 400
        record('trusted-peer normalized forwarding, rewritten internal Host, malformed chain and HTTPS downgrade rejection')
        proxy.shutdown()
        # Rollback is a full directory restore, never only changing the binary.
        rollback = container('rollback',[volume('rollback-data')+':/app'],env+['FLARE_VISIBILITY=PRIVATE'],args.old_image,False)
        seed(rollback,fixture)
        rollback_port=start(rollback)
        assert 'PRIVATE-ACCEPTANCE' not in request(rollback_port,'/')[2]
        assert 'PUBLIC-ACCEPTANCE' not in request(rollback_port,'/')[2]
        record('isolated 0.6.7 rollback from original YAML with explicit whole-site PRIVATE protection')
    print(json.dumps({'passed':results,'daemon':json.loads(docker('version','--format','{{json .Server}}'))['Version'],'candidate_sha256':hashlib.sha256(pathlib.Path(args.binary).read_bytes()).hexdigest(),'candidate_image':args.image,'candidate_image_id':json.loads(docker('image','inspect',args.image))[0]['Id'],'use_image_binary':args.use_image_binary},indent=2))
finally:
    for name in reversed(containers):
        docker('rm','-f',name,check=False)
    for name in reversed(volumes):
        docker('volume','rm',name,check=False)
