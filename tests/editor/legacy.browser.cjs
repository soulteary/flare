// Compatibility acceptance for valid legacy YAML configurations.
const {chromium}=require(require.resolve('playwright',{paths:[__dirname,`${__dirname}/../clock`]}));
const assert=require('node:assert/strict'),fs=require('node:fs/promises'),path=require('node:path'),net=require('node:net'),{spawn}=require('node:child_process');
(async()=>{
 const dir=await fs.mkdtemp('/tmp/flare-editor-legacy-'),socket=net.createServer();await new Promise(r=>socket.listen(0,'127.0.0.1',r));const port=socket.address().port;await new Promise(r=>socket.close(r));const base=`http://127.0.0.1:${port}`;
 await fs.writeFile(path.join(dir,'apps.yml'),'links:\n- name: Application\n  link: http://nas:8080\n');
 await fs.writeFile(path.join(dir,'bookmarks.yml'),'categories:\n- id: apps\n  title: Legacy named group\nlinks:\n- name: Explicit bookmark\n  link: custom://nas/explicit\n  category: apps\n- name: Implicit bookmark\n  link: http://nas/default?q=1#f\n  private: true\n');
 const child=spawn(process.env.FLARE_BINARY,[],{cwd:dir,env:{...process.env,FLARE_PORT:String(port),FLARE_DISABLE_LOGIN:'true',FLARE_OFFLINE:'true',FLARE_BASELINE:'1'}});let log='';child.stdout.on('data',b=>log+=b);child.stderr.on('data',b=>log+=b);let browser;
 try{
  for(let i=0;i<100;i++){if(child.exitCode!==null)throw Error(log);try{if((await fetch(base+'/ping')).ok)break}catch{}await new Promise(r=>setTimeout(r,100));}
  browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox','--disable-dev-shm-usage','--disable-gpu']});
  const page=await browser.newPage({viewport:{width:390,height:844},hasTouch:true,isMobile:true});page.setDefaultTimeout(8000);const errors=[];page.on('pageerror',e=>errors.push(e.message));
  const document=async()=>await(await page.request.get(base+'/editor/document')).json();
  async function load(){await page.goto(base+'/editor');await page.waitForFunction(()=>window.FlareEditorUI?.store);}
  async function save(){await page.locator('#save-changes').click();await page.waitForFunction(()=>window.document.getElementById('save-status').textContent==='所有更改已保存');}
  async function edit(name,newName,location){await page.getByRole('button',{name,exact:true}).click();assert.equal(await page.locator('#entry-location').inputValue(),location);await page.locator('#entry-name').fill(newName);await page.locator('#entry-form').getByRole('button',{name:'完成',exact:true}).click();await save();}
  await load();await page.locator('#location-filter').selectOption('category:apps');assert.equal(await page.locator('.entry').count(),2);
  await edit('Explicit bookmark','Explicit renamed','category:apps');let doc=await document();assert.equal(doc.apps.length,1);assert.equal(doc.bookmarks.find(b=>b.Name==='Explicit renamed').Category,'apps');
  await page.locator('#new-entry').click();assert.equal(await page.locator('#entry-location').inputValue(),'category:apps');await page.locator('#entry-name').fill('Added to legacy group');await page.locator('#entry-url').fill('http://nas/new');await page.locator('#entry-form').getByRole('button',{name:'完成',exact:true}).click();await save();doc=await document();assert.equal(doc.apps.length,1);assert.equal(doc.bookmarks.find(b=>b.Name==='Added to legacy group').Category,'apps');
  await page.locator('#location-filter').selectOption('category:');await edit('Implicit bookmark','Implicit renamed','category:');doc=await document();const implicit=doc.bookmarks.find(b=>b.Name==='Implicit renamed');assert.equal(implicit.Category,'');assert.equal(implicit.Private,true);
  // Removing all named groups remains a valid legacy document when its links
  // are implicit. An application-only edit must not force reclassification.
  const response=await page.request.post(base+'/editor/document',{headers:{'X-Flare-Editor':'1'},data:{...doc,categories:[],bookmarks:[implicit]}});assert.equal(response.status(),200);
  await load();await edit('Application','Application changed','apps');await edit('Implicit renamed','Still uncategorized','category:');doc=await document();assert.equal(doc.categories?.length||0,0);assert.equal(doc.apps.length,1);assert.equal(doc.bookmarks.length,1);assert.equal(doc.bookmarks[0].Category,'');assert.equal(doc.bookmarks[0].Private,true);
  await page.locator('#location-filter').selectOption('category:');await page.locator('#new-entry').click();assert.equal(await page.locator('#entry-location').inputValue(),'category:');await page.locator('#entry-name').fill('New uncategorized');await page.locator('#entry-url').fill('custom://nas/new?q=1#x');await page.locator('#entry-form').getByRole('button',{name:'完成',exact:true}).click();await save();
  doc=await document();assert.equal(doc.apps.length,1);assert.equal(doc.bookmarks.length,2);assert.ok(doc.bookmarks.every(b=>b.Category===''));
  if(await page.evaluate(()=>!!window.FlareEditorBatch)){
    await page.locator('.menu summary').click();await page.locator('#open-batch').click();await page.waitForFunction(()=>window.FlareEditorBatch.active);
    await page.locator('#batch-table .ht_master tbody tr').nth(1).locator('td').first().tap();await page.locator('#batch-edit').click();const input=page.locator('#batch-table textarea.handsontableInput:visible');await input.fill('Bulk uncategorized');await input.press('Enter');await save();await page.locator('#leave-batch').click();
    doc=await document();assert.equal(doc.apps.length,1);assert.equal(doc.bookmarks.find(b=>b.Name==='Bulk uncategorized').Category,'');
  }
  // Nonempty missing IDs are still rejected; compatibility is not a blanket bypass.
  doc.bookmarks[0].Category='missing';const invalid=await page.request.post(base+'/editor/document',{headers:{'X-Flare-Editor':'1'},data:doc});assert.equal(invalid.status(),422);
  const yaml=await fs.readFile(path.join(dir,'bookmarks.yml'),'utf8');assert.equal(/\n\s+category:/.test(yaml),false);
  assert.deepEqual(errors,[]);console.log('PASS: category ID apps, implicit first-group filtering, unclassified links, no groups, app-only edits, add/edit preservation and unknown-ID rejection');
 }finally{if(browser)await browser.close();child.kill();await fs.rm(dir,{recursive:true,force:true});}
})().catch(e=>{console.error(e);process.exitCode=1;});
