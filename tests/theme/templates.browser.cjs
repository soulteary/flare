// FLARE_BINARY=/path/to/binary node tests/theme/templates.browser.cjs
// Templates of themes, as documented in docs/theme-templates.md, with the
// example theme of docs/theme-template-guide.md, whose files are read from
// that page: the site allows templates, the templates of the selected theme
// are confirmed, and the navigation pages and their entries are rendered
// with them. A template runs no script and loads nothing from elsewhere, the
// scripts of the application keep working inside a page of the theme, a
// theme has the templates of the theme it is based on, an archive brings
// templates, and an import asks for the confirmation again.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const zlib = require('node:zlib');
const {spawn} = require('node:child_process');

const NIGHT = 'contract: 1\nextends: harbor\nname: Night\ntokens:\n  accent: "#00ff88"\n';
const NIGHT_MARK = '<li class="night-mark"><a href="{{.Href}}"><span>{{.Name}}</span></a></li>\n';
// What a template cannot hold: each is not accepted, and none reaches a page.
const REFUSED = {
  'applications.html': ['<div><script>window.themeScriptRan = true</script>{{template "flare/subpage-header"}}{{template "flare/apps"}}{{template "flare/toolbar"}}{{template "flare/footer"}}</div>', /element a template cannot hold\. script/],
  'bookmarks.html': ['<div onclick="window.themeScriptRan = true">{{template "flare/subpage-header"}}{{template "flare/bookmarks"}}{{template "flare/toolbar"}}{{template "flare/footer"}}</div>', /attribute, or a value of one, that a template cannot hold\. onclick/],
};

// example reads the files of the example theme from the guide: each is a
// block of code under a line that names the file in the themes directory.
async function example() {
  const guide = await fs.readFile(path.join(__dirname, '..', '..', 'docs', 'theme-template-guide.md'), 'utf8');
  const files = {};
  for (const [, name, content] of guide.matchAll(/\*\*`themes\/harbor\/([a-z./]+)`\*\*\n\n```[a-z]+\n(.*?)\n```\n/gs)) files[name] = content + '\n';
  assert.deepEqual(Object.keys(files).sort(), ['templates/app.html', 'templates/bookmark.html', 'templates/group.html', 'templates/home.html', 'theme.css', 'theme.yml']);
  return files;
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

async function start(files) {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-theme-templates-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'),
    'Locale: en\nTitle: Mine & <yours>\nTheme: blackboard\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowDateTime: true\nShowTitle: true\nShowWeather: false\nHomepage:\n  CategoryNavigation: collapse\n');
  await fs.writeFile(path.join(dir, 'apps.yml'),
    'links:\n- name: Alpha & <one>\n  link: https://alpha.example.test/a\n  icon: circle\n  desc: A description\n- name: beta\n  link: http://beta.example.test:8080/b\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'),
    'categories:\n- id: one\n  title: First\n- id: two\n  title: Second\nlinks:\n- name: Gamma\n  link: https://gamma.example.test/\n  icon: home\n  category: one\n- name: Delta\n  link: https://delta.example.test/\n  category: two\n');
  const themes = path.join(dir, 'themes');
  for (const [name, content] of Object.entries(files)) {
    await fs.mkdir(path.dirname(path.join(themes, 'harbor', name)), {recursive: true});
    await fs.writeFile(path.join(themes, 'harbor', name), content);
  }
  for (const [name, [content]] of Object.entries(REFUSED)) await fs.writeFile(path.join(themes, 'harbor', 'templates', name), content);
  await fs.mkdir(path.join(themes, 'night', 'templates'), {recursive: true});
  await fs.writeFile(path.join(themes, 'night', 'theme.yml'), NIGHT);
  await fs.writeFile(path.join(themes, 'night', 'templates', 'bookmark.html'), NIGHT_MARK);
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

