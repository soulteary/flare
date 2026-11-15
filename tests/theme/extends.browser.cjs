// FLARE_BINARY=/path/to/binary node tests/theme/extends.browser.cjs
// A theme that is based on another theme, as documented in
// docs/theme-files.md: it has the tokens, the options and the image of the
// theme below it where it gives none of its own, and its pages load the
// stylesheet of that theme before its own. What a page shows is read from
// the styles a browser computes and from a screenshot of one pixel where
// nothing but the background is.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const zlib = require('node:zlib');
const {spawn} = require('node:child_process');

const CONFIG = 'Locale: en\nTheme: night\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowWeather: false\n';
const ZELDA = `contract: 1
name: Zelda
tokens:
  background: "#10241b"
  primary: "#e8f1e4"
  accent: "#f2b84b"
  radius: 10px
background:
  image: assets/wall.png
  dim: 40
options:
  - id: frames
    type: boolean
    label: Frames
`;
// The stylesheet of the theme below: a rule the themes above leave alone,
// one they replace, and one that selects by the value of an option.
const ZELDA_STYLES = `.app-item { outline: 3px solid rgb(1, 2, 3); }
.app-title { letter-spacing: 2px; }
body[data-theme-frames="true"] .app-item { outline-color: rgb(4, 5, 6); }
`;
const NIGHT = 'contract: 1\nextends: zelda\nname: Night\ntokens:\n  accent: "#00ff88"\n  radius: 2px\n';
// The same selector as below, so the later stylesheet holds, and an image of
// its own assets.
const NIGHT_STYLES = '.app-title { letter-spacing: 5px; }\n.app-desc { background-image: url(assets/dot.png); }\n';
const COMPACT = 'contract: 1\nextends: zelda\nname: Compact\ntokens:\n  radius: 2px\n';
const WALLED = 'contract: 1\nextends: zelda\nname: Walled\nbackground:\n  image: assets/wall.png\n';

// The background color of the themes, and the one of the stylesheet of the
// application, used when a theme cannot be used.
const DARK_COLOR = [16, 36, 27];
const DEFAULT_COLOR = [45, 52, 54];
const RED = [220, 20, 30], BLUE = [30, 60, 200], GREEN = [20, 160, 60];

const CRC = Array.from({length: 256}, (_, n) => {
  for (let k = 0; k < 8; k++) n = n & 1 ? 0xEDB88320 ^ (n >>> 1) : n >>> 1;
  return n >>> 0;
});
const crc = buffer => (buffer.reduce((sum, byte) => CRC[(sum ^ byte) & 0xFF] ^ (sum >>> 8), 0xFFFFFFFF) ^ 0xFFFFFFFF) >>> 0;

// png makes an image of one color.
function png(size, [red, green, blue]) {
  const chunk = (type, data) => {
    const body = Buffer.concat([Buffer.from(type, 'latin1'), data]);
    const frame = Buffer.alloc(8);
    frame.writeUInt32BE(data.length, 0);
    frame.writeUInt32BE(crc(body), 4);
    return Buffer.concat([frame.subarray(0, 4), body, frame.subarray(4)]);
  };
  const header = Buffer.alloc(13);
  header.writeUInt32BE(size, 0);
  header.writeUInt32BE(size, 4);
  header.set([8, 2], 8); // Eight bits for each of red, green and blue.
  const row = Buffer.from([0, ...Array(size).fill([red, green, blue]).flat()]);
  return Buffer.concat([Buffer.from('\x89PNG\r\n\x1a\n', 'latin1'), chunk('IHDR', header),
    chunk('IDAT', zlib.deflateSync(Buffer.concat(Array(size).fill(row)))), chunk('IEND', Buffer.alloc(0))]);
}

// firstPixel reads the color of an image of one pixel, as a screenshot of
// one pixel is.
function firstPixel(image) {
  const data = [];
  for (let at = 8; at < image.length;) {
    const length = image.readUInt32BE(at);
    if (image.toString('latin1', at + 4, at + 8) === 'IDAT') data.push(image.subarray(at + 8, at + 8 + length));
    at += 12 + length;
  }
  return Array.from(zlib.inflateSync(Buffer.concat(data)).subarray(1, 4));
}

