// FLARE_BINARY=/path/to/binary node tests/editor/dialogs.browser.cjs
// The dialogs of the editor on the page as it is served, in the frame of the
// settings and with the stylesheet of the application: each dialog stands in
// the middle of the window and inside it, and the checkbox of the entry form
// stands beside its name. The lists draw their arrow as far from the edge as
// their text is, and the entry form links to the page of all icons, which
// opens in a window of its own and leaves the form as it is. A new entry
// stands among those of its place, at the position the form names.
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
  await fs.writeFile(path.join(dir, 'apps.yml'), 'links:\n- name: App 1\n  link: https://app1.example.test/\n- name: App 2\n  link: https://app2.example.test/\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'), 'categories:\n- id: a\n  title: First\n- id: b\n  title: Second\n- id: c\n  title: Third\nlinks:\n' +
    [['a1', 'a'], ['b1', 'b'], ['a2', 'a'], ['b2', 'b']].map(([name, category]) => `- name: ${name}\n  link: https://${name}.example.test/\n  category: ${category}\n`).join(''));
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
  return {base, dir, async stop() {child.kill(); await fs.rm(dir, {recursive: true, force: true});}};
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
    // The position of an entry among those of its place. A new entry stands
    // after the last of its category unless the form says otherwise, in the
    // list as on the home page.
    const page = await (await browser.newContext({viewport: {width: 1280, height: 900}})).newPage();
    page.on('console', message => {if (message.type() === 'error') errors.push(message.text());});
    page.on('pageerror', error => errors.push(String(error)));
    await page.goto(server.base + '/settings/content');
    await page.waitForFunction(() => window.FlareEditorUI?.store && document.querySelector('#entry-list .entry'));
    const order = () => page.evaluate(() => [...document.querySelectorAll('#entry-list .entry .entry-name')].map(name => name.textContent).join(' '));
    const choices = () => page.evaluate(() => { const list = document.querySelector('#entry-position'); return [list.disabled, list.value, [...list.options].map(option => option.textContent)]; });
    const add = async (name, location, position) => {
      await page.locator('#new-entry').click();
      await page.locator('#entry-name').fill(name);
      await page.locator('#entry-url').fill(`https://${name}.example.test/`);
      await page.locator('#entry-location').selectOption(location);
      if (position) await page.getByLabel('Position', {exact: true}).selectOption({label: position});
      await page.locator('#entry-form .dialog-actions .primary').click();
      await page.waitForFunction(() => !document.querySelector('#entry-dialog').open);
    };
    assert.equal(await order(), 'App 1 App 2 a1 b1 a2 b2');
    // Before a place is chosen there is no position to choose.
    await page.locator('#new-entry').click();
    assert.deepEqual(await choices(), [true, '', []]);
    await page.locator('#entry-location').selectOption('category:a');
    assert.deepEqual(await choices(), [false, 'end', ['At the end', 'At the start', 'After “a1”', 'After “a2”']]);
    await page.locator('#entry-location').selectOption('category:c');
    assert.deepEqual(await choices(), [false, 'end', ['At the end', 'At the start']]);
    await page.locator('#entry-location').selectOption('apps');
    assert.deepEqual((await choices())[2], ['At the end', 'At the start', 'After “App 1”', 'After “App 2”']);
    page.once('dialog', question => question.accept());
    await page.locator('#cancel-entry').click();
    await page.waitForFunction(() => !document.querySelector('#entry-dialog').open);

    await add('a3', 'category:a');
    assert.equal(await order(), 'App 1 App 2 a1 b1 a2 a3 b2');
    await add('b0', 'category:b', 'At the start');
    assert.equal(await order(), 'App 1 App 2 a1 b0 b1 a2 a3 b2');
    await add('a1x', 'category:a', 'After “a1”');
    assert.equal(await order(), 'App 1 App 2 a1 a1x b0 b1 a2 a3 b2');
    await add('App 0', 'apps', 'At the start');
    assert.equal(await order(), 'App 0 App 1 App 2 a1 a1x b0 b1 a2 a3 b2');

    // An entry that is edited stays where it is unless a position is chosen,
    // and one that changes its place joins the end of the new one.
    const edit = name => page.locator('#entry-list .entry', {has: page.locator('.entry-name', {hasText: new RegExp(`^${name}$`)})}).locator('.entry-actions button').first().click();
    await edit('a1');
    assert.deepEqual(await choices(), [false, 'keep', ['Where it is', 'At the end', 'At the start', 'After “a1x”', 'After “a2”', 'After “a3”']]);
    await page.locator('#entry-name').fill('a1 renamed');
    await page.locator('#entry-form .dialog-actions .primary').click();
    assert.equal(await order(), 'App 0 App 1 App 2 a1 renamed a1x b0 b1 a2 a3 b2');
    await edit('a1 renamed');
    await page.getByLabel('Position', {exact: true}).selectOption({label: 'After “a2”'});
    await page.locator('#entry-form .dialog-actions .primary').click();
    assert.equal(await order(), 'App 0 App 1 App 2 a1x b0 b1 a2 a1 renamed a3 b2');
    await edit('a3');
    await page.locator('#entry-location').selectOption('category:b');
    assert.deepEqual((await choices()).slice(0, 2), [false, 'end']);
    await page.locator('#entry-form .dialog-actions .primary').click();
    assert.equal(await order(), 'App 0 App 1 App 2 a1x b0 b1 a2 a1 renamed b2 a3');

    // What is saved has the order of the list: the files are what the home
    // page shows, a category at a time, in their order.
    page.on('dialog', question => question.accept());
    await page.locator('#save-changes').click();
    await page.waitForFunction(() => !window.FlareEditorUI.store.dirty);
    const saved = async file => [...(await fs.readFile(path.join(server.dir, file), 'utf8')).matchAll(/- name: (.*)/g)].map(match => match[1]).join(', ');
    assert.equal(await saved('bookmarks.yml'), 'a1x, b0, b1, a2, a1 renamed, b2, a3');
    assert.equal(await saved('apps.yml'), 'App 0, App 1, App 2');

    assert.deepEqual(errors, []);
    console.log('PASS: the dialogs of the editor stand in the middle of the window in the frame of the settings, the lists draw their arrow, the entry form links to the page of all icons, and an entry stands at the chosen position of its place');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
