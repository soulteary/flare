// FLARE_BINARY=/path/to/binary node tests/theme/directories.browser.cjs
// A theme as a directory, as documented in docs/theme-files.md: a directory
// of the themes directory with a theme.yml is listed and selected by its
// name, the pages follow its theme.yml, and a name has one theme when a file
// and a directory share it.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

const theme = (background, radius, name = '') => `contract: 1
${name ? `name: ${name}\n` : ''}tokens:
  background: "${background}"
  primary: "#e8f1e4"
  accent: "#f2b84b"
  radius: ${radius}
`;
const ZELDA = theme('#10241b', '10px', 'Zelda');
const BROKEN = theme('#10241b', '4em');

// The colors of the stylesheet, used when the selected theme cannot be used.
const DEFAULT_BACKGROUND = 'rgb(45, 52, 54)';
const BLACKBOARD_BACKGROUND = 'rgb(26, 26, 26)';

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-directories-'));
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
  for (const directory of ['zelda', 'broken', 'notes', 'shadowed']) {
    await fs.mkdir(path.join(themes, directory), {recursive: true});
  }
  await fs.writeFile(path.join(themes, 'zelda', 'theme.yml'), ZELDA);
  // Of the directory of a theme, theme.yml is read and nothing else.
  await fs.writeFile(path.join(themes, 'zelda', 'theme.css'), 'body { background: red !important; }\n');
  await fs.writeFile(path.join(themes, 'plain.yml'), theme('#101010', '4px'));
  await fs.writeFile(path.join(themes, 'broken', 'theme.yml'), BROKEN);
  await fs.writeFile(path.join(themes, 'notes', 'readme.txt'), 'A directory without a theme.yml is left alone.\n');
  await fs.writeFile(path.join(themes, 'shadowed.yml'), theme('#202020', '2px'));
  await fs.writeFile(path.join(themes, 'shadowed', 'theme.yml'), theme('#303030', '12px'));
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

