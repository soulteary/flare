# Mobile bookmark layout

Appearance settings offer a sequential single-column layout (the default) and
opt-in compact columns on 360–768 CSS-pixel viewports. Compact columns read down
the first column and then down the second; a bookmark group is kept together.
Below 360px the layout falls back to a single column. Neither option changes the
saved category order. Desktop layout is unchanged.

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
boundaries and DOM category order. Configuration and HTTP tests reject invalid
choices without writing and preserve saved choices when an older form omits them.