// dimmed is the color of an image under a layer of a color that covers a
// number of percent of it.
const dimmed = (image, color, dim) => image.map((value, i) => value * (100 - dim) / 100 + color[i] * dim / 100);
// A browser rounds where the layers are put together.
const assertColor = (actual, expected, message) => {
  assert.ok(actual.every((value, i) => Math.abs(value - expected[i]) <= 2),
    `${message}: the page shows rgb(${actual.join(', ')}), expected about rgb(${expected.map(value => Math.round(value)).join(', ')})`);
};

// writeTheme writes a theme that is a directory: its theme file, and files
// by their paths in the directory.
const writeTheme = async (themes, name, manifest, files = {}) => {
  await fs.mkdir(path.join(themes, name), {recursive: true});
  await fs.writeFile(path.join(themes, name, 'theme.yml'), manifest);
  for (const [file, content] of Object.entries(files)) {
    await fs.mkdir(path.dirname(path.join(themes, name, file)), {recursive: true});
    await fs.writeFile(path.join(themes, name, file), content);
  }
};

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-extends-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'), CONFIG);
  await fs.writeFile(path.join(dir, 'apps.yml'),
    'links:\n- name: Application\n  link: https://example.test/a\n  icon: circle\n  desc: A description\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'),
    'links:\n- name: One\n  link: https://example.test/b\n  icon: home\n');
  const themes = path.join(dir, 'themes');
  await fs.mkdir(themes);
  await writeTheme(themes, 'zelda', ZELDA, {'theme.css': ZELDA_STYLES, 'assets/wall.png': png(2, RED)});
  await writeTheme(themes, 'night', NIGHT, {'theme.css': NIGHT_STYLES, 'assets/dot.png': png(2, GREEN)});
  await writeTheme(themes, 'walled', WALLED, {'assets/wall.png': png(2, BLUE)});
  await fs.writeFile(path.join(themes, 'compact.yml'), COMPACT);
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

// Images are loaded and painted after the page is.
const settled = page => page.evaluate(() => document.fonts.ready.then(() =>
  new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))));

// An address of a theme without the hash in it, which changes with the files.
const plain = address => address.replace(/\/[0-9a-f]{16}\//, '/<hash>/');

// What the home page has of the theme and of the themes below it.
const look = page => page.evaluate(() => {
  const plain = address => address.replace(location.origin, '').replace(/\/[0-9a-f]{16}\//, '/<hash>/');
  const body = getComputedStyle(document.body);
  const tile = getComputedStyle(document.querySelector('.app-item'));
  const image = /url\("([^"]*)"\)/.exec(body.backgroundImage);
  const dot = /url\("([^"]*)"\)/.exec(getComputedStyle(document.querySelector('.app-desc')).backgroundImage);
  return {
    sheets: Array.from(document.querySelectorAll('head link[rel="stylesheet"]'), link => plain(link.href)).filter(address => address.startsWith('/theme/')),
    outline: tile.outlineStyle === 'none' ? 'none' : `${tile.outlineColor} ${tile.outlineWidth}`,
    spacing: getComputedStyle(document.querySelector('.app-title')).letterSpacing,
    accent: body.getPropertyValue('--color-accent').trim(),
    radius: body.getPropertyValue('--flare-radius').trim(),
    image: image ? plain(image[1]) : '',
    dot: dot ? plain(dot[1]) : '',
    frames: document.body.getAttribute('data-theme-frames'),
  };
});

