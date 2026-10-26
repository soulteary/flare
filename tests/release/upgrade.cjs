// Isolated published-baseline upgrade, mutation, restart and restored-backup rollback.
// FLARE_OLD_BINARY=... FLARE_BINARY=... FLARE_RELEASE_EVIDENCE=... node tests/release/upgrade.cjs
const {request} = require('../clock/node_modules/playwright');
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
const os = require('node:os');
const net = require('node:net');
const {spawn} = require('node:child_process');
const {createHash} = require('node:crypto');

(async () => {
  assert.ok(process.env.FLARE_OLD_BINARY && process.env.FLARE_BINARY);
  const root = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-release-upgrade-'));
  const data = path.join(root, 'data'), backup = path.join(root, 'backup');
  await fs.mkdir(data);
  const socket = net.createServer();
  await new Promise(resolve => socket.listen(0, '127.0.0.1', resolve));
  const port = socket.address().port;
  await new Promise(resolve => socket.close(resolve));
  const base = `http://127.0.0.1:${port}`;
  await fs.writeFile(path.join(data, 'config.yml'), `AppVersion: 0.6.5
Title: Upgrade fixture
Locale: en
Theme: blackboard
ShowTitle: true
ShowApps: true
ShowBookmarks: true
ShowSearchComponent: true
ShowWeather: false
Location: 北京市
ExtensionFixture: keep-me
Search:
  Mode: select
  DefaultTarget: local
  EnableKeywords: true
  Engines:
    - ID: example
      Name: Example
      Keyword: g
      URLTemplate: https://example.com/search?q={query}
      Enabled: true
      ExtensionFixture: keep-engine
`);
  await fs.writeFile(path.join(data, 'apps.yml'), 'links:\n- name: Application\n  link: http://nas:8080\n  private: true\n');
  await fs.writeFile(path.join(data, 'bookmarks.yml'), 'categories:\n- id: stable\n  title: Stable category\nlinks:\n- name: Needle bookmark\n  link: https://example.com/path?q=1#fragment\n  category: stable\n  desc: Preserve description\n');
  const client = await request.newContext({baseURL: base});
  let child;
  const evidence = {scope: 'Disposable synthetic legacy data, actual published 0.6.5 host binary; offline weather. Full data backup is restored for rollback.', phases: []};
  async function stop() {
    if (!child || child.exitCode !== null) return;
    const process = child;
    await new Promise((resolve, reject) => {
      const timer = setTimeout(() => { process.kill('SIGKILL'); reject(Error('shutdown timed out')); }, 10000);
      process.once('exit', () => { clearTimeout(timer); resolve(); });
      process.kill('SIGTERM');
    });
    child = null;
  }
  async function start(binary) {
    child = spawn(path.resolve(binary), [], {cwd: data, env: {...process.env,
      FLARE_PORT: String(port), FLARE_USER: 'release-fixture', FLARE_PASS: 'temporary-release-fixture',
      FLARE_VISIBILITY: 'PRIVATE', FLARE_DISABLE_LOGIN: 'false', FLARE_OFFLINE: 'true', FLARE_BASELINE: '1'}});
    let log = '';
    child.stdout.on('data', b => log += b); child.stderr.on('data', b => log += b);
    for (let i = 0; i < 100; i++) {
      if (child.exitCode !== null) throw Error(log);
      try { if ((await client.get('/ping', {timeout: 1000})).ok()) return; } catch {}
      await new Promise(resolve => setTimeout(resolve, 100));
    }
    throw Error('startup timeout: ' + log);
  }
  async function hashes(directory) {
    const result = {};
    for (const entry of await fs.readdir(directory, {withFileTypes: true})) {
      const file = path.join(directory, entry.name);
      if (entry.isDirectory()) {
        for (const [name, hash] of Object.entries(await hashes(file))) result[entry.name + '/' + name] = hash;
      } else result[entry.name] = createHash('sha256').update(await fs.readFile(file)).digest('hex');
    }
    return result;
  }
  async function authentication(phase, login = false) {
    const anonymous = await request.newContext({baseURL: base});
    try {
      assert.equal((await anonymous.get('/', {maxRedirects: 0})).status(), 302);
      assert.equal((await anonymous.post('/login', {form: {username: 'release-fixture', password: 'wrong'}, maxRedirects: 0})).status(), 400);
    } finally { await anonymous.dispose(); }
    if (login) assert.equal((await client.post('/login', {form: {username: 'release-fixture', password: 'temporary-release-fixture'}, maxRedirects: 0})).status(), 302);
    for (const route of ['/', '/applications', '/bookmarks', '/editor', '/settings/search', '/settings/weather']) {
      assert.equal((await client.get(route, {maxRedirects: 0})).status(), 200, route);
    }
    evidence.phases.push({phase, private_auth: true, invalid_login_rejected: true, session_retained: !login, data_sha256: await hashes(data)});
  }
  const document = async () => (await client.get('/editor/document')).json();
  try {
    await start(process.env.FLARE_OLD_BINARY);
    await authentication('published-0.6.5', true);
    const originalDoc = await document();
    await stop();
    await fs.cp(data, backup, {recursive: true});
    const originalHashes = await hashes(backup);
    await start(process.env.FLARE_BINARY);
    await authentication('upgrade-before-write');
    assert.deepEqual(await hashes(data), originalHashes, 'read-only upgrade must preserve all data bytes');
    assert.deepEqual(await document(), originalDoc);
    const html = await (await client.get('/settings/search')).text();
    const hidden = name => { const match = html.match(new RegExp(`name="${name}" value="([^"]*)"`)); assert.ok(match, name); return match[1]; };
    const form = {'csrf-token': hidden('csrf-token'), 'config-revision': hidden('config-revision'),
      action: 'save', 'search-mode': 'local-first', 'default-target': 'local', 'show-search-component': '1',
      'auto-focus-setting': '1', 'search-auto-focus': '1', 'engine-count': '1', 'engine-0-id': 'example',
      'engine-0-name': 'Example', 'engine-0-url': 'https://example.com/search?q={query}', 'engine-0-enabled': '1'};
    const saved = await client.post('/settings/search', {form, headers: {Origin: base}});
    assert.equal(saved.status(), 200);
    assert.match(await saved.text(), /Search settings saved/);
    const yaml = await fs.readFile(path.join(data, 'config.yml'), 'utf8');
    assert.match(yaml, /AppVersion: ["']?0\.6\.6/);
    assert.match(yaml, /Mode: local-first/);
    assert.match(yaml, /ExtensionFixture: keep-me/);
    assert.match(yaml, /ExtensionFixture: keep-engine/);
    const firstBackup = path.join(data, '.flare-search-backups', 'first-upgrade-to-0.6.6.bak');
    assert.deepEqual(await fs.readFile(firstBackup), await fs.readFile(path.join(backup, 'config.yml')));
    const stale = await document(), changed = structuredClone(stale);
    changed.apps[0].Name = 'Application saved after upgrade';
    assert.equal((await client.post('/editor/document', {headers: {'X-Flare-Editor': '1'}, data: changed})).status(), 200);
    stale.apps[0].Name = 'Stale overwrite';
    assert.equal((await client.post('/editor/document', {headers: {'X-Flare-Editor': '1'}, data: stale})).status(), 409);
    const committed = await document();
    assert.equal(committed.apps[0].Private, true);
    assert.deepEqual(committed.bookmarks, originalDoc.bookmarks);
    assert.deepEqual(committed.categories, originalDoc.categories);
    const mutatedHashes = await hashes(data);
    await stop(); await start(process.env.FLARE_BINARY);
    await authentication('restart-after-search-and-editor-save');
    assert.deepEqual(await document(), committed);
    assert.deepEqual(await hashes(data), mutatedHashes);
    await stop(); await fs.rm(data, {recursive: true}); await fs.cp(backup, data, {recursive: true});
    assert.deepEqual(await hashes(data), originalHashes);
    await start(process.env.FLARE_OLD_BINARY);
    await authentication('rollback-with-restored-complete-backup');
    assert.deepEqual(await document(), originalDoc);
    assert.deepEqual(await hashes(data), originalHashes);
    evidence.ok = true;
    evidence.checks = ['read-only byte preservation', 'search migration backup', 'unknown YAML field preservation',
      'editor mutation', 'stale editor conflict rejected', 'restart persistence', 'complete backup rollback', 'session continuity'];
    if (process.env.FLARE_RELEASE_EVIDENCE) await fs.writeFile(process.env.FLARE_RELEASE_EVIDENCE, JSON.stringify(evidence, null, 2) + '\n');
    console.log('PASS: published 0.6.5 upgrade, search/editor mutations, restart, restored-backup rollback and authentication');
  } finally { await stop(); await client.dispose(); await fs.rm(root, {recursive: true, force: true}); }
})().catch(error => { console.error(error); process.exitCode = 1; });
