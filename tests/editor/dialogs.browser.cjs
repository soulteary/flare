// FLARE_BINARY=/path/to/binary node tests/editor/dialogs.browser.cjs
// The dialogs of the editor on the page as it is served, in the frame of the
// settings and with the stylesheet of the application: each dialog stands in
// the middle of the window and inside it, and the checkbox of the entry form
// stands beside its name. The lists draw their arrow as far from the edge as
// their text is, and the entry form links to the page of all icons, which
// opens in a window of its own and leaves the form as it is.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-editor-dialogs-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'), 'Locale: en\nTitle: Site\nTheme: blackboard\nShowWeather: false\n');
  const child = spawn(process.env.FLARE_BINARY, [], {cwd: dir, env: {...process.env,
    FLARE_PORT: String(port), FLARE_DISABLE_LOGIN: 'true', FLARE_OFFLINE: 'true', FLARE_EDITOR: 'true', FLARE_BASELINE: '1'}});
  let log = '';
  child.stderr.on('data', data => log += data);
  child.stdout.on('data', data => log += data);
  const base = `http://127.0.0.1:${port}`;
  let ready = false;
  for (let i = 0; i < 100 && !ready; i++) {
    if (child.exitCode !== null) throw Error(log);
    try {ready = (await fetch(base + '/ping')).ok;} catch {}
    if (!ready) await new Promise(resolve => setTimeout(resolve, 100));
  }
  assert.ok(ready, log);
  return {base, async stop() {child.kill(); await fs.rm(dir, {recursive: true, force: true});}};
}

// Where the open dialog stands in the window: the space left on each side.
const placed = selector => {
  const dialog = document.querySelector(selector), box = dialog.getBoundingClientRect();
  const width = document.documentElement.clientWidth, height = window.innerHeight;
  return {open: dialog.open, left: Math.round(box.left), right: Math.round(width - box.right), top: Math.round(box.top), bottom: Math.round(height - box.bottom)};
};

(async () => {
  assert.ok(process.env.FLARE_BINARY, 'FLARE_BINARY is required');
  const server = await start();
  const browser = await chromium.launch({executablePath: process.env.CHROMIUM_EXECUTABLE || undefined});
  try {
    const errors = [];
    for (const [name, options] of [['wide', {viewport: {width: 1280, height: 900}}], ['low', {viewport: {width: 1024, height: 500}}], ['phone', {viewport: {width: 360, height: 700}, isMobile: true, hasTouch: true}]]) {
      const page = await (await browser.newContext(options)).newPage();
      page.on('console', message => {if (message.type() === 'error') errors.push(message.text());});
      page.on('pageerror', error => errors.push(String(error)));
      await page.goto(server.base + '/settings/content');
      await page.waitForFunction(() => window.FlareEditorUI?.store && document.querySelector('#entry-list .entry'));
      // The page is the one of the settings: it has their menu.
      assert.equal(await page.locator('#page-settings nav.sidebar').count(), 1, name);

      const centered = async (selector, what) => {
        const at = await page.evaluate(placed, selector);
        assert.equal(at.open, true, `${name}: ${what} is open`);
        assert.ok(at.left >= 0 && at.right >= 0 && at.top >= 0 && at.bottom >= 0, `${name}: ${what} is inside the window: ${JSON.stringify(at)}`);
        assert.ok(Math.abs(at.left - at.right) <= 2, `${name}: ${what} is in the middle from side to side: ${JSON.stringify(at)}`);
        assert.ok(Math.abs(at.top - at.bottom) <= 2, `${name}: ${what} is in the middle from top to bottom: ${JSON.stringify(at)}`);
      };

      // The arrow of a list is drawn by the page, 12 pixels from the edge, and
      // the text ends before it.
      const arrow = selector => page.evaluate(sel => {
        const style = getComputedStyle(document.querySelector(sel));
        return [style.appearance, parseFloat(style.paddingRight) >= 30, (style.backgroundImage.match(/linear-gradient/g) || []).length, style.backgroundPosition, style.backgroundRepeat.split(', ')[0]];
      }, selector);
      const drawn = ['none', true, 2, 'calc(100% - 18px) 50%, calc(100% - 12px) 50%', 'no-repeat'];
      assert.deepEqual(await arrow('#location-filter'), drawn, name);
      assert.ok(await page.locator('#location-filter').evaluate(list => list.offsetHeight) >= 44, name);

      await page.locator('#new-entry').click();
      await centered('#entry-dialog', 'the entry form');
      assert.deepEqual(await arrow('#entry-location'), drawn, name);
      // The checkbox is a box beside its name, on a line a finger can hit.
      const box = await page.evaluate(() => {
        const input = document.querySelector('#entry-private'), label = input.closest('label');
        const a = input.getBoundingClientRect(), b = label.getBoundingClientRect();
        const text = document.createRange(); text.selectNodeContents(label);
        const words = [...text.getClientRects()].filter(rect => rect.width > 0 && rect.left > a.right - 1)[0];
        return {size: [Math.round(a.width), Math.round(a.height)], line: Math.round(b.height), beside: Boolean(words) && words.left - a.right >= 4 && Math.abs((words.top + words.bottom) / 2 - (a.top + a.bottom) / 2) < 6};
      });
      assert.ok(box.size[0] === box.size[1] && box.size[0] <= 28, `${name}: the checkbox is ${box.size}`);
      assert.ok(box.line >= 44 && box.beside, `${name}: ${JSON.stringify(box)}`);

      await page.locator('#entry-options summary').click();
      // Below the icon is the way to the page of all icons. It opens in a
      // window of its own, and what was typed into the form stays.
      await page.locator('#entry-name').fill('Kept while looking');
      const link = page.locator('#entry-dialog a#all-icons');
      assert.deepEqual([await link.getAttribute('href'), await link.getAttribute('target'), await link.getAttribute('rel'), await link.isVisible()], ['/icons/', '_blank', 'noopener', true], name);
      assert.ok(await link.evaluate(a => a.getBoundingClientRect().top > document.querySelector('#entry-icon').getBoundingClientRect().bottom), name);
      const [icons] = await Promise.all([page.context().waitForEvent('page'), link.click()]);
      await icons.waitForLoadState('domcontentloaded');
      assert.equal(new URL(icons.url()).pathname, '/icons/', name);
      assert.equal((await icons.request.get(icons.url())).status(), 200, name);
      await icons.close();
      assert.deepEqual([await page.locator('#entry-dialog').evaluate(dialog => dialog.open), await page.locator('#entry-name').inputValue()], [true, 'Kept while looking'], name);
      await page.locator('#choose-icon').click();
      await page.waitForSelector('#icon-picker .icon-choice');
      await centered('#icon-picker', 'the icon picker');
      await page.locator('#icon-picker .icon-picker-pages button').last().click();
      // Leaving a form something was typed into asks first.
      page.once('dialog', question => question.accept());
      await page.locator('#cancel-entry').click();
      await page.waitForFunction(() => !document.querySelector('#entry-dialog').open);

      await page.locator('#manage-categories').click();
      await centered('#categories-dialog', 'the categories');
      await page.locator('#close-categories').click();
    }
    assert.deepEqual(errors, []);
    console.log('PASS: the dialogs of the editor stand in the middle of the window in the frame of the settings, the lists draw their arrow, and the entry form links to the page of all icons');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
