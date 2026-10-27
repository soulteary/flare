#!/usr/bin/env python3
"""Real 0.6.7 -> candidate -> complete backup rollback HTTP acceptance.
Usage: python3 tests/security-recovery/acceptance.py OLD_BINARY NEW_BINARY [REPORT]
Both binaries are executed in disposable directories; no user state is used.
"""
import hashlib,http.cookiejar,json,os,pathlib,re,shutil,socket,subprocess,sys,tempfile,time,urllib.request,urllib.parse,urllib.error
OLD,NEW=map(os.path.abspath,sys.argv[1:3]); RESULTS=[]
class NoRedirect(urllib.request.HTTPRedirectHandler):
 def redirect_request(self,*a):return None
def browser():return urllib.request.build_opener(urllib.request.ProxyHandler({}),NoRedirect,urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()))
def request(b,base,path='/',data=None):
 req=urllib.request.Request(base+path,data=None if data is None else urllib.parse.urlencode(data).encode(),headers={'Origin':base})
 try:r=b.open(req,timeout=4)
 except urllib.error.HTTPError as e:r=e
 return r.code,r.read().decode(errors='replace')
def login(b,base,password,new):
 data={'username':'flare','password':password}
 if new:
  code,body=request(b,base,'/login');assert code==200,(code,body)
  data['csrf-token']=re.search(r'name="csrf-token" value="([^"]+)"',body)[1]
 code,body=request(b,base,'/login',data);assert code in (302,303),(code,body)
def hashes(d):return {n:hashlib.sha256((d/n).read_bytes()).hexdigest() for n in ('apps.yml','bookmarks.yml','config.yml')}
def fixture(d,private=True):
 (d/'apps.yml').write_text('links: []\n')
 (d/'bookmarks.yml').write_text('categories:\n  - id: test\n    title: Test\nlinks:\n  - name: PUBLIC-SENTINEL\n    link: https://public.example\n    category: test\n'+('  - name: PRIVATE-SENTINEL\n    link: https://secret.example\n    category: test\n    private: true\n' if private else ''))
 (d/'config.yml').write_text('Title: recovery acceptance\nShowApps: true\nShowBookmarks: true\nAppVersion: 0.6.7\n')
class Server:
 def __init__(self,binary,d,env,port):
  self.base=f'http://127.0.0.1:{port}';self.log=open(d/'process.log','ab')
  clean={k:v for k,v in os.environ.items() if not k.startswith('FLARE_')};clean.update(env);clean['FLARE_PORT']=str(port)
  self.p=subprocess.Popen([binary],cwd=d,env=clean,stdout=self.log,stderr=subprocess.STDOUT)
  for _ in range(40):
   try:request(browser(),self.base,'/health');return
   except (OSError,urllib.error.URLError):time.sleep(.05)
   if self.p.poll() is not None:break
  self.stop();raise AssertionError((d/'process.log').read_text())
 def stop(self):
  if self.p.poll() is None:self.p.terminate()
  try:self.p.wait(timeout=5)
  except subprocess.TimeoutExpired:self.p.kill();self.p.wait()
  self.log.close()
def record(name,fn):
 start=time.monotonic();fn();RESULTS.append({'case':name,'status':'pass','seconds':round(time.monotonic()-start,3)});print('PASS',name,flush=True)
