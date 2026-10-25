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
  // Pending dialog values must be protected before they enter the working document.
  async function unloadBlocked(){return page.evaluate(()=>{const event=new Event('beforeunload',{cancelable:true});window.dispatchEvent(event);return event.defaultPrevented;});}
  assert.equal(await unloadBlocked(),false);
  await page.locator('#new-entry').click();assert.equal(await unloadBlocked(),false);
  for(const [selector,value] of [['#entry-name','Pending name'],['#entry-url','http://pending'],['#entry-description','Pending description']]){
   await page.locator('#entry-options').evaluate(e=>e.open=true);await page.locator(selector).fill(value);assert.equal(await unloadBlocked(),true,selector);assert.equal(await page.evaluate(()=>window.FlareEditorUI.store.dirty),false);
   await page.locator(selector).fill('');assert.equal(await unloadBlocked(),false,selector+' reverted');
  }
  await page.locator('#entry-name').fill('Keep this form');
  const beforeUnload=page.waitForEvent('dialog');const reloading=page.reload().catch(error=>error);
  const leave=await beforeUnload;assert.equal(leave.type(),'beforeunload');await leave.dismiss();assert.ok((await reloading) instanceof Error);
  assert.equal(await page.locator('#entry-name').inputValue(),'Keep this form');assert.equal(await page.locator('#entry-dialog').evaluate(e=>e.open),true);
  page.once('dialog',d=>d.accept());await page.locator('#cancel-entry').click();assert.equal(await unloadBlocked(),false);
  await page.getByRole('button',{name:'Application',exact:true}).click();assert.equal(await unloadBlocked(),false);
  await page.locator('#entry-name').fill('Pending edit');assert.equal(await unloadBlocked(),true);await page.locator('#entry-name').fill('Application');assert.equal(await unloadBlocked(),false);
  await page.locator('#cancel-entry').click();
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
  // Batch mode shares unsaved list edits and lazily loads its table dependency.
  assert.equal(await page.locator('script[src*=handsontable]').count(),0);
  await edit('Retry me','List pending');
  await page.locator('.menu summary').click();await page.locator('#open-batch').click();await page.waitForFunction(()=>window.FlareEditorBatch?.active);
  assert.match(await page.locator('#batch-table').textContent(),/List pending/);
  async function batchEdit(row,col,value){await page.locator('#batch-table .ht_master tbody tr').nth(row).locator('td').nth(col).tap();await page.locator('#batch-edit').click();const input=page.locator('#batch-table textarea.handsontableInput:visible');await input.fill(value);await input.press('Enter');}
  await batchEdit(0,0,'Batch pending');await page.locator('#leave-batch').click();assert.equal(await page.getByRole('button',{name:'Batch pending',exact:true}).count(),1);await save();
  // Incomplete rows block switching without losing the table buffer.
  await page.locator('.menu summary').click();await page.locator('#open-batch').click();await page.waitForFunction(()=>window.FlareEditorBatch?.active);await page.locator('#batch-add').click();await page.locator('#batch-table textarea.handsontableInput:visible').fill('Incomplete row');await page.locator('#batch-table textarea.handsontableInput:visible').press('Enter');await page.locator('#leave-batch').click();assert.equal(await page.locator('#batch-view').isVisible(),true);assert.match(await page.locator('#save-status').textContent(),/填写地址/);
  const last=(await page.locator('#batch-table .ht_master tbody tr').count())-1;await batchEdit(last,1,'http://nas:9999/path?a=b#c');await page.locator('#leave-batch').click();assert.equal(await page.getByRole('button',{name:'Incomplete row',exact:true}).count(),1);await save();
  // Active cell edits must be flushed by Save, not only by Enter.
  await page.locator('.menu summary').click();await page.locator('#open-batch').click();await page.waitForFunction(()=>window.FlareEditorBatch?.active);await page.locator('#batch-table .ht_master tbody tr').first().locator('td').first().tap();await page.locator('#batch-edit').click();await page.locator('#batch-table textarea.handsontableInput:visible').fill('Saved active cell');await save();await page.locator('#leave-batch').click();assert.equal(await page.getByRole('button',{name:'Saved active cell',exact:true}).count(),1);
  // Restoring a downloaded draft is explicit and keeps the loaded revision.
  const imported=await(await page.request.get(base+'/editor/document')).json();imported.apps[0].Name='Restored draft';
  page.once('dialog',d=>d.accept());await page.locator('#draft-file').setInputFiles({name:'draft.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify(imported))});await page.waitForFunction(()=>document.getElementById('save-status').textContent.includes('草稿已恢复'));assert.equal(await page.getByRole('button',{name:'Restored draft',exact:true}).count(),1);await save();
  // Handsontable clears cells to null; downloaded incomplete drafts must round-trip.
  await page.setViewportSize({width:1440,height:844});
  await page.locator('.menu summary').click();await page.locator('#open-batch').click();await page.waitForFunction(()=>window.FlareEditorBatch?.active);
  await batchEdit(0,4,'Pending draft description');
  const newer=await(await page.request.get(base+'/editor/document')).json();newer.apps[0].Name='Newer server value';assert.equal((await page.request.post(base+'/editor/document',{headers:{'X-Flare-Editor':'1'},data:newer})).status(),200);
  await page.locator('#save-changes').click();await page.waitForFunction(()=>document.getElementById('export-draft').hidden===false);
  for(const [row,col]of[[0,0],[1,1]]){await page.locator('#batch-table .ht_master tbody tr').nth(row).locator('td').nth(col).tap();await page.keyboard.press('Delete');}
  await page.locator('#batch-add').click();await page.locator('#batch-table textarea.handsontableInput:visible').fill('Pending draft row');await page.locator('#batch-table textarea.handsontableInput:visible').press('Enter');
  const clearedDownload=page.waitForEvent('download');await page.locator('#export-draft').click();const clearedFile=await clearedDownload,clearedBuffer=await fs.readFile(await clearedFile.path()),clearedDraft=JSON.parse(clearedBuffer);
  assert.equal(clearedDraft.apps[0].Name,null);assert.equal(clearedDraft.apps[1].URL,null);assert.equal(clearedDraft.apps[0].Desc,'Pending draft description');assert.equal(clearedDraft.apps.at(-1).Name,'Pending draft row');
  page.once('dialog',d=>d.accept());await page.locator('#discard-changes').click();await loaded();
  const loadedVersion=await page.evaluate(()=>window.FlareEditorUI.store.toJSON().version);
  page.once('dialog',d=>d.accept());await page.locator('#draft-file').setInputFiles({name:'cleared-draft.json',mimeType:'application/json',buffer:clearedBuffer});await page.waitForFunction(()=>document.getElementById('save-status').textContent.includes('草稿已恢复'));
  const normalizedDraft={...clearedDraft,version:loadedVersion,apps:clearedDraft.apps.map(item=>({...item,Name:item.Name===null?'':item.Name,URL:item.URL===null?'':item.URL}))};
  assert.deepEqual(await page.evaluate(()=>window.FlareEditorUI.store.toJSON()),normalizedDraft);
  await page.locator('.menu summary').click();await page.locator('#open-batch').click();await page.waitForFunction(()=>window.FlareEditorBatch?.active);await batchEdit(0,4,'Still pending after restore');
  const pendingDraft=await page.evaluate(()=>window.FlareEditorBatch.exportDraft());
  for(const [field,value]of[['Name',42],['URL',{}],['Name',undefined],['URL',false]]){const malformed=JSON.parse(JSON.stringify(normalizedDraft));malformed.apps[0][field]=value;await page.locator('#draft-file').setInputFiles({name:'malformed.json',mimeType:'application/json',buffer:Buffer.from(JSON.stringify(malformed))});await page.waitForFunction(()=>document.getElementById('draft-file').value==='');assert.match(await page.locator('#save-status').textContent(),/条目格式无效/);assert.deepEqual(await page.evaluate(()=>window.FlareEditorBatch.exportDraft()),pendingDraft);}
  await page.locator('#save-changes').click();assert.match(await page.locator('#save-status').textContent(),/填写名称/);await batchEdit(0,0,'Recovered cleared name');await page.locator('#save-changes').click();assert.match(await page.locator('#save-status').textContent(),/填写地址/);await batchEdit(1,1,'http://recovered-url');await batchEdit(normalizedDraft.apps.length-1,1,'http://pending-draft-row');await save();await page.locator('#leave-batch').click();
  const recovered=await(await page.request.get(base+'/editor/document')).json();assert.equal(recovered.apps[0].Name,'Recovered cleared name');assert.equal(recovered.apps[0].Desc,'Still pending after restore');assert.equal(recovered.apps[0].Private,true);assert.equal(recovered.apps[1].URL,'http://recovered-url');assert.equal(recovered.apps.at(-1).Name,'Pending draft row');assert.deepEqual(recovered.categories,clearedDraft.categories);assert.deepEqual(recovered.bookmarks,clearedDraft.bookmarks);
  await page.setViewportSize({width:390,height:844});
  // A legacy category literally named apps must remain distinct from application placement.
  const current=await(await page.request.get(base+'/editor/document')).json();current.categories.push({ID:'apps',Name:'Legacy apps category'});current.bookmarks.push({Name:'Legacy category bookmark',URL:'https://example.com/legacy',Category:'apps'});await page.request.post(base+'/editor/document',{headers:{'X-Flare-Editor':'1'},data:current});await loaded();await page.locator('#location-filter').selectOption('category:apps');assert.equal(await page.locator('.entry').count(),1);await edit('Legacy category bookmark','Legacy kept');await save();const checked=await(await page.request.get(base+'/editor/document')).json();assert.equal(checked.bookmarks.find(b=>b.Name==='Legacy kept').Category,'apps');await page.locator('#location-filter').selectOption('all');
  // Existing clipboard paste and visual row ordering survive the adapter.
  const beforeEmpty=await(await page.request.get(base+'/editor/document')).json();
  await context.grantPermissions(['clipboard-read','clipboard-write']);
  await page.locator('.menu summary').click();await page.locator('#open-batch').click();await page.waitForFunction(()=>window.FlareEditorBatch?.active);
  await page.locator('#batch-table .ht_master tbody tr').first().locator('td').first().tap();
  await page.evaluate(()=>navigator.clipboard.writeText('Pasted one\thttp://paste/1\nPasted two\thttp://paste/2'));await page.keyboard.press(process.platform==='darwin'?'Meta+V':'Control+V');
  await page.waitForFunction(()=>document.querySelector('#batch-table .ht_master tbody tr td').textContent==='Pasted one');
  await page.locator('#batch-table .ht_master tbody tr').first().locator('td').first().tap();await page.locator('#batch-down').click();await page.locator('#leave-batch').click();await save();
  const pasted=await(await page.request.get(base+'/editor/document')).json();assert.deepEqual(pasted.apps.slice(0,2).map(i=>i.Name),['Pasted two','Pasted one']);assert.equal(pasted.apps.find(i=>i.Name==='Pasted one').Private,true);
  await page.locator('.menu summary').click();await page.locator('#open-batch').click();await page.waitForFunction(()=>window.FlareEditorBatch?.active);
  while(await page.locator('#batch-table .ht_master tbody tr td').count()){await page.locator('#batch-table .ht_master tbody tr').first().locator('td').first().tap();page.once('dialog',d=>d.accept());await page.locator('#batch-delete').click();}
  await save();const empty=await(await page.request.get(base+'/editor/document')).json();assert.equal((empty.apps||[]).length+(empty.bookmarks||[]).length,0);
  await page.locator('#batch-add').click();await page.locator('#batch-table textarea.handsontableInput:visible').fill('After empty');await page.locator('#batch-table textarea.handsontableInput:visible').press('Enter');await batchEdit(0,1,'http://after-empty');await save();await page.locator('#leave-batch').click();
  const newVersion=(await(await page.request.get(base+'/editor/document')).json()).version;const restored=await page.request.post(base+'/editor/document',{headers:{'X-Flare-Editor':'1'},data:{...beforeEmpty,version:newVersion}});assert.equal(restored.status(),200);await loaded();
  // Measure, rather than prematurely adding pagination or virtualization.
  const performanceResult=await page.evaluate(()=>{const ui=window.FlareEditorUI,original=ui.store.toJSON();const start=performance.now();ui.store.accept({...original,bookmarks:Array.from({length:1000},(_,i)=>({Name:'Load '+i,URL:'http://nas/'+i,Category:'beta'}))});ui.render();const result={rows:document.querySelectorAll('.entry').length,milliseconds:Math.round(performance.now()-start)};ui.store.accept(original);ui.render();return result;});assert.ok(performanceResult.rows>=1000);console.log('1000-entry render',performanceResult);
  if(process.env.EDITOR_SCREENSHOT_DIR){await fs.mkdir(process.env.EDITOR_SCREENSHOT_DIR,{recursive:true});await page.screenshot({path:path.join(process.env.EDITOR_SCREENSHOT_DIR,'mobile-list.png'),fullPage:true});await page.locator('#new-entry').click();await page.screenshot({path:path.join(process.env.EDITOR_SCREENSHOT_DIR,'mobile-form.png')});await page.locator('#cancel-entry').click();await page.setViewportSize({width:1440,height:960});await page.screenshot({path:path.join(process.env.EDITOR_SCREENSHOT_DIR,'desktop-list.png'),fullPage:true});await page.locator('.menu summary').click();await page.locator('#open-batch').click();await page.waitForFunction(()=>window.FlareEditorBatch?.active);await page.screenshot({path:path.join(process.env.EDITOR_SCREENSHOT_DIR,'desktop-batch.png'),fullPage:true});await page.locator('#leave-batch').click();}
  assert.deepEqual(errors,[]);console.log('PASS: responsive list, real filtering, complete filtered saves, forms, pending form unload protection, stable IDs, category migration, undo, actual dirty state, stale-window conflict, draft export/restore, retry/reload, shared batch state, active-cell saves, incomplete rows and legacy category IDs');
 }finally{if(browser)await browser.close();child.kill();await fs.rm(dir,{recursive:true,force:true});}
})().catch(e=>{console.error(e);process.exitCode=1;});
