const browserAuth=require('../browser-auth.cjs');
// Exercise localized production markup and a draft larger than the former 4 MiB limit.
const {chromium}=require(require.resolve('playwright',{paths:[__dirname,`${__dirname}/../clock`]}));
const assert=require('node:assert/strict'),fs=require('node:fs/promises'),path=require('node:path'),net=require('node:net'),{spawn}=require('node:child_process');
(async()=>{
 const dir=await fs.mkdtemp('/tmp/flare-editor-reliability-'),socket=net.createServer();await new Promise(r=>socket.listen(0,'127.0.0.1',r));const port=socket.address().port;await new Promise(r=>socket.close(r));const base=`http://127.0.0.1:${port}`;
 await fs.writeFile(path.join(dir,'config.yml'),'Locale: en\nShowWeather: false\n');
 await fs.writeFile(path.join(dir,'apps.yml'),'links:\n- name: Application\n  link: http://nas:8080\n  private: true\n');
 await fs.writeFile(path.join(dir,'bookmarks.yml'),'links: []\n');
 const recovery=path.join(dir,'.flare-editor-recovery','save-interrupted');await fs.mkdir(recovery,{recursive:true});
 await fs.copyFile(path.join(dir,'apps.yml'),path.join(recovery,'apps.yml'));await fs.copyFile(path.join(dir,'bookmarks.yml'),path.join(recovery,'bookmarks.yml'));
 const child=spawn(process.env.FLARE_BINARY,[],{cwd:dir,env:{...process.env,FLARE_PORT:String(port),...browserAuth.credentials,FLARE_OFFLINE:'true',FLARE_BASELINE:'1'}});let log='';child.stdout.on('data',b=>log+=b);child.stderr.on('data',b=>log+=b);let browser;
 try{
  for(let i=0;i<100;i++){if(child.exitCode!==null)throw Error(log);try{if((await fetch(base+'/ping')).ok)break}catch{}await new Promise(r=>setTimeout(r,100));}
  browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox','--disable-dev-shm-usage']});
  const page=await browser.newPage();async function screenshot(name){if(process.env.EDITOR_SCREENSHOT_DIR){await fs.mkdir(process.env.EDITOR_SCREENSHOT_DIR,{recursive:true});await page.screenshot({path:path.join(process.env.EDITOR_SCREENSHOT_DIR,name),fullPage:true});}}const errors=[];page.on('pageerror',error=>errors.push(error.message));
  await browserAuth.login(page,base);await page.goto(base+'/editor');await page.waitForFunction(()=>window.FlareEditorUI?.store);
  assert.equal(await page.locator('html').getAttribute('lang'),'en');assert.equal(await page.title(),'Manage navigation · Flare');
  assert.equal(await page.locator('#save-changes').textContent(),'Save changes');assert.equal(await page.locator('#entry-search').getAttribute('placeholder'),'Search names, addresses and descriptions');
  assert.match(await page.locator('#recovery-warning').textContent(),/Server recovery copies.*save-interrupted/);
  assert.match(log,/Editor recovery copies detected/);await screenshot('editor-en-list.png');assert.doesNotMatch(await page.locator('body').innerText(),/[\u3400-\u9fff]/);
  await page.locator('#new-entry').click();assert.equal(await page.locator('#entry-dialog-title').textContent(),'New entry');
  await page.locator('#entry-form .dialog-actions button.primary').click();assert.equal(await page.locator('#error-name').textContent(),'Enter a name');
  await screenshot('editor-en-form.png');await page.locator('#entry-options').evaluate(el=>el.open=true);await page.locator('#choose-icon').click();assert.equal(await page.locator('#icon-picker-title').textContent(),'Choose a built-in icon');await page.keyboard.press('Escape');await page.locator('#cancel-entry').click();
  await page.locator('.menu summary').click();await page.locator('#open-batch').click();await page.waitForFunction(()=>window.FlareEditorBatch?.active);await screenshot('editor-en-batch.png');await page.locator('#leave-batch').click();
  const desc='x'.repeat(5*1024*1024),draft={categories:[],apps:[{Name:'Large draft',URL:'custom://host/path?x=1#f',Desc:desc,Private:true}],bookmarks:[]};
  const payload=Buffer.from(JSON.stringify(draft));assert.ok(payload.length>4*1024*1024);
  await page.locator('.menu summary').click();page.once('dialog',dialog=>dialog.accept());
  await page.locator('#draft-file').setInputFiles({name:'large-draft.json',mimeType:'application/json',buffer:payload});
  await page.waitForFunction(()=>window.FlareEditorUI.store.toJSON().apps[0].Name==='Large draft');
  assert.equal(await page.evaluate(()=>window.FlareEditorUI.store.toJSON().apps[0].Private),true);
  // Export uses the same capacity as restore and does not lose private fields.
  await page.evaluate(()=>document.getElementById('export-draft').hidden=false);const download=page.waitForEvent('download');await page.locator('#export-draft').click();const file=await download;
  const exported=JSON.parse(await fs.readFile(await file.path(),'utf8'));assert.equal(exported.apps[0].Desc.length,desc.length);assert.equal(exported.apps[0].Private,true);assert.equal(exported.apps[0].URL,draft.apps[0].URL);
  assert.deepEqual(errors,[]);console.log('PASS: English editor, validation, icon picker, recovery warning, and >4 MiB draft round trip');
 }finally{if(browser)await browser.close();child.kill('SIGTERM');await fs.rm(dir,{recursive:true,force:true});}
})().catch(error=>{console.error(error);process.exitCode=1;});
