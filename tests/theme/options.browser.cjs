// FLARE_BINARY=/path/to/binary node tests/theme/options.browser.cjs
// The options of a theme, as documented in docs/theme-files.md: a theme file
// declares settings of its own, the theme page offers them once the theme is
// selected, and the values that are chosen change tokens and layout of the
// theme and are written on the body of the navigation pages for its
// stylesheet.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

const CONFIG = 'Locale: en\nTheme: zelda\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowWeather: false\n';
const MANIFEST = `contract: 1
name: Zelda
tokens:
  background: "#10241b"
  primary: "#e8f1e4"
  accent: "#f2b84b"
  radius: 10px
options:
  - id: density
    type: enum
    label: { en: Density, zh: 密度 }
    choices:
      - value: normal
        label: { en: Normal, zh: 标准 }
      - value: compact
        label: { en: Compact, zh: 紧凑 }
        tokens: { radius: 2px }
        layout: { HideAppDescriptions: true }
  - id: frames
    type: boolean
    label: { en: Frames, zh: 边框 }
  - id: highlight
    type: color
    label: { en: Highlight, zh: 强调色 }
    token: accent
`;
// The stylesheet selects by the value of an option.
const STYLESHEET = `body[data-theme-frames="true"] .app-item { outline: 3px solid rgb(1, 2, 3); }
`;

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-options-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'), CONFIG);
  await fs.writeFile(path.join(dir, 'apps.yml'),
    'links:\n- name: Application\n  link: https://example.test/a\n  icon: circle\n  desc: A description\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'),
    'links:\n- name: One\n  link: https://example.test/b\n  icon: home\n');
  await fs.mkdir(path.join(dir, 'themes', 'zelda'), {recursive: true});
  await fs.writeFile(path.join(dir, 'themes', 'zelda', 'theme.yml'), MANIFEST);
  await fs.writeFile(path.join(dir, 'themes', 'zelda', 'theme.css'), STYLESHEET);
  await fs.writeFile(path.join(dir, 'themes', 'plain.yml'), MANIFEST.slice(0, MANIFEST.indexOf('options:')));
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

// What the home page has of the options of the theme.
const look = page => page.evaluate(() => {
  const body = getComputedStyle(document.body);
  const tile = getComputedStyle(document.querySelector('.app-item'));
  const description = document.querySelector('.app-desc');
  return {
    attributes: Object.fromEntries(Array.from(document.body.attributes).filter(attribute => attribute.name.startsWith('data-theme-')).map(attribute => [attribute.name, attribute.value])),
    radius: tile.borderTopLeftRadius,
    outline: tile.outlineStyle === 'none' ? 'none' : `${tile.outlineColor} ${tile.outlineWidth}`,
    accent: body.getPropertyValue('--color-accent').trim(),
    descriptions: description !== null && getComputedStyle(description).display !== 'none',
  };
});

// The form of the options as the theme page shows it.
const form = page => page.evaluate(() => {
  const root = document.querySelector('.theme-options-form');
  if (!root) return null;
  const select = root.querySelector('select');
  return {
    title: root.querySelector('h2').textContent,
    labels: Array.from(root.querySelectorAll('label'), label => label.textContent.trim()),
    density: select.value,
    values: Array.from(select.options, option => `${option.value}=${option.textContent.trim()}`),
    frames: root.querySelector('input[type="checkbox"]').checked,
    highlight: root.querySelector('input[type="color"]').value,
    status: Array.from(root.querySelectorAll('.theme-notice[role="status"]'), notice => notice.textContent),
  };
});

