// Actual native HTML form submit through containerized Nginx TLS, with JavaScript disabled.
const {chromium}=require(process.env.FLARE_PLAYWRIGHT_MODULE||'../clock/node_modules/playwright');
const assert=require('node:assert/strict');
(async()=>{
 const browser=await chromium.launch({headless:true,...(process.env.FLARE_CHROMIUM_EXECUTABLE?{executablePath:process.env.FLARE_CHROMIUM_EXECUTABLE}:{})});
 const result={engine:'chromium',version:browser.version(),javascript:false,cases:[]};
 try{
  for(const base of [process.env.FLARE_NGINX_CANONICAL,process.env.FLARE_NGINX_ALIAS]){
   const context=await browser.newContext({ignoreHTTPSErrors:true,javaScriptEnabled:false});
   const page=await context.newPage();
   const writes=[];page.on('response',r=>{if(r.request().method()==='POST')writes.push({path:new URL(r.url()).pathname,status:r.status(),origin:r.request().headers().origin});});
   await page.goto(base+'/login');
   await page.locator('input[name=username]').fill('admin');
   await page.locator('input[name=password]').fill('acceptance-only-password');
   await Promise.all([page.waitForURL(base+'/'),page.getByRole('button',{name:'Login / 登录',exact:true}).click()]);
   await page.getByText('PRIVATE-NGINX-ACCEPTANCE',{exact:true}).waitFor();
   const cookies=await context.cookies();assert(cookies.some(c=>c.httpOnly&&c.secure));
   await page.goto(base+'/settings/application');
   const logout=page.locator('form[action="/logout"]');
   await Promise.all([page.waitForURL(base+'/login'),logout.getByRole('button').click()]);
   assert(await page.locator('input[name=username]').isVisible());
   assert(writes.some(w=>w.path==='/login'&&w.status===303));
   assert(writes.some(w=>w.path==='/logout'&&w.status===303));
   result.cases.push({origin:base,passed:'native login/logout; Secure HttpOnly cookies; JavaScript disabled',writes});
   await context.close();
  }
  console.log(JSON.stringify(result,null,2));
 }finally{await browser.close();}
})().catch(e=>{console.error(e);process.exitCode=1});
