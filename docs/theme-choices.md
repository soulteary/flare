# Themes a visitor can choose

> Availability: not in a release yet. This page describes the source after
> 0.7.0, where the list is set in `config.yml` and a choice is made with a
> form. The settings page does not offer the list, and the pages show no
> control for the choice.

The pages of an instance are shown with one theme, the selected one. The
administrator can name further themes, and a visitor can have the pages
shown with one of those instead. The choice is kept in the browser of the
visitor. Nothing about it is stored on the instance.

Read this page for:

- [Which themes can be chosen](#which-themes-can-be-chosen).
- [What a choice changes](#what-a-choice-changes), and what it does not.
- [How a choice is made](#how-a-choice-is-made): the form and the cookie.
- [Caches](#caches) between the instance and the visitor.

## Which themes can be chosen

`ThemeChoices` in `config.yml` lists the names of the themes, as `Theme`
names the selected one:

```yaml
Theme: blackboard
ThemeChoices:
  - forest
  - tron
```

| Entry of the list | What it means |
| --- | --- |
| A bundled theme, or a theme of the themes directory that is in use | It can be chosen |
| `custom` | The [custom colors](theme-colors.md) can be chosen, once colors are stored for them |
| The selected theme | It is what the pages show without a choice, and is skipped. It becomes a choice when another theme is selected |
| A name that is no theme in use | It is skipped, and can be chosen once a theme of that name is in use: a theme file that is added later, or one that is read again after [safe mode](theme-files.md#safe-mode) |
| A name twice | It counts once |

- Without the list, with an empty one, or with one that leaves nothing to
  choose, the pages are shown as before: no cookie is read and no response
  changes.
- The list is read with every page, so an edit of `config.yml` shows on the
  next page load.
- In safe mode for the instance, no theme of the themes directory can be
  chosen. The bundled themes and the custom colors can.

## What a choice changes

A visitor who chose a theme gets the home page, the application page, the
bookmark page and the help page as they are when that theme is selected:

| Part of the theme | With a choice |
| --- | --- |
| Colors and [tokens](theme-tokens.md), in light and dark | Those of the chosen theme, under the setting for light and dark of the site |
| [Stylesheet](theme-stylesheets.md), images, fonts and background | Those of the chosen theme |
| [Layout](theme-files.md#layout) | The layout the chosen theme specifies, as far as the administrator has not turned it off for that theme |
| Options of the theme | The values the administrator stored for that theme |
| [Templates](theme-templates.md) | Those of the chosen theme, if the site allows templates and those of that theme are confirmed. Otherwise the theme is shown with the markup of the application |

What a choice does not change:

- The settings pages and the editor, which are shown with the selected
  theme.
- The icon of the site and the icons a theme colors, which follow the
  selected theme.
- What a page holds. The entries, the search and what a visitor may see are
  the same under every theme.
- Any setting. A choice selects no theme and writes nothing to `config.yml`.

The templates of a theme are confirmed on the theme page while the theme is
selected. To let visitors choose a theme with its templates, select it once,
confirm its templates, and select the theme of the site again: the
confirmation is stored for the theme and stays.

A theme of the themes directory that is edited while the instance runs is
followed at once while it is the selected theme. As a choice it is read
again when the theme page is opened, when the themes directory is read for
another reason, and at a restart.

For the administrator, a [preview](theme-files.md#preview) and
[safe mode for one page](theme-files.md#safe-mode) come before a choice: a
response that is asked for with either shows what it names.

## How a choice is made

A choice is a form sent with `POST` to `/theme-choice`:

| Field | Value |
| --- | --- |
| `theme` | The name of a theme that can be chosen. Empty, or the name of the selected theme, gives the choice up |
| `back` | `/`, `/applications` or `/bookmarks`: the page the visitor is sent to afterwards. Anything else is read as `/` |

```html
<form method="POST" action="/theme-choice">
  <input type="hidden" name="back" value="/bookmarks">
  <button name="theme" value="forest">Forest</button>
  <button name="theme" value="">The theme of the site</button>
</form>
```

- The answer is `303` with the page to go to, and a cookie.
- A name that cannot be chosen is answered with `400`, and the cookie is
  left as it is.
- The address takes no `GET`: a link cannot make a choice.
- On a site that is shown to the administrator only, the form needs the
  session and its token like every other form.

The cookie:

| Property | Value |
| --- | --- |
| Name | `flare_theme` |
| Value | The name of the theme, taken from the list and not from the request |
| Lifetime | One year from the choice. Giving the choice up removes it |
| Attributes | `Path=/`, `HttpOnly`, `SameSite=Lax`, and `Secure` on a connection with TLS |

It is a preference. It identifies nobody, holds nothing but the name of a
theme, and is honored only for a name that can be chosen at the time of the
request: a cookie with any other value changes nothing, whoever wrote it.

## Caches

While there is a theme to choose, the same address shows different pages to
different visitors.

| Response | Header |
| --- | --- |
| Every page, while there is a theme to choose | `Vary: Cookie`, beside the `Vary: User-Agent` of the home page |
| A page shown with a chosen theme | `Cache-Control: private`. An instance that answers every page with `Cache-Control: private, no-store`, as one does that keeps its accounts in a security store, keeps that |
| The answer to the form | `Cache-Control: no-store` |

A proxy or a content delivery network that keeps pages of the instance has
to honor `Vary`, or leave the pages alone. One that keeps a page under its
address only would hand the page of one visitor to the next.

The instance keeps rendered lists of entries for a while. They are kept
apart by what the entries are written with, the item templates and the mode
of the category navigation among it, so a visitor never gets a list that
was rendered for the theme of another.

## Verification

- `internal/server`: the pages with and without a choice, for a theme file
  and a bundled theme; names that are no choice; the form, the cookie, the
  page a visitor is sent back to and the names it refuses; that nothing is
  selected or written; a list that leaves nothing to choose; a chosen theme
  with confirmed templates, with templates that are not confirmed, and after
  a confirmation is withdrawn; safe mode for the instance; a preview and
  safe mode for one page before a choice; a site shown to the administrator
  only.
- Not covered: a proxy between the instance and a visitor, and a browser
  that makes the choice, because the pages have no control for it yet.