(async () => {
  const browser = await chromium.launch({headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox']});
  const server = await start();
  try {
    const page = await browser.newPage({viewport: {width: 1280, height: 900}});
    page.setDefaultTimeout(10000);
    await page.emulateMedia({colorScheme: 'dark'});
    // Everything the pages ask for comes from the instance, and what they
    // ask of a theme is noted with the answer.
    const elsewhere = [];
    let asked = [];
    page.on('request', request => {
      if (!request.url().startsWith(server.base + '/') && !request.url().startsWith('data:')) elsewhere.push(request.url());
    });
    page.on('response', response => {
      if (response.url().startsWith(server.base + '/theme/')) {
        asked.push(`${response.status()} ${plain(response.url().slice(server.base.length))} ${(response.headers()['content-type'] || '').split(';')[0]}`);
      }
    });
    const home = async (address = '/') => {
      asked = [];
      await page.goto(server.base + address, {waitUntil: 'load'});
      await page.addStyleTag({content: '* { transition: none !important; }'});
      await settled(page);
      return look(page);
    };
    // The color the page has where nothing but its background is painted.
    const corner = async () => firstPixel(await page.screenshot({clip: {x: 2, y: 890, width: 1, height: 1}}));
    const settings = async () => {
      await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
      return {
        based: Object.fromEntries(await page.locator('.theme-file').evaluateAll(cards => cards.map(card => [
          card.querySelector('.theme-choose').value,
          Array.from(card.querySelectorAll('.theme-about'), note => note.textContent.trim()).filter(text => text.startsWith('Based on')),
        ]))),
        lines: await page.locator('.theme-file-lines li').evaluateAll(items => items.map(line => line.textContent.replace(/\s+/g, ' ').trim())),
      };
    };
    const select = async name => {
      await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
      const [response] = await Promise.all([
        page.waitForResponse(candidate => candidate.request().method() === 'POST'),
        page.locator(`.theme-file input[value="${name}"]`).click(),
      ]);
      assert.equal(response.status(), 200, name);
      await page.waitForLoadState('load');
    };
    const zeldaSheet = '/theme/zelda/<hash>/theme.css', nightSheet = '/theme/night/<hash>/theme.css';
    const wall = '/theme/zelda/<hash>/assets/wall.png';

    // A directory based on a directory. The page loads the stylesheet of
    // the theme below and then its own: a rule the theme leaves alone holds,
    // and one it writes again is replaced. The tokens of its file hold over
    // those below, the image is the one of the theme below, served among
    // the assets of that theme, and its own stylesheet refers to its own
    // assets.
    const night = await home();
    assert.deepEqual(night, {sheets: [zeldaSheet, nightSheet], outline: 'rgb(1, 2, 3) 3px', spacing: '5px', accent: '#00ff88', radius: '2px',
      image: wall, dot: '/theme/night/<hash>/assets/dot.png', frames: 'false'});
    assert.deepEqual(asked.toSorted(), [
      '200 /theme/night/<hash>/assets/dot.png image/png', `200 ${nightSheet} text/css`,
      `200 ${wall} image/png`, `200 ${zeldaSheet} text/css`,
    ]);
    assertColor(await corner(), dimmed(RED, DARK_COLOR, 40), 'the image of the theme below, dimmed by 40');
    // The other navigation pages load the same stylesheets, in that order.
    for (const address of ['/applications', '/bookmarks', '/help']) {
      await page.goto(server.base + address, {waitUntil: 'load'});
      const sheets = await page.locator('head link[rel="stylesheet"]').evaluateAll(links => links.map(link => new URL(link.href).pathname).filter(file => file.startsWith('/theme/')));
      assert.deepEqual(sheets.map(plain), [zeldaSheet, nightSheet], address);
    }

    // The theme page says what each theme is based on, and offers the
    // option of the theme below for the selected theme. A value chosen
    // there reaches the rule of the stylesheet below that selects by it.
    const listed = await settings();
    assert.deepEqual(listed.based, {compact: ['Based on Zelda.'], night: ['Based on Zelda.'], walled: ['Based on Zelda.'], zelda: []});
    assert.deepEqual(listed.lines, []);
    // The settings take the tokens and no stylesheet of a theme.
    assert.deepEqual(await page.evaluate(() => [getComputedStyle(document.body).getPropertyValue('--color-accent').trim(),
      Array.from(document.querySelectorAll('link[rel="stylesheet"]')).filter(link => link.href.includes('/theme/')).length]), ['#00ff88', 0]);
    assert.equal(await page.locator('.theme-options-form h2').textContent(), 'Options of Night');
    await page.locator('.theme-options-form input[type="checkbox"]').check();
    const [saved] = await Promise.all([
      page.waitForResponse(candidate => candidate.request().method() === 'POST'),
      page.locator('.theme-options-form input.btn-submit').click(),
    ]);
    assert.equal(saved.status(), 200);
    await page.waitForLoadState('load');
    const framed = await home();
    assert.deepEqual([framed.frames, framed.outline], ['true', 'rgb(4, 5, 6) 3px']);

    // A single file based on the directory has its stylesheet and its
    // image, and values of its own for the option.
    await select('compact');
    const compact = await home();
    assert.deepEqual(compact, {sheets: [zeldaSheet], outline: 'rgb(1, 2, 3) 3px', spacing: '2px', accent: '#f2b84b', radius: '2px',
      image: wall, dot: '', frames: 'false'});
    assertColor(await corner(), dimmed(RED, DARK_COLOR, 40), 'the image of the theme below a single file');

    // An image of its own replaces the one below, with its own dim.
    await select('walled');
    const walled = await home();
    assert.deepEqual([walled.sheets, walled.image, walled.radius], [[zeldaSheet], '/theme/walled/<hash>/assets/wall.png', '10px']);
    assertColor(await corner(), BLUE, 'the image of the theme itself');

    // An edit of the theme below shows on the next page load of a theme
    // based on it: its stylesheet, its theme file and its image.
    await select('compact');
    await fs.writeFile(path.join(server.themes, 'zelda', 'theme.css'), ZELDA_STYLES.replace('rgb(1, 2, 3)', 'rgb(7, 8, 9)'));
    assert.equal((await home()).outline, 'rgb(7, 8, 9) 3px');
    await fs.writeFile(path.join(server.themes, 'zelda', 'theme.yml'), ZELDA.replace('#f2b84b', '#ff0000'));
    assert.equal((await home()).accent, '#ff0000');
    await fs.writeFile(path.join(server.themes, 'zelda', 'assets', 'wall.png'), png(4, GREEN));
    assert.equal((await home()).image, wall);
    assertColor(await corner(), dimmed(GREEN, DARK_COLOR, 40), 'the image of the theme below after it was replaced');

    // Without the theme it is based on, a theme is not in use: the page has
    // the colors of the application, and the theme page names the reason.
    // The theme is back once the theme below is.
    await fs.rename(path.join(server.themes, 'zelda'), path.join(server.themes, '.zelda'));
    const without = await home();
    assert.deepEqual([without.sheets, without.image, without.outline], [[], '', 'none']);
    assertColor(await corner(), DEFAULT_COLOR, 'a theme without the theme below it');
    const reasons = await settings();
    assert.deepEqual(reasons.based, {});
    assert.deepEqual(reasons.lines, [
      'compact.yml Not used: the theme it is based on is not in use: zelda',
      'night/theme.yml Not used: the theme it is based on is not in use: zelda',
      'walled/theme.yml Not used: the theme it is based on is not in use: zelda',
    ]);
    await fs.rename(path.join(server.themes, '.zelda'), path.join(server.themes, 'zelda'));
    const back = await home();
    assert.deepEqual([back.sheets, back.image, back.outline, back.accent], [[zeldaSheet], wall, 'rgb(7, 8, 9) 3px', '#ff0000']);

    // A theme that is based on itself by way of another is not in use, and
    // neither is the other.
    await fs.writeFile(path.join(server.themes, 'zelda', 'theme.yml'), ZELDA + 'extends: compact\n');
    assert.deepEqual((await home()).sheets, []);
    assert.deepEqual((await settings()).lines.filter(line => line.includes('based on itself')), [
      'compact.yml Not used: the theme is based on itself, by way of: zelda → compact',
      'zelda/theme.yml Not used: the theme is based on itself, by way of: compact → zelda',
    ]);

    assert.deepEqual(elsewhere, []);
    console.log('PASS: a theme based on another has its tokens, its option and its image, loads its stylesheet before its own, follows its files, and is not in use without it');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
