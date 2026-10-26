// Opt-in only: uses real backend search, save revalidation and live weather requests.
const {chromium} = require('../clock/node_modules/playwright');
const assert = require('node:assert/strict');
(async () => {
  const browser = await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE || undefined});
  const cases = [
    {keyword:'北京', address:'北京, 北京'},
    {keyword:'北京市', address:'北京, 北京'},
    {keyword:'朝阳', address:'北京, 北京, 朝阳'},
    {keyword:'朝阳', address:'吉林, 长春, 朝阳'},
    {keyword:'朝阳', address:'辽宁, 朝阳'},
    {keyword:'津市', address:'湖南, 常德, 津市'},
  ];
  try {
    for (const viewport of [{width:1280,height:900},{width:390,height:844}]) {
      const page = await browser.newPage({viewport});
      const errors=[];
      page.on('pageerror',error=>errors.push(error.message));
      for (const test of cases) {
        await page.goto(process.env.FLARE_WEATHER_URL+'/settings/weather');
        await page.locator('#weather-keyword').fill(test.keyword);
        await page.locator('#weather-search').click();
        const candidate=page.getByRole('button',{name:test.address.split(', ').join(' / '),exact:true});
        await candidate.waitFor();
        if (test.keyword==='朝阳') assert.ok(await page.locator('.weather-candidate').count()>=3);
        await candidate.click();
        assert.equal(await page.locator('#settings-location').inputValue(),test.address);
        await Promise.all([page.waitForNavigation(),page.locator('#weather-form input[type=submit]').click()]);
        await page.reload();
        assert.equal(await page.locator('#settings-location').inputValue(),test.address,'saved exact address survives reload');
        await Promise.all([page.waitForNavigation(),page.locator('form[action="/settings/weather/test"] input[type=submit]').click()]);
        const result=await page.locator('#weather-test-result').innerText();
        assert.ok(result.includes(test.address),result);
        assert.match(result,/-?\d+\s*°C/);
        assert.match(result,/\d{4}-\d{2}-\d{2} \d{2}:\d{2}/);
        assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),false);
        console.log(`${viewport.width}px: ${result}`);
      }
      if (process.env.FLARE_WEATHER_SCREENSHOTS) {
        const fs=require('node:fs/promises');
        await fs.mkdir(process.env.FLARE_WEATHER_SCREENSHOTS,{recursive:true});
        await page.screenshot({path:process.env.FLARE_WEATHER_SCREENSHOTS+`/weather-${viewport.width}.png`,fullPage:true});
        await page.locator('#weather-keyword').fill('朝阳');
        await page.locator('#weather-search').click();
        await page.locator('.weather-candidate').nth(2).waitFor();
        await page.screenshot({path:process.env.FLARE_WEATHER_SCREENSHOTS+`/weather-search-${viewport.width}.png`,fullPage:true});
      }
      assert.deepEqual(errors,[]);
      await page.close();
    }
  } finally {await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
