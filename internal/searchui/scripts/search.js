(function () {
  'use strict';
  var form = document.getElementById('search-form');
  var input = document.getElementById('search');
  var list = document.getElementById('search-suggestions');
  var status = document.getElementById('search-status');
  if (!form || !input || !list || !status) return;
  var target = document.getElementById('search-target');
  var keywords = {};
  try { keywords = JSON.parse(form.dataset.keywords || '{}'); } catch (_) {}
  var timer, controller, generation = 0, active = -1, links = [], composing = false, pointingAtSuggestion = false;
  input.setAttribute('role', 'combobox');
  input.setAttribute('aria-autocomplete', 'list');
  input.setAttribute('aria-controls', list.id);
  input.setAttribute('aria-expanded', 'false');
  function clear() {
    list.replaceChildren(); list.hidden = true; links = []; active = -1; status.textContent = '';
    input.setAttribute('aria-expanded', 'false'); input.removeAttribute('aria-activedescendant');
  }
  function cancel() {
    clearTimeout(timer); generation += 1;
    if (controller) controller.abort();
  }
  function select(index) {
    active = index;
    links.forEach(function (link, i) {
      link.parentElement.setAttribute('aria-selected', i === index ? 'true' : 'false');
    });
    if (index >= 0) { input.setAttribute('aria-activedescendant', links[index].parentElement.id); links[index].scrollIntoView({block: 'nearest'}); }
    else input.removeAttribute('aria-activedescendant');
  }
  function refresh() {
    cancel(); clear(); status.textContent = '';
    if (composing || !input.value.trim()) return;
    var command = /^!([^\s]+)/.exec(input.value.trim());
    if (command && Object.prototype.hasOwnProperty.call(keywords, command[1].toLowerCase())) return;
    var query = input.value.trim(), version = generation;
    timer = setTimeout(function () {
      controller = new AbortController();
      fetch('/search/suggest', {
        method: 'POST', credentials: 'same-origin', cache: 'no-store', redirect: 'error',
        headers: {'Content-Type': 'application/x-www-form-urlencoded'},
        body: new URLSearchParams({search: query}).toString(), signal: controller.signal
      }).then(function (response) {
        if (!response.ok) throw new Error('suggestions unavailable');
        return response.json();
      }).then(function (items) {
        if (version !== generation || input.value.trim() !== query || composing) return;
        clear();
        items.forEach(function (item, i) {
          var url;
          try { url = new URL(item.url, window.location.href); } catch (_) { return; }
          if (!['http:', 'https:', 'chrome-extension:'].includes(url.protocol)) return;
          var li = document.createElement('li'), link = document.createElement('a');
          li.id = 'search-match-' + i; li.setAttribute('role', 'option'); li.setAttribute('aria-selected', 'false');
          link.href = url.href; link.textContent = item.name;
          link.rel = 'noopener noreferrer';
          if (item.newTab) link.target = '_blank';
          li.appendChild(link); list.appendChild(li); links.push(link);
        });
        list.hidden = !links.length;
        input.setAttribute('aria-expanded', links.length ? 'true' : 'false');
        status.textContent = (form.dataset.matchCount || '%s local matches').replace('%s', String(links.length));
      }).catch(function () {
        if (version === generation) clear();
      });
    }, 180);
  }
  function dismiss() { cancel(); clear(); }
  if (target) target.addEventListener('change', dismiss);
  list.addEventListener('pointerdown', function () { pointingAtSuggestion = true; });
  document.addEventListener('pointerup', function () {
    setTimeout(function () { pointingAtSuggestion = false; }, 0);
  });
  document.addEventListener('pointercancel', function () { pointingAtSuggestion = false; dismiss(); });
  list.addEventListener('click', function () {
    // Let the real anchor's default click/tap navigation finish before removing it.
    setTimeout(dismiss, 0);
  });
  form.addEventListener('focusout', function (event) {
    cancel();
    if (!pointingAtSuggestion && !list.contains(event.relatedTarget)) clear();
  });
  input.addEventListener('focus', refresh);
  window.addEventListener('blur', dismiss);
  input.addEventListener('compositionstart', function () { composing = true; cancel(); clear(); });
  input.addEventListener('compositionend', function () { composing = false; refresh(); });
  input.addEventListener('input', refresh);
  input.addEventListener('keydown', function (event) {
    if (composing || event.isComposing || event.keyCode === 229) return;
    if (event.key === 'Escape') { cancel(); clear(); }
    if (!links.length) return;
    if (event.key === 'ArrowDown') { event.preventDefault(); select((active + 1) % links.length); }
    if (event.key === 'ArrowUp') { event.preventDefault(); select(active < 0 ? links.length - 1 : (active - 1 + links.length) % links.length); }
    if (event.key === 'Enter' && active >= 0) { event.preventDefault(); links[active].click(); }
  });
  form.addEventListener('submit', function (event) {
    if (composing) { event.preventDefault(); return; }
    var checkedTarget = target && target.querySelector('input[name="target"]:checked');
    var selectedTarget = checkedTarget ? checkedTarget.value : 'local';
    if (event.submitter && event.submitter.name === 'target') selectedTarget = event.submitter.value;
    var command = /^!([^\s]+)/.exec(input.value.trim());
    if (command && Object.prototype.hasOwnProperty.call(keywords, command[1].toLowerCase())) selectedTarget = keywords[command[1].toLowerCase()];
    var explicitTarget = event.submitter && event.submitter.getAttribute('formtarget');
    form.target = explicitTarget || (form.dataset.openNewTab === 'true' && selectedTarget !== 'local' ? '_blank' : '_self');
    cancel(); clear();
  });
  document.addEventListener('click', function (event) { if (!form.contains(event.target)) { cancel(); clear(); } });
})();
