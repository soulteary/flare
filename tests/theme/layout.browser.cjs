// FLARE_BINARY=/path/to/binary node tests/theme/layout.browser.cjs
// The layout of a theme, as documented in docs/theme-files.md: a theme can
// specify layout options, they replace those of the site while the theme is
// selected, and the appearance page can turn them off one by one or all at
// once without the settings of the site ever being rewritten.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

const ARRANGED = `contract: 1
name: Arranged
tokens:
  background: "#10241b"
  primary: "#e8f1e4"
  accent: "#f2b84b"
layout:
  DesktopColumns: 6
  TitleSize: compact
  HideAppDescriptions: true
  CategoryNavigation: tabs
`;

// What the site is set to, and what the theme makes of it.
const SITE = {columns: '4', title: 'prominent', descriptions: true, tabs: false};
const THEMED = {columns: '6', title: 'compact', descriptions: false, tabs: true};

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-layout-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'),
    'Locale: en\nTitle: Site\nTheme: blackboard\nIconMode: DEFAULT\nShowApps: true\nShowBookmarks: true\nShowTitle: true\nShowWeather: false\n' +
    'Homepage:\n  DesktopColumns: "4"\n  TitleSize: prominent\n');
  await fs.writeFile(path.join(dir, 'apps.yml'),
    'links:\n- name: Application\n  link: https://example.com/a\n  icon: circle\n  desc: A description\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'),
    'categories:\n- id: a\n  title: First\n- id: b\n  title: Second\n' +
    'links:\n- name: One\n  link: https://example.com/b\n  category: a\n- name: Two\n  link: https://example.com/c\n  category: b\n');
  await fs.mkdir(path.join(dir, 'themes'));
  await fs.writeFile(path.join(dir, 'themes', 'arranged.yml'), ARRANGED);
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

// The layout of the home page as a visitor gets it.
const layout = page => page.evaluate(() => {
  const classes = Array.from(document.body.classList);
  const pick = prefix => (classes.find(name => name.startsWith(prefix)) || '').slice(prefix.length);
  return {
    columns: pick('desktop-columns-'),
    title: pick('homepage-title-'),
    descriptions: getComputedStyle(document.querySelector('.app-desc')).display !== 'none',
    tabs: document.querySelector('.bookmark-category-nav') !== null,
  };
});

(async () => {
  const browser = await chromium.launch({headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox']});
  const server = await start();
  try {
    const page = await browser.newPage({viewport: {width: 1280, height: 900}});
    page.setDefaultTimeout(10000);
    const home = async () => {
      await page.goto(server.base, {waitUntil: 'load'});
      return layout(page);
    };
    const submit = async locator => {
      const [saved] = await Promise.all([
        page.waitForResponse(response => response.request().method() === 'POST'),
        locator.click(),
      ]);
      assert.equal(saved.status(), 200, await saved.text());
      await page.waitForLoadState('load');
    };
    const selectTheme = async selector => {
      await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
      await submit(page.locator(selector));
    };
    // The appearance page: the switch, and for each remark whether its box
    // is ticked, or null when it has none.
    const appearance = async () => {
      await page.goto(server.base + '/settings/appearance', {waitUntil: 'load'});
      return page.evaluate(() => ({
        all: document.querySelector('#theme-layout-all')?.value ?? null,
        boxes: Object.fromEntries(Array.from(document.querySelectorAll('.theme-layout-note'), note => {
          const box = note.querySelector('input[type="checkbox"]');
          const option = note.closest('.form-group').querySelector('select').name;
          return [option, box ? box.checked : null];
        })),
        // The controls keep showing the settings of the site.
        site: {columns: document.querySelector('#desktop-columns').value, title: document.querySelector('#title-size').value},
      }));
    };
    const save = () => submit(page.locator('input.btn-submit'));
    const configuration = () => fs.readFile(path.join(server.dir, 'config.yml'), 'utf8');

    // A bundled theme leaves the layout to the site, and the appearance
    // page has nothing to say about it.
    assert.deepEqual(await home(), SITE);
    assert.deepEqual(await appearance(), {all: null, boxes: {}, site: {columns: '4', title: 'prominent'}});

    // The card names what the theme sets. Selected, the theme's layout
    // replaces the options it specifies.
    await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
    assert.match(await page.locator('.theme-file').innerText(), /Sets the layout: Homepage title size, Application descriptions, Bookmark category navigation, Desktop columns/);
    await selectTheme('.theme-file input[value="arranged"]');
    assert.deepEqual(await home(), THEMED);
    const applied = {'title-size': false, 'hide-app-descriptions': false, 'category-navigation': false, 'desktop-columns': false};
    assert.deepEqual(await appearance(), {all: 'apply', boxes: applied, site: {columns: '4', title: 'prominent'}});

    // One option switched back to the setting of the site.
    await page.locator('input[name="theme-layout-ignore-DesktopColumns"]').check();
    await save();
    assert.deepEqual(await home(), {...THEMED, columns: '4'});
    assert.deepEqual((await appearance()).boxes, {...applied, 'desktop-columns': true});

    // The setting of the site for that option applies when it is changed.
    await page.locator('#desktop-columns').selectOption('8');
    await save();
    assert.deepEqual(await home(), {...THEMED, columns: '8'});
    await appearance();
    await page.locator('#desktop-columns').selectOption('4');
    await save();

    // All of it turned off at once: the site settings apply, the options
    // have no boxes, and the choice per option is kept.
    await appearance();
    await page.locator('#theme-layout-all').selectOption('ignore');
    await save();
    assert.deepEqual(await home(), SITE);
    assert.deepEqual(await appearance(), {all: 'ignore',
      boxes: {'title-size': null, 'hide-app-descriptions': null, 'category-navigation': null, 'desktop-columns': null},
      site: {columns: '4', title: 'prominent'}});
    await page.locator('#theme-layout-all').selectOption('apply');
    await save();
    assert.deepEqual(await home(), {...THEMED, columns: '4'});
    assert.deepEqual((await appearance()).boxes, {...applied, 'desktop-columns': true});

    // Unticked, the whole layout of the theme applies again. The settings
    // of the site were never rewritten with values of the theme.
    await page.locator('input[name="theme-layout-ignore-DesktopColumns"]').uncheck();
    await save();
    assert.deepEqual(await home(), THEMED);
    const stored = await configuration();
    assert.match(stored, /^ {4}DesktopColumns: "4"$/m);
    assert.match(stored, /^ {4}TitleSize: prominent$/m);
    assert.match(stored, /^ {4}HideAppDescriptions: false$/m);

    // An edit of the layout in the file is followed like an edit of a token.
    await fs.writeFile(path.join(server.dir, 'themes', 'arranged.yml'), ARRANGED.replace('DesktopColumns: 6', 'DesktopColumns: 5').replace('  CategoryNavigation: tabs\n', ''));
    assert.deepEqual(await home(), {...THEMED, columns: '5', tabs: false});

    // Another theme, and the layout is that of the site again.
    await selectTheme('.theme-blackboard input');
    assert.deepEqual(await home(), SITE);
    assert.deepEqual(await appearance(), {all: null, boxes: {}, site: {columns: '4', title: 'prominent'}});
    console.log('PASS: the layout of a theme replaces site options while selected and can be turned off per option or as a whole');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