const measure = page => page.evaluate(() => {
  const style = (selector, property) => getComputedStyle(document.querySelector(selector))[property];
  return {
    background: style('body', 'backgroundColor'),
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
    const home = async (query = '') => {
      await page.goto(server.base + '/' + query, {waitUntil: 'load'});
      await page.addStyleTag({content: '* { transition: none !important; }'});
      // The corners of an application show under the pointer.
      await page.locator('.app-item').hover();
      return measure(page);
    };
    const settings = async () => {
      await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
      return {
        cards: await page.locator('.theme-file').evaluateAll(cards => cards.map(card => ({
          value: card.querySelector('input').value,
          title: card.querySelector('.theme-title').textContent,
          links: Array.from(card.querySelectorAll('a'), link => link.textContent),
        }))),
        lines: await page.locator('.theme-file-lines li').evaluateAll(lines => lines.map(line =>
          Array.from(line.querySelectorAll('code'), code => code.textContent))),
        alert: await page.locator('form:not(.theme-import-form) .theme-notice[role="alert"]').allTextContents(),
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
    const manifest = path.join(server.themes, 'zelda', 'theme.yml');
    const file = path.join(server.themes, 'zelda.yml');

    // A directory with a theme.yml is listed as a theme among the theme
    // files. It has a preview and no download. A directory without a
    // theme.yml is left alone, and of a file and a directory of one name the
    // file is reported.
    let listed = await settings();
    assert.deepEqual(listed.cards, [
      {value: 'plain', title: 'plain', links: ['Preview', 'Download']},
      {value: 'shadowed', title: 'shadowed', links: ['Preview']},
      {value: 'zelda', title: 'Zelda', links: ['Preview']},
    ]);
    assert.deepEqual(listed.lines, [
      ['broken/theme.yml', 'tokens.radius'],
      ['shadowed.yml'],
    ]);
    assert.deepEqual(listed.alert, []);
    assert.equal((await home()).background, BLACKBOARD_BACKGROUND);

    // Selecting it stores the name of the directory and applies theme.yml.
    // The other files of the directory have no effect.
    await settings();
    await select(page.locator('.theme-file input[value="zelda"]'));
    assert.match(await fs.readFile(path.join(server.dir, 'config.yml'), 'utf8'), /^Theme: zelda$/m);
    const zelda = {background: 'rgb(16, 36, 27)', radius: '10px'};
    assert.deepEqual(await home(), zelda);

    // An edit of theme.yml shows on the next page load.
    await fs.writeFile(manifest, theme('#0b1a13', '2px', 'Zelda'));
    const edited = {background: 'rgb(11, 26, 19)', radius: '2px'};
    assert.deepEqual(await home(), edited);

    // A file of the same name does not take the place of the directory, and
    // the theme page says that it is not read.
    await fs.writeFile(file, theme('#402020', '6px'));
    assert.deepEqual(await home(), edited);
    listed = await settings();
    assert.deepEqual(listed.lines.find(line => line[0] === 'zelda.yml'), ['zelda.yml']);
    assert.equal(listed.cards.filter(card => card.value === 'zelda').length, 1);

    // Without its theme.yml the directory is no theme, and the file is the
    // theme of the name: it is offered as a download again.
    await fs.rm(manifest);
    const asFile = {background: 'rgb(64, 32, 32)', radius: '6px'};
    assert.deepEqual(await home(), asFile);
    listed = await settings();
    assert.deepEqual(listed.cards.find(card => card.value === 'zelda'), {value: 'zelda', title: 'zelda', links: ['Preview', 'Download']});
    assert.equal(listed.lines.some(line => line[0] === 'zelda.yml'), false);

    // A theme.yml that comes back takes the place of the file, which did not
    // change.
    await fs.writeFile(manifest, ZELDA);
    assert.deepEqual(await home(), zelda);

    // A theme.yml that is not usable leaves the pages with the colors of the
    // stylesheet: the file of the same name does not stand in for it.
    await fs.writeFile(manifest, BROKEN);
    assert.equal((await home()).background, DEFAULT_BACKGROUND);
    listed = await settings();
    assert.equal(listed.alert.length, 1);
    assert.match(listed.alert[0], /“zelda”/);
    assert.deepEqual(listed.lines.find(line => line[0] === 'zelda/theme.yml'), ['zelda/theme.yml', 'tokens.radius']);
    await fs.writeFile(manifest, ZELDA);
    assert.deepEqual(await home(), zelda);
    assert.deepEqual((await settings()).alert, []);

    // The preview of a name with both forms shows the directory, and selects
    // nothing.
    assert.deepEqual(await home('?theme-preview=shadowed'), {background: 'rgb(48, 48, 48)', radius: '12px'});
    assert.deepEqual(await home(), zelda);

    // No file is imported under the name of a directory that is a theme,
    // with the box for replacing ticked or not.
    await fs.rm(file);
    for (const replace of [false, true]) {
      await settings();
      await page.locator('#theme-import-file').setInputFiles({name: 'zelda.yml', mimeType: 'application/yaml',
        buffer: Buffer.from(theme('#402020', '6px'))});
      if (replace) await page.locator('.theme-import-form input[name="replace"]').check();
      const [response] = await Promise.all([
        page.waitForResponse(candidate => candidate.request().method() === 'POST'),
        page.locator('.theme-import-form input.btn-submit').click(),
      ]);
      await page.waitForLoadState('load');
      assert.equal(response.status(), 409);
      const refused = await page.locator('.theme-import-form .theme-notice[role="alert"]').allTextContents();
      assert.equal(refused.length, 1);
      assert.match(refused[0], /directory/);
      assert.match(refused[0], /Nothing was imported\./);
      await assert.rejects(fs.stat(file));
    }
    assert.deepEqual(await home(), zelda);

    // Removing the directory leaves the selection without a theme, and
    // putting it back brings the theme back.
    await fs.rm(path.join(server.themes, 'zelda'), {recursive: true});
    assert.equal((await home()).background, DEFAULT_BACKGROUND);
    assert.equal((await settings()).alert.length, 1);
    await fs.mkdir(path.join(server.themes, 'zelda'));
    await fs.writeFile(manifest, ZELDA);
    assert.deepEqual(await home(), zelda);
    console.log('PASS: a directory with a theme.yml is listed, selected and followed as a theme, and a name has one theme');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
