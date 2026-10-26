/* Go serializes translated messages into an escaped attribute. Static labels use templates. */
(function () {
  'use strict';
  let messages = {};
  try { messages = JSON.parse(document.getElementById('editor-messages')?.dataset.messages || '{}'); } catch (_) {}
  function t(key, params = {}) {
    const message = Object.hasOwn(messages, key) ? messages[key] : key;
    // One pass: inserted values (including user text) are never interpreted as placeholders.
    return message.replace(/\{([a-zA-Z][a-zA-Z0-9_]*)\}/g, (placeholder, name) =>
      Object.hasOwn(params, name) ? String(params[name]) : placeholder);
  }
  window.FlareEditorI18n = {t};
})();
