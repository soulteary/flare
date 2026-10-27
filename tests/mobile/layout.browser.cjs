// FLARE_BINARY=/path/to/binary node tests/mobile/layout.browser.cjs
const {chromium}=require(require.resolve('playwright',{paths:[__dirname+'/../clock']}));
const assert=require('node:assert/strict'),fs=require('node:fs/promises'),os=require('node:os'),path=require('node:path'),net=require('node:net'),{spawn}=require('node:child_process');
(async()=>{
 const dir=await fs.mkdtemp(path.join(os.tmpdir(),'flare-layout-'));const listener=net.createServer();await new Promise(r=>listener.listen(0,'127.0.0.1',r));const port=listener.address().port;await new Promise(r=>listener.close(r));
 await fs.writeFile(path.join(dir,'config.yml'),'Title: Layout\nShowTitle: true\nShowDateTime: true\nShowBookmarks: true\nShowApps: true\nGreetings: A very long greeting that must not overlap weather at tablet widths\nHomepage:\n  MobileBookmarkLayout: compact\n');
 await fs.writeFile(path.join(dir,'apps.yml'),'links:\n'+Array.from({length:8},(_,i)=>`- name: App ${i}\n  link: https://example.com/${i}\n  icon: home\n  desc: HTTPS://LINK.EXAMPLE.COM/LongAddressWithoutSpaces\n`).join(''));
 await fs.writeFile(path.join(dir,'bookmarks.yml'),'categories:\n- id: a\n  title: First\n- id: b\n  title: Second\nlinks:\n'+Array.from({length:12},(_,i)=>`- name: Bookmark ${i}\n  link: https://example.com/${i}\n  category: ${i<10?'a':'b'}\n`).join(''));
 const child=spawn(process.env.FLARE_BINARY,[],{cwd:dir,env:{...process.env,FLARE_PORT:String(port),FLARE_DISABLE_LOGIN:'true',FLARE_OFFLINE:'true',FLARE_BASELINE:'1'}});let browser;
 try {const base=`http://127.0.0.1:${port}`;for(let i=0;i<100;i++){try{if((await fetch(base+'/ping')).ok)break;}catch{}await new Promise(r=>setTimeout(r,100));}
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});const page=await browser.newPage();
 for(const width of [320,359,360,390,599,600,744,767,768,769,820,1024,1025]) {await page.setViewportSize({width,height:900});await page.goto(base);const result=await page.locator('.bookmark-groups').evaluate(e=>({columns:getComputedStyle(e).columnCount,scroll:document.documentElement.scrollWidth,client:document.documentElement.clientWidth}));assert.equal(result.columns,width>=360&&width<=599?'2':'auto');assert.ok(result.scroll<=result.client+1);assert.deepEqual(await page.locator('.bookmark-group-title').allTextContents(),['First','Second']);

 await page.addStyleTag({content: "a.bookmark, .app-item { transition: none !important; }"});
 // Compare the same production document with only its opt-in class changed.
 const measureDensity=()=>page.locator('#container-bookmarks').evaluate(el=>({apps:document.querySelector('.apps-container').getBoundingClientRect().height,height:el.getBoundingClientRect().height,row:el.querySelector('a.bookmark').getBoundingClientRect().height}));
 const compact=await measureDensity();
 if(width<=1024){
  const descriptions=await page.locator('.app-desc').evaluateAll(els=>els.map(el=>({height:el.getBoundingClientRect().height,line:parseFloat(getComputedStyle(el).lineHeight),overflow:getComputedStyle(el).textOverflow})));
  for(const desc of descriptions){assert.ok(desc.height<=desc.line+1,'description must stay on one line');assert.equal(desc.overflow,'ellipsis');}
  assert.equal(await page.locator('.app-icon svg, .app-icon img').count(),8,'real icons must render');
  const cards=await page.locator('.app-item').evaluateAll(els=>els.map(el=>{const icon=el.querySelector('.app-icon').getBoundingClientRect(),text=el.querySelector('.app-text').getBoundingClientRect();return {iconRight:icon.right,textLeft:text.left};}));
  for(const card of cards)assert.ok(card.textLeft>=card.iconRight+7,'icons and text need a consistent gutter');
 }
 if(width<=1024){
  const toolbar=await page.locator('.toolbar-container').boundingBox();
  const content=await page.locator('#container-bookmarks').boundingBox();
  assert.ok(toolbar.y>=content.y+content.height,'utility buttons must not cover bookmarks');
 }
 if(process.env.MOBILE_SCREENSHOT_DIR&&[390,744].includes(width)){
  await fs.mkdir(process.env.MOBILE_SCREENSHOT_DIR,{recursive:true});
  await page.screenshot({path:path.join(process.env.MOBILE_SCREENSHOT_DIR,`compact-${width}.png`),fullPage:true});
 }

 await page.locator('body').evaluate(el=>el.classList.remove('mobile-bookmarks-compact'));
 const standard=await measureDensity();
 if(width<=1024){assert.ok(compact.apps<standard.apps,JSON.stringify({width,compact,standard}));assert.ok(compact.row<=standard.row,JSON.stringify({width,compact,standard}));assert.ok(compact.height<standard.height,JSON.stringify({width,compact,standard}));}
 else assert.deepEqual(compact,standard,'desktop density unchanged');
 await page.locator('body').evaluate(el=>el.classList.add('mobile-bookmarks-compact'));
 for(const fontSize of ['125%','200%']){
  await page.locator('html').evaluate((el,size)=>el.style.fontSize=size,fontSize);
  const overflow=await page.evaluate(()=>document.documentElement.scrollWidth>innerWidth);
  assert.equal(overflow,false,'compact enlarged text at '+width);
  const boxes=await page.locator('a.bookmark').evaluateAll(links=>links.map(link=>{const b=link.getBoundingClientRect();return {top:b.top,bottom:b.bottom,left:b.left,right:b.right};}));
  for(let i=1;i<boxes.length;i++){const a=boxes[i-1],b=boxes[i];assert.ok(a.bottom<=b.top+1||a.right<=b.left+1||b.right<=a.left+1,'compact links overlap at '+width);}
 }
 await page.locator('html').evaluate(el=>el.style.fontSize='');
 await page.locator('#hero-container').evaluate(el=>{const weather=document.createElement('div');weather.id='plugin-weather';weather.className='plugin-container clearfix';const icon=document.createElement('div');icon.className='pull-left';icon.textContent='☀';const values=document.createElement('div');values.className='pull-right weather-value-container';for(const value of ['35°C','100%']){const p=document.createElement('p');p.textContent=value;values.append(p);}weather.append(icon,values);el.appendChild(weather);});
 const boxes=await page.locator('#hero-container').evaluate(el=>{const a=el.querySelector('#plugin-datetime').getBoundingClientRect(),b=el.querySelector('#plugin-weather').getBoundingClientRect();return {a:{left:a.left,right:a.right,top:a.top,bottom:a.bottom},b:{left:b.left,right:b.right,top:b.top,bottom:b.bottom}};});
 assert.ok(boxes.a.right<=boxes.b.left+1||boxes.b.right<=boxes.a.left+1||boxes.a.bottom<=boxes.b.top+1||boxes.b.bottom<=boxes.a.top+1,'greeting/weather overlap at '+width);
}
 console.log('PASS: compact phone/tablet density, default/desktop preservation, enlarged text, narrow fallback and category order');
 }finally{if(browser)await browser.close();child.kill();await fs.rm(dir,{recursive:true,force:true});}
})().catch(e=>{console.error(e);process.exitCode=1;});
