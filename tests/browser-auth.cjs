// Exercise the production session/CSRF contract without bypassing middleware.
const assert=require('node:assert/strict');
const credentials={FLARE_DISABLE_LOGIN:'false',FLARE_USER:'browser-fixture',FLARE_PASS:'browser-fixture-password'};
async function login(page,base){
 await page.goto(base+'/login');await page.locator('[name=username]').fill(credentials.FLARE_USER);await page.locator('[name=password]').fill(credentials.FLARE_PASS);
 await Promise.all([page.waitForURL(base+'/'),page.getByRole('button',{name:'Login / 登录',exact:true}).click()]);
}
async function post(page,url,options={}){
 const base=new URL(url).origin,response=await page.request.get(base+'/editor/csrf');assert.equal(response.status(),200);
 const {csrfToken}=await response.json();assert(csrfToken);
 return page.request.post(url,{...options,headers:{Origin:base,'X-CSRF-Token':csrfToken,'X-Flare-Confirm-Public':'1',...options.headers}});
}
module.exports={credentials,login,post};
