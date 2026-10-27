// Production templates and APIs with JavaScript enabled: privacy controls must be wired.
const {chromium}=require(process.env.FLARE_PLAYWRIGHT_MODULE||'../clock/node_modules/playwright');
const assert=require('node:assert/strict'),fs=require('node:fs/promises'),os=require('node:os'),path=require('node:path'),net=require('node:net'),{spawn}=require('node:child_process');
(async()=>{
 let phase='startup';const progress=label=>{phase=label;console.log('CHECK: '+label);};
 const watchdog=setTimeout(()=>{console.error('Editor transfer browser test exceeded 120s during '+phase);process.exit(1);},120000);watchdog.unref();
 async function bounded(promise,label){let timer;try{return await Promise.race([promise,new Promise((_,reject)=>{timer=setTimeout(()=>reject(Error('Timed out during '+label)),15000);})]);}finally{clearTimeout(timer);}}
 const root=await fs.mkdtemp(path.join(os.tmpdir(),'flare-editor-transfer-'));
 const socket=net.createServer();await new Promise(r=>socket.listen(0,'127.0.0.1',r));const port=socket.address().port;await new Promise(r=>socket.close(r));
 const base=`http://127.0.0.1:${port}`,env=Object.fromEntries(Object.entries(process.env).filter(([k])=>!k.startsWith('FLARE_')&&!['http_proxy','https_proxy','all_proxy','no_proxy'].includes(k.toLowerCase())));
 await fs.writeFile(path.join(root,'config.yml'),'Title: Editor regression\nLocale: en\nShowWeather: false\n');
 await fs.writeFile(path.join(root,'apps.yml'),'links: []\n');
 await fs.writeFile(path.join(root,'bookmarks.yml'),'categories:\n- id: secret\n  title: Secret\n  private: true\nlinks: []\n');
 const child=spawn(path.resolve(process.env.FLARE_BINARY),[],{cwd:root,env:{...env,FLARE_PORT:String(port),FLARE_USER:'admin',FLARE_PASS:'regression-password',FLARE_DISABLE_LOGIN:'false',FLARE_OFFLINE:'true'},stdio:['ignore','pipe','pipe']});let logs='',browser;
 child.stdout.on('data',b=>logs+=b);child.stderr.on('data',b=>logs+=b);
 try{
  let ready=false;for(let i=0;i<200;i++){if(child.exitCode!==null)throw Error(logs);try{if((await fetch(base+'/login')).status===200){ready=true;break;}}catch{}await new Promise(r=>setTimeout(r,100));}assert(ready,logs);
  browser=await chromium.launch({headless:true,env});const page=await browser.newPage();page.setDefaultTimeout(15000);const errors=[];page.on('pageerror',e=>errors.push(e.message));
  progress('login and editor privacy');await page.goto(base+'/login');await page.locator('[name=username]').fill('admin');await page.locator('[name=password]').fill('regression-password');await Promise.all([page.waitForURL(base+'/'),page.getByRole('button',{name:'Login / 登录',exact:true}).click()]);
  await page.goto(base+'/editor');await page.waitForFunction(()=>window.FlareEditorUI?.store);await page.locator('#new-entry').click();
  assert(await page.locator('#entry-private').isChecked());await page.locator('#entry-name').fill('Editor private');await page.locator('#entry-url').fill('https://editor.example/');await page.locator('#entry-location').selectOption('apps');
  await page.locator('#entry-private').uncheck();assert.match(await page.locator('#privacy-hint').innerText(),/anonymous/i);
  // Checkbox-only edits participate in unsaved-form protection.
  await page.locator('#entry-private').check();await page.locator('#entry-location').selectOption('category:secret');assert.match(await page.locator('#privacy-hint').innerText(),/categor/i);await page.locator('#entry-private').check();
  await page.locator('#entry-form button.primary').click();await page.route('**/editor/document',r=>r.request().method()==='POST'?r.fulfill({status:403,contentType:'application/json',body:JSON.stringify({error:{code:'REAUTH_REQUIRED',message:'REAUTH_REQUIRED'},request_id:'browser-reauth-fixture'})}):r.continue());page.once('dialog',d=>d.accept());await page.locator('#save-changes').click();await page.locator('#verify-session').waitFor();assert.match(await page.locator('#save-status').innerText(),/Verify password/);await page.unroute('**/editor/document');page.once('dialog',d=>d.accept());await page.locator('#save-changes').click();await page.waitForFunction(()=>!window.FlareEditorUI.store.dirty&&!window.FlareEditorUI.saving).catch(async e=>{throw Error(e.message+' status='+await page.locator('#save-status').innerText()+' errors='+JSON.stringify(errors));});
  progress('draft and import preview');const doc=await(await page.request.get(base+'/editor/document')).json();assert.equal(doc.bookmarks[0].Private,true);
  page.once('dialog',d=>d.accept());await page.locator('#draft-file').setInputFiles({name:'private-draft.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify(doc))});await page.waitForFunction(()=>document.getElementById('save-status').textContent.includes('Draft restored'));assert.equal(await page.evaluate(()=>window.FlareEditorUI.store.toJSON().categories.find(c=>c.ID==='secret').Private),true);
  await page.goto(base+'/editor/transfer');await page.locator('#file').setInputFiles({name:'import.yml',mimeType:'application/yaml',buffer:Buffer.from('links:\n- name: Imported\n  link: https://import.example/\n')});
  await page.locator('#preview-button').click();await page.locator('#preview').waitFor();assert.equal(await page.locator('#import-public').isChecked(),false);
  await page.locator('#import-public').check();assert(await page.locator('#preview').isHidden());assert(await page.locator('#commit').isDisabled()||await page.locator('#preview').isHidden());
  await page.locator('#preview-button').click();await page.locator('#preview').waitFor();assert.match(await page.locator('#summary').innerText(),/1/);
  const reauthResponse={status:403,contentType:'application/json',body:JSON.stringify({error:{code:'REAUTH_REQUIRED',message:'REAUTH_REQUIRED'},request_id:'transfer-reauth-fixture'})};
  async function verifyGuidance(){await page.locator('#verify-session').waitFor();const message=await page.locator('#status').innerText();assert.match(message,/Verify password/);assert.doesNotMatch(message,/\[object Object\]|Preview again/);assert.equal(await page.locator('#verify-session').getAttribute('href'),'/settings/application');assert.equal(await page.locator('#verify-session').getAttribute('target'),'_blank');}
  const beforeDenied=await fs.readFile(path.join(root,'bookmarks.yml'),'utf8');let deniedCommits=0;
  await page.route('**/editor/transfer/commit',r=>{deniedCommits++;return r.fulfill(reauthResponse);});
  await page.locator('#commit').click();await verifyGuidance();assert.equal(await page.locator('#commit').isDisabled(),true);assert.equal(await page.locator('#import-public').isChecked(),true);assert.equal(await fs.readFile(path.join(root,'bookmarks.yml'),'utf8'),beforeDenied);
  progress('manual reauthentication');const popupPromise=page.waitForEvent('popup');await page.locator('#verify-session').click();const account=await popupPromise;const verifyForm=account.locator('form[action="/account/reauthenticate"]');await verifyForm.waitFor();assert.equal(deniedCommits,1,'opening verification must not auto-submit');await verifyForm.locator('[name=password]').fill('regression-password');const verified=account.waitForResponse(r=>r.request().method()==='POST'&&r.url()===base+'/account/reauthenticate');await verifyForm.locator('button').click();assert.equal((await verified).status(),303);await account.close();assert.equal(deniedCommits,1,'verification must not auto-submit the pending import');
  await page.unroute('**/editor/transfer/commit');await page.locator('#preview-button').click();await page.locator('#preview').waitFor();await page.locator('#commit').click();await page.waitForFunction(()=>document.getElementById('status').textContent.startsWith('Import complete:'));assert.equal(await page.locator('#verify-session').isHidden(),true);
  progress('export and backup downloads');for(const selector of ['#export-html','#export-yaml','#backup']){
   progress('denied download '+selector);await page.reload();let downloads=0;const record=()=>downloads++;page.on('download',record);
   await page.route('**/editor/transfer/**',r=>r.fulfill(reauthResponse));page.once('dialog',d=>d.accept());await page.locator(selector).click();await verifyGuidance();assert.equal(downloads,0);
   progress('allowed download '+selector);await page.unroute('**/editor/transfer/**');const downloadPromise=page.waitForEvent('download');page.once('dialog',d=>d.accept());await page.locator(selector).click();const downloaded=await downloadPromise;assert.equal(await bounded(downloaded.failure(),'download completion '+selector),null);assert.ok((await fs.readFile(await bounded(downloaded.path(),'download path '+selector))).length);assert.equal(await page.locator('#verify-session').isHidden(),true);page.off('download',record);
  }
  progress('download error responses');for(const response of [{error:{code:'STORAGE_UNAVAILABLE',message:'Storage unavailable'},request_id:'transfer-storage-fixture'},{error:'Legacy download failure'}]){
   await page.route('**/editor/transfer/**',r=>r.fulfill({status:503,contentType:'application/json',body:JSON.stringify(response)}));page.once('dialog',d=>d.accept());await page.locator('#export-yaml').click();await page.waitForFunction(expected=>document.getElementById('status').textContent===expected,typeof response.error==='string'?response.error:response.error.message);assert.match(await page.locator('#status').innerText(),/Storage unavailable|Legacy download failure/);assert.equal(await page.locator('#verify-session').isHidden(),true);await page.unroute('**/editor/transfer/**');
  }

  const exported=await(await page.request.get(base+'/editor/document')).json();assert.equal(!!exported.bookmarks.find(b=>b.Name==='Imported').Private,false);assert.deepEqual(errors,[]);
  console.log('PASS: editor privacy/save, draft preservation, import preview/commit, structured reauthentication guidance and manual retry for public import, HTML/YAML exports and backup, plus nested and legacy download errors');
 }finally{
  progress('browser and server cleanup');
  try{if(browser)await bounded(browser.close(),'browser close');}
  finally{
   if(child.exitCode===null&&child.signalCode===null)await new Promise(resolve=>{
    const done=()=>{clearTimeout(timer);resolve();};
    const timer=setTimeout(()=>{child.kill('SIGKILL');done();},5000);
    child.once('exit',done);child.kill('SIGTERM');
   });
   await fs.rm(root,{recursive:true,force:true});clearTimeout(watchdog);
  }
 }
})().catch(e=>{console.error(e);process.exitCode=1;});
