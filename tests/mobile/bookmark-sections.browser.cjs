// Exercise generated production resources through a real Flare server.
const {chromium}=require(require.resolve('playwright',{paths:[__dirname+'/../clock']}));
const assert=require('node:assert/strict'),fs=require('node:fs/promises'),os=require('node:os'),path=require('node:path'),net=require('node:net'),{spawn}=require('node:child_process');
(async()=>{
 const dir=await fs.mkdtemp(path.join(os.tmpdir(),'flare-sections-'));
 const listener=net.createServer();await new Promise(r=>listener.listen(0,'127.0.0.1',r));const port=listener.address().port;await new Promise(r=>listener.close(r));
 await fs.writeFile(path.join(dir,'config.yml'),'ShowBookmarks: true\nShowApps: false\nHomepage:\n  BookmarkLayout: sections\n  DesktopColumns: "6"\n  CategoryNavigation: tabs\n  MobileBookmarkLayout: compact\n');
 await fs.writeFile(path.join(dir,'apps.yml'),'links: []\n');
 const counts=[100,3,8,2,2,2,2,2,2,2];
 await fs.writeFile(path.join(dir,'bookmarks.yml'),'categories:\n'+counts.map((_,i)=>`- id: c${i}\n  title: Category ${i}\n`).join('')+'links:\n'+counts.map((n,i)=>Array.from({length:n},(_,j)=>`- name: Bookmark-${i}-${j}\n  link: https://example.com/${i}/${j}\n  category: c${i}\n`).join('')).join(''));
 const start=()=>spawn(process.env.FLARE_BINARY,[],{cwd:dir,env:{...process.env,FLARE_PORT:String(port),FLARE_DISABLE_LOGIN:'true',FLARE_OFFLINE:'true',FLARE_BASELINE:'1'}});let child=start(),browser;
 try {
 const base=`http://127.0.0.1:${port}`;let ready=false;
 for(let i=0;i<100;i++){try{if((await fetch(base+'/ping')).ok){ready=true;break;}}catch{}await new Promise(r=>setTimeout(r,100));}assert.ok(ready,'server failed to start');
 browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});const page=await browser.newPage();
 for(const route of ['/','/bookmarks'])for(const width of [320,390,599,600,768,769,1024,1920]){
 await page.setViewportSize({width,height:1080});await page.goto(base+route);
 const result=await page.locator('.bookmark-groups').evaluate(el=>{
 const groups=[...el.querySelectorAll('.bookmark-group-container')],items=[...groups[0].querySelectorAll('li')],boxes=items.map(x=>x.getBoundingClientRect());
 return {display:getComputedStyle(el).display,phoneColumns:getComputedStyle(el).columnCount,columns:boxes.filter(x=>Math.abs(x.top-boxes[0].top)<1).length,firstBottom:groups[0].getBoundingClientRect().bottom,secondTop:groups[1].getBoundingClientRect().top,overflow:document.documentElement.scrollWidth>innerWidth,order:items.map(x=>x.textContent.trim())};});
 assert.equal(result.overflow,false,JSON.stringify({width,result}));assert.equal(await page.locator('a.bookmark').count(),counts.reduce((a,b)=>a+b,0));
 assert.deepEqual(result.order,Array.from({length:100},(_,j)=>`Bookmark-0-${j}`));
 if(width>=600){assert.equal(result.display,'block');assert.ok(result.columns>1);assert.ok(result.columns<=6);assert.ok(result.secondTop>=result.firstBottom);if(width===1920)assert.equal(result.columns,6);}
 else assert.equal(result.phoneColumns,width>=360?'2':'auto');
 }

 await page.setViewportSize({width:2560,height:1080});await page.goto(base+'/bookmarks');
 // Exercise all column rules from the generated production stylesheet.
 for(const choice of ['', 'auto', '4', '5', '6', '8']) {
 await page.locator('body').evaluate((el,choice)=>{for(const c of [...el.classList])if(c.startsWith('desktop-columns-'))el.classList.remove(c);if(choice)el.classList.add('desktop-columns-'+choice);},choice);
 const columns=await page.locator('.bookmark-list').first().evaluate(el=>getComputedStyle(el).gridTemplateColumns.split(' ').length);
 if(choice!=='auto')assert.equal(columns,Number(choice||4));else assert.ok(columns>=8);
 }
 for(const size of ['100%','200%']) {
 await page.locator('html').evaluate((el,size)=>{el.style.fontSize=size;},size);
 await page.locator('.bookmark-groups').evaluate(el=>{el.style.width='250px';el.querySelector('a.bookmark span').textContent='LongBookmarkWithoutSpaces长书签名称'.repeat(10);});
 const narrow=await page.locator('.bookmark-list').first().evaluate(el=>({columns:getComputedStyle(el).gridTemplateColumns.split(' ').length,width:el.getBoundingClientRect().width,scroll:document.documentElement.scrollWidth,client:document.documentElement.clientWidth}));
 assert.equal(narrow.columns,1);assert.ok(narrow.width<=250);assert.ok(narrow.scroll<=narrow.client);
 }
 await page.locator('html').evaluate(el=>{el.style.fontSize='';});
 await page.setViewportSize({width:1920,height:1080});await page.goto(base+'/bookmarks?category=c0');assert.equal(await page.locator('a.bookmark').count(),100);
 await page.goto(base+'/bookmarks?category=c1&search=Bookmark-0-99');assert.equal(await page.locator('a.bookmark').count(),0);
 await page.goto(base+'/bookmarks?search=Bookmark-0-99');assert.equal(await page.locator('a.bookmark').count(),1);
 if(process.env.FLARE_SCREENSHOT){await page.goto(base+'/bookmarks');await page.screenshot({path:process.env.FLARE_SCREENSHOT,fullPage:true});}
 await page.goto(base+'/settings/appearance');assert.equal(await page.locator('#bookmark-layout').inputValue(),'sections');
 // Change navigation using the real settings form, preserving unrelated fields.
 await page.locator('#category-navigation').selectOption('collapse');await Promise.all([page.waitForNavigation(),page.locator('#bookmark-layout').evaluate(el=>el.form.requestSubmit())]);
 await page.goto(base+'/bookmarks');const details=page.locator('details').first();assert.equal(await details.getAttribute('open'),null);assert.equal(await details.locator('ul').isVisible(),false);await details.locator('summary').click();assert.equal(await details.locator('ul').isVisible(),true);
 await page.goto(base+'/bookmarks?search=Bookmark-0-99');assert.equal(await page.locator('a.bookmark').count(),1);assert.equal(await page.locator('a.bookmark').isVisible(),true);
 await fs.writeFile(path.join(dir,'bookmarks.yml'),'links:\n'+Array.from({length:12},(_,j)=>`- name: Ungrouped ${j}\n  link: https://example.com/${j}\n`).join(''));
 await new Promise(resolve=>{child.once('exit',resolve);child.kill();});child=start();
 ready=false;for(let i=0;i<100;i++){try{if((await fetch(base+'/ping')).ok){ready=true;break;}}catch{}await new Promise(r=>setTimeout(r,100));}assert.ok(ready,'restarted server failed to start');
 await page.goto(base+'/bookmarks');assert.equal(await page.locator('a.bookmark').count(),12);assert.equal(await page.locator('.bookmark-group-container').count(),1);assert.equal(await page.locator('.bookmark-list').evaluate(el=>getComputedStyle(el).display),'grid');
 console.log('PASS: generated resources, sections, six columns, phone modes, order, search, category selection and collapse');
 }finally{if(browser)await browser.close();child.kill();await fs.rm(dir,{recursive:true,force:true});}
})().catch(e=>{console.error(e);process.exitCode=1;});
