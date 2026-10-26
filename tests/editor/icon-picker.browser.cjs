// A mocked HTTP document isolates picker behavior from server persistence.
const engines=require(require.resolve('playwright',{paths:[__dirname,`${__dirname}/../clock`]}));
const fs=require('node:fs/promises'),path=require('node:path'),assert=require('node:assert/strict');
(async()=>{
 const fixture=await require('./fixture.cjs').editorFixture();
 const browser=await engines[process.env.TEST_BROWSER||'chromium'].launch({headless:true,executablePath:process.env.BROWSER_EXECUTABLE||process.env.CHROMIUM_EXECUTABLE||undefined,args:process.env.TEST_BROWSER&&process.env.TEST_BROWSER!=='chromium'?[]:['--no-sandbox']});
 try{
  const page=await browser.newPage({viewport:{width:390,height:844},hasTouch:true,isMobile:process.env.TEST_BROWSER!=='firefox'});
  let fail=false,missingHome=false,iconColor='#FFFDEA';const requests=[],iconRequests=[];
  await page.route('http://picker.test/**',async route=>{
   const url=new URL(route.request().url());
   if(url.pathname==='/')return route.fulfill({contentType:'text/html',body:fixture.html});
   if(url.pathname.startsWith('/assets/editor-ui/'))return route.fulfill({contentType:url.pathname.endsWith('.css')?'text/css':'application/javascript',body:await fs.readFile(path.join(__dirname,'../../internal/pages/editor/client',path.basename(url.pathname)),'utf8')});
   if(url.pathname==='/color.svg')return route.fulfill({contentType:'image/svg+xml',body:'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="#e02080" d="M0 0h24v24H0z"/></svg>'});
   if(url.pathname==='/editor/document')return route.fulfill({contentType:'application/json',body:JSON.stringify({version:'one',categories:[{ID:'group',Name:'Group'}],apps:[{Name:'Existing',URL:'http://nas'}],bookmarks:[]})});
   if(url.pathname==='/editor/icons'){
    requests.push(url.searchParams.toString());
    if(fail)return route.fulfill({status:500,body:'failure'});
    const offset=Number(url.searchParams.get('offset')),q=url.searchParams.get('q');
    const names=q==='empty'?[]:q==='server'?['servernetwork']:offset?['zebra']:['abacus','home'];
    return route.fulfill({contentType:'application/json',body:JSON.stringify({names,total:q==='empty'?0:q==='server'?1:49,offset,limit:48})});
   }
   if(url.pathname==='/editor/icon' && url.searchParams.get('name')==='login')return route.fulfill({contentType:'text/html',body:'<html>Sign in</html>'});
   if(url.pathname==='/editor/icon' && url.searchParams.get('name')==='broken')return route.fulfill({contentType:'image/svg+xml',body:'invalid SVG'});
   if(url.pathname==='/editor/icon' && (url.searchParams.get('name')==='missing'||missingHome&&url.searchParams.get('name')==='home'))return route.fulfill({status:404,body:''});
   if(url.pathname==='/editor/icon'){iconRequests.push(url.searchParams.get('name'));return route.fulfill({contentType:'image/svg+xml',body:`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path fill="${iconColor}" d="M4 4h16v16H4z"/></svg>`});}
   return route.fulfill({status:404,body:''});
  });
  await page.goto('http://picker.test/');await page.getByRole('button',{name:'Existing',exact:true}).click();
  await page.locator('#entry-name').fill('Pending name');await page.locator('#entry-url').fill('custom://nas/path?x=1#f');await page.locator('#entry-options').evaluate(e=>e.open=true);await page.locator('#entry-description').fill('Pending description');
  await page.locator('#choose-icon').click();await page.getByRole('button',{name:'选择图标 home',exact:true}).waitFor();
  // Compare actual mask pixels with a solid reference in the editor text color.
  async function checkPixels(selector,scheme,color=null,transparent=true){
   await page.emulateMedia({colorScheme:scheme});
   const icon=page.locator(selector).first();await icon.waitFor({state:'visible'});
   await page.waitForFunction(selector=>document.querySelector(selector)?.dataset.loaded==='true'||document.querySelector(selector)?.naturalWidth>0,selector);
   await page.evaluate(color=>{const ref=document.createElement('div');ref.id='color-reference';ref.style.cssText='position:fixed;left:0;top:0;width:28px;height:28px;background:var(--text);z-index:99999';if(color)ref.style.background=color;[...document.querySelectorAll('dialog[open]')].at(-1)?.append(ref);},color);
   async function centerPixels(locator){const box=await locator.boundingBox();return page.screenshot({animations:'disabled',clip:{x:Math.ceil(box.x+box.width/2)-5,y:Math.ceil(box.y+box.height/2)-5,width:10,height:10}});}
   const actual=await centerPixels(icon);
   const expected=await centerPixels(page.locator('#color-reference'));
   assert.deepEqual(actual,expected,`${scheme} mask pixels must match editor text color`);
   if(transparent){
   const box=await icon.boundingBox();
   const corner=await page.screenshot({clip:{x:Math.ceil(box.x)+1,y:Math.ceil(box.y)+1,width:1,height:1}});
   const surface=await icon.evaluate(e=>getComputedStyle(e.closest('button')||e.closest('dialog')).backgroundColor);
   await page.locator('#color-reference').evaluate((e,color)=>e.style.background=color,surface);
   const ref=await page.locator('#color-reference').boundingBox();
   assert.deepEqual(corner,await page.screenshot({clip:{x:ref.x+1,y:ref.y+1,width:1,height:1}}),'transparent SVG edges must reveal the surface');
   }
   await page.locator('#color-reference').evaluate(e=>e.remove());
  }
  await checkPixels('#icon-picker .editor-builtin-icon','light');
  await checkPixels('#icon-picker .editor-builtin-icon','dark');
  assert.equal(iconRequests.filter(name=>name==='abacus').length,1,'one HTTP request per preview');
  await page.locator('#icon-picker-search').fill('server');await page.getByRole('button',{name:'选择图标 servernetwork',exact:true}).click();
  assert.equal(await page.locator('#entry-icon').inputValue(),'servernetwork');assert.equal(await page.locator('#entry-name').inputValue(),'Pending name');assert.equal(await page.locator('#entry-url').inputValue(),'custom://nas/path?x=1#f');assert.equal(await page.locator('#entry-description').inputValue(),'Pending description');
  await page.waitForFunction(()=>document.querySelector('#icon-preview .editor-builtin-icon')?.dataset.loaded==='true');
  for(const name of ['missing','login','broken']){
   await page.locator('#entry-icon').fill(name);await page.waitForFunction(()=>document.getElementById('icon-preview').textContent==='◇');
  }
  iconColor='#000000';await page.locator('#entry-icon').fill('home');
  await page.waitForFunction(()=>document.querySelector('#icon-preview .editor-builtin-icon')?.dataset.loaded==='true');
  await checkPixels('#icon-preview .editor-builtin-icon','dark');
  await checkPixels('#icon-preview .editor-builtin-icon','light');
  const oldImage=await page.locator('#icon-preview img').elementHandle();
  await page.locator('#entry-icon').fill('servernetwork');
  await oldImage.evaluate(e=>e.dispatchEvent(new Event('error')));await oldImage.dispose();
  await page.waitForFunction(()=>document.querySelector('#icon-preview .editor-builtin-icon')?.dataset.loaded==='true');
  await page.locator('#entry-icon').fill('http://picker.test/color.svg');
  assert.equal(await page.locator('#icon-preview > img').getAttribute('src'),'http://picker.test/color.svg');
  assert.equal(await page.locator('#icon-preview .editor-builtin-icon').count(),0);
  await page.waitForFunction(()=>document.querySelector('#icon-preview > img')?.naturalWidth>0);
  assert.equal(await page.locator('#icon-preview > img').evaluate(e=>getComputedStyle(e).filter),'none');
  await checkPixels('#icon-preview > img','light','#e02080',false);
  await checkPixels('#icon-preview > img','dark','#e02080',false);
  await page.locator('#entry-icon').fill('servernetwork');
  await page.locator('#choose-icon').click();await page.getByRole('button',{name:'选择图标 home',exact:true}).waitFor();await page.locator('#icon-picker .icon-picker-pages button').nth(1).click();await page.getByRole('button',{name:'选择图标 zebra',exact:true}).waitFor();assert.ok(requests.some(q=>q.includes('offset=48')));
  await page.locator('#icon-picker-search').fill('empty');await page.waitForFunction(()=>document.getElementById('icon-picker-status').textContent.includes('没有找到'));
  await page.locator('#icon-picker .icon-picker-pages button').last().focus();await page.keyboard.press('Escape');await page.waitForFunction(()=>!document.getElementById('icon-picker').open);assert.equal(await page.locator('#entry-icon').inputValue(),'servernetwork');assert.equal(await page.locator('#entry-dialog').evaluate(e=>e.open),true);
  fail=true;await page.locator('#choose-icon').click();await page.getByRole('button',{name:'重试',exact:true}).waitFor();fail=false;await page.getByRole('button',{name:'重试',exact:true}).click();await page.getByRole('button',{name:'选择图标 home',exact:true}).waitFor();
  await page.keyboard.press('Escape');missingHome=true;await page.locator('#choose-icon').click();
  await page.getByRole('button',{name:'选择图标 home',exact:true}).getByText('预览不可用').waitFor();
  const size=await page.locator('#icon-picker').evaluate(e=>({s:e.scrollWidth,w:e.clientWidth}));assert.ok(size.s<=size.w+1);await page.keyboard.press('Escape');
  console.log('PASS: light/dark mask pixels and transparency, one HTTP request per preview, form/picker failure fallback, external image preservation, icon search, pagination, selection/cancel, preserved form fields, retry and mobile dialog width');
 }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
