// FLARE_BINARY=/absolute/path NODE_PATH=/path/to/playwright/node_modules node tests/search/hybrid.cjs
const {chromium} = require('playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const net = require('node:net');
const {spawn} = require('node:child_process');
(async () => {
 const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-search-'));
 const socket = net.createServer();
 await new Promise(resolve => socket.listen(0, '127.0.0.1', resolve));
 const port = socket.address().port; await new Promise(resolve => socket.close(resolve));
 const base = `http://127.0.0.1:${port}`;
 assert.ok(process.env.FLARE_BINARY, 'Set FLARE_BINARY');
 await fs.writeFile(path.join(dir, 'config.yml'), `Title: Search test
ShowSearchComponent: true
DisabledSearchAutoFocus: true
ShowApps: true
ShowBookmarks: true
ShowDateTime: true
ShowWeather: false
Locale: en
Clock:
  Dynamic: true
  TimezoneMode: custom
  Timezone: UTC
Search:
  Mode: hybrid
  DefaultTarget: example
  EnableKeywords: true
  OpenNewTab: true
  Engines:
    - ID: example
      Name: Example
      Keyword: ex
      URLTemplate: "https://search.example/search?q={query}"
      Enabled: true
`);
 await fs.writeFile(path.join(dir,'apps.yml'),'links:\n- name: Needle application\n  link: "'+base+'/chosen"\n  desc: Find me\n');
 await fs.writeFile(path.join(dir,'bookmarks.yml'),'links:\n- name: "Needle bookmark"\n  link: "https://bookmark.example/"\n');
 const child = spawn(process.env.FLARE_BINARY, [], {cwd:dir,env:{...process.env,FLARE_PORT:String(port),FLARE_DISABLE_LOGIN:'true',FLARE_OFFLINE:'true',FLARE_BASELINE:'1'}});
 let logs=''; child.stdout.on('data',b=>logs+=b); child.stderr.on('data',b=>logs+=b);
 let browser;
 try {
  for(let i=0;i<100;i++){ if(child.exitCode!==null) throw Error(logs); try{if((await fetch(base+'/ping')).ok)break;}catch{} await new Promise(r=>setTimeout(r,100)); }
  browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox','--disable-dev-shm-usage','--disable-gpu']});
  const page=await browser.newPage(); const errors=[];
  page.on('pageerror',e=>errors.push(e.message));
  page.on('console',message=>{if(message.type()==='error' && /Content Security Policy|Refused to execute|Refused to connect/i.test(message.text())) errors.push(message.text());});
  await page.goto(base);
  const clockBefore=await page.locator('#clock-time').textContent();
  await page.waitForFunction(previous=>document.querySelector('#clock-time').textContent!==previous,clockBefore,{timeout:3000});
  await page.locator('#search').fill('Needle');
  await page.waitForFunction(()=>document.querySelectorAll('#search-suggestions a').length===2);
  assert.equal(await page.locator('#search-suggestions img').count(),0);
  assert.equal(await page.locator('#search').getAttribute('aria-expanded'),'true');
  await page.locator('#search').press('Escape');
  assert.equal(await page.locator('#search').inputValue(),'Needle');
  assert.equal(await page.locator('#search').getAttribute('aria-expanded'),'false');
  await page.locator('#search').fill('Needle ');
  await page.waitForFunction(()=>document.querySelectorAll('#search-suggestions a').length===2);
  await page.locator('#search').press('ArrowDown');
  await page.locator('#search').press('Enter');
  await page.waitForURL(base+'/chosen');
  await page.goto(base);
  let requests=0;
  page.on('request',request=>{if(request.url().endsWith('/search/suggest'))requests++;});
  await page.locator('#search').evaluate(el=>el.dispatchEvent(new CompositionEvent('compositionstart',{bubbles:true})));
  await page.locator('#search').fill('Needle');
  await page.locator('#search').press('Enter');
  assert.equal(page.url(),base+'/','composition Enter must not navigate');
  await page.waitForTimeout(250);
  assert.equal(requests,0,'composition must not request suggestions');
  await page.locator('#search').evaluate(el=>el.dispatchEvent(new CompositionEvent('compositionend',{bubbles:true})));
  await page.waitForFunction(()=>document.querySelectorAll('#search-suggestions a').length===2);
  assert.equal(requests,1);
  await page.route('**/search/suggest',async route=>{
   const query=new URLSearchParams(route.request().postData()).get('search');
   await new Promise(r=>setTimeout(r,query==='old'?400:10));
   await route.fulfill({json:[{name:query,url:'https://result.example/'+query,newTab:false},{name:'<img src=x onerror=alert(1)>',url:'https://safe.example/',newTab:false}]});
  });
  await page.locator('#search').fill('old'); await page.waitForTimeout(210);
  await page.locator('#search').fill('new');
  await page.waitForFunction(()=>document.querySelector('#search-suggestions a')?.textContent==='new');
  await page.waitForTimeout(500);
  assert.equal(await page.locator('#search-suggestions a').first().textContent(),'new');
  assert.equal(await page.locator('#search-suggestions img').count(),0);
  await page.locator('#search').fill('!unknown');
  await page.waitForFunction(()=>document.querySelector('#search-suggestions a')?.textContent==='!unknown');
  await page.unroute('**/search/suggest');
  await page.context().route('https://search.example/**',route=>route.fulfill({body:'external search'}));
  await page.locator('#search').fill('!ex external');
  const requestPromise=page.context().waitForEvent('request',request=>request.url().startsWith('https://search.example/'));
  const popupPromise=page.waitForEvent('popup');
  await page.locator('#search').press('Enter');
  const popup=await popupPromise; await popup.waitForLoadState();
  assert.match((await requestPromise).url(),/search\.example\/search\?q=external/);
  await popup.close();
  assert.equal(page.url(),base+'/');
  await page.locator('#search-target').selectOption('local');
  await page.locator('#search').fill('Needle');
  await page.locator('#search').press('Enter');
  await page.waitForLoadState();
  assert.equal(page.url(),base+'/','local search must stay in the same tab');
  assert.deepEqual(errors,[]);
  console.log('Hybrid browser acceptance passed');
 } finally { if(browser)await browser.close(); child.kill('SIGTERM'); await fs.rm(dir,{recursive:true,force:true}); }
})().catch(error=>{console.error(error);process.exitCode=1;});
