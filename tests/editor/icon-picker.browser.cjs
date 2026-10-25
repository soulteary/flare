// A mocked HTTP document isolates picker behavior from server persistence.
const {chromium}=require(require.resolve('playwright',{paths:[__dirname,`${__dirname}/../clock`]}));
const fs=require('node:fs/promises'),path=require('node:path'),assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});
 try{
  const page=await browser.newPage({viewport:{width:390,height:844},hasTouch:true,isMobile:true});
  let fail=false;const requests=[];
  await page.route('http://picker.test/**',async route=>{
   const url=new URL(route.request().url());
   if(url.pathname==='/')return route.fulfill({contentType:'text/html',body:await fs.readFile(path.join(__dirname,'../../embed/templates/editor.html'),'utf8')});
   if(url.pathname.startsWith('/assets/editor-ui/'))return route.fulfill({contentType:url.pathname.endsWith('.css')?'text/css':'application/javascript',body:await fs.readFile(path.join(__dirname,'../../internal/pages/editor/client',path.basename(url.pathname)),'utf8')});
   if(url.pathname==='/editor/document')return route.fulfill({contentType:'application/json',body:JSON.stringify({version:'one',categories:[{ID:'group',Name:'Group'}],apps:[{Name:'Existing',URL:'http://nas'}],bookmarks:[]})});
   if(url.pathname==='/editor/icons'){
    requests.push(url.searchParams.toString());
    if(fail)return route.fulfill({status:500,body:'failure'});
    const offset=Number(url.searchParams.get('offset')),q=url.searchParams.get('q');
    const names=q==='empty'?[]:q==='server'?['servernetwork']:offset?['zebra']:['abacus','home'];
    return route.fulfill({contentType:'application/json',body:JSON.stringify({names,total:q==='empty'?0:q==='server'?1:49,offset,limit:48})});
   }
   if(url.pathname==='/editor/icon')return route.fulfill({contentType:'image/svg+xml',body:'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M0 0h24v24H0z"/></svg>'});
   return route.fulfill({status:404,body:''});
  });
  await page.goto('http://picker.test/');await page.getByRole('button',{name:'Existing',exact:true}).click();
  await page.locator('#entry-name').fill('Pending name');await page.locator('#entry-url').fill('custom://nas/path?x=1#f');await page.locator('#entry-options').evaluate(e=>e.open=true);await page.locator('#entry-description').fill('Pending description');
  await page.locator('#choose-icon').click();await page.getByRole('button',{name:'选择图标 home',exact:true}).waitFor();
  await page.locator('#icon-picker-search').fill('server');await page.getByRole('button',{name:'选择图标 servernetwork',exact:true}).click();
  assert.equal(await page.locator('#entry-icon').inputValue(),'servernetwork');assert.equal(await page.locator('#entry-name').inputValue(),'Pending name');assert.equal(await page.locator('#entry-url').inputValue(),'custom://nas/path?x=1#f');assert.equal(await page.locator('#entry-description').inputValue(),'Pending description');
  await page.locator('#choose-icon').click();await page.getByRole('button',{name:'选择图标 home',exact:true}).waitFor();await page.locator('#icon-picker .icon-picker-pages button').nth(1).click();await page.getByRole('button',{name:'选择图标 zebra',exact:true}).waitFor();assert.ok(requests.some(q=>q.includes('offset=48')));
  await page.locator('#icon-picker-search').fill('empty');await page.waitForFunction(()=>document.getElementById('icon-picker-status').textContent.includes('没有找到'));
  await page.locator('#icon-picker .icon-picker-pages button').last().focus();await page.keyboard.press('Escape');await page.waitForFunction(()=>!document.getElementById('icon-picker').open);assert.equal(await page.locator('#entry-icon').inputValue(),'servernetwork');assert.equal(await page.locator('#entry-dialog').evaluate(e=>e.open),true);
  fail=true;await page.locator('#choose-icon').click();await page.getByRole('button',{name:'重试',exact:true}).waitFor();fail=false;await page.getByRole('button',{name:'重试',exact:true}).click();await page.getByRole('button',{name:'选择图标 home',exact:true}).waitFor();
  const size=await page.locator('#icon-picker').evaluate(e=>({s:e.scrollWidth,w:e.clientWidth}));assert.ok(size.s<=size.w+1);await page.keyboard.press('Escape');
  console.log('PASS: icon search, pagination, selection/cancel, preserved form fields, retry and mobile dialog width');
 }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
