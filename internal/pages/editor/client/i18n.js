/* Messages come from Go's existing locale bundle, serialized into an escaped attribute. */
(function () {
  'use strict';
  let messages = {};
  try { messages = JSON.parse(document.getElementById('editor-messages')?.dataset.messages || '{}'); } catch (_) {}
  function t(source, ...args) {
    return (messages[source] || source).replace(/\{(\d+)\}/g, (_, index) => String(args[index] ?? ''));
  }
  window.FlareEditorI18n = {t};
  // Run before loading any user document: only the static editor shell is translated.
  const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
  let node;
  while ((node = walker.nextNode())) {
    if (['SCRIPT', 'STYLE'].includes(node.parentElement?.tagName)) continue;
    const source = node.textContent.trim();
    if (Object.hasOwn(messages, source)) node.textContent = node.textContent.replace(source, messages[source]);
  }
  for (const element of document.querySelectorAll('[placeholder], [aria-label], [title]')) {
    for (const attribute of ['placeholder', 'aria-label', 'title']) {
      const source = element.getAttribute(attribute);
      if (source && Object.hasOwn(messages, source)) element.setAttribute(attribute, messages[source]);
    }
  }
  if (messages[document.title]) document.title = messages[document.title];
})();
