// Production templates and APIs with JavaScript enabled: privacy controls must be wired.
const {chromium}=require(process.env.FLARE_PLAYWRIGHT_MODULE||'../clock/node_modules/playwright');
const assert=require('node:assert/strict'),fs=require('node:fs/promises'),os=require('node:os'),path=require('node:path'),net=require('node:net'),{spawn}=require('node:child_process');
(async()=>{
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
  await page.goto(base+'/login');await page.locator('[name=username]').fill('admin');await page.locator('[name=password]').fill('regression-password');await Promise.all([page.waitForURL(base+'/'),page.getByRole('button',{name:'Login / 登录',exact:true}).click()]);
  await page.goto(base+'/editor');await page.waitForFunction(()=>window.FlareEditorUI?.store);await page.locator('#new-entry').click();
  assert(await page.locator('#entry-private').isChecked());await page.locator('#entry-name').fill('Editor private');await page.locator('#entry-url').fill('https://editor.example/');await page.locator('#entry-location').selectOption('apps');
  await page.locator('#entry-private').uncheck();assert.match(await page.locator('#privacy-hint').innerText(),/anonymous/i);
  // Checkbox-only edits participate in unsaved-form protection.
  await page.locator('#entry-private').check();await page.locator('#entry-location').selectOption('category:secret');assert.match(await page.locator('#privacy-hint').innerText(),/categor/i);await page.locator('#entry-private').check();
  await page.locator('#entry-form button.primary').click();await page.route('**/editor/document',r=>r.request().method()==='POST'?r.fulfill({status:403,contentType:'application/json',body:JSON.stringify({code:'REAUTH_REQUIRED'})}):r.continue());page.once('dialog',d=>d.accept());await page.locator('#save-changes').click();await page.locator('#verify-session').waitFor();assert.match(await page.locator('#save-status').innerText(),/Verify password/);await page.unroute('**/editor/document');page.once('dialog',d=>d.accept());await page.locator('#save-changes').click();await page.waitForFunction(()=>!window.FlareEditorUI.store.dirty&&!window.FlareEditorUI.saving).catch(async e=>{throw Error(e.message+' status='+await page.locator('#save-status').innerText()+' errors='+JSON.stringify(errors));});
  const doc=await(await page.request.get(base+'/editor/document')).json();assert.equal(doc.bookmarks[0].Private,true);
  page.once('dialog',d=>d.accept());await page.locator('#draft-file').setInputFiles({name:'private-draft.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify(doc))});await page.waitForFunction(()=>document.getElementById('save-status').textContent.includes('Draft restored'));assert.equal(await page.evaluate(()=>window.FlareEditorUI.store.toJSON().categories.find(c=>c.ID==='secret').Private),true);
  await page.goto(base+'/editor/transfer');await page.locator('#file').setInputFiles({name:'import.yml',mimeType:'application/yaml',buffer:Buffer.from('links:\n- name: Imported\n  link: https://import.example/\n')});
  await page.locator('#preview-button').click();await page.locator('#preview').waitFor();assert.equal(await page.locator('#import-public').isChecked(),false);
  await page.locator('#import-public').check();assert(await page.locator('#preview').isHidden());assert(await page.locator('#commit').isDisabled()||await page.locator('#preview').isHidden());
  await page.locator('#preview-button').click();await page.locator('#preview').waitFor();assert.match(await page.locator('#summary').innerText(),/1/);
  await page.locator('#commit').click();await page.waitForFunction(()=>document.getElementById('status').textContent.startsWith('Import complete:'));
  const exported=await(await page.request.get(base+'/editor/document')).json();assert.equal(!!exported.bookmarks.find(b=>b.Name==='Imported').Private,false);assert.deepEqual(errors,[]);
  console.log('PASS: editor initialization, privacy defaults/hints/save, private-category inheritance, import preview/public-option invalidation/commit');
 }finally{if(browser)await browser.close();if(child.exitCode===null)await new Promise(r=>{const timer=setTimeout(()=>child.kill('SIGKILL'),5000);child.once('exit',()=>{clearTimeout(timer);r();});child.kill('SIGTERM');});await fs.rm(root,{recursive:true,force:true});}
})().catch(e=>{console.error(e);process.exitCode=1;});
