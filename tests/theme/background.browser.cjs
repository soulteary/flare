// FLARE_BINARY=/path/to/binary node tests/theme/background.browser.cjs
// The background image of a theme, as documented in docs/theme-files.md: a
// theme that is a directory names an image of its assets in its theme file,
// the navigation pages show it behind their content, and the background color
// of the theme lies over it as far as the theme dims it. What a page shows is
// read from a screenshot of one pixel where nothing but the background is.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const zlib = require('node:zlib');
const {spawn} = require('node:child_process');

const COLORS = 'contract: 1\ntokens:\n  background: "#10241b"\n  primary: "#e8f1e4"\n  accent: "#f2b84b"\n';
const LIGHT = 'modes:\n  light:\n    background: "#f6f3ea"\n    primary: "#2b2a27"\n';
const background = (dim, image = 'wall.png') => `background:\n  image: assets/${image}\n` + (dim === undefined ? '' : `  dim: ${dim}\n`);

// The background colors of the theme, for dark and for light, and the one of
// the stylesheet of the application, used when the theme cannot be used.
const DARK_COLOR = [16, 36, 27];
const LIGHT_COLOR = [246, 243, 234];
const DEFAULT_COLOR = [45, 52, 54];
const RED = [220, 20, 30], GREEN = [20, 160, 60], BLUE = [30, 60, 200];

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

async function start(prepare) {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-background-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'),
    'Locale: en\nTheme: walled\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowWeather: false\n');
  await fs.writeFile(path.join(dir, 'apps.yml'),
    'links:\n- name: Application\n  link: https://example.test/a\n  icon: circle\n  desc: A description\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'),
    'links:\n- name: One\n  link: https://example.test/b\n  icon: home\n');
  const themes = path.join(dir, 'themes');
  await fs.mkdir(themes);
  await prepare(themes);
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

// Images are loaded and painted after the page is.
const settled = page => page.evaluate(() => document.fonts.ready.then(() =>
  new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))));

// What the body of a page has as its background.
const measure = page => page.evaluate(() => {
  const style = getComputedStyle(document.body);
  const address = /url\("([^"]*)"\)/.exec(style.backgroundImage);
  // A browser gives a value once, or once for each layer of the background.
  const ofEveryLayer = value => Array.from(new Set(value.split(', '))).join(' | ');
  return {
    image: address ? address[1] : '',
    layers: style.backgroundImage === 'none' ? 0 : style.backgroundImage.split('url(').length - 1 + (style.backgroundImage.includes('gradient(') ? 1 : 0),
    size: ofEveryLayer(style.backgroundSize),
    attachment: ofEveryLayer(style.backgroundAttachment),
    repeat: ofEveryLayer(style.backgroundRepeat),
    position: ofEveryLayer(style.backgroundPosition),
    color: style.backgroundColor,
  };
});

