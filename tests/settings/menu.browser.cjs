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
  '/settings/themes': 'Look › Theme library', '/settings/theme-access': 'Look › Theme access',
  '/settings/site': 'Site › Site details', '/settings/about': 'Site › About'};
const GROUPS = [['Content', 'Apps and bookmarks', 'Import and export'], ['Home page', 'Modules', 'Search', 'Weather', 'Clock', 'Links'], ['Look', 'Theme', 'Layout', 'Theme library', 'Theme access'], ['Site', 'Site details', 'About']];

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-settings-menu-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'), 'Locale: en\nTitle: Site\nTheme: blackboard\nShowWeather: false\nFooter: <p>Powered by <a href="https://example.org/">an example</a></p>\n');
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
    // Where the footer of the site stands: below the menu and the page,
    // touching neither, and in which column.
    footer: (footer => ({
      below: footer.top >= menu.getBoundingClientRect().bottom && footer.top >= main.getBoundingClientRect().bottom,
      text: document.querySelector('.footer-container').textContent.trim(),
      withPage: Math.abs(footer.left - main.getBoundingClientRect().left) < 1,
      inside: footer.left >= 0 && footer.right <= document.documentElement.clientWidth,
    }))(document.querySelector('.footer-container').getBoundingClientRect()),
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
      assert.deepEqual([state.switch, state.beside, state.overflow, state.links.length], [null, true, false, 13], address);
    }
    // The footer of the site stands below the menu and the page on every
    // page, the short ones among them, at every width, in line with the
    // page.
    for (const width of [1440, 1280, 1024, 900, 769, 768, 600, 360]) {
      const sized = await open({viewport: {width, height: 800}});
      for (const address of [...Object.keys(PAGES), '/settings/application']) {
        await sized.goto(server.base + address);
        const state = await sized.evaluate(shown);
        assert.deepEqual(state.footer, {below: true, text: 'Powered by an example', withPage: true, inside: true}, `${address} at ${width}`);
        assert.equal(state.overflow, false, `${address} at ${width}`);
      }
      await sized.context().close();
    }
    // Every entry leads to a page that stands in the frame of the
    // settings, with its entry marked and no other. The two pages for the
    // content are among them, in a wider column, in the colors of the theme.
    await wide.goto(server.base + '/settings/theme');
    const column = await wide.locator('.setting-group-container').evaluate(element => element.getBoundingClientRect().width);
    for (const [address] of (await wide.evaluate(shown)).links) {
      const response = await wide.goto(server.base + address);
      assert.equal(response.status(), 200, address);
      assert.equal(await wide.locator('nav.sidebar').count(), 1, address);
      assert.deepEqual(await wide.locator('nav.sidebar a[aria-current="page"]').evaluateAll(links => links.map(link => new URL(link.href).pathname)), [address], address);
      assert.equal(await wide.locator('#page-settings .header h1').count(), 1, address);
      assert.equal(await wide.locator('h1').count(), 1, address);
    }
    for (const [address, label] of [['/settings/content', 'Content › Apps and bookmarks'], ['/settings/transfer', 'Content › Import and export']]) {
      await wide.goto(server.base + address);
      const frame = await wide.evaluate(() => {
        const menu = document.querySelector('nav.sidebar').getBoundingClientRect(), page = document.querySelector('.setting-group-container').getBoundingClientRect();
        return {beside: menu.right <= page.left, width: page.width, background: getComputedStyle(document.body).backgroundColor,
          text: getComputedStyle(document.querySelector('.page-title')).color, overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth};
      });
      assert.deepEqual([frame.beside, frame.background, frame.text, frame.overflow], [true, 'rgb(26, 26, 26)', 'rgb(255, 253, 234)', false], address);
      assert.ok(frame.width > column + 300, `${address}: ${frame.width} beside ${column}`);
      // What the page draws its text and its surfaces with are the colors
      // of the theme, not colors of its own.
      const palette = await wide.evaluate(() => {
        const style = getComputedStyle(document.body), read = name => style.getPropertyValue(name).trim().toLowerCase();
        return [read('--text'), read('--flare-primary'), read('--bg'), read('--flare-background')];
      });
      assert.ok(palette[0] !== '' && palette[2] !== '', address);
      assert.deepEqual([palette[0], palette[2]], [palette[1], palette[3]], address);
      const narrow = await open({viewport: {width: 360, height: 700}});
      await narrow.goto(server.base + address);
      assert.match(await narrow.locator('.sidebar-current').innerText(), new RegExp('^' + label + '\\s+Menu'), address);
      assert.equal(await narrow.locator('nav.sidebar a').first().isVisible(), false, address);
      assert.equal(await narrow.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth), false, address);
      await narrow.context().close();
    }
    // The editor asks before it is left with changes that are not saved,
    // by the links it has itself and by those of the frame; and the escape
    // key, which leaves the other pages, does not leave it.
    await wide.goto(server.base + '/settings/content');
    await wide.waitForFunction(() => window.FlareEditorUI?.store);
    const guarded = await wide.evaluate(() => ['#page-settings .header a', '#page-settings nav.sidebar a'].map(selector => [...document.querySelectorAll(selector)].every(link => typeof link.onclick === 'function')));
    assert.deepEqual(guarded, [true, true]);
    // An entry that is being written counts as a change that is not saved.
    await wide.locator('#new-entry').click();
    await wide.locator('#entry-name').fill('Pending');
    const leave = () => wide.evaluate(() => document.querySelector('nav.sidebar a[href="/settings/about"]').click());
    const asked = [];
    wide.once('dialog', dialog => {asked.push(dialog.type()); dialog.dismiss();});
    await leave();
    assert.deepEqual(asked, ['confirm']);
    assert.equal(new URL(wide.url()).pathname, '/settings/content');
    assert.equal(await wide.locator('#entry-name').inputValue(), 'Pending');
    const accept = dialog => dialog.accept();
    wide.on('dialog', accept);
    await Promise.all([wide.waitForURL(server.base + '/settings/about'), leave()]);
    wide.off('dialog', accept);
    // Without a change nothing is asked, and the escape key leaves the
    // editor no more than it did.
    await wide.goto(server.base + '/settings/content');
    await wide.waitForFunction(() => window.FlareEditorUI?.store);
    await wide.keyboard.press('Escape');
    assert.equal(new URL(wide.url()).pathname, '/settings/content');
    await Promise.all([wide.waitForURL(server.base + '/settings/about'), wide.getByRole('link', {name: 'About', exact: true}).click()]);
    // The address of the account leads to the page about the application
    // on an instance without sign-in.
    await wide.goto(server.base + '/settings/application');
    assert.equal(new URL(wide.url()).pathname, '/settings/about');
    // The addresses the editor had, and the page that held the settings of
    // five of the pages, are gone: nothing leads on from them.
    for (const gone of ['/editor', '/editor/transfer', '/settings/appearance']) {
      const response = await wide.request.get(server.base + gone, {maxRedirects: 0});
      assert.equal(response.status(), 404, gone);
    }

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
        assert.equal(await links.count(), 13, address);
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
