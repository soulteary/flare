// FLARE_BINARY=/path/to/binary node tests/theme/safe-mode.browser.cjs
// Safe mode for themes, as documented in docs/theme-files.md: a page can be
// shown without the themes of the themes directory, and an instance can be
// started without reading the directory. The theme of this test has a
// stylesheet that hides parts of the home page.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

const MANIFEST = `contract: 1
name: Hidden
tokens:
  background: "#10241b"
  primary: "#e8f1e4"
  accent: "#f2b84b"
layout:
  DesktopColumns: 6
`;
const STYLESHEET = `/* A stylesheet that takes away what a visitor needs. */
#container-apps, .toolbar-container { display: none; }
body { background: rgb(200, 0, 0) !important; }
`;

const THEMED = 'rgb(200, 0, 0)';
const THEME_BACKGROUND = 'rgb(16, 36, 27)';
// The colors of the stylesheet of the application, used without a theme.
const DEFAULT_BACKGROUND = 'rgb(45, 52, 54)';
const BLACKBOARD_BACKGROUND = 'rgb(26, 26, 26)';

async function start({env = {}, args = []} = {}) {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-safe-mode-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'),
    'Locale: en\nTheme: hidden\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowWeather: false\nHomepage:\n    DesktopColumns: "4"\n');
  await fs.writeFile(path.join(dir, 'apps.yml'),
    'links:\n- name: Application\n  link: https://example.com/a\n  icon: circle\n  desc: A description\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'),
    'links:\n- name: One\n  link: https://example.com/b\n  icon: home\n');
  const theme = path.join(dir, 'themes', 'hidden');
  await fs.mkdir(theme, {recursive: true});
  await fs.writeFile(path.join(theme, 'theme.yml'), MANIFEST);
  await fs.writeFile(path.join(theme, 'theme.css'), STYLESHEET);
  // A file that would be reported if the directory were read.
  await fs.writeFile(path.join(dir, 'themes', 'broken.yml'), MANIFEST + 'tokens: red\n');
  const child = spawn(process.env.FLARE_BINARY, args, {cwd: dir, env: {...process.env,
    FLARE_PORT: String(port), FLARE_DISABLE_LOGIN: 'true', FLARE_OFFLINE: 'true', FLARE_BASELINE: '1', ...env}});
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

// What a visitor finds on the home page.
const look = page => page.evaluate(() => {
  const shown = selector => {
    const element = document.querySelector(selector);
    return element !== null && getComputedStyle(element).display !== 'none' && element.getClientRects().length > 0;
  };
  return {
    background: getComputedStyle(document.body).backgroundColor,
    applications: shown('#container-apps .app-item'),
    toolbar: shown('.toolbar-container'),
    stylesheets: document.querySelectorAll('link[rel="stylesheet"][href^="/theme/"]').length,
    columns: Array.from(document.body.classList).filter(name => name.startsWith('desktop-columns-')),
    notice: Array.from(document.querySelectorAll('.theme-preview-bar span'), notice => notice.textContent),
  };
});

