// FLARE_BINARY=/path/to/binary node tests/mdi/icons.browser.cjs
// Bundled icons take their color from the page in both output modes: as SVG
// elements that reference one icon file each, and inline with FLARE_MINI_REQUEST.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

// The primary color of the blackboard theme, as browsers report it.
const PRIMARY = 'rgb(255, 253, 234)';
const ICONS = '.app-icon svg, a.bookmark svg';

async function start(env) {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-icons-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'),
    'Locale: en\nTheme: blackboard\nShowApps: true\nShowBookmarks: true\nShowWeather: false\n');
  await fs.writeFile(path.join(dir, 'apps.yml'),
    'links:\n- name: Circle\n  link: https://example.com/a\n  icon: circle\n' +
    '- name: Mixed case\n  link: https://example.com/b\n  icon: serverNetwork\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'),
    'links:\n- name: Home\n  link: https://example.com/c\n  icon: home\n- name: Again\n  link: https://example.com/d\n  icon: home\n');
  const child = spawn(process.env.FLARE_BINARY, [], {cwd: dir, env: {...process.env,
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
  return {base, async stop() {child.kill(); await fs.rm(dir, {recursive: true, force: true});}};
}

// Opens the home page and records the icon files the browser asks for.
async function open(browser, base, options) {
  const context = await browser.newContext(options);
  const page = await context.newPage();
  const requests = [];
  page.on('request', request => {
    const url = new URL(request.url());
    if (url.pathname.startsWith('/assets/mdi/')) requests.push(url.pathname);
  });
  await page.goto(base, {waitUntil: 'load'});
  return {context, page, requests};
}

const fills = (page, selector) => page.locator(selector).evaluateAll(
  elements => elements.map(element => getComputedStyle(element).fill));

// The color a browser uses for text under forced colors.
const systemText = page => page.evaluate(() => {
  const probe = document.createElement('span');
  probe.style.color = 'CanvasText';
  document.body.append(probe);
  const color = getComputedStyle(probe).color;
  probe.remove();
  return color;
});

(async () => {
  const browser = await chromium.launch({headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox']});
  let server;
  try {
    // Default output: one colorless file per icon, referenced from the page.
    server = await start({});
    let {context, page, requests} = await open(browser, server.base);
    assert.equal(await page.locator(ICONS).count(), 4);
    assert.equal(await page.locator('.app-icon svg.icon-ref, a.bookmark svg.icon-ref').count(), 4);
    assert.equal(await page.locator('.app-icon img, a.bookmark img').count(), 0);
    // A reference that did not load has an empty bounding box.
    await page.waitForFunction(() => [...document.querySelectorAll('svg.icon-ref use')]
      .every(use => use.getBBox().width > 0 && use.getBBox().height > 0));
    assert.deepEqual(await fills(page, ICONS), [PRIMARY, PRIMARY, PRIMARY, PRIMARY]);
    // Three distinct icons, one of them used twice and one written in mixed case.
    assert.deepEqual([...new Set(requests)].sort(),
      ['/assets/mdi/circle.svg', '/assets/mdi/home.svg', '/assets/mdi/servernetwork.svg']);
    const file = await page.request.get(server.base + '/assets/mdi/circle.svg');
    assert.equal(file.headers()['content-type'], 'image/svg+xml');
    assert.ok(!/fill|style/.test(await file.text()), 'icon files carry no color');
    // The same sizes as the images these icons replace.
    const size = selector => page.locator(selector).first().evaluate(element => {
      const box = element.getBoundingClientRect();
      return [box.width, box.height];
    });
    assert.deepEqual(await size('.app-icon svg'), [32, 32]);
    assert.deepEqual(await size('a.bookmark svg'), [20, 20]);
    // A color change reaches the icons without another request.
    const seen = requests.length;
    await page.evaluate(() => document.body.style.setProperty('--color-primary', '#ff0000'));
    assert.deepEqual(await fills(page, ICONS), Array(4).fill('rgb(255, 0, 0)'));
    assert.equal(requests.length, seen);
    await context.close();

    // Forced colors replace the page background; icons use the system text color.
    ({context, page} = await open(browser, server.base, {forcedColors: 'active'}));
    assert.equal(await page.evaluate(() => matchMedia('(forced-colors: active)').matches), true);
    let text = await systemText(page);
    assert.notEqual(text, PRIMARY);
    assert.deepEqual(await fills(page, ICONS), Array(4).fill(text));
    await context.close();
    await server.stop();

    // FLARE_MINI_REQUEST keeps the icons inside the page.
    server = await start({FLARE_MINI_REQUEST: 'true'});
    ({context, page, requests} = await open(browser, server.base));
    assert.equal(await page.locator(ICONS).count(), 4);
    assert.equal(await page.locator('svg.icon-ref').count(), 0);
    assert.deepEqual(await fills(page, '.app-icon svg path, a.bookmark svg path'), Array(4).fill(PRIMARY));
    assert.deepEqual(requests, []);
    await context.close();
    ({context, page} = await open(browser, server.base, {forcedColors: 'active'}));
    text = await systemText(page);
    assert.deepEqual(await fills(page, '.app-icon svg path, a.bookmark svg path'), Array(4).fill(text));
    await context.close();
    console.log('PASS: bundled icons follow the page color as referenced files and inline, also under forced colors');
  } finally {
    if (server) await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
