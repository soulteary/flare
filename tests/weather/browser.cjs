// Run through FLARE_WEATHER_BROWSER=1 go test ./internal/settings/weather -run TestWeatherBrowser.
const { chromium } = require('../clock/node_modules/playwright');
const assert = require('node:assert/strict');
(async () => {
  const browser = await chromium.launch({headless:true, executablePath:process.env.CHROMIUM_EXECUTABLE || undefined});
  try {
    for (const viewport of [{width:1280,height:900},{width:390,height:844}]) {
      const page = await browser.newPage({viewport});
      const errors = [];
      page.on('pageerror', error => errors.push(error.message));
      await page.goto(process.env.FLARE_WEATHER_URL + '/settings/weather');
      const composingRequests = [];
      const recordSearch = request => { if (request.url().includes('/settings/weather/locations')) composingRequests.push(request.url()); };
      page.on('request', recordSearch);
      await page.locator('#weather-keyword').dispatchEvent('compositionstart');
      await page.locator('#weather-keyword').fill('北京shi');
      await page.waitForTimeout(450);
      assert.equal(composingRequests.length, 0, 'do not search unfinished IME composition');
      await page.locator('#weather-keyword').fill('北京市');
      await page.locator('#weather-keyword').dispatchEvent('compositionend');
      await page.locator('.weather-candidate').waitFor();
      assert.equal(await page.locator('.weather-candidate').innerText(), '北京 / 北京');
      page.off('request', recordSearch);
      await page.locator('#weather-keyword').fill('朝阳');
      await page.locator('#weather-search').click();
      await page.locator('.weather-candidate').nth(1).waitFor();
      assert.equal(await page.locator('.weather-candidate').count(), 2);
      await page.locator('.weather-candidate').filter({hasText:'北京 / 北京 / 朝阳'}).focus();
      await page.keyboard.press('Enter');
      assert.equal(await page.locator('#settings-location').inputValue(), '北京, 北京, 朝阳');
      await Promise.all([page.waitForNavigation(),page.locator('#weather-form input[type=submit]').click()]);
      await page.reload();
      assert.equal(await page.locator('#settings-location').inputValue(),'北京, 北京, 朝阳');
      await page.locator('#weather-keyword').fill('津市');
      assert.equal(await page.locator('#weather-selected').inputValue(),'');
      assert.equal(await page.locator('#settings-location').inputValue(),'');
      await page.locator('#weather-search').click();
      await page.locator('.weather-candidate').filter({hasText:'湖南 / 常德 / 津市'}).click();
      await Promise.all([page.waitForNavigation(),page.locator('#weather-form input[type=submit]').click()]);
      assert.equal(await page.locator('#settings-location').inputValue(),'湖南, 常德, 津市');
      await page.locator('#weather-keyword').fill('无结果');
      await page.locator('#weather-search').click();
      await page.waitForFunction(() => document.querySelector('#weather-status').textContent.includes('没有找到'));
      await page.locator('#weather-keyword').fill('失败');
      await page.locator('#weather-search').click();
      await page.waitForFunction(() => document.querySelector('#weather-status').textContent.includes('失败'));
      await page.locator('#weather-keyword').fill('慢查询');
      await page.locator('#weather-search').click();
      await page.waitForTimeout(100);
      await page.locator('#weather-keyword').fill('朝阳');
      await page.locator('#weather-search').click();
      await page.locator('.weather-candidate').nth(1).waitFor();
      await page.waitForTimeout(700);
      assert.equal(await page.locator('.weather-candidate').count(),2,'old search must not replace new results');
      await page.locator('#weather-detect').click();
      await page.locator('.weather-candidate').filter({hasText:'北京 / 北京'}).waitFor();
      assert.equal(await page.locator('#weather-selected').inputValue(),'','IP suggestion must require confirmation');
      await page.locator('.weather-candidate').click();
      await Promise.all([page.waitForNavigation(),page.locator('#weather-form input[type=submit]').click()]);
      assert.equal(await page.locator('#settings-location').inputValue(),'北京, 北京');
      assert.equal(await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),false,'no horizontal page overflow');
      assert.deepEqual(errors,[]);
      await page.close();
    }
    console.log('Weather browser checks passed: desktop/mobile, duplicate names, county save/reload, empty/error, stale responses, IP confirmation.');
  } finally { await browser.close(); }
})().catch(error => {console.error(error);process.exitCode=1;});