(async () => {
  const browser = await chromium.launch({headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox']});
  try {
    const page = await browser.newPage({viewport: {width: 1280, height: 900}});
    page.setDefaultTimeout(10000);
    const themed = {background: THEMED, applications: false, toolbar: false, stylesheets: 1, columns: ['desktop-columns-6'], notice: []};
    const withoutTheme = {background: DEFAULT_BACKGROUND, applications: true, toolbar: true, stylesheets: 0, columns: ['desktop-columns-4']};
    const safeNotice = ['Safe mode: this page is shown without the themes of the themes directory.'];

    // An instance that uses the theme. The stylesheet hides the applications
    // and the buttons, and the theme sets the columns.
    let server = await start();
    try {
      const open = async address => {
        await page.goto(server.base + address, {waitUntil: 'load'});
        return look(page);
      };
      assert.deepEqual(await open('/'), themed);

      // In safe mode the page is shown without the theme: everything is where
      // the application puts it, with the layout of the site, and a notice
      // says so. The other navigation pages take the parameter as well.
      assert.deepEqual(await open('/?theme-safe-mode'), {...withoutTheme, notice: safeNotice});
      for (const address of ['/applications?theme-safe-mode', '/bookmarks?theme-safe-mode=1', '/help?theme-safe-mode']) {
        const shown = await open(address);
        assert.deepEqual([shown.background, shown.stylesheets, shown.notice], [DEFAULT_BACKGROUND, 0, safeNotice], address);
      }

      // The notice leads back to the settings, which the stylesheet of a
      // theme never reaches: they have the colors of the theme file and
      // nothing of the stylesheet.
      await open('/?theme-safe-mode');
      await Promise.all([page.waitForURL(server.base + '/settings/theme'), page.locator('.theme-preview-bar a').click()]);
      assert.equal(await page.evaluate(() => getComputedStyle(document.body).backgroundColor), THEME_BACKGROUND);
      assert.equal(await page.locator('link[rel="stylesheet"][href^="/theme/"]').count(), 0);
      assert.equal(await page.locator('.theme-safe-mode').count(), 0);
      assert.equal(await page.locator('.theme-file input[value="hidden"]').count(), 1);

      // Nothing was changed: the theme is selected, and the page without the
      // parameter is shown with it.
      assert.match(await fs.readFile(path.join(server.dir, 'config.yml'), 'utf8'), /^Theme: hidden$/m);
      assert.deepEqual(await open('/'), themed);
    } finally {
      await server.stop();
    }

    // The same data, started in safe mode by the environment and by the
    // command line. The themes directory is not read.
    for (const [how, options] of [['FLARE_THEME_SAFE_MODE', {env: {FLARE_THEME_SAFE_MODE: 'true'}}], ['--theme_safe_mode', {args: ['--theme_safe_mode']}]]) {
      server = await start(options);
      try {
        const open = async address => {
          await page.goto(server.base + address, {waitUntil: 'load'});
          return look(page);
        };
        assert.deepEqual(await open('/'), {...withoutTheme, notice: []}, how);
        assert.deepEqual(await open('/?theme-preview=hidden'), {...withoutTheme, notice: []}, how);

        // The theme page lists nothing of the directory, also not the file
        // that is not usable, and says that safe mode is on.
        await page.goto(server.base + '/settings/themes', {waitUntil: 'load'});
        assert.equal(await page.locator('.theme-file').count(), 0, how);
        assert.equal(await page.locator('.theme-file-lines li').count(), 0, how);
        assert.match(await page.locator('.theme-safe-mode').textContent(), /^Safe mode is on: the themes directory is not read/, how);
        const alerts = await page.locator('form:not(.theme-import-form) .theme-notice[role="alert"]').allTextContents();
        assert.equal(alerts.length, 1, how);
        assert.match(alerts[0], /“hidden” is a theme of the themes directory, which is not read in safe mode/, how);

        // No file is imported.
        await page.locator('#theme-import-file').setInputFiles({name: 'added.yml', mimeType: 'application/yaml', buffer: Buffer.from(MANIFEST)});
        const [response] = await Promise.all([
          page.waitForResponse(candidate => candidate.request().method() === 'POST'),
          page.locator('.theme-import-form input.btn-submit').click(),
        ]);
        await page.waitForLoadState('load');
        assert.equal(response.status(), 409, how);
        assert.deepEqual(await page.locator('.theme-import-form .theme-notice[role="alert"]').allTextContents(),
          ['The themes directory is not used in safe mode. Nothing was imported.'], how);
        await assert.rejects(fs.stat(path.join(server.dir, 'themes', 'added.yml')));

        // A bundled theme is selected and applied as always.
        await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
        const [saved] = await Promise.all([
          page.waitForResponse(candidate => candidate.request().method() === 'POST'),
          page.locator('.theme-blackboard input').click(),
        ]);
        assert.equal(saved.status(), 200, how);
        await page.waitForLoadState('load');
        assert.equal((await open('/')).background, BLACKBOARD_BACKGROUND, how);
      } finally {
        await server.stop();
      }
    }
    console.log('PASS: a page is shown without the themes of the themes directory on request, and an instance started in safe mode does not read the directory');
  } finally {
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
