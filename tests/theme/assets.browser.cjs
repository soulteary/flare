// FLARE_BINARY=/path/to/binary node tests/theme/assets.browser.cjs
// The images and fonts of a theme, as documented in docs/theme-stylesheets.md:
// a theme that is a directory can bring them in its assets, its stylesheet
// refers to them as url(assets/<name>), and the pages follow them. A
// stylesheet loads nothing else. That part is tested against the browser:
// stylesheets that name a file in many spellings are given to the instance,
// and to the browser unchecked. For every one the instance accepts, the
// browser has to ask for the assets of the theme and for nothing else.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const http = require('node:http');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const zlib = require('node:zlib');
const {spawn} = require('node:child_process');

const COLORS = 'contract: 1\ntokens:\n  background: "#10241b"\n  primary: "#e8f1e4"\n  accent: "#f2b84b"\n';
const MANIFEST = COLORS + '  font-family: Brand, sans-serif\n';
const STYLESHEET = `@font-face {
  font-family: Brand;
  src: url(assets/brand.woff2) format("woff2");
}
body { background-image: url("assets/paper.png"); }
.app-item { background-image: url(assets/tile.png); }
`;
// A font of the repository, which a browser accepts as a font.
const FONT = path.join(__dirname, '..', '..', 'embed', 'assets', 'vendor', 'mdi-cheat-sheets', 'fonts', 'materialdesignicons-webfont.woff2');

// The background of the stylesheet of the application, used when the
// selected theme cannot be used.
const DEFAULT_BACKGROUND = 'rgb(45, 52, 54)';

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
  // The byte before the pixel says how the row is stored; a row of one pixel
  // without a row above it is stored as it is in every case.
  return Array.from(zlib.inflateSync(Buffer.concat(data)).subarray(1, 4));
}

const RED = [220, 20, 30], GREEN = [20, 160, 60], BLUE = [30, 60, 200];

