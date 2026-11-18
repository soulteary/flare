# The menu of the settings

> Availability: not in a release yet. This page describes the source after
> 0.7.0.

The settings have a menu of groups. Each group has a title and, under it,
its pages.

| Group | Entry | Address | What it is |
| --- | --- | --- | --- |
| Content | Apps and bookmarks | `/settings/content` | The [editor](editor.md) of the applications and the bookmarks |
| Content | Import and export | `/settings/transfer` | [Import and export](bookmark-transfer.md) of the content |
| Home page | Modules | `/settings/modules` | Which parts the home page shows: the title with the greeting, the applications, the bookmarks, the buttons for the settings and the help, and the version |
| Home page | Search | `/settings/search` | The search of the home page |
| Home page | Weather | `/settings/weather` | The weather of the home page |
| Home page | Clock | `/settings/clock` | Whether the home page shows the date and time, and the [clock](clock.md) |
| Home page | Links | `/settings/links` | Opening applications and bookmarks in a new window, encrypted links, the letter case of names, and where icons come from |
| Look | Theme | `/settings/theme` | Choosing a theme, with its preview; light and dark; the options of the selected theme; [custom colors](theme-colors.md) |
| Look | Layout | `/settings/layout` | The layout of a theme, the desktop columns, the bookmark layout, the category navigation, the phone and tablet layout, the text and title sizes, and the descriptions of applications |
| Look | Theme library | `/settings/themes` | The themes the instance brings itself, with their downloads, and the remarks about [theme files](theme-files.md) that are not used; importing a theme; [installing one from an address](theme-files.md#installing-from-an-address); the link to the [theme repository](theme-repository.md) |
| Look | Theme access | `/settings/theme-access` | Whether themes may use [templates](theme-templates.md) and the confirmation of those of the selected theme; the [themes a visitor can choose](theme-choices.md) |
| Site | Site details | `/settings/site` | The language of the interface, the title and the footer |
| Site | Account | `/settings/application` | Sign-in, sign-out and the [account](accounts.md) |
| Site | About | `/settings/about` | What the application is, its project page and its version |

- The entry of the page that is shown is marked.
- The two pages for the content stand in the same frame as the others, with
  the menu beside them and in the colors of the selected theme. Their column
  is wider, because they hold lists and tables where the other pages hold
  forms. An instance without the editor (`FLARE_EDITOR`) has no such group.
- Leaving the editor by the menu or by the way back asks first while there
  are changes that are not saved. The escape key, which leaves the other
  pages, does not leave the editor.
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

## The three pages about themes

The theme page held eleven forms. Choosing a theme stayed on it, with what
belongs to the selected theme. What is done now and then, and changes what
every visitor is shown or what the instance stores, stands on two pages of
its own.

- The theme page lists every theme, with its preview and its download:
  the bundled themes, the custom colors and the themes the instance brings
  itself.
- The theme library lists the themes the instance brings itself, those of
  the themes directory, and says that the others are on the theme page. A
  bundled theme is downloaded there, for example to import it under
  another name.
- Both say which theme files are not used and why. A theme is selected
  from either list; the theme page is shown afterwards.
- Each form submits to the page it stands on, and that page is shown again
  with what the form led to. A form sent to another of the three addresses
  is refused and stores nothing. The form that chooses a theme stands in
  the library as well, and is sent to the theme page from there.
- The theme page links to the two others.

## In a narrow window

In a window up to 768 pixels wide the menu is closed, above the page,
behind a switch that names the group and the page that is shown. The switch
opens and closes the menu. It is a checkbox with a label and needs no
script; the keyboard reaches it and switches it with the space bar. An
entry of the menu leads to its page with the menu closed again.

## Addresses

Addresses that are gone answer with `404`. Nothing leads on from them: a
bookmark or a script that names one has to be changed.

| Address in 0.7.0 and earlier | Now |
| --- | --- |
| `/editor`, the editor | `/settings/content` |
| `/editor/batch`, the editor with its table | `/settings/content/batch` |
| `/editor/transfer`, import and export | `/settings/transfer` |
| `/editor/document`, `/editor/csrf`, `/editor/icon`, `/editor/icons`, which the editor calls | The same names under `/settings/content/` |
| `/editor/transfer/preview`, `commit`, `export`, `backup`, which import and export call | The same names under `/settings/transfer/` |
| `/settings/appearance`, the page with the settings of five pages | `/settings/modules`, `/settings/clock`, `/settings/links`, `/settings/layout`, `/settings/site` |

| Address | Before | Now |
| --- | --- | --- |
| `/settings/application` | Sign-in or account; without accounts in a security store also what the application is | Sign-in or account, at the address sign-in leads to and comes back to. Without sign-in it leads to `/settings/about` |
| `/settings/about` | — | What the application is |

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
  field of another page; what a page refuses; the addresses that are gone,
  which answer with `404` and store nothing; the policy for scripts, which
  only the page of the clock has.
- `internal/server`, for the three pages about themes: the forms of each
  page and the address each submits to; that no form is on two pages apart
  from the one that chooses a theme; the cards and the remarks on the two
  pages that list themes; the links of the theme page; and for seven kinds
  of form, that each is taken at the address of its page, with that page
  shown afterwards, and refused at the two others.
- The browser scripts under `tests/theme`, in Chromium, with their forms on
  the pages they stand on now.
- `tests/settings/save.browser.cjs` and `tests/clock/browser.cjs`, in
  Chromium: the title and the layout saved by their own forms, directly and
  through a proxy; the settings of the clock on their page.
- For the two pages for the content: that they stand in the frame with
  their entry marked, in a wider column and in the colors of the theme,
  and stay inside a window 360 pixels wide (`tests/settings/menu.browser.cjs`);
  the scripts and unit tests of the editor under `tests/editor`, which
  pass unchanged apart from the fixture that serves the page without the
  frame.
- `tests/settings/menu.browser.cjs`, in Chromium: the menu beside the page
  in a wide window; the pages its entries lead to; in a window 360 pixels
  wide, the switch by a click and from the keyboard, with and without
  scripts, and entries at least 44 pixels high.
- Not covered: Firefox and WebKit, and a screen reader.
