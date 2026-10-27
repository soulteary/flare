// Real desktop engine acceptance against an isolated native binary.
// FLARE_BINARY=/tmp/flare-phase1-current FLARE_BROWSER_REPORT=/tmp/report.json node tests/security-browser/acceptance.cjs
const playwrightModule=process.env.FLARE_PLAYWRIGHT_MODULE||'../clock/node_modules/playwright';
const {chromium, firefox, webkit} = require(playwrightModule);
const fs = require('node:fs/promises');
const path = require('node:path');
const os = require('node:os');
const net = require('node:net');
const assert = require('node:assert/strict');
const {spawn} = require('node:child_process');
const {createHash} = require('node:crypto');

async function freePort() {
 const server=net.createServer();await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve)});
 const port=server.address().port;await new Promise(resolve=>server.close(resolve));return port;
}
async function stop(child) {
 if(!child||child.exitCode!==null)return;
 await new Promise(resolve=>{const timer=setTimeout(()=>child.kill('SIGKILL'),7000);child.once('exit',()=>{clearTimeout(timer);resolve()});child.kill('SIGTERM')});
}
async function launchInstance(binary) {
 const root=await fs.mkdtemp(path.join(os.tmpdir(),'flare-security-browser-'));const port=await freePort();
 const originals={
  'config.yml':'Title: Browser acceptance\nLocale: en\nTheme: blackboard\nShowApps: true\nShowBookmarks: true\nShowWeather: false\nFooter: "<span>INTERNAL-FOOTER-SENTINEL</span>"\nLoginFooter: "<a href=\'https://example.com\'>PUBLIC-FOOTER-SENTINEL</a><script>LOGIN-SCRIPT-SENTINEL</script>"\n',
  'apps.yml':'links: []\n',
  'bookmarks.yml':'categories:\n- id: secret\n  title: PRIVATE-CATEGORY-SENTINEL\n  private: true\nlinks:\n- name: PRIVATE-ITEM-SENTINEL\n  link: https://private.example.test/\n  category: secret\n  private: false\n'
 };
 for(const [name,raw] of Object.entries(originals))await fs.writeFile(path.join(root,name),raw,{mode:0o600});
 const env=Object.fromEntries(Object.entries(process.env).filter(([key])=>!key.startsWith('FLARE_')));
 Object.assign(env,{FLARE_PORT:String(port),FLARE_DISABLE_LOGIN:'false',FLARE_VISIBILITY:'PRIVATE',FLARE_USER:'browser-admin',FLARE_PASS:'old-short',FLARE_OFFLINE:'true',FLARE_GUIDE:'false',FLARE_BASELINE:'1'});
 const child=spawn(binary,[],{cwd:root,env,stdio:['ignore','pipe','pipe']});let logs='';child.stdout.on('data',b=>logs+=b);child.stderr.on('data',b=>logs+=b);
 const base=`http://127.0.0.1:${port}`;
 for(let i=0;i<200;i++){
  if(child.exitCode!==null)throw Error(`daemon exited ${child.exitCode}: ${logs}`);
  try{const response=await fetch(base+'/login');if(response.status===200)return {root,base,child,originals};}catch{}
  await new Promise(resolve=>setTimeout(resolve,100));
 }
 await stop(child);throw Error('isolated daemon readiness timeout');
}
async function login(page,base,password) {
 await page.goto(base+'/login');
 await page.locator('input[name=username]').fill('browser-admin');await page.locator('input[name=password]').fill(password);
 await Promise.all([page.waitForURL(base+'/',{timeout:30000}),page.getByRole('button',{name:'Login / 登录',exact:true}).click()]);
 await page.getByText('PRIVATE-ITEM-SENTINEL',{exact:true}).waitFor();
}
async function run(engine,name,binary){
 let page; const responses=[];
 const state=await launchInstance(binary);let browser;const result={engine:name,scope:'desktop headless, synthetic data, JavaScript disabled; not a physical phone/NAS',cases:[]};
 const record=(label)=>{result.cases.push(label);console.error('['+name+'] '+label);};console.error('['+name+'] started');
 try{
  const executable=process.env['FLARE_'+name.toUpperCase()+'_EXECUTABLE'];
  const browserEnv=Object.fromEntries(Object.entries(process.env).filter(([key])=>!['http_proxy','https_proxy','all_proxy','no_proxy'].includes(key.toLowerCase())));
  browser=await engine.launch({headless:true,env:browserEnv,...(name==='firefox'?{firefoxUserPrefs:{'network.proxy.type':0}}:{}),...(executable?{executablePath:executable}:{})});result.version=browser.version();result.driver=require(playwrightModule+'/package.json').version;result.executable_override=Boolean(executable);
  const context=await browser.newContext({javaScriptEnabled:false,viewport:{width:390,height:844},deviceScaleFactor:1});
  context.setDefaultTimeout(30000);page=await context.newPage();page.on("response",r=>{const u=new URL(r.url());if(u.origin===state.base)responses.push({method:r.request().method(),path:u.pathname,status:r.status(),origin:r.request().headers()["origin"],referer:r.request().headers()["referer"],site:r.request().headers()["sec-fetch-site"]})});
  await page.goto(state.base+'/login');let body=await page.locator('body').innerText();
  assert(!body.includes('PRIVATE-ITEM-SENTINEL'));assert(!body.includes('PRIVATE-CATEGORY-SENTINEL'));assert(!body.includes('INTERNAL-FOOTER-SENTINEL'));assert(!body.includes('LOGIN-SCRIPT-SENTINEL'));assert(body.includes('PUBLIC-FOOTER-SENTINEL'));
  record('private login/footer isolation');
  await login(page,state.base,'old-short');record('legacy short password + native cookie handshake without JavaScript');
  assert((await page.locator('body').innerText()).includes('INTERNAL-FOOTER-SENTINEL'));
  const secondContext=await browser.newContext({javaScriptEnabled:false});secondContext.setDefaultTimeout(30000);const second=await secondContext.newPage();await login(second,state.base,'old-short');
  await page.goto(state.base+'/settings/application');const takeover=page.locator('form[action="/account/local"]');
  await takeover.locator('input[name=password]').fill('old-short');await takeover.locator('input[name=confirm]').check();
  await Promise.all([page.waitForURL(state.base+'/settings/application'),takeover.getByRole('button').click()]);
  await second.goto(state.base+'/');assert(await second.locator('input[name=username]').isVisible());
  record('explicit deployment-to-local switch revokes other browser');
  const passwordForm=page.locator('form[action="/account/password"]');
  await passwordForm.locator('input[name=password]').fill('old-short');await passwordForm.locator('input[name=new_password]').fill('Unique browser password 2026!');await passwordForm.locator('input[name=confirm_password]').fill('Unique browser password 2026!');
  await Promise.all([page.waitForURL(state.base+'/settings/application'),passwordForm.getByRole('button').click()]);
  record('password change without JavaScript');
  const reauth=page.locator('form[action="/account/reauthenticate"]');await reauth.locator('input[name=password]').fill('Unique browser password 2026!');
  await Promise.all([page.waitForURL(state.base+'/settings/application'),reauth.getByRole('button').click()]);record('reauthentication without JavaScript');
  const overflow=await page.evaluate(()=>{document.documentElement.style.fontSize='200%';return document.documentElement.scrollWidth>innerWidth+1});assert(!overflow,'account page overflows at 390px / 200% text');record('390px viewport and 200% font no horizontal overflow (emulation)');
  const logout=page.locator('form[action="/logout-all"]');await Promise.all([page.waitForURL(state.base+'/login'),logout.getByRole('button').click()]);await page.goto(state.base+'/');assert(await page.locator('input[name=username]').isVisible());record('logout-all without JavaScript');
  for(const [file,raw] of Object.entries(state.originals))assert.equal(await fs.readFile(path.join(state.root,file),'utf8'),raw,'migration/account operation rewrote '+file);
  const raw=await fs.readFile(path.join(state.root,'.flare-security','state.json'),'utf8');assert(!raw.includes('Unique browser password 2026!'));assert(!raw.includes('old-short'));record('YAML byte preservation and no plaintext persisted password');
  result.status='PASS';await context.close();await secondContext.close();
 }catch(error){result.status='FAIL';result.error=String(error.stack||error);result.responses=responses;result.last_body=page?await page.locator("body").innerText().catch(()=>"unavailable"):"unavailable";process.exitCode=1;}
 finally{if(browser)await browser.close();await stop(state.child);await fs.rm(state.root,{recursive:true,force:true});}
 return result;
}
(async()=>{
 const binary=path.resolve(process.env.FLARE_BINARY||'/tmp/flare-phase1-current');const results=[];
 for(const [name,engine] of [['chromium',chromium],['firefox',firefox],['webkit',webkit]])if(!process.env.FLARE_BROWSER_ENGINE||process.env.FLARE_BROWSER_ENGINE===name)results.push(await run(engine,name,binary));
 const report={timestamp:new Date().toISOString(),binary_sha256:createHash('sha256').update(await fs.readFile(binary)).digest('hex'),results};
 const output=process.env.FLARE_BROWSER_REPORT;if(output)await fs.writeFile(output,JSON.stringify(report,null,2));
 console.log(JSON.stringify(report,null,2));
})().catch(error=>{console.error(error);process.exitCode=1});
