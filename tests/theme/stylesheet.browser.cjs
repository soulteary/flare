// FLARE_BINARY=/path/to/binary node tests/theme/stylesheet.browser.cjs
// The stylesheet of a theme, as documented in docs/theme-stylesheets.md: a
// theme that is a directory can bring a theme.css, the navigation pages load
// it and follow it, and it loads nothing itself. The last part is tested
// against the browser: stylesheets that try to load an image from another
// origin in many spellings are given to the instance, and to the browser
// unchecked. Whatever the browser loads unchecked, the instance has to refuse.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const http = require('node:http');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

const COLORS = 'contract: 1\ntokens:\n  background: "#10241b"\n  primary: "#e8f1e4"\n  accent: "#f2b84b"\n';
const STYLED = `/* The tiles of this theme have a frame. */
.app-item { outline: 3px solid rgb(1, 2, 3); }
.app-title { letter-spacing: 2px; }
/* A token the theme file sets stays as the file has it; one it leaves alone
   can be set here. */
body { --flare-radius: 30px; --flare-hover: rgb(9, 9, 9); }
`;

// The colors of the stylesheet, used when the selected theme cannot be used.
const DEFAULT_BACKGROUND = 'rgb(45, 52, 54)';

// Stylesheets that name an image on another origin, and whether the instance
// is expected to refuse them. Each is given the address to name. The rules
// apply to elements of the navigation pages and of the page the browser is
// shown them on unchecked.
const ALL = 'body, .probe, .app-item';
const AFTER = 'body::after, .probe::after';
const CORPUS = [
  ['plain', true, c => `${ALL} { background: url(${c}) }`],
  ['quoted', true, c => `${ALL} { background: url("${c}") }`],
  ['single-quoted', true, c => `${ALL} { background: url('${c}') }`],
  ['spaces', true, c => `${ALL} { background: url(   ${c}   ) }`],
  ['upper-case', true, c => `${ALL} { background: URL(${c}) }`],
  ['mixed-case', true, c => `${ALL} { background: uRl(${c}) }`],
  ['escaped', true, c => `${ALL} { background: \\75rl(${c}) }`],
  ['escaped-space', true, c => `${ALL} { background: u\\72 l(${c}) }`],
  ['escaped-upper', true, c => `${ALL} { background: \\55\\52\\4c(${c}) }`],
  ['escaped-six-digits', true, c => `${ALL} { background: \\000075rl(${c}) }`],
  ['escaped-letters', true, c => `${ALL} { background: \\u\\r\\l(${c}) }`],
  ['after-comment', true, c => `${ALL} { background: /**/url(${c}) }`],
  ['after-broken-string', true, c => `${ALL} { content: "a\n; background: url(${c}) }`],
  ['after-broken-string-cr', true, c => `${ALL} { content: "a\r; background: url(${c}) }`],
  ['after-broken-string-ff', true, c => `${ALL} { content: 'a\f; background: url(${c}) }`],
  ['after-continued-string', true, c => `${ALL} { quotes: "a\\\nb" "c"; background: url(${c}) }`],
  ['after-closed-comment', true, c => `/* a \\*/ ${ALL} { background: url(${c}) }`],
  ['byte-order-mark', true, c => `\uFEFF${ALL} { background: url(${c}) }`],
  ['custom-property', true, c => `${ALL} { --image: url(${c}); background: var(--image) }`],
  ['in-media', true, c => `@media all { ${ALL} { background: url(${c}) } }`],
  ['in-supports', true, c => `@supports (display: block) { ${ALL} { background: url(${c}) } }`],
  ['in-layer', true, c => `@layer theme { ${ALL} { background: url(${c}) } }`],
  ['nested', true, c => `${ALL} { & { background: url(${c}) } }`],
  ['in-keyframes', true, c => `@keyframes load { from { background: url(${c}) } to { background: url(${c}) } } ${ALL} { animation: load 1s infinite }`],
  ['content', true, c => `${AFTER} { content: url(${c}) }`],
  ['list-style', true, c => `${ALL} { display: list-item; list-style-image: url(${c}) }`],
  ['border-image', true, c => `${ALL} { border: 4px solid; border-image: url(${c}) 1 }`],
  ['mask', true, c => `${ALL} { -webkit-mask-image: url(${c}); mask-image: url(${c}) }`],
  ['cursor', true, c => `${ALL} { cursor: url(${c}), auto }`],
  ['by-attribute', true, c => `a[href^="https://example.test"] { background: url(${c}) }`],
  ['image-set', true, c => `${ALL} { background: image-set("${c}" 1x) }`],
  ['image-set-url', true, c => `${ALL} { background: image-set(url(${c}) 1x) }`],
  ['prefixed-image-set', true, c => `${ALL} { background: -webkit-image-set(url(${c}) 1x) }`],
  ['cross-fade', true, c => `${ALL} { background: -webkit-cross-fade(url(${c}), url(${c}), 50%) }`],
  ['image', true, c => `${ALL} { background: image("${c}") }`],
  ['src', true, c => `${ALL} { background: src("${c}") }`],
  ['font-face', true, c => `@font-face { font-family: loaded; src: url(${c}) } ${ALL} { font-family: loaded }`],
  ['import', true, c => `@import "${c}";`],
  ['import-url', true, c => `@import url(${c});`],
  ['import-upper-case', true, c => `@IMPORT '${c}';`],
  ['import-escaped', true, c => `@\\69mport "${c}";`],
  ['import-layer', true, c => `@import "${c}" layer(theme);`],
  ['import-supports', true, c => `@import url(${c}) supports(display: block);`],
  ['multiplication-sign', true, c => `${ALL} { background: #\u00D7url(${c}) }`],
  ['division-sign', true, c => `${ALL} { background: 1\u00F7url(${c}) }`],
  // The same text where it is not an address.
  ['in-string', false, c => `${AFTER} { content: "url(${c})" }`],
  ['in-comment', false, c => `/* ${ALL} { background: url(${c}) } */ ${ALL} { color: rgb(1, 2, 3) }`],
  ['in-open-comment', false, c => `${ALL} { color: rgb(1, 2, 3) } /* ${ALL} { background: url(${c}) }`],
  ['in-comment-with-backslash', false, c => `/* a *\\/ ${ALL} { background: url(${c}) } */`],
  ['in-open-string', false, c => `${AFTER} { content: "url(${c})`],
  ['after-escaped-quote', false, c => `${AFTER} { content: "a\\" url(${c})" }`],
  ['in-attribute', false, c => `a[href="url(${c})"] { color: rgb(1, 2, 3) }`],
  ['as-unit', false, c => `${ALL} { width: 1url(${c}) }`],
  ['as-hash', false, c => `#url(${c}) { color: rgb(1, 2, 3) }`],
  ['space-before-parenthesis', false, c => `${ALL} { background: url (${c}) }`],
  ['escaped-parenthesis', false, c => `${ALL} { background: url\\(${c}) }`],
];

