# Date, time and time zones / 日期、时间与时区

## Configure / 配置

Open **Settings → Appearance → Module settings**. Keep “Show date and time” on,
then choose **On page load** or **Live clock**, whether to show seconds, and a
display time zone. The time-zone input provides searchable browser suggestions;
you may also type any supported IANA geographical name or `UTC`. Preview updates
when an option changes, using the visitor device clock.

进入 **设置 → 外观 → 模块设置**，启用日期时间后，可以选择页面加载时显示或实时更新、
是否显示秒数，以及显示时区。指定时区支持搜索建议，也可以直接输入 IANA 地理时区名称
或 `UTC`。修改选项会更新预览，预览使用访问设备的系统时间。

| Setting | Behavior / 行为 |
| --- | --- |
| Follow server / 跟随服务器 | Uses the Go process local time zone, usually configured with `TZ`. / 使用进程本地时区，通常通过 `TZ` 配置。 |
| Follow visitor device / 跟随访问设备 | JavaScript uses the visitor's OS/browser zone. / JavaScript 使用访问设备的时区。 |
| Custom / 指定时区 | Uses a validated zone such as `Asia/Shanghai`, `America/New_York`, `Asia/Kathmandu` or `UTC`. / 使用经过校验的指定时区。 |

Example `config.yml` addition (the rest of your configuration remains unchanged):

```yaml
ShowDateTime: true
Clock:
  Dynamic: true
  Seconds: true
  TimezoneMode: custom
  Timezone: Asia/Shanghai
```

`TimezoneMode` accepts `server`, `browser` and `custom`. An omitted `Clock` block
keeps the old behavior: a static server-rendered clock with seconds. Within a
block, omitted `Dynamic` is false, omitted `Seconds` is true, and an omitted
`TimezoneMode` means `server`. `Timezone` is used only in custom mode. Turning
off `ShowDateTime` omits the clock and its script without erasing preferences.

缺少 `Clock` 时保持旧行为：服务端静态显示，包含秒数。配置块内未填写 `Dynamic` 时为
false，未填写 `Seconds` 时为 true，未填写 `TimezoneMode` 时跟随服务器。关闭
`ShowDateTime` 后不输出时钟及其脚本，已保存的偏好保留。

For Docker, set the process zone explicitly:

```yaml
environment:
  TZ: Asia/Shanghai
```

Restart after changing `TZ`. Stop writers before externally editing `config.yml`;
0.6.5 requires a restart to observe those edits, while 0.6.6 checks the file on each
configuration read. Web settings take effect
on the next page load. Page settings never modify `TZ`, `time.Local`, the host
clock, or the weather location.

修改 `TZ` 后请重启。手动编辑 `config.yml` 前请停止写入：0.6.5 需要重启后读取修改，
未发布的 0.6.6 候选版会在下次读取配置时检查文件变化。网页保存的设置在下次加载页面时
生效。显示时区设置不会修改进程全局时区、主机时间或天气城市。

## Time source and fallback / 时间来源与降级

- Static server/custom display uses the server's system clock. Live display uses
  the **visitor device's system clock** in the selected display zone. Device-zone
  static mode initializes once from that device's clock (including when initially
  opened in a background tab), with no recurring timer.
- This is a start-page display, not NTP or a network synchronization service.
  An incorrectly set device clock produces an incorrectly timed live display.
- Date, weekday, time and segmented greetings use the same display zone. Fixed
  greetings and the Help page title remain unchanged. Daylight-saving rules come
  from Go's embedded tzdata on the server and the browser's Intl data on the client;
  keep the application/browser updated if local legal time-zone rules change.
- An invalid custom zone submitted through Settings is rejected before any save.
  Invalid hand-edited clock configuration falls back to the server zone and logs
  one warning per process. If the server exposes only an opaque `Local` zone (for
  example from `/etc/localtime`), its static display is retained; explicitly set
  `TZ=Asia/Shanghai` or choose a custom IANA zone to enable live updates.
- Unsupported Intl/time zones or disabled JavaScript retain readable server HTML.
  Device-zone mode displays a no-script note when JavaScript is disabled. The
  initial server time may briefly appear before device time replaces it.
- Hidden tabs stop recurring work; becoming visible, gaining focus or returning
  from the back/forward cache refreshes the current time. Sleep and missed ticks
  never accumulate drift. Minute mode refreshes at minute boundaries, so a device
  clock change while continuously visible may take up to about a minute to appear.

静态服务器/指定时区模式使用服务器时间；实时模式使用访问设备时间，再按所选时区换算。
时区选择不等于服务器校时，不新增任何外部请求或周期时间接口。设备时区的静态模式只在
初始化时计算一次。日期、星期、时间和分时问候语统一换算，固定问候语、帮助页标题和天气
位置不受动态更新影响。

非法时区的网页提交会被拒绝，不改动原配置。手动编辑产生的非法时钟设置会回退服务器时区，
每次进程运行最多记录一次相关警告。无法向浏览器传递的 `Local` 时区保留静态显示；请显式
配置 `TZ` 或指定 IANA 时区。缺少浏览器能力或禁用 JavaScript 时保留服务端输出。

## Security and performance / 安全与性能

Both enhancement scripts are immutable embedded resources, registered as literal
named templates without `template.JS` auto-escaping bypasses. CSP hashes authorize
only their exact bytes; configuration is HTML-escaped in data attributes, never
interpolated into JavaScript. No `unsafe-inline`, `eval`, CDN, date library or
framework is needed. The default static home retains `script-src 'none'` and no
script tag. The old homepage Escape listener, which default CSP already blocked,
is removed; the Appearance page's existing Escape shortcut is included in its
explicitly authorized settings script. Other settings pages are unchanged.

HTML minification recognizes Go template delimiters; production/debug HTTP tests
check final script bytes against the response CSP hash. Configuration writes use
a temporary file and atomic rename, preserving the original file on validation or
write failure. The live clock has at most one timer, pauses in hidden tabs, and
only updates changed text nodes. It does not announce each tick to screen readers.
The clock script is approximately 1.2 KB gzipped (without HTTP headers).

脚本以精确哈希授权，配置通过经过转义的 HTML 属性传递，不开放任意内联脚本。
默认静态首页继续禁止脚本，动态时钟不依赖外部资源，后台不持续计时，不逐秒触发读屏播报。

## Validation / 验证

```sh
go run build/build.go
go test -race ./...
node --test tests/clock/clock.test.cjs
cd tests/clock
npm ci --ignore-scripts
npx playwright install --with-deps chromium
cd ../..
go build -o /tmp/flare-clock .
FLARE_BINARY=/tmp/flare-clock node tests/clock/browser.cjs
```

`CHROMIUM_EXECUTABLE` optionally selects a preinstalled Chromium executable.
The browser harness launches an isolated offline-mode application with temporary
configuration and disabled login only for that test process, then cleans it up.
CI runs the deterministic lifecycle tests and real Chromium acceptance alongside
normal Go checks. A scratch-container check verifies embedded tzdata without OS
zoneinfo files, with New York as the process zone and Kathmandu as the display zone.
Fixtures cover midnight, month/year/leap-day boundaries, New
York DST in both directions, non-hour offsets, both languages, seconds/minutes,
visibility/BFCache/focus, duplicate initialization, invalid settings, exact CSP,
unrelated script blocking, device-zone rendering, mobile clock and no-JS fallback.
