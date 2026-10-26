# Mobile layout regression

Build production assets and a fresh binary, install the shared Playwright dependency,
and run:

```sh
go run build/build.go
go build -o /tmp/flare-mobile .
npm ci --ignore-scripts --prefix tests/clock
tests/clock/node_modules/.bin/playwright install chromium
node tests/mobile/bookmark-columns.browser.cjs
FLARE_BINARY=/tmp/flare-mobile node tests/mobile/bookmark-sections.browser.cjs
FLARE_BINARY=/tmp/flare-mobile node tests/mobile/browser.cjs
FLARE_BINARY=/tmp/flare-mobile node tests/editor/browser.cjs
```

`CHROMIUM_EXECUTABLE` may point to an existing Chromium binary. The mobile suite
covers home/settings/default-editor widths at
320/360/375/390/430/767/768/769/820/1024 CSS pixels,
long titles, unequal groups, enlarged home body text (110/125/200%), weather/date
reflow through 768px and search autofocus. The 767/768/769 boundary is checked
explicitly; the bookmark column suite also checks actual multi-column tablet
positions, phone layout options, narrow parent containers and category navigation
across the 599/600 boundary. Tablet checks exercise overflow and the settings
navigation transition.
Forms are opened at every width and must fit horizontally with reachable actions.
A dropped save connection must retain the complete pending document, leave disk
data unchanged, and allow a successful retry. Desktop checks cover
1024/1440/1920 widths.
`MOBILE_SCREENSHOT_DIR` optionally captures screenshots.

Editor CRUD moved into `tests/editor/browser.cjs`, which covers the default
list/form UI AND the lazy integrated table adapter, rather than continuing to test
a retired standalone editor. It verifies actual filtering, complete filtered
saves, category migration and stable IDs, cancellation/undo, stale revisions,
draft export/restore, failed-save retry, shared list/table data, active cell saves,
clipboard paste, ordering, empty collections and insertion after deletion.
`EDITOR_SCREENSHOT_DIR` captures mobile list/form and desktop list/table views.

The default editor does not load Handsontable until batch mode is requested.
Tables scroll inside their own region. The batch route remains a compatible
entry URL but uses the same interface and working state. Both modes share one
manual save bar and server-side validation/conflict/recovery protections.

These suites use Chromium touch emulation. Physical iOS Safari, Android Via at
110% native font size, keyboards and safe-area behavior still need device checks.
Search autofocus is suppressed for mobile user agents without adding homepage
scripts; desktop-agent tablets can use the existing autofocus setting.

Follow [the physical device checklist](../../docs/mobile-device-acceptance.md)
before declaring Safari/Via, keyboard and safe-area issues resolved. The checklist
starts as Not verified and requires device/version evidence for each real run.

The sections suite checks generated production resources, per-category columns,
column choices, long names, enlarged fonts, constrained parents, category/search
combinations, collapse visibility, and uncategorized lists. It also runs in CI.
