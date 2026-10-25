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
    - ID: secondary
      Name: Secondary
      Keyword: s
      URLTemplate: "https://search.example/secondary?q={query}"
      Enabled: true
`);
 await fs.writeFile(path.join(dir,'apps.yml'),'links:\n- name: Needle application\n  link: "'+base+'/chosen"\n  desc: Find me\n');
 await fs.writeFile(path.join(dir,'bookmarks.yml'),'links:\n- name: "Needle bookmark"\n  link: "https://bookmark.example/"\n');
 const originalConfig=await fs.readFile(path.join(dir,'config.yml'));
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
  assert.deepEqual(await fs.readFile(path.join(dir,'config.yml')),originalConfig,'read-only startup must preserve legacy config');
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
  await page.locator('#search').fill('old');
  await page.waitForTimeout(210);
  await page.locator('#search-target input:checked').focus();
  await page.waitForTimeout(500);
  assert.equal(await page.locator('#search-suggestions').isVisible(),false,'blurred requests must not reopen suggestions');
  await page.unroute('**/search/suggest');
  await page.context().route('https://search.example/**',route=>route.fulfill({body:'external search'}));
  async function knownCommandDoesNotSuggest(query) {
   const before=requests;
   await page.locator('#search').fill(query);
   await page.waitForTimeout(230);
   assert.equal(requests,before,query+' must bypass suggestions');
   assert.equal(await page.locator('#search-suggestions').isVisible(),false);
  }
  async function externalSubmit(query, selected, expectedPath) {
   await page.locator('#search-target label').filter({has: page.locator('input[value="' + selected + '"]')}).click();
   await knownCommandDoesNotSuggest(query);
   const requestPromise=page.context().waitForEvent('request',request=>request.url().startsWith('https://search.example/'));
   const popupPromise=page.waitForEvent('popup');
   await page.locator('#search').press('Enter');
   const popup=await popupPromise;
   assert.equal(new URL((await requestPromise).url()).pathname,expectedPath);
   await popup.close();
   assert.equal(page.url(),base+'/');
  }
  // A shortcut must override a contradictory selection, including names containing s.
  await externalSubmit('!ex external','secondary','/search');
  await externalSubmit('!s external','example','/secondary');
  await page.locator('#search-target label').filter({has: page.locator('input[value="example"]')}).click();
  await knownCommandDoesNotSuggest('!local Needle');
  const pagesBefore=page.context().pages().length;
  const localResponse=page.waitForResponse(response=>response.url()===base+'/' && response.request().method()==='POST');
  await page.locator('#search').press('Enter');
  await localResponse;
  await page.waitForLoadState();
  assert.equal(page.context().pages().length,pagesBefore,'!local from external default must stay in current tab');
  assert.equal(await page.locator('#search-target input:checked').inputValue(),'example');
  // Changing the engine invalidates the previously keyboard-selected local match.
  await page.goto(base);
  await page.locator('#search').fill('Needle');
  await page.waitForFunction(()=>document.querySelectorAll('#search-suggestions a').length===2);
  await page.locator('#search').press('ArrowDown');
  await page.locator('#search-target label').filter({has: page.locator('input[value="secondary"]')}).click();
  assert.equal(await page.locator('#search').getAttribute('aria-activedescendant'),null);
  assert.equal(await page.locator('#search-suggestions').isVisible(),false);
  await page.locator('#search').focus();
  const changedRequest=page.context().waitForEvent('request',request=>request.url().startsWith('https://search.example/secondary'));
  const changedPopup=page.waitForEvent('popup');
  await page.locator('#search').press('Enter');
  await changedRequest;
  await (await changedPopup).close();
  assert.equal(page.url(),base+'/');
  // Native anchor mouse and touch activation must survive the input losing focus.
  await page.goto(base);
  await page.locator('#search').fill('Needle');
  await page.waitForFunction(()=>document.querySelectorAll('#search-suggestions a').length===2);
  await page.locator('#search-suggestions a').first().click();
  await page.waitForURL(base+'/chosen');
  const touchContext=await browser.newContext({hasTouch:true,isMobile:true,viewport:{width:390,height:844}});
  const touchPage=await touchContext.newPage();
  await touchPage.goto(base);
  await touchPage.locator('#search').fill('Needle');
  await touchPage.waitForFunction(()=>document.querySelectorAll('#search-suggestions a').length===2);
  await touchPage.locator('#search-suggestions a').first().tap();
  await touchPage.waitForURL(base+'/chosen');
  await touchContext.close();
  assert.deepEqual(errors,[]);
  // Switch modes through the actual settings form and verify saved behavior.
  await page.goto(base+'/settings/search');
  await page.locator('input[name="search-mode"][value="local-first"]').check();
  await page.locator('button.btn-submit').click();
  await page.goto(base);
  assert.equal(await page.locator('#search-target').count(),0);
  await page.locator('#search').fill('Needle');
  await page.waitForFunction(()=>document.querySelectorAll('#search-suggestions a').length===2);
  await page.locator('#search').press('Enter');
  await page.waitForLoadState();
  assert.equal(new URL(page.url()).origin,base);
  await page.locator('.search-external-actions').waitFor();
  const manualRequest=page.context().waitForEvent('request',request=>request.url().startsWith('https://search.example/search'));
  const manualPopup=page.waitForEvent('popup');
  await page.locator('.search-external-actions button[value="example"]').click();
  const webPage=await manualPopup;
  assert.equal(new URL((await manualRequest).url()).searchParams.get('q'),'Needle');
  assert.equal(new URL(page.url()).origin,base);
  await webPage.close();
  await page.goto(base+'/settings/search');
  await page.locator('input[name="search-mode"][value="select"]').check();
  await page.locator('button.btn-submit').click();
  await page.goto(base);
  await page.locator('#search-target label').filter({has:page.locator('input[value="local"]')}).click();
  await page.locator('#search').fill('Needle');
  await page.locator('#search').press('Enter');
  await page.waitForLoadState();
  assert.equal(await page.locator('#search-target input:checked').inputValue(),'local');
  assert.equal(await page.locator('#search').getAttribute('role'),null);
  assert.deepEqual(errors,[]);
  // Reproduce the old hybrid + local combination using an actual disk edit.
  const legacyConfig=(await fs.readFile(path.join(dir,'config.yml'),'utf8'))
   .replace(/Mode: select/,'Mode: hybrid').replace(/DefaultTarget: [^\n]+/,'DefaultTarget: local').replace(/^AppVersion:.*\n/m,'');
  await fs.writeFile(path.join(dir,'config.yml'),legacyConfig);
  await page.goto(base);
  assert.equal(await page.locator('#search-target').count(),0,'legacy conflicting mode must run locally');
  assert.deepEqual(await fs.readFile(path.join(dir,'config.yml')),Buffer.from(legacyConfig),'compatibility must not persist on read');
  await page.goto(base+'/settings/search');
  assert.equal(await page.locator('input[name="search-mode"][value="local-first"]').isChecked(),true);
  await page.locator('input[name="search-mode"][value="hybrid"]').check();
  await page.locator('button.btn-submit').click();
  assert.equal(await page.locator('[role="alert"]').count(),0);
  assert.equal(await page.locator('#default-target').inputValue(),'example');
  const backupDir=path.join(dir,'.flare-search-backups');
  const manifest=JSON.parse(await fs.readFile(path.join(backupDir,'manifest.json'),'utf8'));
  assert.ok(manifest.upgrade_file.includes(manifest.upgrade_version));
  assert.deepEqual(await fs.readFile(path.join(backupDir,manifest.upgrade_file)),originalConfig,'permanent backup must remain the first original');
  assert.deepEqual(await fs.readFile(path.join(backupDir,'latest.bak')),Buffer.from(legacyConfig));
  assert.equal((await fs.readdir(backupDir)).filter(name=>name.endsWith('.bak')).length,2);
  const beforeConfig=await fs.readFile(path.join(dir,'config.yml'));
  const beforeManifest=await fs.readFile(path.join(backupDir,'manifest.json'));
  const beforeLatest=await fs.stat(path.join(backupDir,'latest.bak'));
  await page.locator('button.btn-submit').click();
  assert.deepEqual(await fs.readFile(path.join(dir,'config.yml')),beforeConfig);
  assert.deepEqual(await fs.readFile(path.join(backupDir,'manifest.json')),beforeManifest);
  assert.equal((await fs.stat(path.join(backupDir,'latest.bak'))).ino,beforeLatest.ino,'unchanged save must not rotate backup');
  console.log('PASS: three modes, legacy upgrade, permanent/latest backups, unchanged save and settings switching');
 } finally { if(browser)await browser.close(); child.kill('SIGTERM'); await fs.rm(dir,{recursive:true,force:true}); }
})().catch(error=>{console.error(error);process.exitCode=1;});