// Stylesheets that name a file, and whether the instance is expected to
// refuse them. Each theme of the list has the assets a.png and b.png, and
// files that are not assets: notes.txt and more/c.png in the directory of the
// assets, and secret.png beside it. A stylesheet is given the address of
// another origin to name. The rules apply to elements of the navigation
// pages and of the page the browser is shown them on unchecked.
const ALL = 'body, .probe, .app-item';
const AFTER = 'body::after, .probe::after';
const CORPUS = [
  // A file of the assets, in the ways it can be written.
  ['plain', false, () => `${ALL} { background: url(assets/a.png) }`],
  ['quoted', false, () => `${ALL} { background: url("assets/a.png") }`],
  ['single-quoted', false, () => `${ALL} { background: url('assets/a.png') }`],
  ['spaces', false, () => `${ALL} { background: url(   assets/a.png   ) }`],
  ['line-breaks', false, () => `${ALL} { background: url(\n"assets/a.png"\n) }`],
  ['upper-case', false, () => `${ALL} { background: URL(assets/a.png) }`],
  ['escaped-name', false, () => `${ALL} { background: \\75rl(assets/a.png) }`],
  ['escaped-letter', false, () => `${ALL} { background: url(assets/\\61.png) }`],
  ['escaped-dot', false, () => `${ALL} { background: url(assets/a\\.png) }`],
  ['escaped-in-quotes', false, () => `${ALL} { background: url("assets/\\61 .png") }`],
  ['two-files', false, () => `${ALL} { background: url(assets/a.png), url('assets/b.png') }`],
  ['custom-property', false, () => `${ALL} { --image: url(assets/a.png); background: var(--image) }`],
  ['in-media', false, () => `@media all { ${ALL} { background: url(assets/a.png) } }`],
  ['content', false, () => `${AFTER} { content: url(assets/a.png) }`],
  ['border-image', false, () => `${ALL} { border: 4px solid; border-image: url(assets/a.png) 1 }`],
  ['mask', false, () => `${ALL} { -webkit-mask-image: url(assets/a.png); mask-image: url(assets/a.png) }`],
  ['list-style', false, () => `${ALL} { display: list-item; list-style-image: url(assets/b.png) }`],
  ['font-face', false, () => `@font-face { font-family: loaded; src: url(assets/a.png) format("woff2") } ${ALL} { font-family: loaded }`],
  // The address of another origin where it is not an address.
  ['in-string', false, o => `${AFTER} { content: "url(${o}/hit.png)" } ${ALL} { background: url(assets/a.png) }`],
  ['in-comment', false, o => `/* ${ALL} { background: url(${o}/hit.png) } */ ${ALL} { background: url(assets/a.png) }`],
  // Addresses a browser brings into the form of a file of the assets on its
  // way to a request, and addresses that are that file with more to them.
  ['this-directory', true, () => `${ALL} { background: url(./assets/a.png) }`],
  ['inner-this-directory', true, () => `${ALL} { background: url(assets/./a.png) }`],
  ['backslash', true, () => `${ALL} { background: url(assets\\\\a.png) }`],
  ['encoded-letter', true, () => `${ALL} { background: url(assets/%61.png) }`],
  ['tab', true, () => `${ALL} { background: url("assets/a\\9 .png") }`],
  ['line-feed', true, () => `${ALL} { background: url("assets/a\\a .png") }`],
  ['space-before', true, () => `${ALL} { background: url(" assets/a.png") }`],
  ['fragment', true, () => `${ALL} { background: url(assets/a.png#part) }`],
  ['query', true, () => `${ALL} { background: url(assets/a.png?v=2) }`],
  // Addresses that lead out of the assets.
  ['parent', true, () => `${ALL} { background: url(assets/../secret.png) }`],
  ['parent-backslash', true, () => `${ALL} { background: url("assets\\\\..\\\\secret.png") }`],
  ['parent-encoded', true, () => `${ALL} { background: url(assets/%2e%2e/secret.png) }`],
  ['parent-escaped', true, () => `${ALL} { background: url(assets/\\2e\\2e/secret.png) }`],
  ['parent-tab', true, () => `${ALL} { background: url("assets/.\\9 ./secret.png") }`],
  ['other-theme', true, () => `${ALL} { background: url(assets/../../../c-plain/0000000000000000/assets/a.png) }`],
  ['beside', true, () => `${ALL} { background: url(secret.png) }`],
  ['theme-file', true, () => `${ALL} { background: url(theme.yml) }`],
  ['nested', true, () => `${ALL} { background: url(assets/more/c.png) }`],
  ['double-slash', true, () => `${ALL} { background: url(assets//a.png) }`],
  ['from-root', true, () => `${ALL} { background: url(/assets/a.png) }`],
  ['upper-case-directory', true, () => `${ALL} { background: url(ASSETS/a.png) }`],
  ['other-origin', true, o => `${ALL} { background: url(${o}/assets/a.png) }`],
  ['other-origin-no-scheme', true, o => `${ALL} { background: url(${o.replace('http:', '')}/assets/a.png) }`],
  ['data', true, () => `${ALL} { background: url(data:image/gif;base64,R0lGODlhAQABAAAAACw=) }`],
  // Files that are not among the assets.
  ['not-an-asset', true, () => `${ALL} { background: url(assets/notes.txt) }`],
  ['missing', true, () => `${ALL} { background: url(assets/c.png) }`],
  ['other-case', true, () => `${ALL} { background: url(assets/A.png) }`],
  // Other ways to name a file, and a rule after an address that is accepted.
  ['modifier', true, () => `${ALL} { background: url("assets/a.png" crossorigin(anonymous)) }`],
  ['image-set', true, () => `${ALL} { background: image-set("assets/a.png" 1x) }`],
  ['image-set-url', true, () => `${ALL} { background: image-set(url(assets/a.png) 1x) }`],
  ['import', true, () => `@import "assets/a.png";`],
  ['font-face-elsewhere', true, o => `@font-face { font-family: loaded; src: url(${o}/font.woff2) } ${ALL} { font-family: loaded }`],
  ['after-accepted', true, o => `${ALL} { background: url(assets/a.png) } .probe { background: url(${o}/hit.png) }`],
];
// The hash the stylesheets have where the browser gets them unchecked.
const UNCHECKED = '0000000000000000';

async function freePort() {
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  return port;
}

