# Style tokens

> Availability: in current source; not part of 0.6.15, whose stylesheets use the
> three color variables directly.

The stylesheets of the themed pages take their colors, corner radius and font
from CSS custom properties, the style tokens. A token names the role of a
value, for example the color of a border, instead of the value.

The names and meanings in the table are stable: a stylesheet or a footer that
uses them keeps working across releases. Element ids, class names and the
nesting of the pages are not part of this promise and can change.

## Tokens

The first three tokens are the colors of the [theme](theme-colors.md). The
others have a default that derives from them, so three colors make a complete
theme. The bundled themes and custom colors set these three; a
[theme file](theme-files.md) can set any token.

| Token | Default | Used for |
| --- | --- | --- |
| `--flare-background` | Background color of the theme | Page background |
| `--flare-primary` | Primary color of the theme | Text, and the fill of a pressed control |
| `--flare-accent` | Accent color of the theme | Toolbar buttons and the current category tab |
| `--flare-icon` | `--flare-primary` | Icons from the bundled icon set |
| `--flare-muted` | `--flare-accent` | Group titles and application descriptions |
| `--flare-border` | `--flare-accent` | Borders of the search field, its controls and the category tabs |
| `--flare-focus` | `--flare-accent` | Focus outlines |
| `--flare-inverse` | `--flare-background` | Text and icons drawn on a filled element |
| `--flare-hover` | `rgba(0, 0, 0, 0.2)` | Background of an application under the pointer |
| `--flare-radius` | `4px` | Corners of small controls; larger corners are multiples of it |
| `--flare-font-family` | `Roboto, sans-serif` | Text |

## Scope

- The home page, the application and bookmark pages and the help page use all
  tokens.
- The settings pages use the three theme colors, the radius and the font. The
  other color tokens do not apply to them.
- The editor, the import/export page and the sign-in page keep their own
  colors and use no token.

## Details

- **Where tokens are defined.** On `body`. A derived token is computed there:
  setting `--flare-primary` on an element inside the page does not change
  `--flare-icon` below that element. Set tokens on `body`.
- **Earlier names.** `--color-background`, `--color-primary` and
  `--color-accent` remain available and hold the theme colors. Existing footer
  markup that uses them is unaffected.
- **Forced colors.** When the browser forces colors, bundled icons are drawn in
  the system text color whatever `--flare-icon` is; see
  [bundled icons](theme-colors.md#bundled-icons).
- **Round shapes.** The toolbar buttons and switches keep their round shape and
  do not follow `--flare-radius`.

## Verification

`FLARE_BINARY=/absolute/path/to/flare node tests/theme/tokens.browser.cjs` sets
each token in Chromium and checks that the parts of the home page listed above
take the value and that no other measured part changes. It does the same for
the three earlier names.
