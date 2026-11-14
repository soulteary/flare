// FLARE_BINARY=/path/to/binary node tests/theme/modes.browser.cjs
// Light and dark, as documented in docs/theme-files.md: a theme with values
// for both follows the device, the site setting can fix one of the two, and a
// theme without such values looks the same whatever the device asks for.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

// Dark as written, with other values for light. The radius differs as well:
// a mode can replace any token, not only colors.
const PAIRED = `contract: 1
name: Paired
tokens:
  background: "#10241b"
  primary: "#e8f1e4"
  accent: "#f2b84b"
  radius: 8px
modes:
  light:
    background: "#f6f3ea"
    primary: "#2b2a27"
    muted: "#6b6860"
    radius: 2px
`;

const LIGHT = {
  background: 'rgb(246, 243, 234)',
  text: 'rgb(43, 42, 39)',
  icon: 'rgb(43, 42, 39)',
  description: 'rgb(107, 104, 96)',
  radius: '2px',
};
// What dark does not set keeps its default: descriptions use the accent color.
const DARK = {
  background: 'rgb(16, 36, 27)',
  text: 'rgb(232, 241, 228)',
  icon: 'rgb(232, 241, 228)',
  description: 'rgb(242, 184, 75)',
  radius: '8px',
};
const BLACKBOARD = {
  background: 'rgb(26, 26, 26)',
  text: 'rgb(255, 253, 234)',
  icon: 'rgb(255, 253, 234)',
  description: 'rgb(92, 92, 92)',
  radius: '4px',
};

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-modes-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'),
    'Locale: en\nTheme: blackboard\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowWeather: false\n');
  await fs.writeFile(path.join(dir, 'apps.yml'),
    'links:\n- name: Application\n  link: https://example.com/a\n  icon: circle\n  desc: A description\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'),
    'links:\n- name: One\n  link: https://example.com/b\n  icon: home\n');
  await fs.mkdir(path.join(dir, 'themes'));
  await fs.writeFile(path.join(dir, 'themes', 'paired.yml'), PAIRED);
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

const measure = page => page.evaluate(() => {
  const style = (selector, property) => getComputedStyle(document.querySelector(selector))[property];
  return {
    background: style('body', 'backgroundColor'),
    text: style('.app-title', 'color'),
    icon: style('.app-icon svg', 'fill'),
    description: style('.app-desc', 'color'),
    radius: style('.app-item', 'borderTopLeftRadius'),
  };
});

(async () => {
  const browser = await chromium.launch({headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox']});
  const server = await start();
  try {
    const page = await browser.newPage({viewport: {width: 1280, height: 900}});
    page.setDefaultTimeout(10000);
    const still = () => page.addStyleTag({content: '* { transition: none !important; }'});
    const home = async scheme => {
      await page.emulateMedia({colorScheme: scheme});
      await page.goto(server.base, {waitUntil: 'load'});
      await still();
      // The corners of an application show under the pointer.
      await page.locator('.app-item').hover();
      return measure(page);
    };
    // The home page for a device that asks for light and for one that asks
    // for dark. The second is measured without loading the page again.
    const both = async () => {
      const light = await home('light');
      await page.emulateMedia({colorScheme: 'dark'});
      return {light, dark: await measure(page)};
    };
    const settings = async () => {
      await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
      await still();
    };
    const submit = async locator => {
      const [saved] = await Promise.all([
        page.waitForResponse(response => response.request().method() === 'POST'),
        locator.click(),
      ]);
      assert.equal(saved.status(), 200, await saved.text());
      await page.waitForLoadState('load');
      await still();
    };
    const setMode = async mode => {
      await settings();
      await page.locator('#theme-mode').selectOption(mode);
      await submit(page.locator('.theme-mode-form input.btn-submit'));
      assert.equal(await page.locator('#theme-mode').inputValue(), mode);
    };

    // A bundled theme is the same for every device, and the setting is not
    // offered for it. The card of the theme file shows both sets of colors.
    assert.deepEqual(await both(), {light: BLACKBOARD, dark: BLACKBOARD});
    await settings();
    assert.equal(await page.locator('#theme-mode').count(), 0);
    assert.equal(await page.locator('.theme-file input[value="paired"]').count(), 1);
    assert.equal(await page.locator('.theme-file .theme-color').count(), 6);

    // Selected, the theme follows the device, on the settings pages too.
    await submit(page.locator('.theme-file input[value="paired"]'));
    assert.equal(await page.locator('#theme-mode').inputValue(), 'auto');
    assert.deepEqual(await both(), {light: LIGHT, dark: DARK});
    await page.emulateMedia({colorScheme: 'light'});
    await settings();
    const pageBackground = () => page.evaluate(() => getComputedStyle(document.body).backgroundColor);
    assert.equal(await pageBackground(), LIGHT.background);
    await page.emulateMedia({colorScheme: 'dark'});
    assert.equal(await pageBackground(), DARK.background);

    // Set to one of the two, every device gets that one.
    await setMode('dark');
    assert.match(await fs.readFile(path.join(server.dir, 'config.yml'), 'utf8'), /^ThemeMode: dark$/m);
    assert.deepEqual(await both(), {light: DARK, dark: DARK});
    await setMode('light');
    assert.deepEqual(await both(), {light: LIGHT, dark: LIGHT});
    await setMode('auto');
    assert.deepEqual(await both(), {light: LIGHT, dark: DARK});

    // An edit of the file is followed in both modes.
    await fs.writeFile(path.join(server.dir, 'themes', 'paired.yml'),
      PAIRED.replace('radius: 8px', 'radius: 12px').replace('#f6f3ea', '#ffffff'));
    assert.deepEqual(await both(), {
      light: {...LIGHT, background: 'rgb(255, 255, 255)'},
      dark: {...DARK, radius: '12px'},
    });

    // The setting is kept while another theme is in use, and has no effect on it.
    await setMode('dark');
    await settings();
    await submit(page.locator('.theme-blackboard input'));
    assert.equal(await page.locator('#theme-mode').count(), 0);
    assert.deepEqual(await both(), {light: BLACKBOARD, dark: BLACKBOARD});
    await settings();
    await submit(page.locator('.theme-file input[value="paired"]'));
    assert.equal(await page.locator('#theme-mode').inputValue(), 'dark');
    console.log('PASS: a theme with light and dark values follows the device, the site setting fixes one, other themes are unaffected');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
