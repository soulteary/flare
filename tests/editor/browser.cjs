const {chromium}=require(require.resolve('playwright',{paths:[__dirname,`${__dirname}/../clock`]}));
const assert=require('node:assert/strict'),fs=require('node:fs/promises'),path=require('node:path'),net=require('node:net'),{spawn}=require('node:child_process');
(async()=>{
 const dir=await fs.mkdtemp('/tmp/flare-editor-list-'),socket=net.createServer();await new Promise(r=>socket.listen(0,'127.0.0.1',r));const port=socket.address().port;await new Promise(r=>socket.close(r));const base=`http://127.0.0.1:${port}`;
 await fs.writeFile(path.join(dir,'apps.yml'),'links:\n- name: Application\n  link: http://nas:8080\n  private: true\n');
 await fs.writeFile(path.join(dir,'bookmarks.yml'),'categories:\n- id: alpha\n  title: Alpha\n- id: beta\n  title: Beta\nlinks:\n- name: First bookmark\n  link: custom://host/a?x=1&y=2#fragment\n  category: alpha\n  desc: Preserve me\n- name: Hidden bookmark\n  link: https://example.com/hidden\n  category: beta\n');
 const child=spawn(process.env.FLARE_BINARY,[],{cwd:dir,env:{...process.env,FLARE_PORT:String(port),FLARE_DISABLE_LOGIN:'true',FLARE_OFFLINE:'true',FLARE_BASELINE:'1'}});let log='';child.stdout.on('data',b=>log+=b);child.stderr.on('data',b=>log+=b);let browser;
 try{
  for(let i=0;i<100;i++){if(child.exitCode!==null)throw Error(log);try{if((await fetch(base+'/ping')).ok)break}catch{}await new Promise(r=>setTimeout(r,100));}
  browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox','--disable-dev-shm-usage','--disable-gpu']});
  const context=await browser.newContext({viewport:{width:390,height:844},isMobile:true,hasTouch:true});const page=await context.newPage();page.setDefaultTimeout(8000);const errors=[];page.on('pageerror',e=>errors.push(e.message));
  async function loaded(p=page){await p.goto(base+'/editor');await p.waitForFunction(()=>window.FlareEditorUI?.store);}
  async function save(p=page){await p.locator('#save-changes').click();await p.waitForFunction(()=>document.getElementById('save-status').textContent==='所有更改已保存');}
  async function edit(name,newName,p=page){await p.getByRole('button',{name,exact:true}).click();assert.equal(await p.locator('#entry-name').inputValue(),name);await p.locator('#entry-name').fill(newName);await p.locator('#entry-form').getByRole('button',{name:'完成',exact:true}).click();}
  for(const width of [320,360,390,430,768,1440]){await page.setViewportSize({width,height:844});await loaded();const size=await page.locator('html').evaluate(e=>({s:e.scrollWidth,w:e.clientWidth}));assert.ok(size.s<=size.w+1,JSON.stringify(size));assert.ok((await page.locator('.entry').first().boundingBox()).y<600);assert.equal(await page.locator('#save-changes').isDisabled(),true);}
  await page.setViewportSize({width:390,height:844});await loaded();
  await page.locator('#entry-search').fill('First');assert.equal(await page.locator('.entry').count(),1);
  await edit('First bookmark','First edited');await save();
  let doc=await (await page.request.get(base+'/editor/document')).json();assert.equal(doc.bookmarks.length,2);assert.equal(doc.bookmarks[1].Name,'Hidden bookmark');assert.equal(doc.bookmarks[0].Desc,'Preserve me');assert.equal(doc.apps[0].Private,true);
  // Reverting the actual value removes dirty state, rather than retaining a sticky flag.
  await edit('First edited','First temporary');await edit('First temporary','First edited');assert.equal(await page.locator('#save-changes').isDisabled(),true);
  await page.locator('#entry-search').fill('');await page.locator('#new-entry').click();await page.locator('#entry-form').getByRole('button',{name:'完成',exact:true}).click();assert.match(await page.locator('#error-name').textContent(),/填写/);
  await page.locator('#entry-name').fill('New intranet');await page.locator('#entry-url').fill('custom://nas:1234/path?q=1&z=2#x');await page.locator('#entry-location').selectOption('');await page.locator('#entry-form').getByRole('button',{name:'完成',exact:true}).click();assert.match(await page.locator('#error-location').textContent(),/位置/);
  await page.locator('#entry-location').selectOption('category:beta');await page.locator('#entry-form').getByRole('button',{name:'完成',exact:true}).click();
  await page.locator('#manage-categories').click();await page.locator('[data-category=alpha]').fill('Renamed');await page.locator('[data-category=alpha]').press('Tab');
  await page.locator('#new-category-name').fill('Gamma');await page.locator('#category-add-form button').click();
  const alpha=page.locator('.category').filter({has:page.locator('[data-category=alpha]')});await alpha.getByRole('button',{name:'删除',exact:true}).click();await alpha.locator('.migration select').selectOption('category:beta');await alpha.getByRole('button',{name:'迁移并删除',exact:true}).click();await page.locator('#close-categories').click();
  assert.equal(await page.locator('.entry').count(),4);
  const hidden=page.locator('.entry').filter({has:page.getByRole('button',{name:'Hidden bookmark',exact:true})});await hidden.getByRole('button',{name:'删除',exact:true}).click();assert.equal(await page.locator('.entry').count(),3);await page.locator('#undo-delete').click();assert.equal(await page.locator('.entry').count(),4);await save();
  doc=await(await page.request.get(base+'/editor/document')).json();assert.equal(doc.categories.some(c=>c.ID==='alpha'),false);assert.ok(doc.bookmarks.every(b=>b.Category==='beta'));assert.equal(doc.categories.find(c=>c.Name==='Beta').ID,'beta');assert.equal(doc.bookmarks.length,3);
  // A second window must never overwrite a newer revision.
  const second=await context.newPage();second.setDefaultTimeout(8000);await loaded(second);
  await edit('Application','Application saved');await save();await edit('Application','Other window',second);await second.locator('#save-changes').click();await second.waitForFunction(()=>document.getElementById('export-draft').hidden===false);assert.match(await second.locator('#save-status').textContent(),/已变化/);
  const download=second.waitForEvent('download');await second.locator('#export-draft').click();const file=await download;const draft=JSON.parse(await fs.readFile(await file.path(),'utf8'));assert.equal(draft.apps[0].Name,'Other window');await second.close({runBeforeUnload:false});
  // Failed writes retain working data and can be retried.
  await edit('Application saved','Retry me');await page.route('**/editor/document',route=>route.request().method()==='POST'?route.fulfill({status:500,contentType:'application/json',body:'{"error":"模拟失败"}'}):route.continue());await page.locator('#save-changes').click();await page.waitForFunction(()=>document.getElementById('save-status').textContent.includes('模拟失败'));assert.equal(await page.getByRole('button',{name:'Retry me',exact:true}).count(),1);await page.unroute('**/editor/document');await save();
  await loaded();assert.equal(await page.getByRole('button',{name:'Retry me',exact:true}).count(),1);
  // A cancelled form must not mutate the working document.
  await page.getByRole('button',{name:'Retry me',exact:true}).click();await page.locator('#entry-name').fill('Cancelled');page.once('dialog',d=>d.accept());await page.locator('#cancel-entry').click();assert.equal(await page.locator('#save-changes').isDisabled(),true);
  if(process.env.EDITOR_SCREENSHOT_DIR){await fs.mkdir(process.env.EDITOR_SCREENSHOT_DIR,{recursive:true});await page.screenshot({path:path.join(process.env.EDITOR_SCREENSHOT_DIR,'mobile-list.png'),fullPage:true});await page.locator('#new-entry').click();await page.screenshot({path:path.join(process.env.EDITOR_SCREENSHOT_DIR,'mobile-form.png')});await page.locator('#cancel-entry').click();await page.setViewportSize({width:1440,height:960});await page.screenshot({path:path.join(process.env.EDITOR_SCREENSHOT_DIR,'desktop-list.png'),fullPage:true});}
  assert.deepEqual(errors,[]);console.log('PASS: responsive list, real filtering, complete filtered saves, forms, stable IDs, category migration, undo, actual dirty state, stale-window conflict, draft export, retry and reload');
 }finally{if(browser)await browser.close();child.kill();await fs.rm(dir,{recursive:true,force:true});}
})().catch(e=>{console.error(e);process.exitCode=1;});
