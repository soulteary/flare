#!/usr/bin/env python3
"""Disposable Nginx TLS + real no-JavaScript Chromium form acceptance.
Requires existing nginx:stable-alpine and candidate Linux binary. Never pulls or pushes.
"""
import argparse,json,os,pathlib,ssl,subprocess,tempfile,time,uuid,http.client,hashlib
p=argparse.ArgumentParser();p.add_argument('--binary',required=True);p.add_argument('--image',default='alpine:3.24');p.add_argument('--nginx-image',default='nginx:stable-alpine');p.add_argument('--use-image-binary',action='store_true');a=p.parse_args()
prefix='flare-nginx-accept-'+uuid.uuid4().hex[:8];names=[];network=prefix+'-net'
def d(*args):
 r=subprocess.run(['docker',*args],capture_output=True,text=True)
 if r.returncode:raise RuntimeError('docker '+args[0]+': '+r.stderr)
 return r.stdout.strip()
try:
 d('image','inspect',a.image);d('image','inspect',a.nginx_image);d('network','create',network)
 with tempfile.TemporaryDirectory(prefix=prefix) as tmp:
  work=pathlib.Path(tmp);data=work/'data';data.mkdir()
  (data/'apps.yml').write_text('links:\n- name: PRIVATE-NGINX-ACCEPTANCE\n  link: https://private.example/\n  private: true\n')
  (data/'bookmarks.yml').write_text('links: []\n')
  (data/'config.yml').write_text('Title: Nginx acceptance\nShowApps: true\nShowBookmarks: true\nLocale: en\nTheme: blackboard\n')
  proxy=prefix+'-proxy';names.append(proxy)
  d('create','--name',proxy,'--network',network,'-p','127.0.0.1::8443',a.nginx_image)
  proxy_info=json.loads(d('inspect',proxy))[0]
  # Docker assigns ephemeral port upon start; temporary config returns 503 until backend exists.
  initial='events {}\nhttp { server {listen 8443; return 503;} }\n'
  (work/'nginx.conf').write_text(initial);d('cp',str(work/'nginx.conf'),proxy+':/etc/nginx/nginx.conf');d('start',proxy)
  port=json.loads(d('inspect',proxy))[0]['NetworkSettings']['Ports']['8443/tcp'][0]['HostPort']
  canonical='https://localhost:'+port;alias='https://127.0.0.1:'+port
  backend=prefix+'-backend';names.append(backend)
  env=['FLARE_USER=admin','FLARE_PASS=acceptance-only-password','FLARE_DISABLE_LOGIN=false','FLARE_VISIBILITY=PRIVATE','FLARE_GUIDE=false','FLARE_OFFLINE=true','FLARE_EXTERNAL_URL='+canonical,'FLARE_ALLOWED_HOSTS=127.0.0.1:'+port]
  cmd=['create','--name',backend,'--network',network,'--network-alias','flare','-w','/app']
  for item in env:cmd+=['-e',item]
  d(*cmd,a.image,'/bin/flare')
  if not a.use_image_binary:d('cp',str(pathlib.Path(a.binary).resolve()),backend+':/bin/flare')
  d('cp',str(data)+'/.',backend+':/app');d('start',backend)
  subprocess.run(['openssl','req','-x509','-newkey','rsa:2048','-nodes','-keyout',str(work/'key.pem'),'-out',str(work/'cert.pem'),'-days','1','-subj','/CN=localhost'],check=True,capture_output=True)
  conf='''events {}\nhttp { server {listen 8443 ssl; server_name localhost 127.0.0.1; ssl_certificate /etc/nginx/cert.pem; ssl_certificate_key /etc/nginx/key.pem; location / {proxy_pass http://flare:5005; proxy_set_header Host $http_host; proxy_set_header X-Forwarded-Host ""; proxy_set_header X-Forwarded-Proto ""; proxy_set_header X-Forwarded-For ""; proxy_set_header Forwarded "";}}}\n'''
  (work/'nginx.conf').write_text(conf)
  d('stop',proxy)
  for file,target in [('nginx.conf','/etc/nginx/nginx.conf'),('key.pem','/etc/nginx/key.pem'),('cert.pem','/etc/nginx/cert.pem')]:d('cp',str(work/file),proxy+':'+target)
  d('start',proxy)
  for i in range(100):
   try:
    c=http.client.HTTPSConnection('127.0.0.1',port,context=ssl._create_unverified_context());c.request('GET','/login',headers={'Host':'localhost:'+port});r=c.getresponse();r.read();c.close()
    if r.status==200:break
   except OSError:pass
   time.sleep(.1)
  else:raise RuntimeError('Nginx TLS/backend readiness timeout')
  env=dict(os.environ,FLARE_NGINX_CANONICAL=canonical,FLARE_NGINX_ALIAS=alias)
  subprocess.run(['node',str(pathlib.Path(__file__).with_name('nginx-browser.cjs'))],env=env,check=True)
  image=json.loads(d('image','inspect',a.nginx_image))[0]
  print(json.dumps({'candidate_sha256':hashlib.sha256(pathlib.Path(a.binary).read_bytes()).hexdigest(),'candidate_image':a.image,'candidate_image_id':json.loads(d('image','inspect',a.image))[0]['Id'],'use_image_binary':a.use_image_binary,'nginx_image':image['Id'],'nginx_digests':image.get('RepoDigests',[]),'scope':'actual Nginx TLS termination + container backend + Chromium native forms without JavaScript'},indent=2))
finally:
 for name in reversed(names):subprocess.run(['docker','rm','-f',name],capture_output=True)
 subprocess.run(['docker','network','rm',network],capture_output=True)
