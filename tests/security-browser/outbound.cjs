// T11 supplement: intercepted browser outbound requests, no real external fetch.
const playwrightModule=process.env.FLARE_PLAYWRIGHT_MODULE||'../clock/node_modules/playwright';
const {chromium,firefox,webkit}=require(playwrightModule);
const fs=require('node:fs/promises'),path=require('node:path'),os=require('node:os'),net=require('node:net'),assert=require('node:assert/strict');
const {spawn}=require('node:child_process'),{createHash}=require('node:crypto');
let PRIVATE,ICON,NOICON,PUBLIC;
async function launch(binary){
 const sink=require('node:http').createServer((req,res)=>{res.writeHead(200,{'Content-Type':'text/html'});res.end('<html><body>INTERCEPTED-LOOPBACK-SINK</body></html>');});await new Promise(r=>sink.listen(0,'127.0.0.1',r));const sinkPort=sink.address().port;PRIVATE=`http://127.0.0.1:${sinkPort}/private-url-sentinel`;ICON=`http://127.0.0.1:${sinkPort}/private-icon-sentinel.png`;NOICON=`http://127.0.0.2:${sinkPort}/private-noicon-sentinel`;PUBLIC=`http://127.0.0.1:${sinkPort}/public-url`;
 const root=await fs.mkdtemp(path.join(os.tmpdir(),'flare-outbound-'));const server=net.createServer();await new Promise(r=>server.listen(0,'127.0.0.1',r));const port=server.address().port;await new Promise(r=>server.close(r));
 const files={'config.yml':'Title: Outbound acceptance\nLocale: en\nTheme: blackboard\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowWeather: false\nOpenBookmarkNewTab: false\nEnableEncryptedLink: true\nIconMode: FILLING\n','apps.yml':'links: []\n','bookmarks.yml':`links:\n- name: PUBLIC-LINK-SENTINEL\n  link: ${PUBLIC}\n  desc: Public description\n- name: PRIVATE-LINK-SENTINEL\n  desc: PRIVATE-DESCRIPTION-SENTINEL\n  link: ${PRIVATE}\n  icon: ${ICON}\n  private: true\n- name: PRIVATE-NOICON-SENTINEL\n  link: ${NOICON}\n  private: true\n`};
 for(const [name,raw]of Object.entries(files))await fs.writeFile(path.join(root,name),raw);
 const env=Object.fromEntries(Object.entries(process.env).filter(([k])=>!k.startsWith('FLARE_')));Object.assign(env,{FLARE_PORT:String(port),FLARE_USER:'outbound-admin',FLARE_PASS:'outbound-test-password',FLARE_DISABLE_LOGIN:'false',FLARE_VISIBILITY:'DEFAULT',FLARE_OFFLINE:'true',FLARE_GUIDE:'false'});
 const child=spawn(binary,[],{cwd:root,env,stdio:['ignore','pipe','pipe']});let logs='';child.stdout.on('data',b=>logs+=b);child.stderr.on('data',b=>logs+=b);const base=`http://127.0.0.1:${port}`;
 for(let i=0;i<200;i++){if(child.exitCode!==null)throw Error(logs);try{if((await fetch(base+'/login')).status===200)return{root,child,base,sink};}catch{}await new Promise(r=>setTimeout(r,100));}throw Error('readiness timeout');
}
async function stop(child){if(child.exitCode!==null)return;await new Promise(r=>{const timer=setTimeout(()=>child.kill('SIGKILL'),7000);child.once('exit',()=>{clearTimeout(timer);r()});child.kill('SIGTERM')});}
async function run(name,engine,binary){
 const state=await launch(binary),result={engine:name,cases:[],requests:[],scope:'real desktop browser; external requests route-aborted; redirect destinations additionally intercepted by local loopback HTTP sink; no Internet target fetch; synthetic data'};let browser;
 try{
  const env=Object.fromEntries(Object.entries(process.env).filter(([k])=>!['http_proxy','https_proxy','all_proxy','no_proxy'].includes(k.toLowerCase())));
  browser=await engine.launch({headless:true,env,...(name==='firefox'?{firefoxUserPrefs:{'network.proxy.type':0}}:{})});result.version=browser.version();result.driver=require(playwrightModule+'/package.json').version;
  const context=await browser.newContext({javaScriptEnabled:false});context.setDefaultTimeout(15000);let phase='anonymous';state.sink.on('request',req=>result.requests.push({phase,url:'http://'+req.headers.host+req.url,type:'navigation-sink',referer:req.headers.referer||null}));
  await context.route('**/*',async route=>{const request=route.request();if(new URL(request.url()).origin===state.base)return route.continue();result.requests.push({phase,url:request.url(),type:request.resourceType(),referer:(await request.allHeaders()).referer||null});await route.abort('blockedbyclient');});
  const page=await context.newPage();
  for(const location of ['/','/?search=PRIVATE','/settings/application']){
   const response=await page.goto(state.base+location);assert.equal(response.status(),200,'navigation '+location);const html=await page.content();
   for(const sentinel of [PRIVATE,ICON,NOICON,'PRIVATE-LINK-SENTINEL','PRIVATE-DESCRIPTION-SENTINEL','PRIVATE-NOICON-SENTINEL'])assert(!html.includes(sentinel),'anonymous DOM contains '+sentinel);
  }
  assert(!result.requests.some(r=>/private-(url|icon|noicon)-sentinel|PRIVATE-/.test(r.url)));
  result.cases.push('anonymous public home/search/login DOM and intercepted outbound exclude private title/description/URL/icon');
  // A forged legacy encoded redirect must not produce an external navigation.
  const legacy=state.base+'/redir/url?go='+encodeURIComponent(Buffer.from(PRIVATE).toString('base64'));
  const denied=await page.goto(legacy);assert.equal(denied.status(),404);assert(!result.requests.some(r=>r.url===PRIVATE));
  result.cases.push('anonymous legacy private redirect returns404 without outbound destination');
  await page.goto(state.base+'/');
  const publicHref=await page.getByRole('link',{name:'PUBLIC-LINK-SENTINEL',exact:true}).getAttribute('href');assert(publicHref.startsWith('/redir/url?id='));
  const publicExternal=page.waitForRequest(r=>r.url()===PUBLIC);await page.getByRole('link',{name:'PUBLIC-LINK-SENTINEL',exact:true}).click({noWaitAfter:true}).catch(()=>{});await publicExternal;await page.waitForTimeout(100);
  assert.equal(result.requests.find(r=>r.url===PUBLIC).referer,null);
  result.cases.push('public same-origin opaque redirect reaches authorized target with no external Referer');
  phase='administrator';await page.goto(state.base+'/login');await page.locator('input[name=username]').fill('outbound-admin');await page.locator('input[name=password]').fill('outbound-test-password');await Promise.all([page.waitForURL(state.base+'/'),page.getByRole('button',{name:'Login / 登录',exact:true}).click()]);
  await page.getByRole('link',{name:'PRIVATE-LINK-SENTINEL',exact:true}).waitFor();
  await page.waitForTimeout(100);assert(result.requests.some(r=>r.phase==='administrator'&&r.url===ICON));
  assert.equal(result.requests.find(r=>r.url===ICON).referer,null);
  assert(!result.requests.some(r=>r.url!==ICON&&(r.url.includes('127.0.0.2')||r.url.includes('private-noicon-sentinel'))));
  result.cases.push('administrator sees explicit private icon without Referer; private no-icon entry never enters external favicon fallback');
  const href=await page.getByRole('link',{name:'PRIVATE-LINK-SENTINEL',exact:true}).getAttribute('href');assert(href.startsWith('/redir/url?id='));assert(!href.includes(PRIVATE));
  assert(!result.requests.some(r=>r.url===PRIVATE),'private target fetched before explicit click');
  const privateExternal=page.waitForRequest(r=>r.url()===PRIVATE);await page.getByRole('link',{name:'PRIVATE-LINK-SENTINEL',exact:true}).click({noWaitAfter:true}).catch(()=>{});await privateExternal;await page.waitForTimeout(100);
  assert.equal(result.requests.find(r=>r.url===PRIVATE).referer,null);
  result.cases.push('authorized private opaque redirect reaches private target only after explicit click, without external Referer');
  result.status='PASS';await context.close();
 }catch(e){result.status='FAIL';result.error=String(e.stack||e);process.exitCode=1;}
 finally{if(browser)await browser.close();await stop(state.child);await new Promise(r=>state.sink.close(r));await fs.rm(state.root,{recursive:true,force:true});}
 return result;
}
(async()=>{const binary=path.resolve(process.env.FLARE_BINARY||'/tmp/flare-phase1-final-candidate'),results=[];for(const [name,engine]of Object.entries({chromium,firefox,webkit}))if(!process.env.FLARE_BROWSER_ENGINE||process.env.FLARE_BROWSER_ENGINE===name)results.push(await run(name,engine,binary));const report={timestamp:new Date().toISOString(),binary_sha256:createHash('sha256').update(await fs.readFile(binary)).digest('hex'),results};if(process.env.FLARE_BROWSER_REPORT)await fs.writeFile(process.env.FLARE_BROWSER_REPORT,JSON.stringify(report,null,2));console.log(JSON.stringify(report,null,2));})().catch(e=>{console.error(e);process.exitCode=1});
