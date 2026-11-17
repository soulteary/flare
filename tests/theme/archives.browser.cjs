// FLARE_BINARY=/path/to/binary node tests/theme/archives.browser.cjs
// Archives of themes, as documented in docs/theme-files.md: a theme that is
// a directory is downloaded as a ZIP file with its theme file, its
// stylesheet, its assets and its text documents, and such a file is imported
// as a directory of the themes directory after the checks a theme of the
// directory gets. Only files of a theme are written, whatever an archive
// holds and whatever its entries are named.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const zlib = require('node:zlib');
const {spawn} = require('node:child_process');

const COLORS = 'contract: 1\ntokens:\n  background: "#10241b"\n  primary: "#e8f1e4"\n  accent: "#f2b84b"\n';
const ZELDA = COLORS + '  radius: 10px\nname: Zelda\nbackground:\n  image: assets/wall.png\n';
const STYLES = '.app-item { outline: 3px solid rgb(1, 2, 3); }\n';
const LICENSE = 'Permission is hereby granted, free of charge.\n';
const RED = [220, 20, 30];

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

// pack writes a ZIP file with the given entries, a name and the content of
// each, in their order and without packing them smaller. The names are
// written as they are given.
function pack(entries) {
  const files = [], directory = [];
  let offset = 0;
  for (const [name, content] of entries) {
    const title = Buffer.from(name), data = Buffer.from(content);
    const local = Buffer.alloc(30), central = Buffer.alloc(46);
    local.writeUInt32LE(0x04034b50, 0);
    central.writeUInt32LE(0x02014b50, 0);
    for (const [header, at] of [[local, 4], [central, 6]]) {
      header.writeUInt16LE(20, at);          // The version that reads it.
      header.writeUInt16LE(0x0800, at + 2);  // Names are UTF-8.
      header.writeUInt16LE(0x21, at + 8);    // The first day a ZIP file can state.
      header.writeUInt32LE(crc(data), at + 10);
      header.writeUInt32LE(data.length, at + 14);
      header.writeUInt32LE(data.length, at + 18);
      header.writeUInt16LE(title.length, at + 22);
    }
    central.writeUInt16LE(20, 4);
    central.writeUInt32LE(offset, 42);
    files.push(local, title, data);
    directory.push(central, title);
    offset += local.length + title.length + data.length;
  }
  const listed = Buffer.concat(directory), end = Buffer.alloc(22);
  end.writeUInt32LE(0x06054b50, 0);
  end.writeUInt16LE(entries.length, 8);
  end.writeUInt16LE(entries.length, 10);
  end.writeUInt32LE(listed.length, 12);
  end.writeUInt32LE(offset, 16);
  return Buffer.concat([...files, listed, end]);
}

// unpack reads the entries of a ZIP file from its directory: the name and
// the content of each, in their order.
function unpack(archive) {
  let end = archive.length - 22;
  while (end >= 0 && archive.readUInt32LE(end) !== 0x06054b50) end--;
  assert.ok(end >= 0, 'not a ZIP file');
  const entries = [];
  for (let at = archive.readUInt32LE(end + 16), left = archive.readUInt16LE(end + 10); left > 0; left--) {
    assert.equal(archive.readUInt32LE(at), 0x02014b50);
    const method = archive.readUInt16LE(at + 10), packed = archive.readUInt32LE(at + 20);
    const title = archive.readUInt16LE(at + 28), local = archive.readUInt32LE(at + 42);
    const start = local + 30 + archive.readUInt16LE(local + 26) + archive.readUInt16LE(local + 28);
    const data = archive.subarray(start, start + packed);
    entries.push([archive.toString('utf8', at + 46, at + 46 + title), method === 0 ? Buffer.from(data) : zlib.inflateRawSync(data)]);
    at += 46 + title + archive.readUInt16LE(at + 30) + archive.readUInt16LE(at + 32);
  }
  return entries;
}

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-archives-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'),
    'Locale: en\nTheme: blackboard\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowWeather: false\n');
  await fs.writeFile(path.join(dir, 'apps.yml'),
    'links:\n- name: Application\n  link: https://example.test/a\n  icon: circle\n  desc: A description\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'), 'links:\n- name: One\n  link: https://example.test/b\n  icon: home\n');
  const themes = path.join(dir, 'themes');
  await fs.mkdir(path.join(themes, 'zelda', 'assets'), {recursive: true});
  await fs.writeFile(path.join(themes, 'zelda', 'theme.yml'), ZELDA);
  await fs.writeFile(path.join(themes, 'zelda', 'theme.css'), STYLES);
  await fs.writeFile(path.join(themes, 'zelda', 'assets', 'wall.png'), png(2, RED));
  await fs.writeFile(path.join(themes, 'zelda', 'LICENSE.txt'), LICENSE);
  // Files of the directory that are no files of a theme.
  await fs.writeFile(path.join(themes, 'zelda', 'notes.html'), '<p>notes</p>');
  await fs.writeFile(path.join(themes, 'zelda', 'assets', 'source.psd'), 'layers');
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

