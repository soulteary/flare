// Focused browser regressions against the production client, without a Go binary.
const {test}=require('node:test');
const assert=require('node:assert/strict');
const http=require('node:http');
const fs=require('node:fs/promises');
const path=require('node:path');
const {chromium}=require(require.resolve('playwright',{paths:[__dirname,path.join(__dirname,'../clock')]}));

test('mobile editor preserves focus and drafts when save outcome is unknown',async()=>{
 const root=path.resolve(__dirname,'../..');
 const {messages,render:renderEditor}=await require('./fixture.cjs').editorFixture(root);
 const doc={version:'initial',categories:[{ID:'one',Name:'First'},{ID:'two',Name:'Second'}],apps:[{Name:'Original',URL:'http://nas:8080',Category:''}],bookmarks:[]};
 const server=http.createServer(async(req,res)=>{
  if(req.url==='/editor/csrf'){res.setHeader('content-type','application/json');res.end(JSON.stringify({csrfToken:'fixture-token'}));return;}
  if(req.url==='/editor/document'){res.setHeader('content-type','application/json');res.end(JSON.stringify(doc));return;}
  const pathname=new URL(req.url,'http://flare.test').pathname;
  const file=pathname==='/editor'?path.join(root,'embed/templates/editor.html'):pathname.startsWith('/assets/editor-ui/')?path.join(root,'internal/pages/editor/client',path.basename(pathname)):null;
  if(!file){res.writeHead(404);res.end();return;}
  res.setHeader('content-type',file.endsWith('.js')?'application/javascript':file.endsWith('.css')?'text/css':'text/html');const content=await fs.readFile(file);res.end(pathname==='/editor'?renderEditor(content.toString('utf8')):content);
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 let browser;
 try{
  browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});
  const page=await browser.newPage({viewport:{width:320,height:640},isMobile:true,hasTouch:true});
  page.on('dialog',d=>d.accept());
  await page.goto(`http://127.0.0.1:${server.address().port}/editor`);
  await page.waitForFunction(()=>window.FlareEditorUI?.store);
  assert.equal(await page.locator('html').getAttribute('lang'),'zh-CN');
  assert.equal(await page.title(),messages['editor.manage_navigation_flare']);
  assert.equal(await page.locator('#new-entry').textContent(),messages['editor.new']);
  await page.locator('#new-entry').click();
  await page.locator('#entry-name').fill('Added');await page.locator('#entry-url').fill('http://new');await page.locator('#entry-location').selectOption('apps');
  await page.locator('#entry-form').getByRole('button',{name:'完成',exact:true}).click();
  assert.equal(await page.evaluate(()=>document.activeElement.textContent),'Added');
  await page.locator('.entry').filter({has:page.getByRole('button',{name:'Added',exact:true})}).getByRole('button',{name:'删除',exact:true}).click();
  assert.equal(await page.evaluate(()=>document.activeElement.id),'undo-delete');
  await page.locator('#undo-delete').click();assert.equal(await page.evaluate(()=>document.activeElement.textContent),'Added');
  await page.locator('#manage-categories').click();await page.locator('.category').first().getByRole('button',{name:'下移',exact:true}).click();
  assert.equal(await page.evaluate(()=>document.activeElement.value),'First');await page.locator('#close-categories').click();
  // A lost response may occur after a successful server write. Never call this
  // an ordinary failed save or discard the client's revision and pending data.
  await page.route('**/editor/document',route=>route.request().method()==='POST'?route.abort('failed'):route.continue());
  await page.locator('#save-changes').click();await page.waitForFunction(()=>document.getElementById('save-status').textContent.includes('保存结果待确认'));
  assert.equal(await page.locator('#export-draft').isVisible(),true);assert.equal(await page.locator('#save-changes').isEnabled(),true);
  assert.equal(await page.evaluate(()=>window.FlareEditorUI.store.toJSON().version),'initial');
  assert.equal(await page.getByRole('button',{name:'Added',exact:true}).count(),1);
  // Large status text and multiple recovery buttons must never cover the final
  // entry. Padding follows the rendered bar instead of a guessed fixed height.
  await page.evaluate(()=>document.documentElement.style.fontSize='32px');
  await page.waitForTimeout(100);
  const dimensions=await page.evaluate(()=>({padding:parseFloat(getComputedStyle(document.querySelector('.shell')).paddingBottom),bar:document.querySelector('.savebar').getBoundingClientRect().height,scroll:document.documentElement.scrollWidth,width:document.documentElement.clientWidth}));
  assert.ok(dimensions.padding>=dimensions.bar+23,JSON.stringify(dimensions));assert.ok(dimensions.scroll<=dimensions.width+1,JSON.stringify(dimensions));
  await page.unroute('**/editor/document');
  // Rejected writes and proxy error pages must not be reported as expired sessions.
  for(const response of [
   {status:403,contentType:'application/json',body:JSON.stringify({code:'write_source_rejected',error:'Origin rejected'})},
   {status:403,contentType:'application/json',body:JSON.stringify({error:{code:'SOURCE_REJECTED',message:'SOURCE_REJECTED'},request_id:'fixture-request'})},
   {status:503,contentType:'application/json',body:JSON.stringify({error:{code:'STORAGE_UNAVAILABLE',message:'STORAGE_UNAVAILABLE'},request_id:'fixture-storage'}),expected:'STORAGE_UNAVAILABLE'},
   {status:403,contentType:'text/plain',body:'Write request rejected. Use the Flare page on this origin and retry.'},
   {status:502,contentType:'text/html',body:'<h1>Bad gateway</h1>'},
  ]){
   await page.route('**/editor/document',route=>route.request().method()==='POST'?route.fulfill(response):route.continue());
   await page.locator('#save-changes').click();
   const expected=response.expected|| (response.contentType==='application/json'?'来源校验':`HTTP ${response.status}`);
   await page.waitForFunction(text=>document.getElementById('save-status').textContent.includes(text),expected);
   const message=await page.locator('#save-status').textContent();assert.doesNotMatch(message,/登录可能已过期/);
   if(response.contentType==='application/json'&&!response.expected)assert.match(message,/Host.*外部端口/);
   assert.equal(await page.evaluate(()=>window.FlareEditorUI.store.dirty),true);
   assert.equal(await page.evaluate(()=>window.FlareEditorUI.store.toJSON().version),'initial');
   assert.equal(await page.locator('#save-changes').isEnabled(),true);
   assert.equal(await page.getByRole('button',{name:'Added',exact:true}).count(),1);
   await page.unroute('**/editor/document');
  }
  // A genuine authentication redirect still receives the session guidance.
  await page.route('**/editor/document',route=>route.request().method()==='POST'?route.fulfill({status:302,headers:{location:'/editor'}}):route.continue());
  await page.locator('#save-changes').click();
  await page.waitForFunction(()=>document.getElementById('save-status').textContent.includes('登录可能已过期'));
  assert.equal(await page.evaluate(()=>window.FlareEditorUI.store.dirty),true);
  await page.unroute('**/editor/document');
  await page.route('**/editor/document',route=>route.request().method()==='POST'?route.fulfill({status:422,contentType:'application/json',body:JSON.stringify({saved:false,field:'apps.1.Name',error:'Server rejected name'})}):route.continue());
  await page.locator('#save-changes').click();await page.waitForFunction(()=>document.getElementById('entry-dialog').open);
  assert.equal(await page.evaluate(()=>document.activeElement.id),'entry-name');assert.match(await page.locator('#error-name').textContent(),/Server rejected name/);
  await page.locator('#cancel-entry').click();await page.unroute('**/editor/document');
  await page.route('**/editor/document',route=>route.request().method()==='POST'?route.fulfill({status:500,contentType:'application/json',body:JSON.stringify({saved:false,recoveryRequired:true,error:'Recovery incomplete'})}):route.continue());
  await page.locator('#save-changes').click();await page.waitForFunction(()=>document.getElementById('save-status').textContent.includes('Recovery incomplete'));
  assert.equal(await page.locator('#save-changes').isDisabled(),true);assert.equal(await page.locator('#export-draft').isVisible(),true);
  await page.locator('#new-entry').click();
  const dialog=await page.locator('#entry-dialog').evaluate(el=>({height:el.getBoundingClientRect().height,viewport:window.visualViewport.height}));assert.ok(dialog.height<=dialog.viewport);
 }finally{if(browser)await browser.close();await new Promise(resolve=>server.close(resolve));}
});