(async () => {
  const browser = await chromium.launch({headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox']});
  const server = await start(async themes => {
    await writeTheme(themes, 'walled', COLORS + background(40), {'assets/wall.png': png(2, RED)});
    await writeTheme(themes, 'paired', COLORS + LIGHT + background(50), {'assets/wall.png': png(2, BLUE)});
    // A stylesheet that sets a background image of its own.
    await writeTheme(themes, 'styled', COLORS + background(), {
      'assets/wall.png': png(2, RED), 'assets/other.png': png(2, GREEN),
      'theme.css': 'body { background-image: url(assets/other.png); }\n',
    });
    await writeTheme(themes, 'bare', COLORS, {'assets/wall.png': png(2, RED)});
  });
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
        asked.push(`${response.status()} ${response.url().slice(server.base.length)} ${response.headers()['content-type'] || ''}`);
      }
    });
    const open = async address => {
      asked = [];
      await page.goto(server.base + address, {waitUntil: 'load'});
      await settled(page);
      return measure(page);
    };
    // The color the page has where nothing but its background is painted.
    const corner = async () => firstPixel(await page.screenshot({clip: {x: 2, y: 890, width: 1, height: 1}}));
    const lines = async () => {
      await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
      return {
        lines: await page.locator('.theme-file-lines li').evaluateAll(items => items.map(line => line.textContent.replace(/\s+/g, ' ').trim())),
        alert: await page.locator('form:not(.theme-import-form) .theme-notice[role="alert"]').allTextContents(),
      };
    };
    const manifest = path.join(server.themes, 'walled', 'theme.yml');
    const image = path.join(server.themes, 'walled', 'assets', 'wall.png');

    // The home page of the selected theme has the image behind it: the
    // browser asks for it at the address of the assets and gets an image.
    // It fills the window and stays in place, and the background color of
    // the theme lies over it as far as the theme dims it.
    const first = await open('/');
    assert.match(first.image, new RegExp(`^${server.base}/theme/walled/[0-9a-f]{16}/assets/wall\\.png$`));
    assert.deepEqual({...first, image: ''}, {image: '', layers: 2, size: 'cover', attachment: 'fixed', repeat: 'no-repeat',
      position: '50% 50%', color: `rgb(${DARK_COLOR.join(', ')})`});
    assert.deepEqual(asked, [`200 ${first.image.slice(server.base.length)} image/png`]);
    assertColor(await corner(), dimmed(RED, DARK_COLOR, 40), 'an image dimmed by 40');
    // It is behind the page while the page is scrolled.
    await page.evaluate(() => {
      document.body.style.minHeight = '3000px';
      window.scrollTo(0, 1500);
    });
    await settled(page);
    assert.equal(await page.evaluate(() => window.scrollY), 1500);
    assertColor(await corner(), dimmed(RED, DARK_COLOR, 40), 'the image behind a page that is scrolled');
    // The other navigation pages have it as well.
    for (const location of ['/applications', '/bookmarks', '/help']) {
      const shown = await open(location);
      assert.equal(shown.image, first.image, location);
      assertColor(await corner(), dimmed(RED, DARK_COLOR, 40), location);
    }

    // The settings take the colors of the theme and no image, and ask for
    // nothing of the theme.
    const settings = await open('/settings/theme');
    assert.deepEqual([settings.image, settings.layers, settings.color], ['', 0, `rgb(${DARK_COLOR.join(', ')})`]);
    assert.deepEqual(asked, []);
    assertColor(await corner(), DARK_COLOR, 'the settings');

    // The theme file is followed: without a dim the image is shown as it
    // is, and dimmed all the way nothing of it is seen.
    await fs.writeFile(manifest, COLORS + background());
    const undimmed = await open('/');
    assert.deepEqual([undimmed.image, undimmed.layers], [first.image, 1]);
    assertColor(await corner(), RED, 'an image that is not dimmed');
    await fs.writeFile(manifest, COLORS + background(100));
    assert.equal((await open('/')).layers, 2);
    assertColor(await corner(), DARK_COLOR, 'an image dimmed by 100');
    await fs.writeFile(manifest, COLORS + background(40));
    assertColor((await open('/'), await corner()), dimmed(RED, DARK_COLOR, 40), 'the dim put back');

    // An image that is edited shows on the next page load, under another
    // address.
    await fs.writeFile(image, png(3, GREEN));
    const edited = await open('/');
    assert.notEqual(edited.image, first.image);
    assert.match(edited.image, /\/assets\/wall\.png$/);
    assertColor(await corner(), dimmed(GREEN, DARK_COLOR, 40), 'the edited image');
    assert.equal((await fetch(first.image)).status, 404);

    // A theme with values for light and for dark dims the same image
    // towards the background color of each. A preview shows it without
    // selecting the theme.
    for (const [scheme, color] of [['light', LIGHT_COLOR], ['dark', DARK_COLOR]]) {
      await page.emulateMedia({colorScheme: scheme});
      const paired = await open('/?theme-preview=paired');
      assert.match(paired.image, /\/theme\/paired\/[0-9a-f]{16}\/assets\/wall\.png$/, scheme);
      assert.equal(paired.color, `rgb(${color.join(', ')})`, scheme);
      assertColor(await corner(), dimmed(BLUE, color, 50), `the image of a theme in ${scheme}`);
    }
    await page.emulateMedia({colorScheme: 'dark'});

    // The background of the theme file holds where the stylesheet of the
    // theme sets one on the body as well.
    const styled = await open('/?theme-preview=styled');
    assert.match(styled.image, /\/theme\/styled\/[0-9a-f]{16}\/assets\/wall\.png$/);
    assertColor(await corner(), RED, 'the image of the theme file over that of the stylesheet');

    // A theme without a background image has none, and neither has a page
    // in safe mode.
    const bare = await open('/?theme-preview=bare');
    assert.deepEqual([bare.image, bare.layers], ['', 0]);
    assertColor(await corner(), DARK_COLOR, 'a theme without an image');
    const safe = await open('/?theme-safe-mode');
    assert.deepEqual([safe.image, safe.layers, safe.color], ['', 0, `rgb(${DEFAULT_COLOR.join(', ')})`]);
    assert.deepEqual(asked, []);

    // Without its image the theme is not in use: the pages have the colors
    // of the stylesheet of the application, and the theme page says why.
    await fs.rm(image);
    const missing = await open('/');
    assert.deepEqual([missing.image, missing.layers, missing.color], ['', 0, `rgb(${DEFAULT_COLOR.join(', ')})`]);
    let listed = await lines();
    assert.equal(listed.alert.length, 1);
    assert.match(listed.alert[0], /“walled”/);
    assert.deepEqual(listed.lines.filter(line => line.startsWith('walled/')),
      ['walled/theme.yml The background image is not among the images in the assets of the theme: assets/wall.png']);
    await fs.writeFile(image, png(2, RED));
    assertColor((await open('/'), await corner()), dimmed(RED, DARK_COLOR, 40), 'the image put back');
    assert.deepEqual((await lines()).alert, []);

    // A background that is not written as the documentation says leaves the
    // theme out of use too, with the part that is not accepted.
    await fs.writeFile(manifest, COLORS + 'background:\n  image: assets/wall.png\n  dim: 0.4\n');
    assert.equal((await open('/')).color, `rgb(${DEFAULT_COLOR.join(', ')})`);
    listed = await lines();
    assert.deepEqual(listed.lines.filter(line => line.startsWith('walled/')),
      ['walled/theme.yml The background of the theme is not accepted: background.dim']);
    await fs.writeFile(manifest, COLORS + background(40));
    assert.equal((await open('/')).layers, 2);

    // A theme file with a background image is not imported: it would be a
    // theme of one file, and the image would not be there.
    await page.goto(server.base + '/settings/themes', {waitUntil: 'load'});
    await page.locator('#theme-import-file').setInputFiles({name: 'added.yml', mimeType: 'application/yaml', buffer: Buffer.from(COLORS + background(40))});
    const [response] = await Promise.all([
      page.waitForResponse(candidate => candidate.request().method() === 'POST'),
      page.locator('.theme-import-form input.btn-submit').click(),
    ]);
    await page.waitForLoadState('load');
    assert.equal(response.status(), 400);
    assert.deepEqual(await page.locator('.theme-import-form .theme-notice[role="alert"]').allTextContents(),
      ['Nothing was imported. The background image is not among the images in the assets of the theme: assets/wall.png']);
    await assert.rejects(fs.stat(path.join(server.themes, 'added.yml')));

    assert.deepEqual(elsewhere, []);
    console.log('PASS: the background image of a theme is behind the navigation pages, dimmed towards the background color of the mode, ' +
      'follows the theme file and the image, and is not on the settings');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