with tempfile.TemporaryDirectory(prefix='flare-recovery-') as tmp:
 root=pathlib.Path(tmp)
 with socket.socket() as sk:sk.bind(('127.0.0.1',0));port=sk.getsockname()[1]
 for visibility in ('DEFAULT','PRIVATE'):
  def upgrade(visibility=visibility):
   d=root/visibility;d.mkdir();fixture(d)
   env={'FLARE_DISABLE_LOGIN':'false','FLARE_VISIBILITY':visibility,'FLARE_PASS':'short'}
   s=Server(OLD,d,env,port);oldbrowser=browser()
   try:
    login(oldbrowser,s.base,'short',False)
    assert 'PRIVATE-SENTINEL' in request(oldbrowser,s.base)[1]
    assert ('PRIVATE-SENTINEL' in request(browser(),s.base)[1])==(visibility=='DEFAULT')
   finally:s.stop()
   original=hashes(d);backup=root/(visibility+'-backup');shutil.copytree(d,backup)
   s=Server(NEW,d,env,port)
   try:
    assert 'PRIVATE-SENTINEL' not in request(browser(),s.base)[1]
    assert 'PRIVATE-SENTINEL' not in request(oldbrowser,s.base)[1], 'old cookie accepted by candidate'
    login(browser2:=browser(),s.base,'short',True)
    assert 'PRIVATE-SENTINEL' in request(browser2,s.base)[1]
    assert hashes(d)==original,'migration changed YAML'
   finally:s.stop()
   s=Server(NEW,d,env,port)
   try:assert 'PRIVATE-SENTINEL' in request(browser2,s.base)[1], 'session lost on restart'
   finally:s.stop()
   # Roll back the complete data directory and exact old credential environment.
   shutil.rmtree(d);shutil.copytree(backup,d);assert hashes(d)==original
   s=Server(OLD,d,env,port)
   try:
    assert 'PRIVATE-SENTINEL' in request(oldbrowser,s.base)[1], 'restore should demonstrate old signed-cookie resurrection'
    assert ('PRIVATE-SENTINEL' in request(browser(),s.base)[1])==(visibility=='DEFAULT')
   finally:s.stop()
   if visibility=='DEFAULT':
    env['FLARE_VISIBILITY']='PRIVATE'
    s=Server(OLD,d,env,port)
    try:assert 'PRIVATE-SENTINEL' not in request(browser(),s.base)[1]
    finally:s.stop()
  record('old-to-new-'+visibility.lower()+'-weak-password-full-rollback',upgrade)
 def trusted():
  d=root/'trusted';d.mkdir();fixture(d,False);env={'FLARE_DISABLE_LOGIN':'true'}
  for binary in (OLD,NEW,NEW):
   s=Server(binary,d,env,port)
   try:assert 'PUBLIC-SENTINEL' in request(browser(),s.base)[1];assert request(browser(),s.base,'/editor')[0]==200
   finally:s.stop()
  assert json.loads((d/'.flare-security/installation.json').read_text())['committed']
 record('trusted-old-to-new-restart',trusted)
 for problem in ('missing-password','trusted-private-conflict'):
  def bad(problem=problem):
   d=root/problem;d.mkdir();fixture(d)
   env={'FLARE_DISABLE_LOGIN': 'true' if problem=='trusted-private-conflict' else 'false'}
   before=hashes(d);s=Server(NEW,d,env,port)
   try:
    for path in ('/','/editor','/health'):
     code,body=request(browser(),s.base,path);assert code==503,(path,code);assert 'PRIVATE-SENTINEL' not in body
    assert hashes(d)==before
   finally:s.stop()
  record(problem+'-fails-closed',bad)
 def localreset():
  d=root/'localreset';d.mkdir();fixture(d);env={'FLARE_DISABLE_LOGIN':'false','FLARE_PASS':'short'}
  s=Server(NEW,d,env,port);s.stop()
  secret=d/'reset-secret';secret.write_text('A new unique recovery phrase 42!\n')
  clean={k:v for k,v in os.environ.items() if not k.startswith('FLARE_')};clean.update(env)
  fail=subprocess.run([NEW,'security','reset-password','--password-file',str(secret)],cwd=d,env=clean,capture_output=True);assert fail.returncode==2
  ok=subprocess.run([NEW,'security','reset-password','--take-local-control','--password-file',str(secret)],cwd=d,env=clean,capture_output=True);assert ok.returncode==0,ok.stderr
  s=Server(NEW,d,env,port)
  try:login(browser(),s.base,secret.read_text().strip(),True)
  finally:s.stop()
  for name in ('state.json','complete','installation.json','commit.json'):
   backup=root/('local-state-'+name);shutil.copytree(d,backup)
   (d/'.flare-security'/name).write_text('corrupt')
   s=Server(NEW,d,env,port)
   try:assert request(browser(),s.base)[0]==503
   finally:s.stop()
   shutil.rmtree(d);shutil.copytree(backup,d)
  for names in (('complete',),('installation.json',),('state.json','complete')):
   backup=root/('missing-'+names[0]);shutil.copytree(d,backup)
   for name in names:(d/'.flare-security'/name).unlink()
   s=Server(NEW,d,env,port)
   try:assert request(browser(),s.base)[0]==503
   finally:s.stop()
   shutil.rmtree(d);shutil.copytree(backup,d)
  s=Server(NEW,d,env,port)
  try:
   restored_browser=browser();login(restored_browser,s.base,secret.read_text().strip(),True)
   assert 'PRIVATE-SENTINEL' in request(restored_browser,s.base)[1]
  finally:s.stop()
  # A stopped complete backup includes the account, mode, content and a valid
  # session. Preserve the pre-restore directory, restore, then revoke offline.
  consistent=root/'new-complete-backup';shutil.copytree(d,consistent)
  before_restore=root/'new-before-restore';shutil.move(str(d),before_restore)
  shutil.copytree(consistent,d)
  before_hashes=hashes(d)
  revoked=subprocess.run([NEW,'security','revoke-all'],cwd=d,env=clean,capture_output=True)
  assert revoked.returncode==0,revoked.stderr
  s=Server(NEW,d,env,port)
  try:
   assert 'PRIVATE-SENTINEL' not in request(restored_browser,s.base)[1], 'restored session survived offline revoke-all'
   login(browser(),s.base,secret.read_text().strip(),True)
   assert hashes(d)==before_hashes,'complete recovery/revocation changed YAML'
   # Real IPv6 loopback HTTP uses a bracketed authority and its own CSRF origin.
   ipv6=f'http://[::1]:{port}'
   login(ipv6_browser:=browser(),ipv6,secret.read_text().strip(),True)
   assert 'PRIVATE-SENTINEL' in request(ipv6_browser,ipv6)[1]
  finally:s.stop()
 record('local-reset-corruption-full-restore-offline-revoke-and-ipv6',localreset)
 def secret_failure():
  d=root/'secret-file';d.mkdir();fixture(d)
  secret=d/'deployment-secret';secret.write_text('original short\n')
  env={'FLARE_DISABLE_LOGIN':'false','FLARE_PASS_FILE':str(secret)}
  s=Server(NEW,d,env,port)
  try:
   login(b:=browser(),s.base,'original short',True)
   clean={k:v for k,v in os.environ.items() if not k.startswith('FLARE_')};clean.update(env)
   locked=subprocess.run([NEW,'security','revoke-all'],cwd=d,env=clean,capture_output=True)
   assert locked.returncode==3,locked.stderr
  finally:s.stop()
  for bad in ('empty','missing'):
   if bad=='empty':secret.write_text('')
   else:secret.unlink()
   s=Server(NEW,d,env,port)
   try:assert request(browser(),s.base)[0]==503
   finally:s.stop()
  secret.write_text('original short\n')
  s=Server(NEW,d,env,port)
  try:
   assert 'PRIVATE-SENTINEL' in request(b,s.base)[1], 'secret read failure must preserve last valid state'
   login(browser(),s.base,'original short',True)
  finally:s.stop()
 record('deployment-secret-file-empty-missing-restore-and-live-lock',secret_failure)
report={'baseline':'git tag 0.6.7 (2f1a924f40202210bce0531d43e16e5ac8acd4fb), stamped 0.6.7','candidate':'workspace final candidate, stamped 0.6.8-dev','baseline_sha256':hashlib.sha256(pathlib.Path(OLD).read_bytes()).hexdigest(),'candidate_sha256':hashlib.sha256(pathlib.Path(NEW).read_bytes()).hexdigest(),'cases':RESULTS}
if len(sys.argv)>3:pathlib.Path(sys.argv[3]).write_text(json.dumps(report,indent=2)+'\n')
