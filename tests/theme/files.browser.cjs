// FLARE_BINARY=/path/to/binary node tests/theme/files.browser.cjs
// Theme files, as documented in docs/theme-files.md: the settings page lists
// the files of the themes directory, a listed theme can be selected, and the
// pages follow the file of the selected theme without a restart.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

const FOREST = `contract: 1
name:
  en: Forest at dusk
  zh: 暮色森林
version: "1.2"
author: Example Author
license: MIT
tokens:
  background: "#10241b"
  primary: "#e8f1e4"
  accent: "#f2b84b"
  muted: "#9db8a6"
  hover: "#ffffff1a"
  radius: 10px
  font-family: Georgia, serif
`;
const COLORS = 'contract: 1\ntokens:\n  background: "#101010"\n  primary: "#f0f0f0"\n  accent: "#808080"\n';

// The colors of the stylesheet, used when the selected theme cannot be used.
const DEFAULT_BACKGROUND = 'rgb(45, 52, 54)';
const BLACKBOARD_BACKGROUND = 'rgb(26, 26, 26)';

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-files-'));
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
  const themes = path.join(dir, 'themes');
  await fs.mkdir(themes);
  await fs.writeFile(path.join(themes, 'forest.yml'), FOREST);
  await fs.writeFile(path.join(themes, 'later.yml'),
    COLORS + '  font-scale: "1.1"\nmodes:\n  dark:\n    background: "#000000"\n');
  await fs.writeFile(path.join(themes, 'bad-radius.yml'), COLORS + '  radius: 4em\n');
  await fs.writeFile(path.join(themes, 'Mixed.yml'), COLORS);
  await fs.writeFile(path.join(themes, 'notes.yaml'), COLORS);
  await fs.writeFile(path.join(themes, 'README.md'), 'Not a theme file.\n');
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
  return {base, dir, themes, async stop() {child.kill(); await fs.rm(dir, {recursive: true, force: true});}};
}

// What the home page looks like: the parts that follow the tokens a theme
// file can set, and the browser-tab icon.
const measure = page => page.evaluate(() => {
  const style = (selector, property) => getComputedStyle(document.querySelector(selector))[property];
  return {
    background: style('body', 'backgroundColor'),
    text: style('.app-title', 'color'),
    description: style('.app-desc', 'color'),
    icon: style('.app-icon svg', 'fill'),
    hover: style('.app-item', 'backgroundColor'),
    radius: style('.app-item', 'borderTopLeftRadius'),
    font: style('body', 'fontFamily'),
    themedTabIcon: document.querySelector('link[rel~="icon"]').getAttribute('href').includes('/assets/favicon/'),
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
      await page.addStyleTag({content: '* { transition: none !important; }'});
      await page.locator('.app-item').hover();
      return measure(page);
    };
    const settings = async () => {
      await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
      return {
        cards: await page.locator('.theme-file').evaluateAll(cards => cards.map(card => ({
          value: card.querySelector('input').value,
          label: card.querySelector('input').getAttribute('aria-label'),
          title: card.querySelector('.theme-title').textContent,
          about: card.querySelector('.theme-about')?.textContent ?? '',
        }))),
        lines: await page.locator('.theme-file-lines li').evaluateAll(lines => lines.map(line =>
          Array.from(line.querySelectorAll('code'), code => code.textContent))),
        alert: await page.locator('.theme-notice[role="alert"]').allTextContents(),
      };
    };
    const select = async locator => {
      const [saved] = await Promise.all([
        page.waitForResponse(response => response.request().method() === 'POST'),
        locator.click(),
      ]);
      assert.equal(saved.status(), 200, await saved.text());
      await page.waitForLoadState('load');
    };

    // The page lists the usable files as themes and says why the others are
    // not used. A file that is not a theme file by its name is left alone.
    let listed = await settings();
    assert.deepEqual(listed.cards, [
      {value: 'forest', label: 'Forest at dusk', title: 'Forest at dusk', about: '1.2 · Example Author · MIT'},
      {value: 'later', label: 'later', title: 'later', about: ''},
    ]);
    assert.deepEqual(listed.lines, [
      ['Mixed.yml'],
      ['bad-radius.yml', 'tokens.radius'],
      ['later.yml', 'modes, tokens.font-scale'],
      ['notes.yaml'],
    ]);
    assert.deepEqual(listed.alert, []);
    assert.equal((await home()).background, BLACKBOARD_BACKGROUND);

    // Selecting a theme file applies every token it sets.
    await settings();
    await select(page.locator('.theme-file input[value="forest"]'));
    assert.match(await fs.readFile(path.join(server.dir, 'config.yml'), 'utf8'), /^Theme: forest$/m);
    const forest = {
      background: 'rgb(16, 36, 27)',
      text: 'rgb(232, 241, 228)',
      description: 'rgb(157, 184, 166)',
      icon: 'rgb(232, 241, 228)',
      hover: 'rgba(255, 255, 255, 0.1)',
      radius: '10px',
      font: 'Georgia, serif',
      themedTabIcon: false,
    };
    assert.deepEqual(await home(), forest);

    // An edit of the selected file shows on the next page load.
    const file = path.join(server.themes, 'forest.yml');
    await fs.writeFile(file, FOREST.replace('#10241b', '#0b1a13').replace('radius: 10px', 'radius: 2px'));
    assert.deepEqual(await home(), {...forest, background: 'rgb(11, 26, 19)', radius: '2px'});

    // A file that is no longer usable leaves the pages with the colors of the
    // stylesheet, and the settings page says what is wrong.
    await fs.writeFile(file, FOREST.replace('  primary: "#e8f1e4"\n', ''));
    assert.equal((await home()).background, DEFAULT_BACKGROUND);
    listed = await settings();
    assert.deepEqual(listed.cards.map(card => card.value), ['later']);
    assert.deepEqual(listed.lines.find(line => line[0] === 'forest.yml'), ['forest.yml', 'tokens.primary']);
    assert.equal(listed.alert.length, 1);
    assert.match(listed.alert[0], /“forest”/);

    // Correcting the file brings the theme back, and so does putting back a
    // file that was removed.
    await fs.writeFile(file, FOREST);
    assert.deepEqual(await home(), forest);
    await fs.rm(file);
    assert.equal((await home()).background, DEFAULT_BACKGROUND);
    assert.equal((await settings()).alert.length, 1);
    await fs.writeFile(file, FOREST);
    assert.deepEqual(await home(), forest);
    assert.deepEqual((await settings()).alert, []);

    // A bundled theme can be selected again; it has its own browser-tab icon.
    await select(page.locator('.theme-blackboard input'));
    const blackboard = await home();
    assert.equal(blackboard.background, BLACKBOARD_BACKGROUND);
    assert.equal(blackboard.radius, '4px');
    assert.equal(blackboard.themedTabIcon, true);
    console.log('PASS: theme files are listed, selected, followed without a restart and reported when unusable');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
