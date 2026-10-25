(function () {
  'use strict';
  document.addEventListener('keydown', function (e) { if (e.key === 'Escape') history.back(); });
  var show = document.getElementById('settings-show-datetime');
  var group = document.getElementById('clock-settings');
  var mode = document.getElementById('clock-mode');
  var zone = document.getElementById('clock-zone');
  var custom = document.getElementById('clock-custom');
  var preview = document.getElementById('clock-preview');
  if (!show || !group || !mode || !zone || !custom || !preview) return;
  var zones = ['UTC','Asia/Shanghai','Asia/Hong_Kong','Asia/Tokyo','Asia/Singapore','Asia/Kolkata','Asia/Kathmandu','Europe/London','Europe/Berlin','America/New_York','America/Los_Angeles','Australia/Sydney','Pacific/Auckland'];
  if (typeof Intl !== 'undefined' && typeof Intl.supportedValuesOf === 'function') {
    try { zones = ['UTC'].concat(Intl.supportedValuesOf('timeZone')); } catch (_) { /* common suggestions remain available */ }
  }
  var list = document.getElementById('clock-zones');
  zones.forEach(function (name) { var option = document.createElement('option'); option.value = name; list.appendChild(option); });
  function update() {
    group.hidden = show.value !== '1';
    custom.hidden = mode.value !== 'custom';
    zone.required = !group.hidden && mode.value === 'custom';
    var timezone = mode.value === 'custom' ? zone.value : mode.value === 'server' ? preview.dataset.serverZone : undefined;
    try {
      preview.textContent = new Intl.DateTimeFormat(preview.dataset.locale === 'zh' ? 'zh-CN' : 'en-US', {
        timeZone: timezone, year: 'numeric', month: '2-digit', day: '2-digit',
        weekday: 'long', hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23'
      }).format(new Date());
    } catch (_) { preview.textContent = preview.dataset.error; }
  }
  show.addEventListener('change', update);
  mode.addEventListener('change', update);
  zone.addEventListener('input', update);
  update();
}());
