# The menu of the settings

> Availability: not in a release yet. This page describes the source after
> 0.7.0.

The settings have a menu of groups. Each group has a title and, under it,
its pages.

| Group | Entry | Address | What it is |
| --- | --- | --- | --- |
| Content | Apps and bookmarks | `/editor` | The editor. It is a page of its own, outside the settings |
| Content | Import and export | `/editor/transfer` | Import and export of the content, also a page of its own |
| Home page | Modules | `/settings/modules` | Which parts the home page shows: the title with the greeting, the applications, the bookmarks, the buttons for the settings and the help, and the version |
| Home page | Search | `/settings/search` | The search of the home page |
| Home page | Weather | `/settings/weather` | The weather of the home page |
| Home page | Clock | `/settings/clock` | Whether the home page shows the date and time, and the [clock](clock.md) |
| Home page | Links | `/settings/links` | Opening applications and bookmarks in a new window, encrypted links, the letter case of names, and where icons come from |
| Look | Theme | `/settings/theme` | [Themes and custom colors](theme-colors.md) |
| Look | Layout | `/settings/layout` | The layout of a theme, the desktop columns, the bookmark layout, the category navigation, the phone and tablet layout, the text and title sizes, and the descriptions of applications |
| Site | Site details | `/settings/site` | The language of the interface, the title and the footer |
| Site | Account | `/settings/application` | Sign-in, sign-out and the [account](accounts.md) |
| Site | About | `/settings/about` | What the application is, its project page and its version |

- The entry of the page that is shown is marked.
- The two entries of the content are marked with an arrow, because they
  leave the settings. An instance without the editor (`FLARE_EDITOR`) has no
  such group.
- An instance that needs no sign-in (`FLARE_DISABLE_LOGIN`) has no account
  and no entry for it. The address of the account leads to the page about
  the application there.
- The page about the application needs the session, like the other pages
  of the settings. The version is left out where the site hides it.

## One page, one subject

The five pages for the modules, the clock, the links, the layout and the
site details hold what one page, the appearance page, held before. No
setting was added or removed, and `config.yml` has the same keys.

- Each page has one form with the fields of its subject, and a button that
  saves them.
- Saving a page stores the fields of its form. A field the form does not
  have keeps its stored value: saving the modules changes nothing about the
  links or the layout.
- A text that is submitted empty is cleared. A text that is not submitted
  stays.
- The switch for the date and time is on the page of the clock, beside the
  settings it turns on. The page of the modules says so and links there.
- The field for the way icons are fetched is always shown. A configuration
  that names no way shows the default.

## In a narrow window

In a window up to 768 pixels wide the menu is closed, above the page,
behind a switch that names the group and the page that is shown. The switch
opens and closes the menu. It is a checkbox with a label and needs no
script; the keyboard reaches it and switches it with the space bar. An
entry of the menu leads to its page with the menu closed again.

## Addresses

No address of a page of the settings went away:

| Address | Before | Now |
| --- | --- | --- |
| `/settings/application` | Sign-in or account; without accounts in a security store also what the application is | Sign-in or account. Without sign-in it leads to `/settings/about` |
| `/settings/about` | — | What the application is |
| `/settings/appearance` | The page with the settings of five of the pages above | Leads to `/settings/layout`. A form of the page as an earlier release rendered it is still taken there |

The help page links to `/settings/about` for the version.

## Verification

- `internal/server`: the groups and entries of the menu on every page of
  the settings in both languages, with and without the editor and with and
  without sign-in; the marked entry; that every entry leads to a page; the
  page about the application, the address of the account without sign-in,
  and that the page needs the session on an instance with accounts.
- `internal/server`, for the five pages: the fields of each page and that
  none is on two; that a page stores what it submits; that saving a page
  as it is shown changes no setting, with every setting other than its
  default; single fields, a text that is cleared, and a form that names a
  field of another page; what a page refuses; the address of the appearance
  page and a whole form sent to it; the policy for scripts, which only the
  page of the clock has.
- `tests/settings/save.browser.cjs` and `tests/clock/browser.cjs`, in
  Chromium: the title and the layout saved by their own forms, directly and
  through a proxy; the settings of the clock on their page.
- `tests/settings/menu.browser.cjs`, in Chromium: the menu beside the page
  in a wide window; the pages its entries lead to; in a window 360 pixels
  wide, the switch by a click and from the keyboard, with and without
  scripts, and entries at least 44 pixels high.
- Not covered: Firefox and WebKit, and a screen reader.