async function freePort() {
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  return port;
}

// The other origin. It notes what is requested from it, and shows the
// browser a stylesheet unchecked, on a page with elements its rules apply to.
async function startOtherOrigin(stylesheets) {
  const hits = new Set();
  const server = http.createServer((request, response) => {
    const [, kind, name] = new URL(request.url, 'http://other').pathname.split('/');
    if (kind === 'hit') {
      hits.add(name.replace(/\.png$/, ''));
      response.writeHead(204);
      return response.end();
    }
    if (kind === 'unchecked' && stylesheets.has(name)) {
      response.writeHead(200, {'Content-Type': 'text/html; charset=utf-8'});
      return response.end(`<!doctype html><meta charset="utf-8"><link rel="stylesheet" href="/css/${name}">` +
        `<body><div class="probe app-item">probe</div><a class="probe" href="https://example.test/page">link</a></body>`);
    }
    if (kind === 'css' && stylesheets.has(name)) {
      response.writeHead(200, {'Content-Type': 'text/css; charset=utf-8'});
      return response.end(stylesheets.get(name));
    }
    response.writeHead(404);
    response.end();
  });
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve));
  return {base: `http://127.0.0.1:${server.address().port}`, hits, stop: () => new Promise(resolve => server.close(resolve))};
}

async function start(prepare) {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-stylesheet-'));
  const port = await freePort();
  await fs.writeFile(path.join(dir, 'config.yml'),
    'Locale: en\nTheme: blackboard\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowWeather: false\n');
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

const writeTheme = async (themes, name, manifest, stylesheet) => {
  await fs.mkdir(path.join(themes, name), {recursive: true});
  await fs.writeFile(path.join(themes, name, 'theme.yml'), manifest);
  if (stylesheet !== undefined) await fs.writeFile(path.join(themes, name, 'theme.css'), stylesheet);
};

// Fonts and the first frames of an animation are loaded after the page is.
const settled = page => page.evaluate(() => document.fonts.ready.then(() =>
  new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)))));

