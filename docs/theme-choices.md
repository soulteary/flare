# Themes a visitor can choose

> Availability: not in a release yet. This page describes the source after
> 0.7.0.

The pages of an instance are shown with one theme, the selected one. The
administrator can name further themes, and a visitor can have the pages
shown with one of those instead. The choice is kept in the browser of the
visitor. Nothing about it is stored on the instance.

Read this page for:

- [Which themes can be chosen](#which-themes-can-be-chosen): the form of
  the page for theme access, and the list in `config.yml`.
- [The control of the pages](#the-control-of-the-pages) a visitor chooses
  with.
- [What a choice changes](#what-a-choice-changes), and what it does not.
- [How a choice is made](#how-a-choice-is-made): the form and the cookie.
- [Caches](#caches) between the instance and the visitor.

## Which themes can be chosen

Settings → Theme access (`/settings/theme-access`) has a form, **Themes
visitors can choose**, with a box for every theme that can be selected: the bundled themes, the custom colors once
colors are stored for them, and the themes of the themes directory. The
ticked themes are stored, and the pages offer them from the next page load.

- The selected theme has a box too, and the form says which it is. Ticking
  it changes nothing now. It keeps the theme among the choices when another
  theme is selected later.
- With no box ticked, an empty list is stored and the pages have no control.
- In safe mode for the instance the themes of the themes directory have no
  box. Those that were ticked before stay on the list when the form is
  saved, and can be chosen again when the instance runs without safe mode.

The form stores `ThemeChoices` in `config.yml`, which lists the names of the
themes, as `Theme` names the selected one. The themes are stored in the
order of the theme page. The list can be written by hand as well, in any
order, and that order is the one the control offers them in:

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

## The control of the pages

While there is a theme to choose, the home page, the application page, the
bookmark page and the help page have a control below the footer: a list
with a label, and a button.

| Part | What it is |
| --- | --- |
| The first entry of the list | The selected theme, named as the theme of the site. Choosing it gives a choice up |
| The other entries | The themes that can be chosen, in the order of the list, each under the name its theme file gives in the language of the interface |
| The marked entry | The theme the page is shown with |
| The button | Submits the choice. The visitor comes back to the same page, shown with the chosen theme |

- The control is a form and needs no script.
- It is part of the footer. A theme that brings a
  [template of a page](theme-templates.md) places the footer, which every
  template has to, and has the control with it.
- It is written in the colors of the theme the page is shown with. A
  stylesheet of a theme styles it as `form.theme-choice` with its `label`,
  `select` and `button`. A stylesheet that hides it leaves a visitor who
  chose that theme without the way back; the cookie can still be removed in
  the browser, and the administrator can take the theme off the list.
- A response that is a preview, or shown in safe mode for one page, has no
  control: it is not the page a visitor gets.
- The pages of the settings and the editor have no control.

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

The templates of a theme are confirmed under theme access while the theme is
selected. To let visitors choose a theme with its templates, select it once,
confirm its templates, and select the theme of the site again: the
confirmation is stored for the theme and stays.

A theme of the themes directory that is edited while the instance runs is
followed at once while it is the selected theme. As a choice it is read
again when a page of the theme settings is opened, when the themes directory is read for
another reason, and at a restart.

For the administrator, a [preview](theme-files.md#preview) and
[safe mode for one page](theme-files.md#safe-mode) come before a choice: a
response that is asked for with either shows what it names.

## How a choice is made

A choice is a form sent with `POST` to `/theme-choice`:

| Field | Value |
| --- | --- |
| `theme` | The name of a theme that can be chosen. Empty, or the name of the selected theme, gives the choice up |
| `back` | `/`, `/applications`, `/bookmarks` or `/help`: the page the visitor is sent to afterwards. Anything else is read as `/` |

The control of the pages is such a form. Any form of this shape makes a
choice, for example one with a button for each theme:

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
  session and its token like every other form. The control of the pages
  carries the token there. On a public site it carries none: the form is
  taken from anyone, and a page every visitor gets holds no token of a
  session.

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
- `internal/server`, for the control and the form under theme access: the
  control on the four pages with its entries, the marked one and the page it
  leads back to; a name of a theme with markup in it; pages without a
  choice; a page of a theme with templates; the token on a site shown to
  the administrator only; no control on a preview and in safe mode for one
  page; the boxes of the form, what it stores and in which order, an empty
  list, the custom colors, and safe mode for the instance.
- `tests/theme/choices.browser.cjs`, in Chromium: the administrator ticks
  themes; a visitor chooses on each of the four pages and comes back to it;
  the cookie and that no script reads it; a second visitor and the
  administrator keep the theme of the site; a theme with a stylesheet; a
  window 320 pixels wide; a browser without scripts; giving the choice up;
  a theme that is taken off the list, and the last one.
- Not covered: a proxy between the instance and a visitor; Firefox and
  WebKit; a screen reader.
