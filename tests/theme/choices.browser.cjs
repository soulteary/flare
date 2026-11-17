// FLARE_BINARY=/path/to/binary node tests/theme/choices.browser.cjs
// Themes a visitor can choose, as documented in docs/theme-choices.md: the
// administrator ticks themes on the theme page, the pages get a control
// below the footer, and a visitor who uses it has every page shown with the
// chosen theme, by a cookie no script can read. Another visitor keeps the
// theme of the site. The control works without a script, fits a narrow
// window, and goes away with the last theme to choose.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

const TOKENS = 'tokens:\n  background: "#10241b"\n  primary: "#e8f1e4"\n  accent: "#f2b84b"\n';
const BLACKBOARD = 'rgb(26, 26, 26)', TRON = 'rgb(36, 43, 51)', FOREST = 'rgb(16, 36, 27)', SKY = 'rgb(11, 29, 51)';
const PAGES = ['/', '/applications', '/bookmarks', '/help'];

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-choices-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'),
    'Locale: en\nTitle: Site\nTheme: blackboard\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowWeather: false\nFooter: The footer of the site\n');
  await fs.writeFile(path.join(dir, 'apps.yml'), 'links:\n- name: Alpha\n  link: https://alpha.example.test/a\n  icon: circle\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'), 'links:\n- name: Gamma\n  link: https://gamma.example.test/\n  icon: home\n');
  const themes = path.join(dir, 'themes');
  await fs.mkdir(path.join(themes, 'sky'), {recursive: true});
  await fs.writeFile(path.join(themes, 'forest.yml'), 'contract: 1\nname: Forest & <dusk>\n' + TOKENS);
  await fs.writeFile(path.join(themes, 'sky', 'theme.yml'), 'contract: 1\nname: Sky\n' + TOKENS.replace('#10241b', '#0b1d33'));
  await fs.writeFile(path.join(themes, 'sky', 'theme.css'), '.footer-container { letter-spacing: 3px; }\n');
  const child = spawn(process.env.FLARE_BINARY, [], {cwd: dir, env: {...process.env,
    FLARE_PORT: String(port), FLARE_DISABLE_LOGIN: 'true', FLARE_OFFLINE: 'true', FLARE_BASELINE: '1'}});
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

// What a page shows of the choice: its background, the control, and what
// stands before the control.
const shown = () => {
  const form = document.querySelector('form.theme-choice');
  const state = {background: getComputedStyle(document.body).backgroundColor, control: null,
    spacing: getComputedStyle(document.querySelector('.footer-container')).letterSpacing};
  if (form) {
    const select = form.querySelector('select'), button = form.querySelector('button'), box = form.getBoundingClientRect();
    state.control = {
      options: [...select.options].map(option => [option.value, option.textContent]),
      value: select.value,
      label: form.querySelector('label').textContent,
      labelled: select.labels.length === 1,
      before: form.previousElementSibling && form.previousElementSibling.className,
      heights: [select.offsetHeight, button.offsetHeight],
      within: box.left >= 0 && box.right <= document.documentElement.clientWidth,
      overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      colors: [getComputedStyle(select).color, getComputedStyle(select).backgroundColor],
    };
  }
  return state;
};