// Another origin. It shows the browser a stylesheet unchecked, under an
// address of the form the instance serves it at, on a page with elements its
// rules apply to, and it has the assets a browser may ask for.
async function startOtherOrigin(stylesheets, image) {
  const server = http.createServer((request, response) => {
    const parts = new URL(request.url, 'http://other').pathname.split('/').slice(1);
    const name = (parts[1] || '').replace(/^c-/, '');
    if (parts[0] === 'unchecked' && stylesheets.has(parts[1])) {
      response.writeHead(200, {'Content-Type': 'text/html; charset=utf-8'});
      return response.end(`<!doctype html><meta charset="utf-8"><link rel="stylesheet" href="/theme/c-${parts[1]}/${UNCHECKED}/theme.css">` +
        `<body><div class="probe app-item">probe</div><a class="probe" href="https://example.test/page">link</a></body>`);
    }
    if (parts[0] === 'theme' && parts[2] === UNCHECKED && stylesheets.has(name)) {
      if (parts.length === 4 && parts[3] === 'theme.css') {
        response.writeHead(200, {'Content-Type': 'text/css; charset=utf-8'});
        return response.end(stylesheets.get(name));
      }
      if (parts.length === 5 && parts[3] === 'assets' && ['a.png', 'b.png'].includes(parts[4])) {
        response.writeHead(200, {'Content-Type': 'image/png'});
        return response.end(image);
      }
    }
    response.writeHead(404);
    response.end();
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  return {base: `http://127.0.0.1:${server.address().port}`, stop: () => new Promise(resolve => server.close(resolve))};
}

async function start(prepare) {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-assets-'));
  const port = await freePort();
  await fs.writeFile(path.join(dir, 'config.yml'),
    'Locale: en\nTheme: linen\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowWeather: false\n');
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

// writeTheme writes a theme that is a directory: its theme file, its
// stylesheet, and files by their paths in the directory.
const writeTheme = async (themes, name, manifest, stylesheet, files = {}) => {
  await fs.mkdir(path.join(themes, name), {recursive: true});
  await fs.writeFile(path.join(themes, name, 'theme.yml'), manifest);
  if (stylesheet !== undefined) await fs.writeFile(path.join(themes, name, 'theme.css'), stylesheet);
  for (const [file, content] of Object.entries(files)) {
    await fs.mkdir(path.dirname(path.join(themes, name, file)), {recursive: true});
    await fs.writeFile(path.join(themes, name, file), content);
  }
};

// Fonts and images are loaded after the page is.
const settled = page => page.evaluate(() => document.fonts.ready.then(() =>
  new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))));

// What a page has of the theme.
const measure = page => page.evaluate(async () => {
  // A page without the element has no value for it.
  const style = (selector, property) => {
    const element = document.querySelector(selector);
    return element ? getComputedStyle(element)[property] : '';
  };
  // The size of an image, read from the address a rule resolved to.
  const size = async value => {
    const address = /^url\("([^"]*)"\)$/.exec(value);
    if (!address) return 0;
    const image = new Image();
    image.src = address[1];
    await image.decode();
    return image.naturalWidth;
  };
  return {
    color: style('body', 'backgroundColor'),
    background: style('body', 'backgroundImage'),
    backgroundSize: await size(style('body', 'backgroundImage')),
    tile: style('.app-item', 'backgroundImage'),
    family: style('body', 'fontFamily'),
    fonts: Array.from(document.fonts, face => `${face.family} ${face.status}`),
    stylesheets: Array.from(document.querySelectorAll('link[rel="stylesheet"]'), link => link.getAttribute('href')),
  };
});

