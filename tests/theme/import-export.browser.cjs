// FLARE_BINARY=/path/to/binary node tests/theme/import-export.browser.cjs
// Import and export of theme files, as documented in docs/theme-files.md: a
// theme is downloaded as a theme file, a theme file is uploaded into the
// themes directory after the checks every theme file gets, and neither
// selects anything.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

const BLACKBOARD = 'rgb(26, 26, 26)';
const TRON = 'rgb(36, 43, 51)';

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-import-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'),
    'Locale: en\nTitle: Site\nTheme: blackboard\nIconMode: DEFAULT\nShowApps: true\nShowBookmarks: true\nShowWeather: false\n');
  await fs.writeFile(path.join(dir, 'apps.yml'), 'links:\n- name: Application\n  link: https://example.com/a\n  icon: circle\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'), 'links:\n- name: One\n  link: https://example.com/b\n');
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
  return {base, dir, themes: path.join(dir, 'themes'),
    async stop() {child.kill(); await fs.rm(dir, {recursive: true, force: true});}};
}

(async () => {
  const browser = await chromium.launch({headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox']});
  const server = await start();
  try {
    const page = await browser.newPage({viewport: {width: 1280, height: 900}});
    page.setDefaultTimeout(10000);
    const settings = () => page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
    const download = async selector => {
      const [file] = await Promise.all([page.waitForEvent('download'), page.locator(selector).click()]);
      return {name: file.suggestedFilename(), content: await fs.readFile(await file.path(), 'utf8')};
    };
    // Uploads a theme file through the form and returns the status of the
    // response and what the form says afterwards.
    const upload = async (name, content, replace = false) => {
      await settings();
      await page.locator('#theme-import-file').setInputFiles({name, mimeType: 'application/yaml', buffer: Buffer.from(content)});
      if (replace) await page.locator('.theme-import-form input[name="replace"]').check();
      const [response] = await Promise.all([
        page.waitForResponse(candidate => candidate.request().method() === 'POST'),
        page.locator('.theme-import-form input.btn-submit').click(),
      ]);
      await page.waitForLoadState('load');
      return {
        status: response.status(),
        done: await page.locator('.theme-import-form .theme-notice[role="status"]').allTextContents(),
        refused: await page.locator('.theme-import-form .theme-notice[role="alert"]').allTextContents(),
      };
    };
    const stored = async () => (await fs.readdir(server.themes).catch(() => [])).sort();
    const cards = () => page.locator('.theme-file input.theme-choose').evaluateAll(inputs => inputs.map(input => input.value));
    const background = async () => {
      await page.goto(server.base, {waitUntil: 'load'});
      await page.addStyleTag({content: '* { transition: none !important; }'});
      return page.evaluate(() => getComputedStyle(document.body).backgroundColor);
    };

    // A bundled theme is downloaded as a theme file, under a name it can be
    // imported under. Nothing is written by a download.
    await settings();
    const copy = await download('.theme-tron .theme-export-link');
    assert.equal(copy.name, 'tron-copy.yml');
    assert.match(copy.content, /^contract: 1$/m);
    assert.deepEqual(await stored(), []);

    // The file is imported as it is. The theme is listed and not selected.
    let result = await upload(copy.name, copy.content);
    assert.deepEqual(result, {status: 200, done: ['The theme “tron-copy” was imported. It is listed under the theme files and is not selected yet.'], refused: []});
    assert.deepEqual(await stored(), ['tron-copy.yml']);
    assert.equal(await fs.readFile(path.join(server.themes, 'tron-copy.yml'), 'utf8'), copy.content);
    assert.deepEqual(await cards(), ['tron-copy']);
    assert.equal(await background(), BLACKBOARD);

    // A file that is not a usable theme is refused with the reason, and a
    // name that cannot be a theme is as well. Every refusal says that
    // nothing was imported.
    const refused = (outcome, status, reason) => {
      assert.equal(outcome.status, status);
      assert.deepEqual(outcome.done, []);
      assert.equal(outcome.refused.length, 1);
      assert.match(outcome.refused[0], reason);
      assert.match(outcome.refused[0], /Nothing was imported\./);
    };
    refused(await upload('broken.yml', copy.content + '  radius: 4em\n'), 400, /tokens\.radius/);
    refused(await upload('Notes.txt', copy.content), 400, /<name>\.yml/);
    assert.deepEqual(await stored(), ['tron-copy.yml']);

    // A theme file of the same name is replaced only with the box ticked.
    const edited = copy.content + 'name: An edited copy\n';
    refused(await upload(copy.name, edited), 409, /exists/);
    assert.equal(await fs.readFile(path.join(server.themes, 'tron-copy.yml'), 'utf8'), copy.content);
    result = await upload(copy.name, edited, true);
    assert.deepEqual(result, {status: 200, done: ['The theme “An edited copy” was imported. It is listed under the theme files and is not selected yet.'], refused: []});
    assert.equal(await fs.readFile(path.join(server.themes, 'tron-copy.yml'), 'utf8'), edited);

    // The imported theme is downloaded as the file that is stored, and it
    // is selected like any theme file.
    await settings();
    assert.deepEqual(await download('.theme-file .theme-export-link'), {name: 'tron-copy.yml', content: edited});
    const [saved] = await Promise.all([
      page.waitForResponse(candidate => candidate.request().method() === 'POST'),
      page.locator('.theme-file input[value="tron-copy"]').click(),
    ]);
    assert.equal(saved.status(), 200, await saved.text());
    await page.waitForLoadState('load');
    assert.equal(await background(), TRON);
    assert.deepEqual(await stored(), ['tron-copy.yml']);
    console.log('PASS: a theme is downloaded as a theme file and a theme file is imported after its checks, without selecting anything');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
