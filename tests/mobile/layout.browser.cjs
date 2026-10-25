// FLARE_BINARY=/path/to/binary node tests/mobile/layout.browser.cjs
const {chromium}=require(require.resolve('playwright',{paths:[__dirname+'/../clock']}));
const assert=require('node:assert/strict'),fs=require('node:fs/promises'),os=require('node:os'),path=require('node:path'),net=require('node:net'),{spawn}=require('node:child_process');
(async()=>{
 const dir=await fs.mkdtemp(path.join(os.tmpdir(),'flare-layout-'));const listener=net.createServer();await new Promise(r=>listener.listen(0,'127.0.0.1',r));const port=listener.address().port;await new Promise(r=>listener.close(r));
 await fs.writeFile(path.join(dir,'config.yml'),'Title: Layout\nShowTitle: true\nShowDateTime: true\nShowBookmarks: true\nGreetings: A very long greeting that must not overlap weather at tablet widths\nHomepage:\n  MobileBookmarkLayout: compact\n');
 await fs.writeFile(path.join(dir,'bookmarks.yml'),'categories:\n- id: a\n  title: First\n- id: b\n  title: Second\nlinks:\n'+Array.from({length:12},(_,i)=>`- name: Bookmark ${i}\n  link: https://example.com/${i}\n  category: ${i<10?'a':'b'}\n`).join(''));
 const child=spawn(process.env.FLARE_BINARY,[],{cwd:dir,env:{...process.env,FLARE_PORT:String(port),FLARE_DISABLE_LOGIN:'true',FLARE_OFFLINE:'true',FLARE_BASELINE:'1'}});let browser;
 try {const base=`http://127.0.0.1:${port}`;for(let i=0;i<100;i++){try{if((await fetch(base+'/ping')).ok)break;}catch{}await new Promise(r=>setTimeout(r,100));}
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});const page=await browser.newPage();
 for(const width of [320,359,360,390,767,768,769,1024]) {await page.setViewportSize({width,height:900});await page.goto(base);const result=await page.locator('.bookmark-groups').evaluate(e=>({columns:getComputedStyle(e).columnCount,scroll:document.documentElement.scrollWidth,client:document.documentElement.clientWidth}));assert.equal(result.columns,width>=360&&width<=768?'2':'auto');assert.ok(result.scroll<=result.client+1);assert.deepEqual(await page.locator('.bookmark-group-title').allTextContents(),['First','Second']);
 await page.locator('#hero-container').evaluate(el=>{const weather=document.createElement('div');weather.id='plugin-weather';weather.className='plugin-container clearfix';const icon=document.createElement('div');icon.className='pull-left';icon.textContent='☀';const values=document.createElement('div');values.className='pull-right weather-value-container';for(const value of ['35°C','100%']){const p=document.createElement('p');p.textContent=value;values.append(p);}weather.append(icon,values);el.appendChild(weather);});
 const boxes=await page.locator('#hero-container').evaluate(el=>{const a=el.querySelector('#plugin-datetime').getBoundingClientRect(),b=el.querySelector('#plugin-weather').getBoundingClientRect();return {a:{left:a.left,right:a.right,top:a.top,bottom:a.bottom},b:{left:b.left,right:b.right,top:b.top,bottom:b.bottom}};});
 assert.ok(boxes.a.right<=boxes.b.left+1||boxes.b.right<=boxes.a.left+1||boxes.a.bottom<=boxes.b.top+1||boxes.b.bottom<=boxes.a.top+1,'greeting/weather overlap at '+width);
}
 console.log('PASS: compact columns, narrow fallback, breakpoint boundaries and preserved DOM order');
 }finally{if(browser)await browser.close();child.kill();await fs.rm(dir,{recursive:true,force:true});}
})().catch(e=>{console.error(e);process.exitCode=1;});