const measure = page => page.evaluate(() => {
  const style = (selector, property) => getComputedStyle(document.querySelector(selector))[property];
  return {
    background: style('body', 'backgroundColor'),
    outline: style('.app-item', 'outlineColor') + ' ' + style('.app-item', 'outlineWidth'),
    spacing: style('.app-title', 'letterSpacing'),
    radius: style('.app-item', 'borderTopLeftRadius'),
    hover: style('.app-item', 'backgroundColor'),
    stylesheets: Array.from(document.querySelectorAll('link[rel="stylesheet"]'), link => link.getAttribute('href')),
  };
});

(async () => {
  const browser = await chromium.launch({headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox']});
  const stylesheets = new Map();
  const other = await startOtherOrigin(stylesheets);
  for (const [name, , write] of CORPUS) stylesheets.set(name, write(`${other.base}/hit/${name}.png`));
  const server = await start(async themes => {
    await writeTheme(themes, 'styled', COLORS + '  radius: 10px\n', STYLED);
    await writeTheme(themes, 'paired', COLORS + '  radius: 8px\nmodes:\n  light:\n    background: "#f6f3ea"\n    primary: "#2b2a27"\n', STYLED);
    await writeTheme(themes, 'bare', COLORS);
    // One theme for each stylesheet of the list, so that each is accepted or
    // refused by itself.
    for (const [name, stylesheet] of stylesheets) await writeTheme(themes, 'c-' + name, COLORS, stylesheet);
  });
  try {
    const page = await browser.newPage({viewport: {width: 1280, height: 900}});
    page.setDefaultTimeout(10000);
    // Everything the pages of the instance ask for comes from the instance.
    const elsewhere = [];
    page.on('request', request => {
      if (!request.url().startsWith(server.base + '/') && !request.url().startsWith('data:')) elsewhere.push(request.url());
    });
    const home = async (query = '') => {
      await page.goto(server.base + '/' + query, {waitUntil: 'load'});
      await page.addStyleTag({content: '* { transition: none !important; }'});
      await page.locator('.app-item').hover();
      await settled(page);
      return measure(page);
    };
    const settings = async () => {
      await page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
      return {
        cards: await page.locator('.theme-file').evaluateAll(cards => cards.map(card => ({
          value: card.querySelector('input').value,
          about: Array.from(card.querySelectorAll('.theme-about'), about => about.textContent),
        }))),
        lines: await page.locator('.theme-file-lines li').evaluateAll(lines => lines.map(line =>
          Array.from(line.querySelectorAll('code'), code => code.textContent))),
        alert: await page.locator('form:not(.theme-import-form) .theme-notice[role="alert"]').allTextContents(),
        stylesheets: await page.locator('link[rel="stylesheet"]').count(),
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

    // The theme page says which themes bring a stylesheet.
    let listed = await settings();
    const about = Object.fromEntries(listed.cards.filter(card => !card.value.startsWith('c-')).map(card => [card.value, card.about]));
    assert.deepEqual(about, {bare: [], paired: ['Has a stylesheet of its own.'], styled: ['Has a stylesheet of its own.']});
    assert.deepEqual((await home()).stylesheets, []);

    // Selected, the navigation pages load the stylesheet after the styles of
    // the application: its rules apply. A token the theme file sets keeps the
    // value of the file, and one the file leaves alone can be set by the
    // stylesheet. The settings do not load it.
    await settings();
    await select(page.locator('.theme-file input[value="styled"]'));
    const styled = await home();
    assert.equal(styled.stylesheets.length, 1);
    assert.match(styled.stylesheets[0], /^\/theme\/styled\/[0-9a-f]{16}\/theme\.css$/);
    assert.deepEqual({...styled, stylesheets: null}, {background: 'rgb(16, 36, 27)', outline: 'rgb(1, 2, 3) 3px',
      spacing: '2px', radius: '10px', hover: 'rgb(9, 9, 9)', stylesheets: null});
    for (const path of ['/applications', '/bookmarks', '/help']) {
      await page.goto(server.base + path, {waitUntil: 'load'});
      assert.deepEqual(await page.locator('link[rel="stylesheet"]').evaluateAll(links => links.map(link => link.getAttribute('href'))),
        styled.stylesheets, path);
    }
    listed = await settings();
    assert.equal(listed.stylesheets, 0);
    assert.equal(await page.evaluate(() => getComputedStyle(document.body).backgroundColor), 'rgb(16, 36, 27)');

    // With values for light and for dark the tokens are rules of the page.
    // They follow the stylesheet, so the value of the theme file holds there
    // too, for both.
    for (const scheme of ['light', 'dark']) {
      await page.emulateMedia({colorScheme: scheme});
      const paired = await home('?theme-preview=paired');
      assert.equal(paired.radius, '8px', scheme);
      assert.equal(paired.outline, 'rgb(1, 2, 3) 3px', scheme);
      assert.equal(paired.background, scheme === 'light' ? 'rgb(246, 243, 234)' : 'rgb(16, 36, 27)', scheme);
    }
    await page.emulateMedia({colorScheme: 'light'});

    // An edit of the stylesheet shows on the next page load, under another
    // address.
    const file = path.join(server.themes, 'styled', 'theme.css');
    await fs.writeFile(file, STYLED.replace('rgb(1, 2, 3)', 'rgb(4, 5, 6)'));
    const edited = await home();
    assert.equal(edited.outline, 'rgb(4, 5, 6) 3px');
    assert.notEqual(edited.stylesheets[0], styled.stylesheets[0]);
    assert.equal((await fetch(server.base + styled.stylesheets[0])).status, 404);

    // A stylesheet that is not accepted leaves the pages with the colors of
    // the stylesheet of the application, and the theme page says where and
    // why.
    await fs.writeFile(file, `@import "${other.base}/hit/broken.png";\n` + STYLED);
    const broken = await home();
    assert.equal(broken.background, DEFAULT_BACKGROUND);
    assert.deepEqual(broken.stylesheets, []);
    listed = await settings();
    assert.equal(listed.alert.length, 1);
    assert.match(listed.alert[0], /“styled”/);
    assert.deepEqual(listed.lines.find(line => line[0] === 'styled/theme.css'), ['styled/theme.css', 'line 1: @import']);
    await fs.writeFile(file, STYLED);
    assert.equal((await home()).outline, 'rgb(1, 2, 3) 3px');
    assert.deepEqual((await settings()).alert, []);
    assert.equal(other.hits.has('broken'), false);

    // What the browser does with each stylesheet of the list unchecked: it
    // is shown on a page of the other origin, and that origin notes whether
    // it was asked for the image.
    const unchecked = await browser.newPage();
    for (const [name] of CORPUS) {
      await unchecked.goto(`${other.base}/unchecked/${name}`, {waitUntil: 'load'});
      await settled(unchecked);
    }
    await unchecked.close();
    const loaded = new Set(other.hits);
    assert.ok(loaded.has('plain'), 'the browser did not load the image of a plain url(): the comparison would show nothing');

    // What the instance does with each: a theme whose stylesheet is accepted
    // is listed, and one whose stylesheet is refused is named with it.
    listed = await settings();
    const accepted = new Set(listed.cards.map(card => card.value));
    const refused = new Set(listed.lines.map(line => line[0]));
    for (const [name, expectRefused] of CORPUS) {
      const isRefused = refused.has(`c-${name}/theme.css`);
      assert.notEqual(accepted.has(`c-${name}`), isRefused, `${name} is neither accepted nor refused, or both`);
      // The guarantee: what loads from elsewhere is never accepted.
      assert.ok(!loaded.has(name) || isRefused, `${name} loads from another origin and is accepted`);
      assert.equal(isRefused, expectRefused, `${name}: ${isRefused ? 'refused' : 'accepted'}`);
    }

    // The accepted ones are applied to a page of the instance, and nothing
    // is asked of the other origin.
    other.hits.clear();
    for (const [name, expectRefused] of CORPUS) {
      if (expectRefused) continue;
      const shown = await home(`?theme-preview=c-${name}`);
      assert.equal(shown.stylesheets.length, 1, name);
      assert.ok(shown.stylesheets[0].startsWith(`/theme/c-${name}/`), name);
    }
    assert.deepEqual(Array.from(other.hits), []);
    assert.deepEqual(elsewhere, []);
    const refusedCount = CORPUS.filter(entry => entry[1]).length;
    const notLoaded = CORPUS.filter(entry => entry[1] && !loaded.has(entry[0])).map(entry => entry[0]);
    console.log(`PASS: the stylesheet of a theme is loaded and followed by the navigation pages, and loads nothing: ` +
      `${refusedCount - notLoaded.length} of the ${refusedCount} refused stylesheets load from another origin when the browser gets them unchecked ` +
      `(this browser loads nothing for: ${notLoaded.join(', ') || 'none'}), ` +
      `and none of the ${CORPUS.length - refusedCount} accepted ones does`);
  } finally {
    await server.stop();
    await other.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
