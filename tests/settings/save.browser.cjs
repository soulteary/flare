// Native settings forms through a proxy that overrides Referrer-Policy.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const http = require('node:http');
const fs = require('node:fs/promises');
const {spawn} = require('node:child_process');
(async () => {
  const dir = await fs.mkdtemp('/tmp/flare-settings-save-');
  const reserve = http.createServer();
  await new Promise(resolve => reserve.listen(0, '127.0.0.1', resolve));
  const port = reserve.address().port;
  await new Promise(resolve => reserve.close(resolve));
  await fs.writeFile(dir + '/config.yml', 'Locale: en\nTitle: Original\nShowWeather: false\n');
  const child = spawn(process.env.FLARE_BINARY, [], {cwd: dir, env: {...process.env,
    FLARE_PORT: String(port), FLARE_DISABLE_LOGIN: 'true', FLARE_OFFLINE: 'true', FLARE_BASELINE: '1'}});
  let log = '', browser;
  child.stderr.on('data', data => log += data);
  child.stdout.on('data', data => log += data);
  const proxy = http.createServer((req, res) => {
    const upstream = http.request({hostname: '127.0.0.1', port, path: req.url,
      method: req.method, headers: req.headers}, response => {
      res.writeHead(response.statusCode, {...response.headers, 'referrer-policy': 'no-referrer'});
      response.pipe(res);
    });
    upstream.on('error', error => {res.writeHead(502); res.end(error.message);});
    req.pipe(upstream);
  });
  try {
    const direct = `http://127.0.0.1:${port}`;
    let ready = false;
    for (let i = 0; i < 100; i++) {
      if (child.exitCode !== null) throw Error(log);
      try {if ((await fetch(direct + '/ping')).ok) {ready = true; break;}} catch {}
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    assert.ok(ready, log);
    await new Promise(resolve => proxy.listen(0, '127.0.0.1', resolve));
    const proxied = `http://127.0.0.1:${proxy.address().port}`;
    browser = await chromium.launch({headless: true, executablePath: process.env.CHROMIUM_EXECUTABLE || undefined,
      args: ['--no-sandbox']});
    const page = await browser.newPage();
    page.setDefaultTimeout(10000);
    for (const base of [direct, proxied]) {
      await page.goto(base + '/settings/appearance');
      const name = base === direct ? 'Saved directly' : 'Saved through proxy';
      await page.locator('#settings-title').fill(name);
      await page.locator('#mobile-bookmark-layout').selectOption('compact');
      const [saved] = await Promise.all([
        page.waitForResponse(r => r.request().method() === 'POST' && r.url().endsWith('/settings/appearance')),
        page.locator('input.btn-submit').click(),
      ]);
      assert.equal(saved.status(), 200, await saved.text());
      assert.equal(saved.request().headers().origin, base);
      await page.waitForLoadState('load');
      await page.reload();
      assert.equal(await page.locator('#settings-title').inputValue(), name);
      assert.equal(await page.locator('#mobile-bookmark-layout').inputValue(), 'compact');
      await page.locator('#mobile-bookmark-layout').selectOption('single');
      await Promise.all([page.waitForNavigation(), page.locator('input.btn-submit').click()]);
      assert.equal(await page.locator('#mobile-bookmark-layout').inputValue(), 'single');
      assert.match(await fs.readFile(dir + '/config.yml', 'utf8'), new RegExp(name));
    }
    // Theme and search use different native submit controls and token paths.
    for (const route of ['theme', 'search']) {
      await page.goto(proxied + '/settings/' + route);
      const [saved] = await Promise.all([
        page.waitForResponse(r => r.request().method() === 'POST' && r.url().endsWith('/settings/' + route)),
        page.locator(route === 'theme' ? 'input.theme-choose' : 'button.btn-submit').first().click(),
      ]);
      assert.equal(saved.status(), 200, await saved.text());
      assert.equal(saved.request().headers().origin, proxied);
      await page.waitForLoadState('load');
    }
    // The shared document policy covers every settings form, including search actions.
    for (const route of ['theme', 'weather', 'search', 'appearance', 'application']) {
      await page.goto(proxied + '/settings/' + route);
      assert.equal(await page.evaluate(() => document.referrerPolicy || document.querySelector('meta[name="referrer"]')?.content), 'same-origin');
    }
    // Fixing legitimate forms must not allow opaque or foreign origins to write.
    const before = await fs.readFile(dir + '/config.yml', 'utf8');
    for (const origin of ['null', 'https://attacker.example']) {
      const response = await fetch(direct + '/settings/appearance', {method: 'POST',
        headers: {'Origin': origin, 'Content-Type': 'application/x-www-form-urlencoded'}, body: 'title=Rejected'});
      assert.equal(response.status, 403);
      assert.equal((await response.json()).error.code, 'SOURCE_REJECTED');
    }
    assert.equal(await fs.readFile(dir + '/config.yml', 'utf8'), before);
    console.log('PASS: native settings save, proxy no-referrer override, persisted reload, all settings policies and hostile origin rejection');
  } finally {
    await browser?.close();
    if (proxy.listening) await new Promise(resolve => proxy.close(resolve));
    const exited = child.exitCode !== null ? Promise.resolve() : new Promise(resolve => child.once('exit', resolve));
    child.kill(); await exited;
    await fs.rm(dir, {recursive: true, force: true});
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
