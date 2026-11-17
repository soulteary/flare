// FLARE_BINARY=/path/to/binary node tests/settings/menu.browser.cjs
// The menu of the settings, as documented in docs/settings-menu.md: the
// groups and their entries, the entry of the page that is shown, the pages
// the entries lead to, and the switch that opens the menu in a narrow
// window, which works without a script and from the keyboard.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

const PAGES = {'/settings/modules': 'Home page › Modules', '/settings/search': 'Home page › Search', '/settings/weather': 'Home page › Weather',
  '/settings/clock': 'Home page › Clock', '/settings/links': 'Home page › Links', '/settings/theme': 'Look › Theme', '/settings/layout': 'Look › Layout',
  '/settings/site': 'Site › Site details', '/settings/about': 'Site › About'};
const GROUPS = [['Content', 'Apps and bookmarks', 'Import and export'], ['Home page', 'Modules', 'Search', 'Weather', 'Clock', 'Links'], ['Look', 'Theme', 'Layout'], ['Site', 'Site details', 'About']];

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-settings-menu-'));
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

// What the menu of a page shows.
const shown = () => {
  const menu = document.querySelector('nav.sidebar'), main = document.querySelector('.setting-group-container');
  const visible = element => element.getClientRects().length > 0;
  return {
    groups: [...menu.querySelectorAll('.sidebar-group')].map(group => [...group.querySelectorAll('.sidebar-title, a')].map(part => part.textContent.trim())),
    current: [...menu.querySelectorAll('a[aria-current="page"]')].map(link => new URL(link.href).pathname),
    links: [...menu.querySelectorAll('a')].filter(visible).map(link => [new URL(link.href).pathname, link.offsetHeight]),
    switch: visible(menu.querySelector('.sidebar-current')) ? menu.querySelector('.sidebar-current span').textContent.trim() : null,
    beside: menu.getBoundingClientRect().right <= main.getBoundingClientRect().left,
    above: menu.getBoundingClientRect().bottom <= main.getBoundingClientRect().top,
    overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
  };
};

(async () => {
  assert.ok(process.env.FLARE_BINARY, 'FLARE_BINARY is required');
  const server = await start();
  const browser = await chromium.launch({executablePath: process.env.CHROMIUM_EXECUTABLE || undefined});
  try {
    const errors = [];
    const open = async options => {
      const page = await (await browser.newContext(options)).newPage();
      page.on('console', message => {if (message.type() === 'error') errors.push(message.text());});
      page.on('pageerror', error => errors.push(String(error)));
      return page;
    };

    // In a wide window the menu stands beside the page, whole, with the
    // entry of the page marked, and there is no switch.
    const wide = await open({viewport: {width: 1280, height: 800}});
    for (const [address] of Object.entries(PAGES)) {
      await wide.goto(server.base + address);
      const state = await wide.evaluate(shown);
      assert.deepEqual(state.groups, GROUPS, address);
      assert.deepEqual(state.current, [address], address);
      assert.deepEqual([state.switch, state.beside, state.overflow, state.links.length], [null, true, false, 11], address);
    }
    // Every entry leads to a page, and the two of the content leave the
    // settings for the editor.
    for (const [address] of (await wide.evaluate(shown)).links) {
      const response = await wide.goto(server.base + address);
      assert.equal(response.status(), 200, address);
      assert.equal(await wide.locator('nav.sidebar').count(), address.startsWith('/editor') ? 0 : 1, address);
    }
    // The address of the account leads to the page about the application
    // on an instance without sign-in.
    await wide.goto(server.base + '/settings/application');
    assert.equal(new URL(wide.url()).pathname, '/settings/about');
    // The address of the page that held the settings of five of the pages
    // leads to the layout.
    await wide.goto(server.base + '/settings/appearance');
    assert.equal(new URL(wide.url()).pathname, '/settings/layout');

    // In a narrow window the menu is closed above the page behind a switch
    // that names the group and the page. The switch opens it, by a click
    // and from the keyboard, with and without scripts.
    for (const javaScriptEnabled of [true, false]) {
      const narrow = await open({viewport: {width: 360, height: 700}, javaScriptEnabled});
      const read = async () => javaScriptEnabled ? narrow.evaluate(shown) : null;
      for (const [address, label] of Object.entries(PAGES)) {
        await narrow.goto(server.base + address);
        const links = narrow.locator('nav.sidebar a');
        assert.equal(await links.first().isVisible(), false, address);
        assert.match(await narrow.locator('.sidebar-current').innerText(), new RegExp('^' + label.replace('›', '›') + '\\s+Menu'), address);
        await narrow.locator('.sidebar-current').click();
        assert.equal(await links.first().isVisible(), true, address);
        assert.equal(await links.count(), 11, address);
        const state = await read();
        if (state) {
          assert.deepEqual([state.above, state.overflow, state.current], [true, false, [address]], address);
          assert.ok(state.links.every(([, height]) => height >= 44), address);
        }
        await narrow.locator('.sidebar-current').click();
        assert.equal(await links.first().isVisible(), false, address);
      }
      await narrow.goto(server.base + '/settings/theme');
      await narrow.locator('#settings-nav-toggle').focus();
      await narrow.keyboard.press('Space');
      assert.equal(await narrow.locator('nav.sidebar a').first().isVisible(), true);
      await Promise.all([narrow.waitForNavigation(), narrow.getByRole('link', {name: 'About'}).click()]);
      assert.equal(new URL(narrow.url()).pathname, '/settings/about');
      // The menu is closed again on the page an entry leads to.
      assert.equal(await narrow.locator('nav.sidebar a').first().isVisible(), false);
    }

    assert.deepEqual(errors, []);
    console.log('PASS: the menu of the settings is grouped, marks the page that is shown, and opens behind a switch in a narrow window');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
