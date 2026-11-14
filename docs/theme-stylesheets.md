# Theme stylesheets

> Availability: in current source; not part of 0.6.15.

A [theme that is a directory](theme-files.md#a-theme-as-a-directory) can bring
a stylesheet. The theme file sets the colors and the other
[style tokens](theme-tokens.md); the stylesheet is for what the tokens do not
say.

## The file

```text
themes/
    zelda/
        theme.yml
        theme.css
```

- The stylesheet is the file `theme.css` beside `theme.yml`. A theme that is
  a single file has none, and a `theme.css` in a directory without a
  `theme.yml` is not read.
- It is a regular file of up to 128 KiB in UTF-8. A link is not followed.
- A theme without a `theme.css`, or with an empty one, has no stylesheet.
- A theme is used as a whole. A theme whose stylesheet is
  [not accepted](#what-a-stylesheet-can-contain) is not in use, and the theme
  page says where and why; the theme is not applied without its stylesheet.
- The card of a theme on the theme page says that the theme has a stylesheet,
  so this is known before the theme is selected.

## Where it applies

| Pages | Stylesheet of the theme |
| --- | --- |
| Home, applications, bookmarks, help | Loaded |
| Settings | Not loaded. They take the colors, the corner radius and the font of the theme, as before |
| Editor, sign-in, import and export | Not loaded. These pages are not themed |

The settings are where a theme is changed, so no stylesheet of a theme can
make them unusable.

A page links the stylesheet after the styles of the application: a rule of the
theme wins over a rule of the application with the same selector. The tokens
of the theme file follow the stylesheet:

- A token the theme file sets has the value of the theme file, also where the
  stylesheet sets it on `body`.
- A token the theme file leaves alone can be set by the stylesheet, on `body`.

A [preview](theme-files.md#preview) loads the stylesheet of the previewed
theme.

## What a stylesheet can contain

The stylesheet of a theme loads nothing. With a selector on the address of a
link and the address of an image, a stylesheet tells whoever serves the image
which links a page has, and an instance shows some links to the administrator
only. A stylesheet therefore names no resource by an address: no image, no
font and no other stylesheet.

The check reads the stylesheet as a browser reads it, token by token. A name
is found however it is written: in another case, with escapes, or behind a
comment. What a comment or a string holds is text and is not looked at.

| Part | Accepted |
| --- | --- |
| `url()` | No. That holds for every way of writing it and every address, `data:` addresses and fragments such as `url(#shape)` included |
| At-rules | `@media`, `@supports`, `@container`, `@layer`, `@scope`, `@starting-style`, `@keyframes`, `@-webkit-keyframes`, `@property`, `@counter-style`, `@page`. No other: not `@import`, `@namespace`, `@font-face` or `@charset` |
| Functions | Those listed below. No other: not `image-set()`, `image()`, `src()`, `cross-fade()`, `element()` or `paint()` |
| Characters outside ASCII | In strings and comments, any. Elsewhere, the letters and other characters that are part of a name in every browser; not signs such as `×` and `→`, and not a no-break space |

The functions:

| Use | Functions |
| --- | --- |
| Variables and arithmetic | `var()`, `env()`, `attr()`, `calc()`, `min()`, `max()`, `clamp()`, `round()`, `mod()`, `rem()`, `abs()`, `sign()`, `sin()`, `cos()`, `tan()`, `asin()`, `acos()`, `atan()`, `atan2()`, `pow()`, `sqrt()`, `hypot()`, `log()`, `exp()` |
| Colors | `rgb()`, `rgba()`, `hsl()`, `hsla()`, `hwb()`, `lab()`, `lch()`, `oklab()`, `oklch()`, `color()`, `color-mix()`, `light-dark()` |
| Gradients | `linear-gradient()`, `radial-gradient()`, `conic-gradient()` and their `repeating-` forms |
| Transforms | `matrix()`, `matrix3d()`, `perspective()`, `translate()`, `scale()`, `rotate()` with their `X`, `Y`, `Z` and `3d` forms, `skew()`, `skewX()`, `skewY()` |
| Filters | `blur()`, `brightness()`, `contrast()`, `drop-shadow()`, `grayscale()`, `hue-rotate()`, `invert()`, `opacity()`, `saturate()`, `sepia()` |
| Shapes | `circle()`, `ellipse()`, `inset()`, `polygon()`, `path()`, `rect()`, `xywh()` |
| Timing | `cubic-bezier()`, `steps()`, `linear()` |
| Sizes | `minmax()`, `repeat()`, `fit-content()` |
| Counters | `counter()`, `counters()` |
| Selectors and conditions | `:not()`, `:is()`, `:where()`, `:has()`, `:lang()`, `:dir()`, `:nth-child()`, `:nth-last-child()`, `:nth-of-type()`, `:nth-last-of-type()`, `selector()`, `style()` |

- An at-rule or a function that is not listed is refused, also one that loads
  nothing. The lists hold what was looked at, so a name that a later browser
  gives a meaning has none here until it is added.
- A function with a vendor prefix, such as `-webkit-linear-gradient()`, is
  not listed; the form without the prefix is.
- `@charset` is refused because it could have the file read in another
  encoding than the one it was checked in. The file is served as UTF-8.
- Whether the rest of a stylesheet is well-formed is left to the browser,
  which drops what it cannot use.

The theme page names the first part that is not accepted, with its line:

| Reason | Shown with |
| --- | --- |
| `url()` is not accepted | The line, for example `line 12: url()` |
| At-rule not accepted | The line and the rule, for example `line 1: @import` |
| Function not accepted | The line and the function, for example `line 7: image-set()` |
| Character not accepted outside strings and comments | The line and the character, for example `line 3: U+00D7` |
| Not UTF-8 text | |
| Larger than 128 KiB | |
| Not a regular file, or cannot be read | |

### What the check does not cover

The check keeps a stylesheet from loading or sending anything, and the
settings do not load it. It does not judge what a stylesheet does to the
navigation pages. A stylesheet can hide, move and restyle everything on them,
and with `content` it can show text of its own. A theme is therefore installed
like the other content of an instance: by the administrator, from a source
the administrator trusts. [Safe mode](theme-files.md#safe-mode) shows a page,
or runs the instance, without the themes of the directory.

## What a stylesheet can rely on

The navigation pages have the elements of this table under these names in
every release that reads this format:

| Selector | Element |
| --- | --- |
| `#page-home` | The page: it holds everything below |
| `#search-container` | The search module |
| `#search` | The text field of the search |
| `#hero-container` | The module with the clock and the greeting |
| `#clock` | The clock |
| `#plugin-greetings` | The greeting |
| `#container-apps` | The application module, with its heading |
| `.app-item` | An application: the link |
| `.app-icon` | The icon of an application |
| `.app-title` | The name of an application |
| `.app-desc` | The description of an application |
| `#container-bookmarks` | The bookmark module, with its heading |
| `.bookmark-category-nav` | The links between the categories, in the [layouts](category-navigation.md) that have them |
| `.bookmark-group-container` | A group of bookmarks |
| `.bookmark-group-title` | The name of a group |
| `.bookmark-list` | The list of the bookmarks of a group |
| `.bookmark` | A bookmark: the link |
| `.toolbar-container` | The buttons for the settings and the help |
| `.footer-container` | The footer |

A module is on the page while the site shows it. Everything else of the pages
can change from one release to the next: other ids and class names, and how
the elements are ordered and nested. A theme whose stylesheet relies on more
than this table names the release it was written for with
[`requires`](theme-files.md#files-written-for-another-release) in its theme
file, and is then not used by an earlier one. The [style tokens](theme-tokens.md) are
the way to change a color, the corner radius, the font or the text size, and
keep working when the pages change.

## How it is served

- The address of the stylesheet is `/theme/<name>/<hash>/theme.css`. The hash
  is taken from the content of the file, so an edit has another address, and
  a browser may keep what it got from an address for good. An address of
  content the theme no longer has is not found.
- It is served as `text/css` in UTF-8, as the file is.
- It is for whoever the pages are for: anyone, or, on an instance with
  `FLARE_VISIBILITY=PRIVATE`, the signed-in administrator.
- The pages follow the stylesheet of the selected theme as they follow its
  theme file: each page load compares the state of `theme.css` too, and an
  edit shows on the next page load without a restart. See
  [changes while the instance runs](theme-files.md#changes-while-the-instance-runs).

## Verification

`go test ./config/model ./config/define ./internal/settings/... ./internal/server -run 'Stylesheet' -count=1`
covers the check, the files that are not accepted, the pages that link the
stylesheet, the address it is served at, and that the elements of the table
above are on the pages.
`FLARE_BINARY=/absolute/path/to/flare node tests/theme/stylesheet.browser.cjs`
selects a theme with a stylesheet in Chromium, edits and breaks the stylesheet
and checks the pages after each step. It also gives the instance, and the
browser unchecked, a list of stylesheets that name an image on another origin
in many spellings: every one the browser loads the image for has to be refused
by the instance, and the accepted ones are applied to a page without anything
being asked of the other origin.
