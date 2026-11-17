# The menu of the settings

> Availability: not in a release yet. This page describes the source after
> 0.7.0.

The settings have a menu of groups. Each group has a title and, under it,
its pages.

| Group | Entry | Address | What it is |
| --- | --- | --- | --- |
| Content | Apps and bookmarks | `/editor` | The editor. It is a page of its own, outside the settings |
| Content | Import and export | `/editor/transfer` | Import and export of the content, also a page of its own |
| Home page | Search | `/settings/search` | The search of the home page |
| Home page | Weather | `/settings/weather` | The weather of the home page |
| Look | Theme | `/settings/theme` | [Themes and custom colors](theme-colors.md) |
| Look | Appearance | `/settings/appearance` | Language, title, footer, links, modules and layout |
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

The help page links to `/settings/about` for the version.

## Verification

- `internal/server`: the groups and entries of the menu on every page of
  the settings in both languages, with and without the editor and with and
  without sign-in; the marked entry; that every entry leads to a page; the
  page about the application, the address of the account without sign-in,
  and that the page needs the session on an instance with accounts.
- `tests/settings/menu.browser.cjs`, in Chromium: the menu beside the page
  in a wide window; the pages its entries lead to; in a window 360 pixels
  wide, the switch by a click and from the keyboard, with and without
  scripts, and entries at least 44 pixels high.
- Not covered: Firefox and WebKit, and a screen reader.
