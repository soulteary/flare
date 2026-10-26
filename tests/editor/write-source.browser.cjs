// Issue #195: exercise the real router, browser headers and persisted files through a proxy.
const {chromium}=require(require.resolve('playwright',{paths:[__dirname,`${__dirname}/../clock`]}));
const assert=require('node:assert/strict'),http=require('node:http'),fs=require('node:fs/promises'),path=require('node:path'),{spawn}=require('node:child_process');
(async()=>{
 const dir=await fs.mkdtemp('/tmp/flare-write-source-');
 const reserve=http.createServer();await new Promise(resolve=>reserve.listen(0,'127.0.0.1',resolve));
 const port=reserve.address().port;await new Promise(resolve=>reserve.close(resolve));
 await fs.writeFile(path.join(dir,'config.yml'),'Locale: en\nShowWeather: false\n');
 await fs.writeFile(path.join(dir,'apps.yml'),'links:\n- name: Original\n  link: https://example.com/original\n');
 await fs.writeFile(path.join(dir,'bookmarks.yml'),'links: []\n');
 const child=spawn(process.env.FLARE_BINARY,[],{cwd:dir,env:{...process.env,FLARE_PORT:String(port),FLARE_DISABLE_LOGIN:'true',FLARE_OFFLINE:'true',FLARE_BASELINE:'1'}});
 let log='',mode='preserve',browser;child.stdout.on('data',data=>log+=data);child.stderr.on('data',data=>log+=data);
 const proxy=http.createServer((request,response)=>{
  const headers={...request.headers};
  if(mode==='forwarded-only')headers['x-forwarded-host']=headers.host;
  if(mode==='rewrite'||mode==='forwarded-only')headers.host='flare:5005';
  if(mode==='drop-port')headers.host='127.0.0.1';
  const upstream=http.request({hostname:'127.0.0.1',port,path:request.url,method:request.method,headers},result=>{
   response.writeHead(result.statusCode,result.headers);result.pipe(response);
  });
  upstream.on('error',error=>{response.writeHead(502);response.end(error.message);});request.pipe(upstream);
 });
 try{
  const direct=`http://127.0.0.1:${port}`;let ready=false;
  for(let i=0;i<100;i++){
   if(child.exitCode!==null)throw Error(log);
   try{if((await fetch(direct+'/ping')).ok){ready=true;break;}}catch{}
   await new Promise(resolve=>setTimeout(resolve,100));
  }
  assert.ok(ready,log);await new Promise(resolve=>proxy.listen(0,'127.0.0.1',resolve));
  const proxied=`http://127.0.0.1:${proxy.address().port}`;
  browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox','--disable-dev-shm-usage']});
  const page=await browser.newPage();page.setDefaultTimeout(10000);page.on('dialog',dialog=>dialog.accept());
  for(const test of [
   {name:'direct',base:direct,mode:'preserve',status:200},
   {name:'proxy-preserve',base:proxied,mode:'preserve',status:200},
   {name:'proxy-rewrite',base:proxied,mode:'rewrite',status:403},
   {name:'proxy-drop-port',base:proxied,mode:'drop-port',status:403},
   {name:'proxy-forwarded-only',base:proxied,mode:'forwarded-only',status:403},
  ]){
   mode=test.mode;const name=`Issue195-${test.name}`;
   await page.goto(test.base+'/settings/appearance');await page.locator('#settings-title').fill(name);
   const [settings]=await Promise.all([page.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith('/settings/appearance')),page.locator('input.btn-submit').click()]);
   assert.equal(settings.status(),test.status);
   assert.equal((await fs.readFile(path.join(dir,'config.yml'),'utf8')).includes(name),test.status===200);
   if(test.status===403){const result=await settings.json();assert.equal(result.code,'write_source_rejected');assert.match(result.error,/Host header including the public port/);}
   await page.goto(test.base+'/editor');await page.waitForFunction(()=>window.FlareEditorUI?.store);
   await page.locator('#new-entry').click();await page.locator('#entry-name').fill(name);await page.locator('#entry-url').fill('https://example.com/'+test.name);
   await page.locator('#entry-location').selectOption('apps');await page.locator('#entry-form .dialog-actions button.primary').click();
   const [saved]=await Promise.all([page.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith('/editor/document')),page.locator('#save-changes').click()]);
   assert.equal(saved.status(),test.status);
   await page.waitForFunction(()=>document.querySelector('#workspace').getAttribute('aria-busy')==='false');
   const message=await page.locator('#save-status').textContent();
   if(test.status===200)assert.equal(message,'All changes saved');
   else{
    assert.match(message,/origin protection.*Host header.*public port/);assert.doesNotMatch(message,/session.*expired/i);
    assert.equal((await saved.json()).code,'write_source_rejected');
    assert.equal((await fs.readFile(path.join(dir,'apps.yml'),'utf8')).includes(name),false);
    assert.equal(await page.evaluate(()=>window.FlareEditorUI.store.dirty),true);
    assert.equal(await page.getByRole('button',{name,exact:true}).count(),1);
    // Correcting the proxy allows the same pending draft to be saved without reloading.
    mode='preserve';await page.locator('#save-changes').click();
    await page.waitForFunction(()=>!window.FlareEditorUI.store.dirty);
    assert.equal(await page.locator('#save-status').textContent(),'All changes saved');
   }
   assert.equal((await fs.readFile(path.join(dir,'apps.yml'),'utf8')).includes(name),true);
   console.log(`PASS: ${test.name}, settings/editor HTTP ${test.status}, draft retry persisted`);
  }
 }finally{
  await browser?.close();if(proxy.listening)await new Promise(resolve=>proxy.close(resolve));
  const exited=child.exitCode!==null?Promise.resolve():new Promise(resolve=>child.once('exit',resolve));child.kill('SIGTERM');await exited;
  await fs.rm(dir,{recursive:true,force:true});
 }
})().catch(error=>{console.error(error);process.exitCode=1;});
