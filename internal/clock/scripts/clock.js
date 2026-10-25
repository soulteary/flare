(function () {
  'use strict';
  var root = document.getElementById('clock');
  if (!root || root.dataset.initialized) return;
  var timer;
  var dynamic = root.dataset.dynamic === 'true';
  var seconds = root.dataset.seconds === 'true';
  var locale = root.dataset.locale === 'en' ? 'en-US' : 'zh-CN';
  var dateNode = document.getElementById('clock-date');
  var dayNode = document.getElementById('clock-day');
  var timeNode = document.getElementById('clock-time');
  var greeting = document.getElementById('plugin-greetings');
  var zone = root.dataset.mode === 'browser' ? undefined : root.dataset.zone;
  var formatter;
  try {
    formatter = new Intl.DateTimeFormat(locale, {
      timeZone: zone, calendar: 'gregory', numberingSystem: 'latn',
      year: 'numeric', month: '2-digit', day: '2-digit', weekday: 'long',
      hour: '2-digit', minute: '2-digit', second: '2-digit', hourCycle: 'h23'
    });
    if (!formatter.formatToParts || !dateNode || !dayNode || !timeNode) return;
  } catch (_) { return; }
  root.dataset.initialized = 'true';
  function write(node, text) { if (node && node.textContent !== text) node.textContent = text; }
  function stop() { clearTimeout(timer); timer = undefined; }
  function update() {
    stop();
    // Static device time must initialize even in a background tab.
    if (dynamic && document.hidden) return;
    try {
      var now = new Date();
      var parts = {};
      formatter.formatToParts(now).forEach(function (p) { parts[p.type] = p.value; });
      // Match the existing Go layouts byte-for-byte in both supported languages.
      var months = ['Jan','Feb','Mar','Apr','May','Jun','Jul','Aug','Sep','Oct','Nov','Dec'];
      var date = locale === 'en-US' ? months[Number(parts.month)-1] + ' ' + parts.day + ', ' + parts.year : parts.year + '年' + parts.month + '月' + parts.day + '日';
      write(dateNode, date);
      write(dayNode, parts.weekday);
      write(timeNode, parts.hour + ':' + parts.minute + (seconds ? ':' + parts.second : ''));
      if (greeting && root.hasAttribute('data-greetings')) {
        var words = root.dataset.greetings.split(';');
        var h = Number(parts.hour);
        var index = h >= 5 && h <= 10 ? 0 : h >= 11 && h <= 13 ? 1 : h >= 14 && h <= 18 ? 2 : 3;
        write(greeting, (words.length === 1 ? words[0] : words[index]) || root.dataset.greetingFallback);
      }
      if (dynamic) {
        var step = seconds ? 1000 : 60000;
        timer = setTimeout(update, step - Date.now() % step + 15);
      }
    } catch (_) { stop(); }
  }
  if (dynamic) {
    document.addEventListener('visibilitychange', function () { if (document.hidden) stop(); else update(); });
    window.addEventListener('focus', update);
    window.addEventListener('pageshow', update);
    window.addEventListener('pagehide', stop);
  }
  update();
}());
