// Exercise the actual clipboard handlers in both vendor and embedded pages.
const { chromium } = require(require.resolve('playwright', { paths: [__dirname + '/../clock'] }));
const fs = require('node:fs/promises');
const path = require('node:path');
const assert = require('node:assert/strict');

(async () => {
  const browser = await chromium.launch({
    headless: true,
    executablePath: process.env.CHROMIUM_EXECUTABLE || undefined,
  });
  try {
    for (const file of [
      'embed/assets/vendor/mdi-cheat-sheets/index.html',
      'internal/resources/mdi/mdi-cheat-sheets/index.html',
    ]) {
      const page = await browser.newPage();
      await page.evaluate(() => {
        window.copiedValues = [];
        document.execCommand = command => {
          window.copiedValues.push({ command, text: document.activeElement.value });
          return true;
        };
      });
      await page.setContent(await fs.readFile(path.resolve(__dirname, '../..', file), 'utf8'));
      for (const [cssName, flareName] of [
        ['access-point', 'accessPoint'],
        ['account-box-edit-outline', 'accountBoxEditOutline'],
        ['account', 'account'],
      ]) {
        const item = page.locator('#icons > div').filter({ has: page.locator(`i.mdi-${cssName}`) });
        const name = item.locator('span');
        assert.equal(await name.textContent(), flareName);
        await name.click();
        assert.equal(await page.locator('#name').textContent(), flareName);
        assert.deepEqual(await page.evaluate(() => window.copiedValues.at(-1)), { command: 'copy', text: flareName });
      }
      // Glyph rendering and codepoint copying retain the upstream CSS names.
      const item = page.locator('#icons > div').filter({ has: page.locator('i.mdi-access-point') });
      await item.locator('code').click();
      assert.equal(await page.evaluate(() => window.copiedValues.at(-1).text), 'F0003');
      await item.locator('i').click();
      assert.equal(await page.evaluate(() => window.copiedValues.at(-1).text), String.fromCodePoint(0xF0003));
      await page.close();
    }
    console.log('PASS: Flare names display and copy correctly in both cheat sheets; glyph/codepoint copying retained');
  } finally {
    await browser.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
