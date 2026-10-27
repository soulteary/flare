const browserAuth=require('../browser-auth.cjs');
// Exercise category controls against production HTTP, CSS and templates.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const os = require('node:os');
const path = require('node:path');
const net = require('node:net');
const {spawn} = require('node:child_process');
(async () => {
  assert.ok(process.env.FLARE_BINARY, 'Set FLARE_BINARY');
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-categories-'));
  const socket = net.createServer();
  await new Promise(resolve => socket.listen(0, '127.0.0.1', resolve));
  const port = socket.address().port;
  await new Promise(resolve => socket.close(resolve));
  const base = `http://127.0.0.1:${port}`;
  await fs.writeFile(path.join(dir, 'config.yml'), 'Title: Categories\nLocale: en\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nHomepage:\n  MobileBookmarkLayout: compact\n  NavigationTextSize: large\n  DesktopColumns: "6"\n  CategoryNavigation: tabs\n');
  await fs.writeFile(path.join(dir, 'apps.yml'), 'links: []\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'), 'categories:\n- id: alpha\n  title: Alpha\n- id: beta\n  title: Beta\nlinks:\n- name: Needle Alpha\n  link: https://example.com/alpha\n  category: alpha\n- name: Needle Beta\n  link: https://example.com/beta\n  category: beta\n' + Array.from({length: 30}, (_, i) => `- name: Long group bookmark ${i}\n  link: https://example.com/${i}\n  category: beta\n`).join(''));
  const child = spawn(process.env.FLARE_BINARY, [], {cwd: dir, env: {...process.env, FLARE_PORT: String(port), FLARE_DISABLE_LOGIN: 'true', FLARE_OFFLINE: 'true', FLARE_BASELINE: '1'}});
  let browser;
  try {
    let ready = false;
    for (let i = 0; i < 100; i++) {
      if (child.exitCode !== null) throw Error('Test server exited');
      try { if ((await fetch(base + '/ping')).ok) { ready = true; break; } } catch {}
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    assert.ok(ready, 'Test server did not start');
    browser = await chromium.launch({headless: true, executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox', '--disable-dev-shm-usage']});
    // These interactions must also work with scripts disabled.
    const page = await browser.newPage({javaScriptEnabled: false});
    const errors = [];
    page.on('pageerror', error => errors.push(error.message));
    async function fits() {
      const overflow = await page.locator('#page-home').evaluate(e => e.scrollWidth > e.clientWidth + 1);
      assert.equal(overflow, false, 'Category controls overflow the page');
    }
    for (const width of [320, 390, 768, 769, 1440]) {
      await page.setViewportSize({width, height: 900});
      await page.goto(base + '/?search=Needle');
      await page.locator('.bookmark-category-nav a', {hasText: 'Beta'}).click();
      assert.equal(new URL(page.url()).searchParams.get('category'), 'beta');
      assert.equal(new URL(page.url()).searchParams.get('search'), 'Needle');
      assert.equal(await page.locator('a.bookmark', {hasText: 'Needle Alpha'}).count(), 0);
      assert.equal(await page.locator('a.bookmark', {hasText: 'Needle Beta'}).count(), 1);
      await fits();
      await page.locator('#search').fill('missing-match');
      await page.locator('#search').press('Enter');
      assert.equal(await page.locator('input[name=category]').inputValue(), 'beta');
      assert.equal(await page.locator('a.bookmark').count(), 0);
      assert.equal(await page.getByRole('status').filter({hasText: 'No matching bookmarks'}).count(), 1);
      await page.goto(base + '/bookmarks?category=alpha');
      assert.equal(await page.locator('a.bookmark').count(), 1);
      await page.locator('.bookmark-category-nav a', {hasText: 'All'}).click();
      assert.equal(await page.locator('a.bookmark').count(), 32);
      await fits();
    }
    const response = await browserAuth.post(page,base + '/settings/appearance', {form: {'category-navigation': 'collapse', 'show-apps': 'true', 'show-bookmarks': 'true', 'locale': 'en'}});
    assert.equal(response.status(), 200);
    for (const width of [320, 390, 768, 769, 1440]) {
      await page.setViewportSize({width, height: 900});
      await page.goto(base);
      const group = page.locator('details.bookmark-collapsible').first();
      assert.equal(await group.getAttribute('open'), null);
      await group.locator('summary').click();
      assert.notEqual(await group.getAttribute('open'), null);
      assert.equal(await group.locator('a.bookmark').first().isVisible(), true);
      await fits();
      await page.goto(base + '/?search=Needle');
      assert.equal(await page.locator('a.bookmark:visible').count(), 2);
      await fits();
    }
    assert.deepEqual(errors, []);
    console.log('PASS: script-free category links, search scope/no-results, subpage navigation and collapsible groups across five viewport widths');
  } finally {
    if (browser) await browser.close();
    child.kill();
    await fs.rm(dir, {recursive: true, force: true});
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