(async () => {
  assert.ok(process.env.FLARE_BINARY, 'FLARE_BINARY is required');
  const server = await start();
  const browser = await chromium.launch({executablePath: process.env.CHROMIUM_EXECUTABLE || undefined});
  try {
    const errors = [], elsewhere = [];
    const visitor = async options => {
      const context = await browser.newContext(options);
      const page = await context.newPage();
      page.on('console', message => {if (message.type() === 'error') errors.push(message.text());});
      page.on('pageerror', error => errors.push(String(error)));
      page.on('request', request => {if (!request.url().startsWith(server.base)) elsewhere.push(request.url());});
      const open = async address => {await page.goto(server.base + address); return page.evaluate(shown);};
      // choose uses the control as a visitor does: the list, then the button.
      const choose = async value => {
        await page.getByLabel('Theme', {exact: true}).selectOption(value);
        await Promise.all([page.waitForNavigation(), page.locator('form.theme-choice button').click()]);
        return page.evaluate(shown);
      };
      const cookie = async () => (await context.cookies()).find(entry => entry.name === 'flare_theme');
      return {context, page, open, choose, cookie};
    };
    const admin = await visitor(), first = await visitor(), second = await visitor();
    const tick = async names => {
      await admin.page.goto(server.base + '/settings/theme-access');
      const form = admin.page.locator('form.theme-choices-form');
      for (const box of await form.locator('input[type="checkbox"]').all()) {
        await box.setChecked(names.includes((await box.getAttribute('name')).replace('theme-choice-', '')));
      }
      await Promise.all([admin.page.waitForNavigation(), form.locator('input[type="submit"]').click()]);
      assert.equal(await admin.page.locator('form.theme-choices-form .theme-notice').count(), 1);
    };

    // Before the administrator allows any theme, the pages have no control,
    // and a cookie that names a theme changes nothing.
    await first.context.addCookies([{name: 'flare_theme', value: 'forest', url: server.base}]);
    for (const address of PAGES) {
      const state = await first.open(address);
      assert.deepEqual([state.background, state.control], [BLACKBOARD, null], address);
    }
    await first.context.clearCookies();

    // The administrator ticks three themes. The form lists every theme that
    // can be selected and names the selected one.
    await admin.page.goto(server.base + '/settings/theme-access');
    assert.match(await admin.page.locator('form.theme-choices-form').innerText(), /blackboard \(the selected theme, shown without a choice\)/);
    await tick(['tron', 'forest', 'sky']);
    const offered = [['', 'blackboard (theme of the site)'], ['tron', 'tron'], ['forest', 'Forest & <dusk>'], ['sky', 'Sky']];

    // Every page has the control below its footer, with the theme of the
    // site marked, in the colors of the page.
    for (const address of PAGES) {
      const state = await first.open(address);
      assert.equal(state.background, BLACKBOARD, address);
      assert.deepEqual(state.control.options, offered, address);
      assert.deepEqual([state.control.value, state.control.label, state.control.labelled, state.control.before], ['', 'Theme', true, 'footer-container'], address);
      assert.ok(state.control.heights.every(height => height >= 44), address);
      assert.deepEqual(state.control.colors, ['rgb(255, 253, 234)', BLACKBOARD], address);
    }

    // A choice leads back to the page it was made on, which is shown with
    // the chosen theme, and so is every other page from then on.
    for (const address of PAGES) {
      await first.open(address);
      const state = await first.choose('forest');
      assert.equal(new URL(first.page.url()).pathname, address);
      assert.deepEqual([state.background, state.control.value], [FOREST, 'forest'], address);
      await first.choose('');
    }
    await first.open('/');
    await first.choose('forest');
    for (const address of PAGES) {
      const state = await first.open(address);
      assert.deepEqual([state.background, state.control.value, state.control.colors[1]], [FOREST, 'forest', FOREST], address);
    }
    // The cookie holds the name of the theme, for a year, and no script of
    // a page can read it.
    const kept = await first.cookie();
    assert.deepEqual([kept.value, kept.path, kept.httpOnly, kept.sameSite], ['forest', '/', true, 'Lax']);
    assert.ok(kept.expires - Date.now() / 1000 > 360 * 24 * 60 * 60);
    assert.equal(await first.page.evaluate(() => document.cookie.includes('flare_theme')), false);

    // Another visitor, and the administrator, keep the theme of the site.
    for (const address of PAGES) {
      const state = await second.open(address);
      assert.deepEqual([state.background, state.control.value], [BLACKBOARD, ''], address);
    }
    assert.equal(await admin.page.evaluate(() => getComputedStyle(document.body).backgroundColor), BLACKBOARD);

    // A bundled theme, and a theme with a stylesheet, which is loaded for
    // the visitor who chose it and for nobody else.
    await first.open('/bookmarks');
    assert.equal((await first.choose('tron')).background, TRON);
    const sky = await first.choose('sky');
    assert.deepEqual([sky.background, sky.spacing, new URL(first.page.url()).pathname], [SKY, '3px', '/bookmarks']);
    assert.equal((await second.open('/bookmarks')).spacing, 'normal');

    // A search keeps the choice.
    await first.open('/?search=alpha');
    assert.equal((await first.page.evaluate(shown)).background, SKY);

    // In a narrow window the control stays inside the page.
    const narrow = await visitor({viewport: {width: 320, height: 640}});
    await narrow.context.addCookies([{name: 'flare_theme', value: 'forest', url: server.base}]);
    for (const address of PAGES) {
      const state = await narrow.open(address);
      assert.deepEqual([state.background, state.control.within, state.control.overflow], [FOREST, true, false], address);
    }

    // Without scripts the control works the same.
    const plain = await visitor({javaScriptEnabled: false});
    await plain.page.goto(server.base + '/applications');
    await plain.page.getByLabel('Theme', {exact: true}).selectOption('forest');
    await Promise.all([plain.page.waitForNavigation(), plain.page.locator('form.theme-choice button').click()]);
    assert.equal(new URL(plain.page.url()).pathname, '/applications');
    assert.equal((await plain.cookie()).value, 'forest');
    assert.match(await plain.page.content(), /--color-background:#10241b/);

    // The theme of the site gives the choice up, and the cookie goes.
    await first.open('/');
    const back = await first.choose('');
    assert.deepEqual([back.background, back.control.value], [BLACKBOARD, '']);
    assert.equal(await first.cookie(), undefined);

    // A theme the administrator takes off the list is no longer shown to a
    // visitor who chose it, and is no longer offered.
    await first.choose('forest');
    await tick(['tron']);
    const left = await first.open('/');
    assert.deepEqual([left.background, left.control.value, left.control.options.map(option => option[0])], [BLACKBOARD, '', ['', 'tron']]);
    // With the last theme the control goes.
    await tick([]);
    for (const address of PAGES) {
      const state = await first.open(address);
      assert.deepEqual([state.background, state.control], [BLACKBOARD, null], address);
    }

    assert.deepEqual(elsewhere, []);
    assert.deepEqual(errors, []);
    console.log('PASS: a visitor chooses among the themes the administrator ticked, every page follows the choice, and other visitors keep the theme of the site');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