// Every file below a directory, by its path in it with slashes.
async function filesOf(directory) {
  const found = [];
  for (const entry of await fs.readdir(directory, {withFileTypes: true, recursive: true})) {
    if (entry.isFile()) found.push(path.relative(directory, path.join(entry.parentPath, entry.name)).split(path.sep).join('/'));
  }
  return found.sort();
}

(async () => {
  const browser = await chromium.launch({headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox']});
  const server = await start();
  try {
    const page = await browser.newPage({viewport: {width: 1280, height: 900}});
    page.setDefaultTimeout(10000);
    await page.emulateMedia({colorScheme: 'dark'});
    const elsewhere = [];
    page.on('request', request => {
      if (!request.url().startsWith(server.base + '/') && !request.url().startsWith('data:')) elsewhere.push(request.url());
    });
    const settings = () => page.goto(server.base + '/settings/themes', {waitUntil: 'load'});
    // Uploads a file through the form and returns the status of the
    // response and what the form says afterwards.
    const upload = async (name, buffer, replace = false) => {
      await settings();
      await page.locator('#theme-import-file').setInputFiles({name, mimeType: 'application/zip', buffer});
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
    const refused = (outcome, status, reason) => {
      assert.equal(outcome.status, status);
      assert.deepEqual(outcome.done, []);
      assert.equal(outcome.refused.length, 1);
      assert.match(outcome.refused[0], reason);
      assert.match(outcome.refused[0], /Nothing was imported\./);
    };
    const stored = async () => (await fs.readdir(server.themes)).sort();
    const imported = name => `The theme “${name}” was imported. It is listed under the theme files and is not selected yet.`;

    // The form takes a theme file or an archive, and the card of a theme
    // that is a directory has a download. It is an archive with a directory
    // of the name of the theme: the theme file, the stylesheet, the image of
    // the assets and the text document, and nothing else of the directory.
    await settings();
    assert.equal(await page.locator('#theme-import-file').getAttribute('accept'), '.yml,.zip');
    const [file] = await Promise.all([page.waitForEvent('download'), page.locator('.theme-file:has(input[value="zelda"]) .theme-export-link').click()]);
    assert.equal(file.suggestedFilename(), 'zelda.zip');
    const archive = await fs.readFile(await file.path());
    const entries = unpack(archive);
    assert.deepEqual(entries.map(([name]) => name), ['zelda/theme.yml', 'zelda/theme.css', 'zelda/assets/wall.png', 'zelda/LICENSE.txt']);
    assert.equal(entries[0][1].toString(), ZELDA);
    assert.equal(entries[1][1].toString(), STYLES);
    assert.ok(entries[2][1].equals(png(2, RED)));
    assert.equal(entries[3][1].toString(), LICENSE);

    // What was downloaded is imported under the name of the uploaded file.
    // The theme is listed and not selected, and its files are those of the
    // first one.
    assert.deepEqual(await upload('hyrule.zip', archive), {status: 200, done: [imported('Zelda')], refused: []});
    assert.deepEqual(await stored(), ['hyrule', 'zelda']);
    assert.deepEqual(await filesOf(path.join(server.themes, 'hyrule')), ['LICENSE.txt', 'assets/wall.png', 'theme.css', 'theme.yml']);
    assert.deepEqual(await page.locator('.theme-file input.theme-choose').evaluateAll(inputs => inputs.map(input => input.value)), ['hyrule', 'zelda']);
    assert.equal(await page.evaluate(() => getComputedStyle(document.body).backgroundColor), 'rgb(26, 26, 26)');

    // Selected, the imported theme is what the first one is: its stylesheet
    // and its image come from its own directory, and the browser shows them.
    const [selected] = await Promise.all([
      page.waitForResponse(candidate => candidate.request().method() === 'POST'),
      page.locator('.theme-file input[value="hyrule"]').click(),
    ]);
    assert.equal(selected.status(), 200);
    await page.waitForLoadState('load');
    await page.goto(server.base + '/', {waitUntil: 'load'});
    await page.evaluate(() => document.fonts.ready.then(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))));
    const shown = await page.evaluate(() => {
      const plain = address => address.replace(location.origin, '').replace(/\/[0-9a-f]{16}\//, '/<hash>/');
      const tile = getComputedStyle(document.querySelector('.app-item'));
      const image = /url\("([^"]*)"\)/.exec(getComputedStyle(document.body).backgroundImage);
      return {
        sheets: Array.from(document.querySelectorAll('head link[rel="stylesheet"]'), link => plain(link.href)).filter(address => address.startsWith('/theme/')),
        outline: `${tile.outlineColor} ${tile.outlineWidth}`,
        image: image ? plain(image[1]) : '',
      };
    });
    assert.deepEqual(shown, {sheets: ['/theme/hyrule/<hash>/theme.css'], outline: 'rgb(1, 2, 3) 3px', image: '/theme/hyrule/<hash>/assets/wall.png'});
    const corner = firstPixel(await page.screenshot({clip: {x: 2, y: 890, width: 1, height: 1}}));
    assert.ok(corner.every((value, i) => Math.abs(value - RED[i]) <= 2), `the page shows rgb(${corner.join(', ')}) where the image is`);

    // An archive made elsewhere: the files in a directory of any name, with
    // entries a system adds and entries that are no files of a theme. Those
    // are left out, and the page names them.
    const crafted = pack([
      ['flare-theme-main/theme.yml', COLORS + 'name: Crafted\n'],
      ['flare-theme-main/theme.css', STYLES],
      ['flare-theme-main/.DS_Store', 'finder'],
      ['__MACOSX/flare-theme-main/._theme.yml', 'resource fork'],
      ['flare-theme-main/notes.html', '<p>notes</p>'],
      ['flare-theme-main/src/design.psd', 'layers'],
      ['flare-theme-main/README.md', '# Crafted\n'],
    ]);
    assert.deepEqual(await upload('crafted.zip', crafted), {status: 200, done: [imported('Crafted'),
      'These entries of the archive are no files of a theme and were left out: flare-theme-main/notes.html, flare-theme-main/src/design.psd'], refused: []});
    assert.deepEqual(await filesOf(path.join(server.themes, 'crafted')), ['README.md', 'theme.css', 'theme.yml']);

    // A theme of the name is replaced only with the box ticked, and then as
    // a whole.
    const smaller = pack([['theme.yml', COLORS + 'name: Smaller\n']]);
    refused(await upload('crafted.zip', smaller), 409, /A theme of this name exists/);
    assert.deepEqual(await filesOf(path.join(server.themes, 'crafted')), ['README.md', 'theme.css', 'theme.yml']);
    assert.deepEqual(await upload('crafted.zip', smaller, true), {status: 200, done: [imported('Smaller')], refused: []});
    assert.deepEqual(await filesOf(path.join(server.themes, 'crafted')), ['theme.yml']);

    // Entries named to reach other places are no files of a theme. They are
    // left out like any other, and nothing is written outside the directory
    // of the theme.
    const outside = await filesOf(server.dir);
    const reaching = pack([
      ['theme.yml', COLORS],
      ['../evil.yml', COLORS],
      ['../../evil.txt', 'outside'],
      ['assets/../../evil.png', png(2, RED)],
      ['/tmp/evil.txt', 'absolute'],
      ['assets/sub/evil.png', png(2, RED)],
      ['..\\evil.txt', 'a backslash'],
    ]);
    const reached = await upload('reaching.zip', reaching);
    assert.equal(reached.status, 200);
    // The last of them begins with a dot, and is left out without a word
    // like any hidden entry.
    assert.deepEqual(reached.done, [imported('reaching'),
      'These entries of the archive are no files of a theme and were left out: ../evil.yml, ../../evil.txt, assets/../../evil.png, /tmp/evil.txt, assets/sub/evil.png']);
    assert.deepEqual(await filesOf(path.join(server.themes, 'reaching')), ['theme.yml']);
    assert.deepEqual((await filesOf(server.dir)).filter(name => !outside.includes(name)), ['themes/reaching/theme.yml']);
    assert.deepEqual((await fs.readdir(server.themes, {recursive: true})).filter(name => name.includes('evil')), []);

    // An archive that is not the archive of a usable theme is refused with
    // the reason, and nothing of it stays.
    const before = await filesOf(server.themes);
    refused(await upload('broken.zip', Buffer.from('contract: 1\n')), 400, /cannot be read as a ZIP file/);
    refused(await upload('empty.zip', pack([['theme.css', STYLES]])), 400, /holds no theme\.yml/);
    refused(await upload('loading.zip', pack([['theme.yml', COLORS], ['theme.css', '@import "a.css";']])), 400, /at-rule that is not accepted: theme\.css line 1: @import/);
    refused(await upload('fake.zip', pack([['theme.yml', COLORS], ['assets/fake.png', '<svg onload=alert(1)>']])), 400, /assets\/fake\.png/);
    refused(await upload('Zelda.zip', archive), 400, /<name>\.zip/);
    assert.deepEqual(await filesOf(server.themes), before);
    assert.deepEqual((await fs.readdir(server.themes, {withFileTypes: true})).filter(entry => entry.name.startsWith('.')).map(entry => entry.name), []);

    assert.deepEqual(elsewhere, []);
    console.log('PASS: a theme that is a directory is downloaded as an archive and imported from one, and only files of a theme are written');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
