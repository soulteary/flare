// Verify production CSS column bounds without a service or external assets.
const {chromium}=require(require.resolve('playwright',{paths:[__dirname,`${__dirname}/../clock`]}));
const assert=require('node:assert/strict'),fs=require('node:fs/promises'),path=require('node:path');
(async()=>{
 const root=path.resolve(__dirname,'../..');
 const css=(await Promise.all(['base','home/apps','home/bookmarks'].map(name=>fs.readFile(path.join(root,'embed/assets/css',name+'.css'),'utf8')))).join('\n');
 const browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox','--disable-dev-shm-usage']});
 try{
  const page=await browser.newPage();
  for(const width of [769,1024,1440,1920])for(const choice of ['auto','4','5','6','8']){
   await page.setViewportSize({width,height:1080});
   await page.setContent(`<style>${css}</style><body class="desktop-columns-${choice}"><div id="page-home" class="pageview"><div class="container"><div id="container-apps"><div class="apps-container">${Array.from({length:16},(_,i)=>`<div class="app-container"><a class="app-item">App ${i}</a></div>`).join('')}</div></div><div id="container-bookmakrs"><div class="bookmark-groups"><nav class="bookmark-category-nav"><a>All</a></nav>${Array.from({length:16},(_,i)=>`<div class="bookmark-group-container"><h3>Group ${i}</h3><ul class="bookmark-list"><li><a class="bookmark">Long bookmark text</a></li></ul></div>`).join('')}</div></div></div></div></body>`);
   const result=await page.locator('.bookmark-groups').evaluate(el=>({contentWidth:el.clientWidth,columns:getComputedStyle(el).gridTemplateColumns.split(' ').length,grid:getComputedStyle(el).display,min:Math.min(...[...el.querySelectorAll('.bookmark-group-container')].map(x=>x.getBoundingClientRect().width)),overflow:document.documentElement.scrollWidth>innerWidth}));
   assert.equal(await page.locator('.apps-container').evaluate(el=>getComputedStyle(el).display),'grid');
   const appColumns=await page.locator('.apps-container').evaluate(el=>getComputedStyle(el).gridTemplateColumns.split(' ').length);
   if(choice!=='auto')assert.ok(appColumns<=Number(choice));
   assert.equal(result.grid,'grid');assert.equal(result.overflow,false,JSON.stringify({width,choice,result}));assert.ok(result.min>=179,JSON.stringify({width,choice,result}));
   if(choice!=='auto') { assert.ok(result.columns<=Number(choice),JSON.stringify({width,choice,result})); assert.equal(result.columns,Math.min(Number(choice),Math.floor((result.contentWidth+16)/196)),JSON.stringify({width,choice,result})); }
   assert.equal(await page.locator('.bookmark-category-nav').evaluate(el=>getComputedStyle(el).gridColumn),'1 / -1');
  }
  await page.setViewportSize({width:390,height:844});
  assert.equal(await page.locator('.bookmark-groups').evaluate(el=>getComputedStyle(el).display),'block');
  console.log('PASS: desktop choices respect column bounds/readability and retain mobile layout');
 }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
