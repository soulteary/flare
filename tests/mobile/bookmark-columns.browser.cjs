// Exercise actual group positions using the production bookmark styles.
const {chromium} = require(require.resolve('playwright', {paths: [__dirname + '/../clock']}));
const assert = require('node:assert/strict');
const fs = require('node:fs/promises');
const path = require('node:path');
(async () => {
  const root = path.resolve(__dirname, '../..');
  const css = (await Promise.all(['base', 'home/bookmarks'].map(name => fs.readFile(path.join(root, 'embed/assets/css', name + '.css'), 'utf8')))).join('\n');
  const browser = await chromium.launch({headless: true, executablePath: process.env.CHROMIUM_EXECUTABLE || undefined});
  try {
    const page = await browser.newPage();
    for (const mode of ['', 'mobile-bookmarks-single-column', 'mobile-bookmarks-compact', 'desktop-columns-4', 'desktop-columns-6 mobile-bookmarks-compact']) {
      for (const width of [320, 359, 360, 390, 430, 599, 600, 640, 744, 767, 768, 769, 820, 1024]) {
        await page.setViewportSize({width, height: 1024});
        await page.setContent(`<meta name="viewport" content="width=device-width, initial-scale=1"><style>${css}</style><body class="${mode}"><div id="page-home" class="pageview"><div class="container"><div id="container-bookmarks"><div class="bookmark-groups"><nav class="bookmark-category-nav"><a>All categories</a></nav>${Array.from({length: 8}, (_, i) => `<section class="bookmark-group-container"><h3 class="bookmark-group-title">Group ${i}</h3><ul class="bookmark-list">${Array.from({length: i % 3 + 1}, () => '<li><a class="bookmark">LongBookmarkWithoutSpaces长书签名称</a></li>').join('')}</ul></section>`).join('')}</div></div></div></div></body>`);
        const measure = () => page.locator('.bookmark-groups').evaluate(el => {
          const groups = [...el.querySelectorAll('.bookmark-group-container')];
          const boxes = groups.map(group => group.getBoundingClientRect());
          const nav = el.querySelector('nav').getBoundingClientRect();
          const links = [...el.querySelectorAll('a.bookmark')];
          return {columns: boxes.filter(box => Math.abs(box.top - boxes[0].top) < 1).length, minWidth: Math.min(...boxes.map(box => box.width)), contentWidth: el.clientWidth, overflow: document.documentElement.scrollWidth > innerWidth, navBottom: nav.bottom, firstTop: boxes[0].top, order: groups.map(group => group.querySelector('h3').textContent), newspaperColumns: getComputedStyle(el).columnCount, rowHeights: [...new Set(links.map(link => link.getBoundingClientRect().height))], truncated: links.every(link => link.scrollWidth > link.clientWidth && getComputedStyle(link).textOverflow === 'ellipsis')};
        });
        const result = await measure();
        const label = JSON.stringify({mode, width, result});
        assert.equal(result.overflow, false, label);
        assert.deepEqual(result.order, Array.from({length: 8}, (_, i) => `Group ${i}`));
        if (width >= 600) {
          assert.equal(result.columns, Math.min(4, Math.floor((result.contentWidth + 16) / (180 + 16))), label);
          assert.ok(result.minWidth >= 179, label);
          assert.ok(result.firstTop >= result.navBottom, label);
          // A narrow parent must reduce the count even on a tablet viewport.
          await page.locator('.bookmark-groups').evaluate(el => { el.style.width = '300px'; });
          const constrained = await measure();
          assert.equal(constrained.columns, 1, JSON.stringify(constrained));
          assert.equal(constrained.overflow, false);
        } else if (mode.includes('mobile-bookmarks-compact') && width >= 360) {
          assert.equal(result.newspaperColumns, '2', label);
        } else if (mode.includes('mobile-bookmarks-')) {
          assert.equal(result.columns, 1, label);
        } else {
          // Classic default: three top-aligned groups per row, one truncated line per link.
          assert.equal(result.columns, 3, label);
          assert.ok(Math.abs(result.minWidth - result.contentWidth * 0.3) < 1, label);
          // Line height 2 at the 14px base size; allow sub-pixel font differences.
          assert.ok(result.rowHeights.every(height => height >= 28 && height < 29), label);
          assert.equal(result.truncated, true, label);
          assert.ok(result.firstTop >= result.navBottom, label);
          // A lone group, such as a selected category, uses the full width.
          await page.locator('.bookmark-groups').evaluate(el => { for (const group of [...el.querySelectorAll('.bookmark-group-container')].slice(1)) group.remove(); });
          const lone = await measure();
          assert.equal(lone.minWidth, lone.contentWidth, JSON.stringify({mode, width, lone}));
          assert.equal(lone.overflow, false);
        }
      }
    }
    // With per-category sections the classic default keeps every category on its own row.
    for (const width of [320, 390, 599]) {
      await page.setViewportSize({width, height: 1024});
      await page.locator('body').evaluate(el => { el.className = 'bookmark-layout-sections'; el.querySelector('.bookmark-groups').style.width = ''; });
      const sections = await page.locator('.bookmark-groups').evaluate(el => {
        const boxes = [...el.querySelectorAll('.bookmark-group-container')].map(group => group.getBoundingClientRect());
        return {widths: boxes.map(box => box.width), stacked: boxes.every((box, i) => i === 0 || box.top >= boxes[i - 1].bottom - 1), contentWidth: el.clientWidth, overflow: document.documentElement.scrollWidth > innerWidth};
      });
      assert.ok(sections.widths.length > 0 && sections.widths.every(value => value === sections.contentWidth), JSON.stringify({width, sections}));
      assert.equal(sections.stacked, true, JSON.stringify({width, sections}));
      assert.equal(sections.overflow, false);
    }
    console.log('PASS: classic, single-column and compact phone layouts, adaptive tablet columns, 768px boundary, narrow parents and category order');
  } finally { await browser.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
