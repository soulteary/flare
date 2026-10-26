# Mobile bookmark layout

> Availability: layout and readability controls shipped in 0.6.5. Adaptive
> tablet columns and the 599/600px breakpoint below belong to the unpublished
> 0.6.6 candidate.

In 0.6.5, sequential single-column layout is the default through 768 CSS pixels;
opt-in compact columns apply from 360 through 768px. Desktop columns start at
769px. Below 360px both choices use a single column.

## 0.6.6 candidate layout

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
to check default/compact layouts, tablet column counts, constrained content widths,
and category navigation across the 599/600 and 767/768/769 boundaries.
Configuration and HTTP tests reject invalid
choices without writing and preserve saved choices when an older form omits them.

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