(async () => {
  const browser = await chromium.launch({headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox']});
  const server = await start();
  try {
    const page = await browser.newPage({viewport: {width: 1280, height: 900}});
    page.setDefaultTimeout(10000);
    const elsewhere = [];
    page.on('request', request => {
      if (!request.url().startsWith(server.base + '/') && !request.url().startsWith('data:')) elsewhere.push(request.url());
    });
    const home = async (query = '') => {
      await page.goto(server.base + '/' + query, {waitUntil: 'load'});
      // The corners of a tile show while the pointer is on it.
      await page.addStyleTag({content: '* { transition: none !important; }'});
      await page.locator('.app-item').hover();
      return look(page);
    };
    const settings = async () => {
      await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
      return form(page);
    };
    const submit = async selector => {
      const [response] = await Promise.all([
        page.waitForResponse(candidate => candidate.request().method() === 'POST'),
        page.locator(selector + ' input.btn-submit').click(),
      ]);
      await page.waitForLoadState('load');
      return response.status();
    };
    const stored = async () => {
      const config = await fs.readFile(path.join(server.dir, 'config.yml'), 'utf8');
      return config.includes('Options:') ? config.slice(config.indexOf('Options:')).split('\n').slice(1).filter(line => /^ {12}\S/.test(line)).map(line => line.trim()) : [];
    };
    const defaults = {attributes: {'data-theme-density': 'normal', 'data-theme-frames': 'false'}, radius: '10px', outline: 'none', accent: '#f2b84b', descriptions: true};

    // Until something is chosen the options have their defaults: the pages
    // are those of the theme file, with the values on the body.
    assert.deepEqual(await home(), defaults);

    // The theme page offers the options of the selected theme, each as the
    // kind of control it is, with the labels of the interface language.
    assert.deepEqual(await settings(), {title: 'Options of Zelda', labels: ['Density', 'Frames', 'Highlight'], density: 'normal',
      values: ['normal=Normal', 'compact=Compact'], frames: false, highlight: '#f2b84b', status: []});
    assert.equal(await page.locator('.theme-file:has(input[value="zelda"]) .theme-about').last().textContent(), 'Has options of its own.');

    // Values that are chosen in the form are stored and applied: a token of
    // the value, its layout, the color of the color option, and the rule of
    // the stylesheet that selects by the value of an option.
    await page.locator('.theme-options-form select').selectOption('compact');
    await page.locator('.theme-options-form input[type="checkbox"]').check();
    await page.locator('.theme-options-form input[type="color"]').fill('#00ff88');
    assert.equal(await submit('.theme-options-form'), 200);
    const saved = await form(page);
    assert.deepEqual([saved.density, saved.frames, saved.highlight, saved.status], ['compact', true, '#00ff88', ['The options were saved and applied.']]);
    assert.deepEqual(await stored(), ['density: compact', 'frames: "true"', "highlight: '#00ff88'"]);
    const chosen = {attributes: {'data-theme-density': 'compact', 'data-theme-frames': 'true'}, radius: '2px', outline: 'rgb(1, 2, 3) 3px', accent: '#00ff88', descriptions: false};
    assert.deepEqual(await home(), chosen);
    // The form shows what is stored when the page is opened again.
    const reopened = await settings();
    assert.deepEqual([reopened.density, reopened.frames, reopened.highlight, reopened.status], ['compact', true, '#00ff88', []]);
    // The settings take the tokens, and have no attribute for a stylesheet
    // they do not load.
    assert.deepEqual(await page.evaluate(() => [getComputedStyle(document.body).getPropertyValue('--color-accent').trim(),
      Array.from(document.body.attributes).filter(attribute => attribute.name.startsWith('data-theme-')).length]), ['#00ff88', 0]);

    // An option that is put back to its default is no longer stored.
    await page.locator('.theme-options-form select').selectOption('normal');
    await page.locator('.theme-options-form input[type="checkbox"]').uncheck();
    assert.equal(await submit('.theme-options-form'), 200);
    assert.deepEqual(await stored(), ["highlight: '#00ff88'"]);
    assert.deepEqual(await home(), {...defaults, accent: '#00ff88'});

    // The defaults of the theme are put back in one step.
    await settings();
    assert.equal(await submit('.theme-options-reset-form'), 200);
    assert.deepEqual(await stored(), []);
    assert.deepEqual(await home(), defaults);

    // Values written into config.yml by hand show on the next page load,
    // and one that is no value of the option changes nothing.
    await fs.writeFile(path.join(server.dir, 'config.yml'), CONFIG + 'ThemeSettings:\n    zelda:\n        Options:\n            density: compact\n            frames: perhaps\n');
    assert.deepEqual(await home(), {...defaults, attributes: {'data-theme-density': 'compact', 'data-theme-frames': 'false'}, radius: '2px', descriptions: false});
    const edited = await settings();
    assert.deepEqual([edited.density, edited.frames, edited.highlight], ['compact', false, '#f2b84b']);

    // The labels follow the interface language.
    await fs.writeFile(path.join(server.dir, 'config.yml'), CONFIG.replace('Locale: en', 'Locale: zh'));
    const chinese = await settings();
    assert.deepEqual([chinese.title, chinese.labels, chinese.values], ['Zelda 的选项', ['密度', '边框', '强调色'], ['normal=标准', 'compact=紧凑']]);
    await fs.writeFile(path.join(server.dir, 'config.yml'), CONFIG);

    // A theme without options has no form and no attributes.
    await settings();
    const [selected] = await Promise.all([
      page.waitForResponse(candidate => candidate.request().method() === 'POST'),
      page.locator('.theme-file input[value="plain"]').click(),
    ]);
    assert.equal(selected.status(), 200);
    await page.waitForLoadState('load');
    assert.equal(await form(page), null);
    assert.equal(await page.locator('.theme-options-reset-form').count(), 0);
    assert.deepEqual((await home()).attributes, {});
    // A preview of the theme with options shows it with its values.
    assert.deepEqual((await home('?theme-preview=zelda')).attributes, {'data-theme-density': 'normal', 'data-theme-frames': 'false'});

    assert.deepEqual(elsewhere, []);
    console.log('PASS: the options of a theme are offered on the theme page, stored for the theme, and change its tokens, its layout and the attributes its stylesheet selects by');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