(async () => {
  const browser = await chromium.launch({headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox']});
  const stylesheets = new Map();
  const other = await startOtherOrigin(stylesheets, png(2, BLUE));
  for (const [name, , write] of CORPUS) stylesheets.set(name, write(other.base));
  const font = await fs.readFile(FONT);
  const server = await start(async themes => {
    await writeTheme(themes, 'linen', MANIFEST, STYLESHEET, {
      'assets/paper.png': png(2, RED),
      'assets/tile.png': png(2, BLUE),
      'assets/brand.woff2': font,
      // Entries of the directory that are not assets.
      'assets/notes.txt': 'NOTES-OF-THE-AUTHOR',
      'assets/.DS_Store': 'state of a file manager',
      'assets/more/nested.png': png(2, GREEN),
      'preview.png': png(2, GREEN),
    });
    // One theme for each stylesheet of the list, so that each is accepted or
    // refused by itself.
    for (const [name, stylesheet] of stylesheets) {
      await writeTheme(themes, 'c-' + name, COLORS, stylesheet, {
        'assets/a.png': png(2, RED), 'assets/b.png': png(2, BLUE), 'assets/notes.txt': 'notes',
        'assets/more/c.png': png(2, GREEN), 'secret.png': png(2, GREEN),
      });
    }
  });
  try {
    const context = await browser.newContext({viewport: {width: 1280, height: 900}});
    const page = await context.newPage();
    page.setDefaultTimeout(10000);
    // Everything the pages of the instance ask for comes from the instance.
    // What they ask of a theme is noted with the answer.
    const elsewhere = [];
    let asked = [];
    page.on('request', request => {
      if (!request.url().startsWith(server.base + '/') && !request.url().startsWith('data:')) elsewhere.push(request.url());
    });
    page.on('response', response => {
      const address = response.url().slice(server.base.length);
      if (response.url().startsWith(server.base + '/theme/')) {
        asked.push(`${response.status()} ${address} ${response.headers()['content-type'] || ''}`);
      }
    });
    const open = async address => {
      asked = [];
      await page.goto(server.base + address, {waitUntil: 'load'});
      await settled(page);
      return measure(page);
    };
    // The color the page has at a place where nothing but the background of
    // the body is painted.
    const corner = async () => firstPixel(await page.screenshot({clip: {x: 2, y: 890, width: 1, height: 1}}));
    const settings = async () => {
      asked = [];
      await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
      await settled(page);
      return {
        cards: await page.locator('.theme-file input').evaluateAll(inputs => inputs.map(input => input.value)),
        lines: await page.locator('.theme-file-lines li').evaluateAll(lines => lines.map(line => line.textContent.replace(/\s+/g, ' ').trim())),
        alert: await page.locator('form:not(.theme-import-form) .theme-notice[role="alert"]').allTextContents(),
        stylesheets: await page.locator('link[rel="stylesheet"]').count(),
        asked: asked.slice(),
      };
    };
    const assets = name => path.join(server.themes, name, 'assets');

    // The home page of the selected theme links the stylesheet. The browser
    // asks for the files the stylesheet refers to beside it, gets each as
    // the kind of file it is, and asks for nothing else of the theme.
    const first = await open('/');
    assert.equal(first.stylesheets.length, 1);
    const address = /^\/theme\/linen\/([0-9a-f]{16})\/theme\.css$/.exec(first.stylesheets[0]);
    assert.ok(address, first.stylesheets[0]);
    const base = `/theme/linen/${address[1]}/assets/`;
    assert.deepEqual(asked.slice().sort(), [
      `200 ${base}brand.woff2 font/woff2`,
      `200 ${base}paper.png image/png`,
      `200 ${base}tile.png image/png`,
      `200 ${first.stylesheets[0]} text/css; charset=utf-8`,
    ]);
    // The image is the background of the page, and the font is in use.
    assert.equal(first.background, `url("${server.base}${base}paper.png")`);
    assert.equal(first.backgroundSize, 2);
    assert.equal(first.tile, `url("${server.base}${base}tile.png")`);
    assert.deepEqual(await corner(), RED);
    assert.equal(first.family, 'Brand, sans-serif');
    assert.deepEqual(first.fonts, ['Brand loaded']);
    // The other navigation pages link the same stylesheet.
    for (const location of ['/applications', '/bookmarks', '/help']) {
      const shown = await open(location);
      assert.deepEqual(shown.stylesheets, first.stylesheets, location);
      assert.equal(shown.background, first.background, location);
    }

    // A file of the assets is kept by the browser for good, and is sent as
    // what it is and nothing else.
    const kept = await fetch(server.base + base + 'paper.png');
    assert.equal(kept.status, 200);
    assert.equal(kept.headers.get('cache-control'), 'public, max-age=31536000, immutable');
    assert.equal(kept.headers.get('x-content-type-options'), 'nosniff');
    assert.deepEqual(Buffer.from(await kept.arrayBuffer()), png(2, RED));
    // What is in the directory and is not an asset is not served.
    for (const file of ['assets/notes.txt', 'assets/.DS_Store', 'assets/more/nested.png', 'assets/more', 'assets/', 'preview.png', 'theme.yml',
      'assets/..%2Ftheme.yml', 'assets/%2e%2e/theme.yml', 'assets/PAPER.PNG']) {
      const response = await fetch(`${server.base}/theme/linen/${address[1]}/${file}`);
      assert.equal(response.status, 404, file);
      assert.doesNotMatch(await response.text(), /NOTES|contract|PNG/, file);
    }

    // The settings take the colors and the font of the theme, and load
    // neither its stylesheet nor its files.
    let listed = await settings();
    assert.equal(listed.stylesheets, 0);
    assert.deepEqual(listed.asked, []);
    assert.equal(await page.evaluate(() => getComputedStyle(document.body).backgroundImage), 'none');
    assert.deepEqual(listed.lines.filter(line => line.startsWith('linen/')), []);
    assert.ok(listed.cards.includes('linen'));

    // An image that is edited shows on the next page load: the stylesheet
    // and the files have another address, and the earlier one is gone.
    await fs.writeFile(path.join(assets('linen'), 'paper.png'), png(3, GREEN));
    const edited = await open('/');
    assert.notEqual(edited.stylesheets[0], first.stylesheets[0]);
    assert.match(edited.background, /^url\("http:\/\/127\.0\.0\.1:\d+\/theme\/linen\/[0-9a-f]{16}\/assets\/paper\.png"\)$/);
    assert.notEqual(edited.background, first.background);
    assert.equal(edited.backgroundSize, 3);
    assert.deepEqual(await corner(), GREEN);
    assert.deepEqual(edited.fonts, ['Brand loaded']);
    assert.equal(asked.filter(line => line.startsWith('200 ')).length, 4);
    assert.equal((await fetch(server.base + base + 'paper.png')).status, 404);
    assert.equal((await fetch(server.base + first.stylesheets[0])).status, 404);

    // A file that is added is served, and a file that is left alone changes
    // nothing.
    await fs.writeFile(path.join(assets('linen'), 'added.png'), png(2, BLUE));
    const added = await open('/');
    assert.notEqual(added.stylesheets[0], edited.stylesheets[0]);
    assert.equal((await fetch(server.base + added.stylesheets[0].replace('theme.css', 'assets/added.png'))).status, 200);
    await fs.writeFile(path.join(assets('linen'), 'license.txt'), 'the text of a license');
    assert.deepEqual((await open('/')).stylesheets, added.stylesheets);

    // A file that is not what its name says leaves the pages without the
    // theme, and the theme page names the file and the reason.
    await fs.writeFile(path.join(assets('linen'), 'tile.png'), '<!doctype html><title>not a picture</title>');
    const broken = await open('/');
    assert.deepEqual([broken.color, broken.background, broken.stylesheets], [DEFAULT_BACKGROUND, 'none', []]);
    assert.deepEqual(asked, []);
    listed = await settings();
    assert.equal(listed.alert.length, 1);
    assert.match(listed.alert[0], /“linen”/);
    assert.deepEqual(listed.lines.filter(line => line.startsWith('linen/')),
      ['linen/assets/tile.png The content is not that of the kind of file the name says.']);
    assert.ok(!listed.cards.includes('linen'));

    // A file the stylesheet refers to that is removed does the same, and
    // putting it back brings the theme back.
    await fs.rm(path.join(assets('linen'), 'tile.png'));
    assert.equal((await open('/')).color, DEFAULT_BACKGROUND);
    listed = await settings();
    assert.deepEqual(listed.lines.filter(line => line.startsWith('linen/')),
      ['linen/theme.css The stylesheet refers to a file that is not among the images and fonts in the assets of the theme: line 6: url(assets/tile.png)']);
    await fs.writeFile(path.join(assets('linen'), 'tile.png'), png(2, BLUE));
    const restored = await open('/');
    assert.match(restored.tile, /\/assets\/tile\.png"\)$/);
    assert.deepEqual(await corner(), GREEN);
    assert.deepEqual((await settings()).alert, []);

    // A preview of the theme has its files as well, while another theme is
    // selected.
    await fs.writeFile(path.join(server.dir, 'config.yml'),
      'Locale: en\nTheme: blackboard\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowWeather: false\n');
    await settings();
    const [saved] = await Promise.all([
      page.waitForResponse(response => response.request().method() === 'POST'),
      page.locator('.theme-blackboard input').click(),
    ]);
    assert.equal(saved.status(), 200);
    await page.waitForLoadState('load');
    const bundled = await open('/');
    assert.deepEqual([bundled.background, bundled.stylesheets, asked], ['none', [], []]);
    const previewed = await open('/?theme-preview=linen');
    assert.equal(previewed.backgroundSize, 3);
    assert.deepEqual(await corner(), GREEN);
    assert.deepEqual(previewed.fonts, ['Brand loaded']);

    // What the browser does with each stylesheet of the list unchecked: it
    // is shown on a page of the other origin, under an address of the form
    // the instance uses, and everything the browser asks for is noted.
    const requested = new Map();
    const unchecked = await browser.newPage();
    for (const [name] of CORPUS) {
      const urls = [];
      const note = request => {
        if (!request.url().startsWith('data:')) urls.push(request.url());
      };
      unchecked.on('request', note);
      await unchecked.goto(`${other.base}/unchecked/${name}`, {waitUntil: 'load'});
      await settled(unchecked);
      unchecked.off('request', note);
      const own = [`${other.base}/unchecked/${name}`, `${other.base}/theme/c-${name}/${UNCHECKED}/theme.css`];
      requested.set(name, urls.filter(url => !own.includes(url)));
    }
    await unchecked.close();
    // What a stylesheet of a theme may make a browser ask for: the files of
    // the assets of that theme.
    const outside = name => requested.get(name).filter(url =>
      !['a.png', 'b.png'].some(file => url === `${other.base}/theme/c-${name}/${UNCHECKED}/assets/${file}`));
    assert.deepEqual(requested.get('plain'), [`${other.base}/theme/c-plain/${UNCHECKED}/assets/a.png`],
      'the browser did not load the image of a plain url(): the comparison would show nothing');

    // What the instance does with each: a theme whose stylesheet is accepted
    // is listed, and one whose stylesheet is refused is named with it.
    listed = await settings();
    const accepted = new Set(listed.cards);
    const refused = new Set(listed.lines.map(line => line.split(' ')[0]));
    for (const [name, expectRefused] of CORPUS) {
      const isRefused = refused.has(`c-${name}/theme.css`);
      assert.notEqual(accepted.has(`c-${name}`), isRefused, `${name} is neither accepted nor refused, or both`);
      // The guarantee: a stylesheet that makes a browser ask for anything
      // but the assets of its theme is never accepted.
      assert.ok(isRefused || outside(name).length === 0, `${name} is accepted and asks for ${outside(name).join(', ')}`);
      assert.equal(isRefused, expectRefused, `${name}: ${isRefused ? 'refused' : 'accepted'}`);
    }

    // The accepted ones are applied to a page of the instance. The browser
    // asks for the stylesheet and for assets of that theme, gets each, and
    // asks for nothing else of a theme.
    for (const [name, expectRefused] of CORPUS) {
      if (expectRefused) continue;
      const shown = await open(`/?theme-preview=c-${name}`);
      assert.equal(shown.stylesheets.length, 1, name);
      const prefix = shown.stylesheets[0].replace(/theme\.css$/, '');
      assert.match(prefix, new RegExp(`^/theme/c-${name}/[0-9a-f]{16}/$`), name);
      assert.ok(asked.includes(`200 ${prefix}theme.css text/css; charset=utf-8`), name);
      for (const line of asked) {
        assert.ok([`200 ${prefix}theme.css text/css; charset=utf-8`, `200 ${prefix}assets/a.png image/png`, `200 ${prefix}assets/b.png image/png`].includes(line),
          `${name}: ${line}`);
      }
      assert.ok(asked.length > 1, `${name}: no file of the assets was asked for`);
    }
    assert.deepEqual(elsewhere, []);

    const refusedNames = CORPUS.filter(entry => entry[1]).map(entry => entry[0]);
    const leaving = refusedNames.filter(name => outside(name).length > 0);
    console.log(`PASS: the images and fonts of a theme are served beside its stylesheet and followed by the navigation pages, ` +
      `and a stylesheet loads nothing else: of the ${refusedNames.length} refused stylesheets, ${leaving.length} make the browser ask for something ` +
      `other than the assets of the theme when it gets them unchecked (this browser stays within the assets, or asks for nothing, for: ` +
      `${refusedNames.filter(name => !leaving.includes(name)).join(', ') || 'none'}), and none of the ${CORPUS.length - refusedNames.length} accepted ones does`);
  } finally {
    await server.stop();
    await other.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
