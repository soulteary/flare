// Render the production editor shell for mocked browser tests without a Go service.
const fs = require('node:fs/promises');
const path = require('node:path');
const assert = require('node:assert/strict');

function escapeHTML(value) {
 return String(value).replace(/[&<>"']/g, char => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[char]));
}

async function editorFixture(root = path.resolve(__dirname, '../..'), locale = 'zh') {
 assert.ok(['zh', 'en'].includes(locale), `Unsupported fixture locale: ${locale}`);
 const messages = JSON.parse(await fs.readFile(path.join(root, 'internal/i18n/locales', locale + '.json'), 'utf8'));
 function render(source) {
  return source.replace(/\{\{T \.Locale "([^"]+)"\}\}/g, (_, key) => {
   assert.ok(Object.hasOwn(messages, key), `Missing editor fixture message: ${key}`);
   return escapeHTML(messages[key]);
  }).replace(/\{\{HTMLLang \.Locale\}\}/g, locale === 'zh' ? 'zh-CN' : 'en')
   .replace(/\{\{\.EditorAssetVersion\}\}/g, 'test')
   .replace(/\{\{\.EditorMessages\}\}/g, escapeHTML(JSON.stringify(messages)));
 }
 return {messages, render, html:render(await fs.readFile(path.join(root, 'embed/templates/editor.html'), 'utf8'))};
}
module.exports = {editorFixture};
