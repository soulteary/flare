(() => {
  'use strict';
  const form = document.getElementById('weather-form');
  if (!form) return;
  const input = document.getElementById('weather-keyword');
  const results = document.getElementById('weather-results');
  const selected = document.getElementById('weather-selected');
  const address = document.getElementById('settings-location');
  const status = document.getElementById('weather-status');
  let timer, controller, sequence = 0, composing = false;
  function clear() {
    sequence++;
    clearTimeout(timer);
    if (controller) controller.abort();
    selected.value = '';
    results.replaceChildren();
  }
  function show(values) {
    status.textContent = values.length ? form.dataset.choose : form.dataset.empty;
    for (const value of values) {
      const option = document.createElement('button');
      option.type = 'button';
      option.className = 'weather-candidate';
      option.textContent = [value.province, value.city, value.county].filter(Boolean).join(' / ');
      option.addEventListener('click', () => {
        selected.value = value.id;
        address.value = [value.province, value.city, value.county].filter(Boolean).join(', ');
        results.replaceChildren();
        status.textContent = option.textContent;
      });
      results.append(option);
    }
  }
  async function search() {
    if (composing) return;
    clear();
    const current = sequence;
    if (!input.value.trim()) { status.textContent = ''; return; }
    controller = new AbortController();
    status.textContent = form.dataset.loading;
    try {
      const response = await fetch('/settings/weather/locations?q=' + encodeURIComponent(input.value.trim()), {signal:controller.signal});
      const values = await response.json();
      if (!response.ok) throw new Error(values.error || form.dataset.failed);
      if (current !== sequence) return;
      show(values);
    } catch (error) {
      if (current === sequence && error.name !== 'AbortError') status.textContent = error.message || form.dataset.failed;
    }
  }
  document.getElementById('weather-detect').addEventListener('click', async () => {
    clear();
    const current = sequence;
    controller = new AbortController();
    status.textContent = form.dataset.loading;
    try {
      const response = await fetch('/settings/weather/detect', {signal:controller.signal});
      if (!response.ok) throw new Error('detect');
      const value = await response.json();
      if (current !== sequence) return;
      input.value = value.city;
      show([value]);
    } catch (error) {
      if (current === sequence && error.name !== 'AbortError') status.textContent = form.dataset.detectFailed;
    }
  });
  function changed() {
    clear();
    address.value = '';
    status.textContent = form.dataset.choose;
    if (!composing) timer = setTimeout(search, 300);
  }
  input.addEventListener('compositionstart', () => { composing = true; clear(); });
  input.addEventListener('compositionend', () => { composing = false; changed(); });
  input.addEventListener('input', changed);
  document.getElementById('weather-search').addEventListener('click', search);
  input.addEventListener('keydown', event => {
    if (event.isComposing || composing || event.keyCode === 229) return;
    if (event.key === 'Enter') { event.preventDefault(); search(); }
  });
})();
