# Mobile layout and editor regression

Build the generated templates/styles and binary first:

```sh
go run build/build.go
go build -o /tmp/flare-mobile .
npm ci --ignore-scripts --prefix tests/clock
tests/clock/node_modules/.bin/playwright install chromium
FLARE_BINARY=/tmp/flare-mobile node tests/mobile/browser.cjs
```

The test uses Playwright (the same dependency as `tests/clock`), a fresh temporary
configuration and an offline server. `CHROMIUM_EXECUTABLE` may point to an existing
Chromium binary. `MOBILE_SCREENSHOT_DIR` optionally captures screenshots.

Coverage: 320/360/375/390/430/768 CSS-pixel widths, body text enlarged to
110/125/200%, long names, unequal groups, mobile search autofocus suppression,
settings navigation, touch selection and explicit editing, category rename without
losing pending bookmark edits, deletion, save/reload, empty-table insertion and
save failure/retry. Desktop checks cover 1024/1440/1920 widths.

Mobile defaults to one bookmark group per row; applications retain two columns
except below 360px. Settings navigation wraps above the form. Tables scroll inside
their own regions, rather than widening the entire page. Category deletion moves
its bookmarks into Flare applications after confirmation. Save errors preserve the
current client-side edits. The existing two-file persistence mechanism remains;
this change does not promise a cross-file transaction or concurrent-editor merge.

Search autofocus remains available on desktop. Mobile/Android/iPad/iPhone user
agents suppress it without adding JavaScript to the static homepage; responses
vary by User-Agent. A tablet presenting a desktop user agent may still autofocus;
the existing setting can disable autofocus for that deployment.

This suite uses Chromium touch emulation. Physical iOS Safari, Android Via at 110%
font size, native font scaling, software keyboards and safe-area behavior still
require device verification. Double-column masonry and a dedicated mobile form
editor are intentionally deferred.
