# Mobile bookmark layout

> Availability: layout and readability controls shipped in 0.6.5. Adaptive
> tablet columns and the 599/600px breakpoint below shipped in 0.6.6.

In 0.6.5, sequential single-column layout is the default through 768 CSS pixels;
opt-in compact columns apply from 360 through 768px. Desktop columns start at
769px. Below 360px both choices use a single column.

## 0.6.15: classic default

> Applies to 0.6.15 and later. Releases 0.6.4 through 0.6.14 use the single-column
> default described in the version sections below.

The default phone and tablet layout is again the classic presentation used
through 0.6.3. `Homepage.MobileBookmarkLayout` accepts three choices:

- `classic` (the default, also used when the setting is omitted). Below 600 CSS
  pixels, bookmark categories form three columns, each 30% of the content width
  and aligned at the top. Every link stays on one line and a long name ends with
  an ellipsis; the line height is twice the text size, 28px at the default 14px
  size. Below 768px application cards use two equal columns and keep the desktop
  card size, 40px high on a 64px row pitch at the default text size. A lone
  category, such as a selected category tab or an uncategorized list, uses the
  full width. With `BookmarkLayout: sections` each category keeps its own row.
- `single-column`. The default of 0.6.4 through 0.6.14: categories are stacked
  below 600px, long names wrap, link rows are at least 44px high through 768px,
  and application cards are at least 52px high.
- `compact`. Unchanged; see the compact sections below.

From 600px the adaptive columns described below apply to all three choices. The
classic default no longer adds the 44px minimum row height, extra padding and
wrapping between 600 and 768px. Layouts above 768px are unchanged.

Versions 0.6.5 through 0.6.14 wrote `MobileBookmarkLayout: single` whenever the
appearance form was saved, whether or not the choice had been changed. A stored
`single` therefore cannot be told apart from an untouched default: it is read as
`classic` and replaced by `classic` the next time the appearance form is saved.
To keep the single-column layout, select "Single column" in Appearance settings;
it is stored as `single-column`.

Classic differs from 0.6.3 in three details. Categories in a row are aligned at
the top instead of at their last line. A lone category uses the full width
instead of a 30% column. Application cards grow instead of clipping their text
when the device or browser enlarges text.

Classic link rows are about 28px high, below the 44px touch-target size that the
single-column layout provides, and long names are truncated rather than wrapped.
Choose the single-column layout when larger touch targets or full names matter
more than density.

## 0.6.6 layout

Appearance settings offer a sequential single-column layout (the default) and
opt-in compact columns on 360–599 CSS-pixel phone viewports. Compact columns read down
the first column and then down the second; a bookmark group is kept together.
Below 360px the layout falls back to a single column. Neither option changes the
saved category order. At 600 CSS pixels and above, bookmark groups automatically
form multiple columns with a preferred minimum width of 11.25rem and a default maximum of four columns.
The 768px boundary no longer switches bookmarks to a single column. Mobile layout
choices only affect phones; desktop column limits still apply from 769px.

Long greetings and weather now use natural wrapping at all viewport widths,
including tablet widths and desktop zoom. Physical Safari/Via font scaling and
software keyboard behavior still need device verification.

Verification:

```sh
go run build/build.go
go build -o /tmp/flare-layout .
FLARE_BINARY=/tmp/flare-layout node tests/mobile/layout.browser.cjs
```

The browser suite checks compact columns, the narrow-screen fallback, breakpoint
boundaries and DOM category order. Run `node tests/mobile/bookmark-columns.browser.cjs`
to check classic/single-column/compact layouts, tablet column counts, constrained content widths,
and category navigation across the 599/600 and 767/768/769 boundaries.
Configuration and HTTP tests reject invalid
choices without writing and preserve saved choices when an older form omits them.

## Current development: compact phone and tablet density

The existing `Homepage.MobileBookmarkLayout: compact` option now also reduces
navigation spacing through 1024 CSS pixels, including landscape iPad Mini widths.
It reduces page side padding, section/category margins, bookmark rows and app-card
spacing. The single-column spacing remains available with `single-column`.
Phone category columns retain the 360–599px range and narrow-screen fallback;
tablet columns and desktop column choices retain their existing behavior.

Compact bookmark rows have a 28px minimum height. This is an explicit density
choice; the single-column layout retains its 44px minimum touch targets. Long
application and bookmark names still wrap and enlarged navigation text still grows
the rows. App icons and text use aligned columns; long app descriptions use a
single line with an ellipsis. On compact phones and tablets, settings/help buttons follow the
content so they cannot obscure bookmarks. Settings and
editor controls are unaffected. Viewports wider than 1024px retain their density.

The production layout suite compares compact and single-column heights for the same
bookmarks, checks the classic default, enlarged-text overflow, category order and
the 1024/1025px boundary.
Physical iPad/Safari verification remains required.

## Readability

Appearance settings can enlarge navigation text (`standard`, `larger`, `large`),
choose a greeting title size (`compact`, `standard`, `prominent`) and hide app
descriptions. Defaults retain the existing presentation. Navigation text and
application descriptions apply to the homepage, bookmarks and applications pages;
greeting size affects the homepage. Rem-based sizes do not disable device text
scaling. Enlarged app cards grow vertically rather than clipping their text.

An omitted Homepage block or omitted fields in older appearance forms preserve
saved preferences. Invalid submitted choices are rejected before writing.

Run `FLARE_BINARY=/tmp/flare-layout node tests/mobile/readability.browser.cjs`
after rebuilding to check text sizes, hidden descriptions and wrapping.
