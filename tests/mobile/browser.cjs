// Run against a fresh binary: FLARE_BINARY=/absolute/path node tests/mobile/browser.cjs
const {chromium} = require(require.resolve('playwright', {paths: [__dirname, `${__dirname}/../clock`]}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const net = require('node:net');
const {spawn} = require('node:child_process');
(async () => {
 const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-mobile-'));
 const socket = net.createServer();
 await new Promise(resolve => socket.listen(0, '127.0.0.1', resolve));
 const port = socket.address().port;
 await new Promise(resolve => socket.close(resolve));
 const base = `http://127.0.0.1:${port}`;
 assert.ok(process.env.FLARE_BINARY, 'Set FLARE_BINARY');
 await fs.writeFile(path.join(dir, 'config.yml'), 'Title: "Mobile test"\nShowTitle: true\nShowDateTime: true\nShowSearchComponent: true\nShowApps: true\nShowBookmarks: true\nGreetings: "很长的问候语欢迎使用移动导航;中午好;下午好;晚上好"\nLocale: "zh"\nTheme: "blackboard"\nShowWeather: false\n');
 await fs.writeFile(path.join(dir, 'apps.yml'), 'links:\n- name: "LongApplicationNameWithoutSpaces长应用名称"\n  link: "https://example.com"\n  desc: "很长的说明内容LongDescriptionWithoutSpaces"\n');
 await fs.writeFile(path.join(dir, 'bookmarks.yml'), 'categories:\n- id: one\n  title: "长分类LongCategoryWithoutSpaces"\n- id: two\n  title: "第二分类"\nlinks:\n' + Array.from({length: 12}, (_, i) => `- name: "${i}长书签LongBookmarkWithoutSpaces"\n  link: "https://example.com/${i}?query=a&value=b#fragment"\n  category: "${i < 10 ? 'one' : 'two'}"\n`).join(''));
 const child = spawn(process.env.FLARE_BINARY, [], {cwd: dir, env: {...process.env, FLARE_PORT: String(port), FLARE_DISABLE_LOGIN: 'true', FLARE_OFFLINE: 'true', FLARE_BASELINE: '1'}});
 let log = ''; child.stdout.on('data', b => log += b); child.stderr.on('data', b => log += b);
 let browser;
 try {
  for (let i = 0; i < 100; i++) {
   if (child.exitCode !== null) throw Error(log);
   try { if ((await fetch(base + '/ping')).ok) break; } catch {}
   await new Promise(r => setTimeout(r, 100));
  }
  browser = await chromium.launch({headless: true, executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox', '--disable-dev-shm-usage', '--disable-gpu']});
  const context = await browser.newContext({hasTouch: true, isMobile: true, userAgent: 'Mozilla/5.0 (Linux; Android 14) AppleWebKit/537.36 Chrome/153.0 Mobile Safari/537.36'});
  const page = await context.newPage();
  const errors = []; page.on('pageerror', e => { errors.push(e.message); console.error('page error', e.message); });
  page.setDefaultTimeout(7000);
  async function fits(selector) {
   const size = await page.locator(selector).evaluate(el => ({scroll: el.scrollWidth, width: el.clientWidth}));
   assert.ok(size.scroll <= size.width + 1, `${selector} overflow: ${JSON.stringify(size)}`);
  }
  for (const width of [320, 360, 375, 390, 430, 768]) {
   console.log("mobile width", width);
   await page.setViewportSize({width, height: 844});
   await page.goto(base);
   assert.equal(await page.locator('#search[autofocus]').count(), 0);
   // Offline fixture: exercise the production weather structure without a network dependency.
   await page.locator('#hero-container').evaluate(el => {
    const weather = document.createElement('div'); weather.id = 'plugin-weather'; weather.className = 'plugin-container clearfix';
    const icon = document.createElement('div'); icon.className = 'pull-left'; icon.textContent = '☀';
    const values = document.createElement('div'); values.className = 'pull-right weather-value-container';
    for (const text of ['35°C', '100%']) { const p = document.createElement('p'); p.textContent = text; values.appendChild(p); }
    weather.append(icon, values);
    el.appendChild(weather);
    const time = document.getElementById('clock-time'); if (time) time.textContent = '23:59:59';
   });
   for (const scale of [1, 1.1, 1.25, 2]) {
    await page.evaluate(scale => { document.body.style.fontSize = `${14 * scale}px`; }, scale);
    await fits('#page-home');
    const hero = await page.locator('#hero-container').evaluate(el => ({dateBottom: el.querySelector('#plugin-datetime').getBoundingClientRect().bottom, weatherTop: el.querySelector('#plugin-weather').getBoundingClientRect().top}));
    assert.ok(hero.weatherTop >= hero.dateBottom - 1, 'weather overlaps date/greeting');
    assert.equal(await page.locator('.bookmark-group-container').first().evaluate(el => getComputedStyle(el).float), 'none');
   }
   await page.goto(base + '/settings/appearance');
   await fits('html'); await fits('#page-settings');
   assert.equal(await page.locator('.sidebar').evaluate(el => getComputedStyle(el).position), 'static');
   await page.goto(base + '/editor'); await fits('html');
  }
  await page.setViewportSize({width: 390, height: 844});
  await page.goto(base + '/editor');
  async function edit(container, row, col, value) {
   const cell = page.locator(`${container} .ht_master tbody tr`).nth(row).locator('td').nth(col);
   await cell.tap();
   await page.locator(container === '#container-category' ? '#edit-category' : '#edit-bookmark').click();
   const input = page.locator(`${container} textarea.handsontableInput:visible`);
   await input.fill(value); await input.press('Enter');
  }
  await edit('#container-bookmarks', 1, 0, '未保存的修改');
  await edit('#container-category', 0, 0, '重命名分类');
  assert.equal(await page.locator('#container-bookmarks .ht_master tbody tr').nth(1).locator('td').first().textContent(), '未保存的修改');
  assert.equal(await page.locator('#container-bookmarks .ht_master tbody tr').nth(1).locator('td').nth(2).textContent(), '重命名分类');
  await page.locator('.update-data').first().click();
  await page.waitForFunction(() => document.getElementById('save-status').textContent === '全部数据已保存');
  await page.reload();
  assert.match(await page.locator('#container-bookmarks').textContent(), /未保存的修改/);
  const saved = await fs.readFile(path.join(dir, 'bookmarks.yml'), 'utf8');
  assert.match(saved, /重命名分类/); assert.match(saved, /query=a&value=b#fragment/);
  // Failure must preserve the current table and a subsequent retry must work.
  await edit('#container-bookmarks', 1, 0, '保存失败后仍保留');
  await page.route('**/editor', async route => {
   if (route.request().method() === 'POST') return route.fulfill({status: 500, contentType: 'application/json', body: '{"saved":false,"error":"模拟保存失败"}'});
   return route.continue();
  });
  await page.locator('.update-data').first().click();
  await page.waitForFunction(() => document.getElementById('save-status').textContent.includes('模拟保存失败'));
  assert.match(await page.locator('#container-bookmarks').textContent(), /保存失败后仍保留/);
  await page.unroute('**/editor');
  await page.locator('.update-data').first().click();
  await page.waitForFunction(() => document.getElementById('save-status').textContent === '全部数据已保存');
  // Delete every category. Associated bookmarks must become applications.
  while (await page.locator('#container-category .ht_master tbody td').count()) {
   await page.locator('#container-category .ht_master tbody tr').first().locator('td').first().tap();
   page.once('dialog', dialog => dialog.accept()); await page.locator('#delete-category').click();
  }
  assert.equal(await page.locator('#container-category .ht_master tbody td').count(), 0);
  assert.match(await page.locator('#container-bookmarks .ht_master tbody tr').first().textContent(), /Flare 应用/);
  // Delete every bookmark, persist empty collections, then add from empty.
  while (await page.locator('#container-bookmarks .ht_master tbody td').count()) {
   await page.locator('#container-bookmarks .ht_master tbody tr').first().locator('td').first().tap();
   page.once('dialog', dialog => dialog.accept()); await page.locator('#delete-bookmark').click();
  }
  await page.locator('.update-data').first().click();
  await page.waitForFunction(() => document.getElementById('save-status').textContent === '全部数据已保存');
  await page.reload();
  assert.equal(await page.locator('#container-bookmarks .ht_master tbody td').count(), 0);
  await page.locator('#add-category').click();
  await page.locator('#container-category textarea.handsontableInput:visible').fill('空表新增分类');
  await page.locator('textarea.handsontableInput:visible').last().press('Enter');
  await page.locator('#add-bookmark').click();
  await page.locator('#container-bookmarks textarea.handsontableInput:visible').fill('空表新增书签');
  await page.locator('textarea.handsontableInput:visible').last().press('Enter');
  await edit('#container-bookmarks', 0, 1, 'https://example.com/new?x=1&y=2#z');
  await page.locator('.update-data').first().click();
  await page.waitForFunction(() => document.getElementById('save-status').textContent === '全部数据已保存');
  await page.reload(); assert.match(await page.locator('#container-bookmarks').textContent(), /空表新增书签/);
  if (process.env.MOBILE_SCREENSHOT_DIR) {
   await fs.mkdir(process.env.MOBILE_SCREENSHOT_DIR, {recursive:true});
   for (const [name, url] of [['editor','/editor'],['settings','/settings/appearance'],['home','/']]) {
    await page.goto(base + url); await page.screenshot({path: path.join(process.env.MOBILE_SCREENSHOT_DIR, name + '.png'), fullPage:true});
   }
  }
  const desktop = await browser.newPage();
  for (const width of [1024, 1440, 1920]) {
   await desktop.setViewportSize({width, height:1080}); await desktop.goto(base);
   assert.equal(await desktop.locator('#search[autofocus]').count(), 1);
   await desktop.goto(base + '/editor');
   const size = await desktop.locator('html').evaluate(el => ({s:el.scrollWidth,w:el.clientWidth}));
   assert.ok(size.s <= size.w + 1);
  }
  assert.deepEqual(errors, []);
  console.log('PASS: 6 mobile widths, enlarged text, settings, editor CRUD, empty data, category rename, save failure/retry and 3 desktop widths');
 } finally { if (browser) await browser.close(); child.kill(); await fs.rm(dir, {recursive:true,force:true}); }
})().catch(error => {console.error(error); process.exitCode=1;});
