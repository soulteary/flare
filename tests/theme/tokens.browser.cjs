// FLARE_BINARY=/path/to/binary node tests/theme/tokens.browser.cjs
// The style tokens documented in docs/theme-tokens.md: each one changes the
// parts of the page it is documented for, and nothing else.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const net = require('node:net');
const os = require('node:os');
const path = require('node:path');
const {spawn} = require('node:child_process');

// The blackboard theme, as browsers report its colors.
const BACKGROUND = 'rgb(26, 26, 26)';
const PRIMARY = 'rgb(255, 253, 234)';
const ACCENT = 'rgb(92, 92, 92)';
const TEST = 'rgb(1, 2, 3)';

// What the page looks like with no token changed.
const DEFAULTS = {
  pageBackground: BACKGROUND,
  text: PRIMARY,
  bookmarkText: PRIMARY,
  searchText: PRIMARY,
  icon: PRIMARY,
  description: ACCENT,
  groupTitle: ACCENT,
  searchUnderline: ACCENT,
  tabBorder: ACCENT,
  focusOutline: ACCENT,
  currentTabBackground: ACCENT,
  toolbarButton: ACCENT,
  currentTabText: BACKGROUND,
  toolbarIcon: BACKGROUND,
  hoverBackground: 'rgba(0, 0, 0, 0.2)',
  hoverRadius: '4px',
  tabRadius: '6px',
  font: 'Roboto, sans-serif',
  titleSize: '14px',
  descriptionSize: '11.2px',
  bookmarkSize: '14px',
  groupTitleSize: '16px',
  searchSize: '20px',
  applicationHeight: '40px',
};

// Token, value to set, and the parts that must take it. A part that follows a
// derived token also follows the token it derives from.
const TOKENS = [
  ['--flare-background', TEST, {pageBackground: TEST, currentTabText: TEST, toolbarIcon: TEST}],
  ['--flare-primary', TEST, {text: TEST, bookmarkText: TEST, searchText: TEST, icon: TEST}],
  ['--flare-accent', TEST, {description: TEST, groupTitle: TEST, searchUnderline: TEST, tabBorder: TEST,
    focusOutline: TEST, currentTabBackground: TEST, toolbarButton: TEST}],
  ['--flare-icon', TEST, {icon: TEST}],
  ['--flare-muted', TEST, {description: TEST, groupTitle: TEST}],
  ['--flare-border', TEST, {searchUnderline: TEST, tabBorder: TEST}],
  ['--flare-focus', TEST, {focusOutline: TEST}],
  ['--flare-inverse', TEST, {currentTabText: TEST, toolbarIcon: TEST}],
  ['--flare-hover', TEST, {hoverBackground: TEST}],
  ['--flare-radius', '10px', {hoverRadius: '10px', tabRadius: '15px'}],
  ['--flare-font-family', 'monospace', {font: 'monospace'}],
  // Text sizes, and the height that would otherwise cut the text of an
  // application off, are multiplied by the scale.
  ['--flare-font-scale', '1.5', {titleSize: '21px', descriptionSize: '16.8px', bookmarkSize: '21px',
    groupTitleSize: '24px', searchSize: '30px', applicationHeight: '60px'}],
  // The names used before the tokens keep working, for footers that set them.
  ['--color-background', TEST, {pageBackground: TEST, currentTabText: TEST, toolbarIcon: TEST}],
  ['--color-primary', TEST, {text: TEST, bookmarkText: TEST, searchText: TEST, icon: TEST}],
  ['--color-accent', TEST, {description: TEST, groupTitle: TEST, searchUnderline: TEST, tabBorder: TEST,
    focusOutline: TEST, currentTabBackground: TEST, toolbarButton: TEST}],
];

