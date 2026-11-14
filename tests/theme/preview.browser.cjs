// FLARE_BINARY=/path/to/binary node tests/theme/preview.browser.cjs
// The preview of a theme, as documented in docs/theme-files.md: the theme
// page links to a preview of every theme, the preview is one page shown with
// that theme, and nothing is selected until the administrator says so.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

const FOREST = `contract: 1
name: Forest at dusk
tokens:
  background: "#10241b"
  primary: "#e8f1e4"
  accent: "#f2b84b"
layout:
  HideAppDescriptions: true
`;
const BLACKBOARD = 'rgb(26, 26, 26)';
const FOREST_BACKGROUND = 'rgb(16, 36, 27)';
const TRON = 'rgb(36, 43, 51)';

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-preview-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'),
    'Locale: en\nTitle: Site\nTheme: blackboard\nIconMode: DEFAULT\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowWeather: false\n');
  await fs.writeFile(path.join(dir, 'apps.yml'),
    'links:\n- name: Application\n  link: https://example.com/a\n  icon: circle\n  desc: A description\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'),
    'links:\n- name: One\n  link: https://example.com/b\n  icon: home\n');
  await fs.mkdir(path.join(dir, 'themes'));
  await fs.writeFile(path.join(dir, 'themes', 'forest.yml'), FOREST);
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

// What a page shows: its background, the notice of a preview if there is
// one, and whether the description of an application is displayed.
const look = page => page.evaluate(() => {
  const notice = document.querySelector('.theme-preview-bar');
  const description = document.querySelector('.app-desc');
  return {
    background: getComputedStyle(document.body).backgroundColor,
    notice: notice ? notice.querySelector('span').textContent : null,
    descriptions: description ? getComputedStyle(description).display !== 'none' : null,
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
    const open = async address => {
      await page.goto(server.base + address, {waitUntil: 'load'});
      await still();
      return look(page);
    };
    const preview = async card => {
      await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
      await Promise.all([page.waitForURL(/theme-preview=/), page.locator(card + ' .theme-preview-link').click()]);
      await page.waitForLoadState('load');
      await still();
      return look(page);
    };
    const selected = async () => (await fs.readFile(path.join(server.dir, 'config.yml'), 'utf8')).match(/^Theme: (.*)$/m)[1];

    const plain = {background: BLACKBOARD, notice: null, descriptions: true};
    assert.deepEqual(await open('/'), plain);

    // The preview is the home page with the tokens and the layout of the
    // theme, and a notice that says which theme it is.
    const forest = {background: FOREST_BACKGROUND, notice: 'Preview of the theme “Forest at dusk”. It is not selected: only this page is shown with it.', descriptions: false};
    assert.deepEqual(await preview('.theme-file'), forest);
    assert.match(page.url(), /\/\?theme-preview=forest$/);

    // Nothing was selected: the home page, loaded again, is as before.
    assert.equal(await selected(), 'blackboard');
    assert.deepEqual(await open('/'), plain);

    // The way back leads to the settings, still with the selected theme.
    await preview('.theme-file');
    await Promise.all([page.waitForURL(/\/settings\/theme$/), page.locator('.theme-preview-bar a').click()]);
    assert.equal((await look(page)).background, BLACKBOARD);
    assert.equal(await selected(), 'blackboard');

    // A bundled theme is previewed in the same way.
    assert.deepEqual(await preview('.theme-tron'), {background: TRON, notice: 'Preview of the theme “tron”. It is not selected: only this page is shown with it.', descriptions: true});

    // The button of the notice selects the previewed theme.
    await preview('.theme-file');
    const [saved] = await Promise.all([
      page.waitForResponse(response => response.request().method() === 'POST'),
      page.locator('.theme-preview-bar button').click(),
    ]);
    assert.equal(saved.status(), 200, await saved.text());
    await page.waitForLoadState('load');
    assert.match(page.url(), /\/settings\/theme$/);
    assert.equal(await selected(), 'forest');
    assert.deepEqual(await open('/'), {background: FOREST_BACKGROUND, notice: null, descriptions: false});
    // The preview of the selected theme is the page itself.
    assert.deepEqual(await open('/?theme-preview=forest'), {background: FOREST_BACKGROUND, notice: null, descriptions: false});
    console.log('PASS: a theme is previewed on one page from its card, and selected only by the button of the notice');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
