// FLARE_BINARY=/path/to/binary node tests/editor/list.browser.cjs
// The list of the editor, as documented in docs/editor.md: an entry is one
// line in a window with room for it and several lines on a phone; the tabs
// narrow the list to the applications or the bookmarks and the address
// names the open tab; and the tabs and the tools of the list stay at the
// top while the list moves under them.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

const LONG = 'A name that is far too long to stand in its column without being cut short at the end';

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-editor-list-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'), 'Locale: en\nTitle: Site\nTheme: blackboard\nShowWeather: false\n');
  const apps = [`- name: ${LONG}\n  link: https://long.example.test/a/path/that/goes/on/and/on/for/a/good/while/longer/than/its/column\n`];
  for (let i = 1; i < 12; i++) apps.push(`- name: App ${i}\n  link: https://app${i}.example.test/\n`);
  await fs.writeFile(path.join(dir, 'apps.yml'), 'links:\n' + apps.join(''));
  const marks = [];
  for (let i = 0; i < 30; i++) marks.push(`- name: Mark ${i}\n  link: https://mark${i}.example.test/\n  category: ${i % 2 ? 'two' : 'one'}\n`);
  await fs.writeFile(path.join(dir, 'bookmarks.yml'), 'categories:\n- id: one\n  title: First\n- id: two\n  title: Second\nlinks:\n' + marks.join(''));
  const child = spawn(process.env.FLARE_BINARY, [], {cwd: dir, env: {...process.env,
    FLARE_PORT: String(port), FLARE_DISABLE_LOGIN: 'true', FLARE_OFFLINE: 'true', FLARE_EDITOR: 'true', FLARE_BASELINE: '1'}});
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

// What the list shows: its rows, the tabs, the count and the filter.
const shown = () => {
  const rows = [...document.querySelectorAll('#entry-list .entry')];
  const middle = element => {const box = element.getBoundingClientRect(); return box.top + box.height / 2;};
  const first = rows[0];
  return {
    rows: rows.length,
    names: rows.slice(0, 3).map(row => row.querySelector('.entry-name').textContent),
    tabs: [...document.querySelectorAll('#kind-tabs button')].map(tab => [tab.dataset.kind, tab.textContent.trim().replace(/\s+/g, ' '), tab.getAttribute('aria-pressed')]),
    count: document.querySelector('#result-count').textContent,
    filter: [document.querySelector('#location-filter').value, document.querySelector('#location-filter').disabled],
    hash: window.location.hash,
    row: first && {
      height: Math.round(first.getBoundingClientRect().height),
      // One line: the name, the address, the place and the actions have their middles at one height.
      line: (middles => Math.max(...middles) - Math.min(...middles) < 4)([first.querySelector('.entry-name'), first.querySelector('.entry-address'), first.querySelector('.entry-location'), first.querySelector('.entry-actions')].map(middle)),
      targets: [...first.querySelectorAll('button')].map(button => button.offsetHeight),
      cut: (name => name.scrollWidth > name.clientWidth)(first.querySelector('.entry-name')),
      title: first.querySelector('.entry-name').title,
      addressTitle: first.querySelector('.entry-address').title,
    },
    controls: getComputedStyle(document.querySelector('.list-controls')).position,
    overflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
  };
};