async function start() {
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'flare-tokens-'));
  const listener = net.createServer();
  await new Promise(resolve => listener.listen(0, '127.0.0.1', resolve));
  const port = listener.address().port;
  await new Promise(resolve => listener.close(resolve));
  await fs.writeFile(path.join(dir, 'config.yml'),
    'Locale: en\nTheme: blackboard\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\n' +
    'ShowWeather: false\nHomepage:\n  CategoryNavigation: tabs\n');
  await fs.writeFile(path.join(dir, 'apps.yml'),
    'links:\n- name: Application\n  link: https://example.com/a\n  icon: circle\n  desc: A description\n');
  await fs.writeFile(path.join(dir, 'bookmarks.yml'),
    'categories:\n- id: a\n  title: First\n- id: b\n  title: Second\n' +
    'links:\n- name: One\n  link: https://example.com/b\n  icon: home\n  category: a\n' +
    '- name: Two\n  link: https://example.com/c\n  category: b\n');
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
  return {base, async stop() {child.kill(); await fs.rm(dir, {recursive: true, force: true});}};
}

// Computed styles of one element for each documented part of the page.
const measure = page => page.evaluate(() => {
  const style = (selector, property) => {
    const element = document.querySelector(selector);
    if (!element) throw Error('missing element: ' + selector);
    return getComputedStyle(element)[property];
  };
  return {
    pageBackground: style('body', 'backgroundColor'),
    text: style('.app-title', 'color'),
    bookmarkText: style('a.bookmark span', 'color'),
    searchText: style('#search', 'color'),
    icon: style('.app-icon svg', 'fill'),
    description: style('.app-desc', 'color'),
    groupTitle: style('.bookmark-group-title', 'color'),
    searchUnderline: style('#search', 'borderBottomColor'),
    tabBorder: style('.bookmark-category-nav a:not([aria-current])', 'borderTopColor'),
    focusOutline: getComputedStyle(document.activeElement).outlineColor,
    currentTabBackground: style('.bookmark-category-nav a[aria-current="page"]', 'backgroundColor'),
    toolbarButton: style('.toolbar-btn-bg', 'backgroundColor'),
    currentTabText: style('.bookmark-category-nav a[aria-current="page"]', 'color'),
    toolbarIcon: style('#btn-open-settings svg path', 'fill'),
    hoverBackground: style('.app-item', 'backgroundColor'),
    hoverRadius: style('.app-item', 'borderTopLeftRadius'),
    tabRadius: style('.bookmark-category-nav a', 'borderTopLeftRadius'),
    font: style('body', 'fontFamily'),
    titleSize: style('.app-title', 'fontSize'),
    descriptionSize: style('.app-desc', 'fontSize'),
    bookmarkSize: style('a.bookmark', 'fontSize'),
    groupTitleSize: style('.bookmark-group-title', 'fontSize'),
    searchSize: style('#search', 'fontSize'),
    applicationHeight: style('.app-item', 'height'),
  };
});

(async () => {
  const browser = await chromium.launch({headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined, args: ['--no-sandbox']});
  const server = await start();
  try {
    const page = await browser.newPage({viewport: {width: 1280, height: 900}});
    const open = async () => {
      await page.goto(server.base, {waitUntil: 'load'});
      // Measure final values, and keep the pointer and the focus where the
      // hover and focus styles apply.
      await page.addStyleTag({content: '* { transition: none !important; }'});
      await page.locator('.app-item').hover();
      // Reach a category link with the keyboard, so that its focus outline shows.
      await page.locator('#search').focus();
      for (let i = 0; i < 30; i++) {
        await page.keyboard.press('Tab');
        if (await page.evaluate(() => document.activeElement.matches('.bookmark-category-nav a'))) return;
      }
      assert.fail('the keyboard did not reach the category navigation');
    };

    await open();
    assert.deepEqual(await measure(page), DEFAULTS);

    for (const [token, value, expected] of TOKENS) {
      await open();
      await page.evaluate(([name, setting]) => document.body.style.setProperty(name, setting), [token, value]);
      assert.deepEqual(await measure(page), {...DEFAULTS, ...expected}, token);
    }
    console.log(`PASS: ${TOKENS.length - 3} style tokens and the 3 earlier names each change their documented parts only`);
  } finally {
    await server.stop();
    await browser.close();
  }
})().catch(error => {console.error(error); process.exitCode = 1;});