// What a navigation page is made of: whether it is the page of the theme
// or of the application, and what its entries are written with.
const shape = () => {
  const text = selector => Array.from(document.querySelectorAll(selector), node => node.textContent.replace(/\s+/g, ' ').trim());
  const root = document.querySelector('.harbor');
  return {
    themed: Boolean(root),
    hero: Boolean(document.querySelector('#hero-container')),
    page: root ? root.dataset.page : '',
    column: root ? root.classList.contains('harbor-with-column') : false,
    site: text('.harbor-site'),
    tiles: text('.harbor-tile strong'),
    notes: text('.harbor-tile small'),
    letters: text('.harbor-letter'),
    marks: text('.harbor-mark span'),
    nightMarks: text('.night-mark span'),
    groups: text('details.harbor-group > summary'),
    builtinApps: document.querySelectorAll('.app-item').length,
    builtinMarks: document.querySelectorAll('a.bookmark').length,
    scripted: 'themeScriptRan' in window,
  };
};

(async () => {
  const files = await example();
  const browser = await chromium.launch({headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox']});
  const server = await start(files);
  try {
    const page = await browser.newPage({viewport: {width: 1280, height: 900}});
    page.setDefaultTimeout(10000);
    const elsewhere = [], errors = [];
    page.on('request', request => {
      if (!request.url().startsWith(server.base + '/') && !request.url().startsWith('data:')) elsewhere.push(request.url());
    });
    page.on('pageerror', error => errors.push(String(error)));
    page.on('console', message => {if (message.type() === 'error') errors.push(message.text());});
    const open = async address => {
      await page.goto(server.base + address, {waitUntil: 'load'});
      return page.evaluate(shape);
    };
    const settings = () => page.goto(server.base + '/settings/theme', {waitUntil: 'load'});
    // Submits a form of the theme page and waits for the page it answers with.
    const submit = async (form, status = 200) => {
      const [response] = await Promise.all([
        page.waitForResponse(candidate => candidate.request().method() === 'POST'),
        page.locator(`${form} input.btn-submit`).click(),
      ]);
      assert.equal(response.status(), status, form);
      await page.waitForLoadState('load');
    };
    const select = async name => {
      await settings();
      const [response] = await Promise.all([
        page.waitForResponse(candidate => candidate.request().method() === 'POST'),
        page.locator(`.theme-file input[value="${name}"]`).click(),
      ]);
      assert.equal(response.status(), 200, name);
      await page.waitForLoadState('load');
    };
    const card = name => page.locator(`.theme-file:has(input[value="${name}"]) .theme-templates-note`).textContent();
    const lines = () => page.locator('.theme-file-lines li').evaluateAll(items => items.map(line => line.textContent.replace(/\s+/g, ' ').trim()));
    const builtin = found => {
      assert.equal(found.themed, false);
      assert.equal(found.hero, true);
      assert.equal(found.builtinApps, 2);
      assert.equal(found.builtinMarks, 2);
      assert.deepEqual([found.tiles, found.marks, found.nightMarks], [[], [], []]);
    };

    // Nothing is allowed. The card of the theme says which templates it
    // brings, the two that are not accepted are listed with their reasons,
    // and the pages are those of the application, also for a preview.
    await settings();
    assert.match(await card('harbor'), /^Brings templates: home\.html, app\.html, bookmark\.html, group\.html\. Templates of themes are turned off/);
    assert.match(await card('night'), /^Brings templates: bookmark\.html, harbor\/home\.html, harbor\/app\.html, harbor\/group\.html\./);
    const listed = await lines();
    for (const [name, [, reason]] of Object.entries(REFUSED)) {
      const line = listed.find(text => text.startsWith(`harbor/templates/${name} `));
      assert.ok(line, name);
      assert.match(line, reason);
    }
    assert.equal(await page.locator('.theme-templates-form input[type="checkbox"]').isChecked(), false);
    assert.equal(await page.locator('.theme-templates-confirm-form').count(), 0);
    builtin(await open('/'));
    builtin(await open('/?theme-preview=harbor'));

    // The site allows templates. The preview of the theme shows them before
    // the theme is selected or anything is confirmed.
    await settings();
    await page.locator('.theme-templates-form input[type="checkbox"]').check();
    await submit('.theme-templates-form');
    assert.equal(await page.locator('.theme-templates-form input[type="checkbox"]').isChecked(), true);
    builtin(await open('/'));
    const previewed = await open('/?theme-preview=harbor');
    assert.equal(previewed.themed, true);
    assert.equal(previewed.hero, false);

    // The theme is selected. Until its templates are confirmed the pages
    // are the application's, and the form lists what is to be confirmed.
    await select('harbor');
    builtin(await open('/'));
    await settings();
    assert.equal(await page.locator('.theme-templates-confirm-form input[name="theme-templates-files"]').inputValue(), 'home.html,app.html,bookmark.html,group.html');
    assert.match(await page.locator('.theme-templates-files').textContent(), /home\.html, app\.html, bookmark\.html, group\.html/);
    // The administrator looks at the page with the templates first.
    await Promise.all([page.waitForNavigation({waitUntil: 'load'}), page.locator('.theme-templates-preview-form input.btn-submit').click()]);
    assert.equal(new URL(page.url()).search, '?theme-preview=harbor');
    assert.equal((await page.evaluate(shape)).themed, true);
    await settings();
    await submit('.theme-templates-confirm-form');
    assert.equal(await page.locator('.theme-templates-in-use').count(), 1);

    // The home page is the page of the theme: its parts stand where the
    // template places them, and its entries are written by the item
    // templates with what each entry has.
    const home = await open('/');
    assert.deepEqual({themed: home.themed, hero: home.hero, page: home.page, column: home.column}, {themed: true, hero: false, page: 'home', column: true});
    assert.deepEqual(home.site, ['Mine & <yours>']);
    assert.deepEqual(home.tiles, ['Alpha & <one>', 'beta']);
    assert.deepEqual(home.notes, ['A description', 'beta.example.test']);
    assert.deepEqual(home.letters, ['b']);
    assert.deepEqual(home.marks, ['Gamma', 'Delta']);
    assert.deepEqual(home.groups, ['First', 'Second']);
    assert.deepEqual([home.builtinApps, home.builtinMarks], [0, 0]);
    const placed = await page.evaluate(() => ({
      search: Boolean(document.querySelector('.harbor-head #search')),
      clock: Boolean(document.querySelector('.harbor-column #clock')),
      greeting: Boolean(document.querySelector('.harbor-column #plugin-greetings')),
      apps: Boolean(document.querySelector('.harbor-lists #container-apps .harbor-tile')),
      bookmarks: Boolean(document.querySelector('.harbor-lists #container-bookmarks details.harbor-group li.harbor-mark')),
      toolbar: Boolean(document.querySelector('.harbor > .toolbar-container')),
      inside: Boolean(document.querySelector('#page-home .container > .harbor')),
      links: Array.from(document.querySelectorAll('.harbor-tile a, .harbor-mark a'), link => [link.getAttribute('href'), link.rel]),
      icons: document.querySelectorAll('.harbor-icon svg, .harbor-icon img').length,
    }));
    assert.deepEqual({...placed, links: undefined}, {search: true, clock: true, greeting: true, apps: true, bookmarks: true, toolbar: true, inside: true, links: undefined, icons: 1});
    assert.deepEqual(placed.links.map(([href]) => href), ['https://alpha.example.test/a', 'http://beta.example.test:8080/b', 'https://gamma.example.test/', 'https://delta.example.test/']);
    for (const [, rel] of placed.links) assert.match(rel, /noopener/);
    // The stylesheet of the theme lays out what the templates write, with
    // the tokens of the theme.
    const laid = await page.evaluate(() => {
      const tile = document.querySelector('.harbor-tile a'), body = document.querySelector('.harbor-body');
      return {radius: getComputedStyle(tile).borderTopLeftRadius, columns: getComputedStyle(body).gridTemplateColumns.split(' ').length,
        accent: getComputedStyle(document.querySelector('.harbor-site')).color, icon: document.querySelector('.harbor-icon svg, .harbor-icon img').getBoundingClientRect().width};
    });
    assert.deepEqual(laid, {radius: '10px', columns: 2, accent: 'rgb(242, 184, 75)', icon: 32});

    // The scripts of the application work inside the page of the theme: the
    // clock runs, and a group is opened and closed by the browser.
    assert.match(await page.locator('#clock-time').textContent(), /\d/);
    const group = page.locator('details.harbor-group').first();
    assert.equal(await group.evaluate(node => node.open), true);
    await group.locator('summary').click();
    assert.equal(await group.evaluate(node => node.open), false);
    assert.equal(await group.locator('li').first().isVisible(), false);
    // A search shows the entries that match, written by the same templates.
    await page.locator('#search').fill('alpha');
    await Promise.all([page.waitForNavigation({waitUntil: 'load'}), page.locator('#search').press('Enter')]);
    const found = await page.evaluate(shape);
    assert.equal(found.themed, true);
    assert.deepEqual(found.tiles, ['Alpha & <one>']);
    assert.deepEqual(found.marks, []);

    // The two other pages take the template of the home page, which leaves
    // the search and the column out there. The templates that are not
    // accepted are not used, and nothing of them runs.
    const applications = await open('/applications');
    assert.deepEqual({themed: applications.themed, page: applications.page, tiles: applications.tiles, marks: applications.marks, scripted: applications.scripted},
      {themed: true, page: 'applications', tiles: ['Alpha & <one>', 'beta'], marks: [], scripted: false});
    assert.equal(await page.locator('.harbor-column, .harbor-head').count(), 0);
    assert.equal(await page.locator('.harbor > .sub-page-header').count(), 1);
    const bookmarks = await open('/bookmarks');
    assert.deepEqual({themed: bookmarks.themed, page: bookmarks.page, tiles: bookmarks.tiles, marks: bookmarks.marks, scripted: bookmarks.scripted},
      {themed: true, page: 'bookmarks', tiles: [], marks: ['Gamma', 'Delta'], scripted: false});
    await page.locator('.harbor').click({position: {x: 5, y: 5}});
    assert.equal(await page.evaluate(() => 'themeScriptRan' in window), false);

    // The option of the theme reaches the template, and the page follows it.
    await settings();
    await page.locator('.theme-options-form input[name="option-column"]').uncheck();
    await submit('.theme-options-form');
    const narrow = await open('/');
    assert.deepEqual({themed: narrow.themed, column: narrow.column}, {themed: true, column: false});
    assert.equal(await page.locator('.harbor-column').isVisible(), false);

    // The appearance page says which layout options the theme does not
    // support, and safe mode shows a page without the theme.
    await page.goto(server.base + '/settings/appearance', {waitUntil: 'load'});
    const unsupported = await page.locator('.theme-layout-note').evaluateAll(notes => notes.map(note => {
      let control = note.previousElementSibling;
      while (control && control.tagName !== 'SELECT') control = control.previousElementSibling;
      return [control ? control.name : '', note.textContent.trim()];
    }));
    assert.deepEqual(unsupported.map(([name]) => name), ['mobile-bookmark-layout', 'bookmark-layout', 'desktop-columns']);
    for (const [, text] of unsupported) assert.match(text, /^The selected theme says its templates do not support this option/);
    builtin(await open('/?theme-safe-mode'));

    // An edit of a template is read by the next page, without a restart
    // and without a new confirmation.
    await fs.writeFile(path.join(server.themes, 'harbor', 'templates', 'app.html'), files['templates/app.html'].replace('harbor-tile', 'harbor-tile harbor-edited'));
    await page.waitForFunction(async address => {
      const text = await (await fetch(address, {cache: 'no-store'})).text();
      return text.includes('harbor-edited');
    }, server.base + '/', {timeout: 10000});
    assert.equal((await open('/')).themed, true);
    assert.equal(await page.locator('.harbor-tile.harbor-edited').count(), 2);

    // A theme that is based on the theme has its templates for what it
    // brings none for, and its own confirmation.
    await select('night');
    builtin(await open('/'));
    await settings();
    assert.equal(await page.locator('.theme-templates-confirm-form input[name="theme-templates-files"]').inputValue(), 'bookmark.html,harbor/home.html,harbor/app.html,harbor/group.html');
    await submit('.theme-templates-confirm-form');
    const night = await open('/');
    assert.deepEqual({themed: night.themed, tiles: night.tiles, marks: night.marks, nightMarks: night.nightMarks, groups: night.groups},
      {themed: true, tiles: ['Alpha & <one>', 'beta'], marks: [], nightMarks: ['Gamma', 'Delta'], groups: ['First', 'Second']});

    // The download of the theme holds its templates, those that are not
    // accepted among them. Imported over the theme, it asks for the
    // confirmation of both themes again, and the pages are the
    // application's until then.
    await settings();
    const [file] = await Promise.all([page.waitForEvent('download'), page.locator('.theme-file:has(input[value="harbor"]) .theme-export-link').click()]);
    assert.equal(file.suggestedFilename(), 'harbor.zip');
    const archive = await fs.readFile(await file.path());
    assert.deepEqual(unpack(archive).map(([name]) => name), ['harbor/theme.yml', 'harbor/theme.css', 'harbor/templates/home.html',
      'harbor/templates/applications.html', 'harbor/templates/bookmarks.html', 'harbor/templates/app.html', 'harbor/templates/bookmark.html', 'harbor/templates/group.html']);
    await settings();
    await page.locator('#theme-import-file').setInputFiles({name: 'harbor.zip', mimeType: 'application/zip', buffer: archive});
    await page.locator('.theme-import-form input[name="replace"]').check();
    await submit('.theme-import-form');
    const said = await page.locator('.theme-import-form .theme-notice[role="status"]').allTextContents();
    assert.equal(said.length, 2);
    assert.match(said[0], /^The theme “Harbor” was imported\./);
    assert.match(said[1], /have to be confirmed again.*: Harbor, Night\.$/);
    assert.deepEqual((await fs.readdir(path.join(server.themes, 'harbor', 'templates'))).sort(), ['app.html', 'applications.html', 'bookmark.html', 'bookmarks.html', 'group.html', 'home.html']);
    builtin(await open('/'));
    await settings();
    assert.equal(await page.locator('.theme-templates-in-use').count(), 0);
    await submit('.theme-templates-confirm-form');
    assert.equal((await open('/')).themed, true);

    // The confirmation is withdrawn, and then the setting of the site: the
    // pages are the application's again each time.
    await settings();
    await submit('.theme-templates-confirm-form');
    builtin(await open('/'));
    await settings();
    await submit('.theme-templates-confirm-form');
    assert.equal((await open('/')).themed, true);
    await settings();
    await page.locator('.theme-templates-form input[type="checkbox"]').uncheck();
    await submit('.theme-templates-form');
    builtin(await open('/'));
    const apps = await open('/applications'), marks = await open('/bookmarks');
    assert.deepEqual([apps.themed, apps.builtinApps, marks.themed, marks.builtinMarks], [false, 2, false, 2]);

    assert.deepEqual(elsewhere, []);
    assert.deepEqual(errors, []);
    console.log('PASS: the pages and the entries are rendered with the templates of a confirmed theme, nothing of a template runs, and an import asks for the confirmation again');
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