(async () => {
  assert.ok(process.env.FLARE_BINARY, 'FLARE_BINARY is required');
  const server = await start();
  const browser = await chromium.launch({executablePath: process.env.CHROMIUM_EXECUTABLE || undefined});
  try {
    const errors = [];
    const open = async (options, address = '/settings/content') => {
      const page = await (await browser.newContext(options)).newPage();
      page.on('console', message => {if (message.type() === 'error') errors.push(message.text());});
      page.on('pageerror', error => errors.push(String(error)));
      await page.goto(server.base + address);
      await page.waitForFunction(() => window.FlareEditorUI?.store && document.querySelector('#entry-list .entry'));
      // Rows are laid out when they come into view, a frame after they are added.
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      return page;
    };

    // A wide window: every entry on one line of at most 46 pixels, with the
    // name and the address cut where they are too long and whole in a title.
    const wide = await open({viewport: {width: 1280, height: 800}});
    let state = await wide.evaluate(shown);
    assert.equal(state.rows, 42);
    assert.deepEqual(state.tabs, [['all', 'All 42', 'true'], ['apps', 'Apps 12', 'false'], ['bookmarks', 'Bookmarks 30', 'false']]);
    assert.deepEqual([state.count, state.filter, state.hash], ['42 entries', ['all', false], '']);
    assert.ok(state.row.height <= 46, `a row is ${state.row.height} pixels high`);
    assert.deepEqual([state.row.line, state.row.cut, state.row.title, state.overflow], [true, true, LONG, false]);
    assert.match(state.row.addressTitle, /^https:\/\/long\.example\.test\//);
    // More than twice as many rows as before fit a window of this height.
    const fitting = await wide.evaluate(() => {
      const list = document.querySelector('#entry-list').getBoundingClientRect(), bar = document.querySelector('.savebar').getBoundingClientRect();
      const row = document.querySelector('#entry-list .entry').getBoundingClientRect().height;
      return Math.floor((bar.top - Math.max(list.top, 0)) / row);
    });
    assert.ok(fitting >= 8, `${fitting} rows fit below the tools`);

    // The tabs narrow the list, name themselves in the address, and the
    // number each shows stays that of all its entries.
    await wide.locator('#kind-tabs [data-kind="apps"]').click();
    state = await wide.evaluate(shown);
    assert.deepEqual([state.rows, state.hash, state.count, state.filter], [12, '#apps', 'Found 12 of 42 entries', ['all', true]]);
    assert.deepEqual(state.tabs.map(tab => tab[2]), ['false', 'true', 'false']);
    assert.equal(state.names[0], LONG);
    await wide.locator('#entry-search').fill('app 1');
    await wide.waitForFunction(() => document.querySelectorAll('#entry-list .entry').length === 3);
    state = await wide.evaluate(shown);
    assert.deepEqual([state.count, state.tabs.map(tab => tab[1])], ['Found 3 of 42 entries', ['All 42', 'Apps 12', 'Bookmarks 30']]);
    await wide.locator('#kind-tabs [data-kind="bookmarks"]').click();
    state = await wide.evaluate(shown);
    assert.deepEqual([state.rows, state.hash, state.filter], [0, '#bookmarks', ['all', false]]);
    // Nothing matches: the way out clears the search and the tab alike.
    await wide.locator('#entry-list .empty button').click();
    state = await wide.evaluate(shown);
    assert.deepEqual([state.rows, state.hash, await wide.locator('#entry-search').inputValue()], [42, '', '']);

    // The places are those of bookmarks. With the bookmarks open, choosing
    // the applications as the place opens all entries again.
    await wide.locator('#kind-tabs [data-kind="bookmarks"]').click();
    await wide.locator('#location-filter').selectOption('category:two');
    state = await wide.evaluate(shown);
    assert.deepEqual([state.rows, state.hash], [15, '#bookmarks']);
    await wide.locator('#location-filter').selectOption('apps');
    state = await wide.evaluate(shown);
    assert.deepEqual([state.rows, state.hash, state.tabs.map(tab => tab[2])], [12, '', ['true', 'false', 'false']]);

    // The address opens a tab, and the tools stay at the top while the list
    // moves under them.
    const marks = await open({viewport: {width: 1280, height: 800}}, '/settings/content#bookmarks');
    state = await marks.evaluate(shown);
    assert.deepEqual([state.rows, state.tabs.map(tab => tab[2]), state.controls], [30, ['false', 'false', 'true'], 'sticky']);
    await marks.evaluate(() => window.scrollTo(0, document.documentElement.scrollHeight));
    await marks.waitForFunction(() => window.scrollY > 200);
    assert.equal(await marks.evaluate(() => Math.round(document.querySelector('.list-controls').getBoundingClientRect().top)), 0);
    assert.equal(await marks.locator('#entry-search').isVisible(), true);
    // An address that names no tab opens all entries.
    const other = await open({viewport: {width: 1280, height: 800}}, '/settings/content#nothing');
    assert.deepEqual((await other.evaluate(shown)).tabs.map(tab => tab[2]), ['true', 'false', 'false']);
    // The address is followed while the page is open, as by the way back of
    // the browser: a tab it names is opened, and anything else is all entries.
    for (const [hash, rows, pressed] of [['#apps', 12, ['false', 'true', 'false']], ['#nothing', 42, ['true', 'false', 'false']], ['#bookmarks', 30, ['false', 'false', 'true']], ['', 42, ['true', 'false', 'false']]]) {
      await other.evaluate(next => {window.location.hash = next;}, hash);
      await other.waitForFunction(count => document.querySelectorAll('#entry-list .entry').length === count, rows);
      assert.deepEqual((await other.evaluate(shown)).tabs.map(tab => tab[2]), pressed, hash);
    }

    // A phone keeps the entry of several lines and targets of 44 pixels,
    // and the tools move with the page.
    const phone = await open({viewport: {width: 360, height: 700}, isMobile: true, hasTouch: true});
    state = await phone.evaluate(shown);
    assert.deepEqual([state.rows, state.row.line, state.controls, state.overflow], [42, false, 'static', false]);
    assert.ok(state.row.height >= 90, `a row is ${state.row.height} pixels high`);
    assert.ok(state.row.targets.every(height => height >= 44), String(state.row.targets));
    assert.ok((await phone.locator('#kind-tabs button').evaluateAll(tabs => tabs.map(tab => tab.offsetHeight))).every(height => height >= 44));
    await phone.locator('#kind-tabs [data-kind="apps"]').tap();
    assert.equal((await phone.evaluate(shown)).rows, 12);

    // A window with room and a finger: one line, with targets of 44 pixels.
    const touch = await open({viewport: {width: 1024, height: 768}, hasTouch: true, isMobile: true});
    state = await touch.evaluate(shown);
    assert.equal(state.row.line, true);
    assert.ok(state.row.targets.every(height => height >= 44), String(state.row.targets));

    assert.deepEqual(errors, []);
    console.log('PASS: an entry of the editor is one line in a wide window, the tabs narrow the list and name themselves in the address, and the tools stay at the top');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
