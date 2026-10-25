// Measure the real list renderer without requiring a running Go service.
const {chromium}=require(require.resolve('playwright',{paths:[__dirname,`${__dirname}/../clock`]}));
const assert=require('node:assert/strict'), fs=require('node:fs/promises'), path=require('node:path');
(async()=>{
 const root=path.resolve(__dirname,'../..');
 const document={version:'measurement',categories:[{ID:'group',Name:'Group'}],apps:[],bookmarks:[]};
 const browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox','--disable-dev-shm-usage']});
 try{
  const page=await browser.newPage({viewport:{width:1440,height:1080}});
  await page.route('http://flare.test/**',async route=>{
   const url=new URL(route.request().url());
   if(url.pathname==='/editor/document')return route.fulfill({contentType:'application/json',body:JSON.stringify(document)});
   const file=url.pathname==='/editor'?path.join(root,'embed/templates/editor.html'):url.pathname.startsWith('/assets/editor-ui/')?path.join(root,'internal/pages/editor/client',path.basename(url.pathname)):null;
   if(!file)return route.fulfill({status:404,body:''});
   return route.fulfill({contentType:file.endsWith('.html')?'text/html':file.endsWith('.css')?'text/css':'application/javascript',body:await fs.readFile(file,'utf8')});
  });
  await page.goto('http://flare.test/editor');await page.waitForFunction(()=>window.FlareEditorUI?.store);
  const measurements=[];
  for(const count of [1000,5000,10000]){
   const result=await page.evaluate(count=>{
    const ui=window.FlareEditorUI;
    const start=performance.now();
    ui.store.accept({version:'measurement',categories:[{ID:'group',Name:'Group'}],apps:[],bookmarks:Array.from({length:count},(_,i)=>({Name:'Entry '+i,URL:'http://nas/'+i,Category:'group'}))});
    ui.render();const renderMs=performance.now()-start;
    const layoutStart=performance.now();document.getElementById("entry-list").getBoundingClientRect();const layoutMs=performance.now()-layoutStart;
    const search=document.getElementById('entry-search'),filterStart=performance.now();search.value='Entry 999';search.dispatchEvent(new Event('input'));
    const filtered=document.querySelectorAll('.entry').length,filterMs=performance.now()-filterStart;
    search.value='';search.dispatchEvent(new Event('input'));
    return {count,renderMs:Math.round(renderMs),layoutMs:Math.round(layoutMs),filterMs:Math.round(filterMs),filtered,visible:document.querySelectorAll('.entry').length,stored:ui.store.toJSON().bookmarks.length,dirty:ui.store.dirty};
   },count);
   assert.equal(result.visible,count);assert.equal(result.stored,count);assert.equal(result.dirty,false);assert.ok(result.filtered>0);
   // Programmatic focus enters the same native focus path as keyboard navigation.
   // Content visibility must reveal and scroll to an initially offscreen row.
   const lastName=page.locator('.entry').last().locator('.entry-name');
   await lastName.focus();
   assert.equal(await lastName.evaluate(el=>document.activeElement===el),true);
   const focused=await lastName.boundingBox();
   assert.ok(focused.y>=0 && focused.y+focused.height<=1080,JSON.stringify({count,focused}));
   await page.keyboard.press('Tab');
   assert.equal(await page.locator('.entry').last().getByRole('button',{name:'编辑',exact:true}).evaluate(el=>document.activeElement===el),true);
   await page.locator('#entry-search').fill('Entry '+(count-1));
   assert.equal(await page.locator('.entry').count(),1);
   assert.equal(await page.locator('.entry-name').textContent(),'Entry '+(count-1));
   await page.locator('#entry-search').fill('');
   assert.equal(await page.locator('.entry').count(),count);
   const preserved=await page.evaluate(()=>({stored:window.FlareEditorUI.store.toJSON().bookmarks.length,dirty:window.FlareEditorUI.store.dirty}));
   assert.deepEqual(preserved,{stored:count,dirty:false});
   measurements.push(result);
  }
  console.log(JSON.stringify({viewport:'1440x1080',measurements},null,2));
 }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
