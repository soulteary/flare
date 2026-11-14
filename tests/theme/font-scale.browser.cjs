// FLARE_BINARY=/path/to/binary node tests/theme/font-scale.browser.cjs
// The font scale of a theme, as documented in docs/theme-tokens.md: it
// multiplies the text of the navigation pages, an application keeps room for
// its two lines, the settings pages keep their sizes, and the text size the
// administrator chose for readability is never made smaller by a theme.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

const theme = scale => `contract: 1\ntokens:\n  background: "#10241b"\n  primary: "#e8f1e4"\n  accent: "#f2b84b"\n  font-scale: ${scale}\n`;
const config = textSize => 'Locale: en\nTitle: Site\nTheme: blackboard\nIconMode: DEFAULT\nShowApps: true\nShowBookmarks: true\n' +
  'ShowSearchComponent: true\nShowWeather: false\nHomepage:\n  NavigationTextSize: ' + textSize + '\n';

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-font-scale-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'), config('standard'));
  await fs.writeFile(path.join(dir, 'apps.yml'),
    'links:\n- name: Application\n  link: https://example.com/a\n  icon: circle\n  desc: A description of it\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'),
    'categories:\n- id: a\n  title: First\nlinks:\n- name: One\n  link: https://example.com/b\n  icon: home\n  category: a\n');
  await fs.mkdir(path.join(dir, 'themes'));
  for (const [name, scale] of [['small', '0.8'], ['scaled', '1.25'], ['big', '1.5']]) {
    await fs.writeFile(path.join(dir, 'themes', name + '.yml'), theme(scale));
  }
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

// Text sizes in pixels, and whether the two lines of an application fit.
const sizes = page => page.evaluate(() => {
  const size = selector => parseFloat(getComputedStyle(document.querySelector(selector)).fontSize);
  const application = document.querySelector('.app-item');
  return {
    title: size('.app-title'),
    description: size('.app-desc'),
    bookmark: size('a.bookmark'),
    groupTitle: size('.bookmark-group-title'),
    // The text of a section heading is a link with a size of its own.
    heading: size('#container-apps h2 a'),
    search: size('#search'),
    applicationHeight: application.getBoundingClientRect().height,
    cut: application.scrollHeight > application.clientHeight,
  };
});
const scaled = (base, factor) => Object.fromEntries(Object.entries(base).map(([name, value]) =>
  [name, typeof value === 'number' ? Math.round(value * factor * 100) / 100 : value]));
const rounded = measured => Object.fromEntries(Object.entries(measured).map(([name, value]) =>
  [name, typeof value === 'number' ? Math.round(value * 100) / 100 : value]));

(async () => {
  const browser = await chromium.launch({headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox']});
  const server = await start();
  try {
    const page = await browser.newPage({viewport: {width: 1280, height: 900}});
    page.setDefaultTimeout(10000);
    const home = async () => {
      await page.goto(server.base, {waitUntil: 'load'});
      return rounded(await sizes(page));
    };
    const select = async selector => {
      await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
      const [saved] = await Promise.all([
        page.waitForResponse(response => response.request().method() === 'POST'),
        page.locator(selector).click(),
      ]);
      assert.equal(saved.status(), 200, await saved.text());
      await page.waitForLoadState('load');
    };
    // The settings pages are for the administrator and keep their sizes.
    const settings = async () => {
      await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
      return page.evaluate(() => {
        const size = selector => getComputedStyle(document.querySelector(selector)).fontSize;
        return {body: size('body'), heading: size('.setting-group-container h2'), title: size('.theme-title'), help: size('.theme-help')};
      });
    };

    const standard = {title: 14, description: 11.2, bookmark: 14, groupTitle: 16, heading: 20, search: 20, applicationHeight: 40, cut: false};
    assert.deepEqual(await home(), standard);
    const management = await settings();

    // A theme multiplies every text size. The height of an application
    // grows with them, so that neither of its two lines is cut off, and
    // does not shrink: its icon keeps its size.
    for (const [name, factor] of [['scaled', 1.25], ['big', 1.5], ['small', 0.8]]) {
      await select(`.theme-file input[value="${name}"]`);
      const expected = scaled(standard, factor);
      expected.applicationHeight = Math.max(standard.applicationHeight, expected.applicationHeight);
      assert.deepEqual(await home(), expected, name);
      assert.deepEqual(await settings(), management, name);
    }
    // The other navigation pages follow the same scale.
    await select('.theme-file input[value="scaled"]');
    await page.goto(server.base + '/bookmarks', {waitUntil: 'load'});
    assert.equal(await page.evaluate(() => getComputedStyle(document.querySelector('a.bookmark')).fontSize), '17.5px');
    await page.goto(server.base + '/applications', {waitUntil: 'load'});
    assert.equal(await page.evaluate(() => getComputedStyle(document.querySelector('.app-title')).fontSize), '17.5px');

    // The larger text the administrator chose is a size in rem. A theme can
    // enlarge it, and cannot make it smaller.
    await fs.writeFile(path.join(server.dir, 'config.yml'), config('large').replace('Theme: blackboard', 'Theme: scaled'));
    const text = async () => {
      const measured = await home();
      return {title: measured.title, bookmark: measured.bookmark, description: measured.description, cut: measured.cut};
    };
    await select('.theme-blackboard input');
    assert.deepEqual(await text(), {title: 20, bookmark: 20, description: 16, cut: false});
    await select('.theme-file input[value="small"]');
    assert.deepEqual(await text(), {title: 20, bookmark: 20, description: 16, cut: false});
    await select('.theme-file input[value="big"]');
    assert.deepEqual(await text(), {title: 30, bookmark: 30, description: 24, cut: false});
    console.log('PASS: the font scale of a theme multiplies the text of the navigation pages only and never shrinks the chosen text size');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
