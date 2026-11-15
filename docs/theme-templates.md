# Theme templates

> Availability: not in a release yet. 0.7.0 does not read the templates of a
> theme.

A [theme that is a directory](theme-files.md#a-theme-as-a-directory) can bring
templates for the home page, the application page and the bookmark page. A
template decides where the parts of a page stand and what stands between
them. The parts themselves, such as the search form, the list of applications
and the footer, are rendered by the application.

Templates are turned off until the administrator allows them and confirms
those of the selected theme.

## The files

```text
themes/
    zelda/
        theme.yml
        theme.css
        templates/
            home.html
            applications.html
            bookmarks.html
```

- The templates are the files `home.html`, `applications.html` and
  `bookmarks.html` in the directory `templates` beside `theme.yml`. Other
  files of that directory are not read. A theme that is a single file has no
  templates.
- A template is a regular file of up to 64 KiB in UTF-8. A link is not
  followed.
- Each file is optional. The application page and the bookmark page take
  `home.html` from a theme that has no template of their own for them, and a
  theme without `home.html` leaves the home page to the application.
- A template that is [not accepted](#what-a-template-can-contain) counts as
  absent, and the theme stays in use: the theme page lists the file with the
  reason and what it is about. This differs from a stylesheet, which a theme
  is not used without.
- An archive of a theme does not hold its templates in this release.

## Turning templates on

Two things are needed, both on Settings → Theme:

1. **Allow themes to use templates.** One setting for the site, off by
   default. It is `ThemeTemplates` in `config.yml`.
2. **Confirm the templates of the selected theme.** Once the site allows
   templates, the page shows a form for the selected theme if it has
   templates. The form lists the template files, says what they can do and
   confirms them. It is `ThemeSettings.<theme>.Templates` in `config.yml`.

```yaml
ThemeTemplates: true
ThemeSettings:
    zelda:
        Templates: true
```

- The confirmation is kept per theme. Selecting another theme and coming
  back does not ask again. It can be withdrawn on the same form, and the
  setting for the site can be turned off; either way the pages have the
  markup of the application again on the next request.
- A confirmation is about the templates the form listed. If the theme has
  other template files by the time the form is submitted, nothing is stored
  and the form lists them again. A later edit of a confirmed theme's
  templates does not ask again.
- The card of a theme on the theme page says which templates the theme
  brings and whether they are in use, so this is known before the theme is
  selected.
- Both settings are read with every page. A change needs no restart.

### Looking before confirming

- The [preview](theme-files.md#preview) of a theme that is not selected
  shows its templates once the site allows templates, before they are
  confirmed.
- For the selected theme, the form has a link to the home page with the
  templates. That response says what it is, is shown to the administrator
  only, and is not stored by browsers or proxies. Visitors get the page
  without the templates until they are confirmed.

### Where no template is used

| Case | Effect |
| --- | --- |
| The instance runs with `FLARE_DISABLE_CSP` | No template of a theme is used, and the theme page says so in place of the setting. That a template runs no script is finally enforced by the Content-Security-Policy of the navigation pages; without it only the check below remains. Tokens, stylesheets and layout of themes are not affected |
| [Safe mode](theme-files.md#safe-mode) for the instance | The themes directory is not read, so there are no templates |
| Safe mode for one page | That response has the markup of the application |
| The help page and the guide | Always the markup of the application: the help page describes the default interface, and the script of the guide finds its elements by it |
| Settings, the editor, sign-in | Not pages of a theme |

## Writing a template

A template is written in the template language of Go, `html/template`. It
places each part of the page by the name of the part:

```html
<div class="shell">
  <aside class="side">
    {{template "flare/clock"}}
    {{template "flare/greeting"}}
    {{template "flare/weather"}}
  </aside>
  <main>
    {{template "flare/subpage-header"}}
    {{template "flare/search"}}
    {{template "flare/apps"}}
    {{template "flare/bookmarks"}}
  </main>
  {{template "flare/toolbar"}}
  {{template "flare/footer"}}
</div>
```

What a template writes stands inside the container of the page,
`div.container` in `#page-home`, in place of the markup of the application
between the parts. The head of the page, its stylesheets, the notice of a
preview and the scripts of the clock and the search stay with the
application.

### The parts

| Name | Part | `home.html` | `applications.html` | `bookmarks.html` |
| --- | --- | --- | --- | --- |
| `flare/search` | The search form | Required | Optional | Optional |
| `flare/clock` | Date and time | Required | Optional | Optional |
| `flare/greeting` | The title with the greeting | Required | Optional | Optional |
| `flare/weather` | The weather | Required | Optional | Optional |
| `flare/subpage-header` | Title and link back on the application page and the bookmark page | Required | Required | Required |
| `flare/apps` | The applications with their heading | Required | Required | Optional |
| `flare/bookmarks` | The bookmarks with their heading | Required | Optional | Required |
| `flare/toolbar` | The settings and help buttons | Required | Required | Required |
| `flare/footer` | The footer | Required | Required | Required |

- A required part is placed exactly once, and an optional part at most once.
- `home.html` is also the template of the two other pages for a theme that
  has none for them. Rendered for one of them, it has to place what the
  column of that page requires, so it can branch on `.Page.Kind` and leave
  out, for example, the search form there.
- What a page shows is for the administrator to decide. A part renders
  nothing while its module is turned off or the page does not have it, so a
  template neither needs to test for that nor can show a part that is
  turned off.
- A part is placed in the content of an element. It cannot be placed in the
  value of an attribute, and a template cannot take the markup of a part
  into a variable or a pipeline.
- A template cannot define a template whose name begins with `flare/`.
- The markup of a part is the application's. The selectors a
  [stylesheet can rely on](theme-stylesheets.md#what-a-stylesheet-can-rely-on)
  hold inside the parts. Rules of the application that depend on the markup
  between the parts, such as those for `#hero-container` and
  `#plugin-datetime` around the clock and the greeting, do not apply to a
  page with a template: the stylesheet of the theme lays out what its
  template writes.

### What a template is given

A template can show one value and branch on the others:

| Value | Meaning |
| --- | --- |
| `.Site.Title` | The title of the site. The only value a template can show; it is filled in as text when a page is rendered |
| `.Site.Lang` | The language of the interface, `zh` or `en` |
| `.Page.Kind` | `home`, `applications` or `bookmarks`: the page the template is rendered for. `home.html` is also rendered for the two other pages |
| `.Page.Searching` | Whether the page shows the results of a search |
| `.Modules.<part>` | Whether the page shows a part, by its name without `flare/`: `search`, `clock`, `greeting`, `weather`, `apps`, `bookmarks`, `toolbar`, `footer`, and `index .Modules "subpage-header"` |
| `.Layout.<option>` | The [layout](theme-files.md#layout) in effect, for the options a theme can specify, with the values a theme file writes them with: `MobileBookmarkLayout`, `TitleSize`, `HideAppDescriptions` (`true` or `false`), `CategoryNavigation`, `BookmarkLayout`, `DesktopColumns` (`legacy` for the layout of releases before the option) |
| `.Theme.Options.<id>` | The value of an [option](theme-files.md#options) of the theme. A boolean option has the text `true` or `false`: `{{if eq .Theme.Options.frames "true"}}` |

Two functions are available besides the built-in ones of the language:

| Function | Result |
| --- | --- |
| `T "<key>"` | A text of the interface in the language of the page, by its key in the language files |
| `asset "<name>"` | The address of an image among the [assets](theme-stylesheets.md#images-and-fonts) of the theme: `<img src="{{asset "mark.png"}}" alt="">`. A name that is no asset of the theme is an error |

A template is given nothing else: no application, no bookmark, no setting
other than those above, nothing of the request and nothing of the session.
The entries of a page reach it only inside the parts, which the application
renders after the template has run.

## What a template can contain

A template is not run for a request. It is run once for a set of values, with
a placeholder in place of the title, and what it writes is read as HTML,
checked and written out again as text with holes for the title and the
parts. A page is rendered by filling the holes. The check is therefore about
what the template writes, whichever way it is written.

- **Elements.** `a`, `abbr`, `address`, `article`, `aside`, `b`,
  `blockquote`, `br`, `caption`, `cite`, `code`, `col`, `colgroup`, `dd`,
  `del`, `details`, `dfn`, `div`, `dl`, `dt`, `em`, `figcaption`, `figure`,
  `footer`, `h1` to `h6`, `header`, `hgroup`, `hr`, `i`, `img`, `ins`, `kbd`,
  `li`, `main`, `mark`, `nav`, `ol`, `p`, `picture`, `pre`, `q`, `s`, `samp`,
  `section`, `small`, `source`, `span`, `strong`, `sub`, `summary`, `sup`,
  `table`, `tbody`, `td`, `tfoot`, `th`, `thead`, `time`, `tr`, `u`, `ul`,
  `var` and `wbr`. Among those that are not accepted are `script`, `style`,
  `iframe`, `object`, `embed`, `base`, `link`, `meta`, `form` and its
  controls, `svg`, `math`, `template`, `noscript` and custom elements.
  Styles belong in the stylesheet of the theme, and graphics among its
  assets.
- **Attributes.** On every element `class`, `title`, `lang`, `dir`,
  `hidden`, `role`, `tabindex`, `translate`, `aria-*` and `data-*`; and those
  that belong to an element, such as `href` on `a`, `src`, `srcset`, `sizes`,
  `alt`, `width` and `height` on `img`, `open` on `details`, `datetime` on
  `time`, and `colspan` and `rowspan` on a cell. `id`, `name`, `style` and
  every attribute that begins with `on` are not accepted: the scripts of the
  application find its own elements by `id`.
- **Links.** The address of a link is `http:`, `https:` or `mailto:`, or has
  no scheme. A link gets `rel="noopener noreferrer"` whatever the template
  writes, and its `target` can be `_blank` or `_self`.
- **Images.** `src` and `srcset` name assets of the theme through `asset`.
  Nothing is loaded from elsewhere.
- **The title.** `.Site.Title` can stand in the content of an element and in
  the attributes `title`, `alt`, `aria-label` and `data-*`. A template cannot
  compute with it: what a template makes of the placeholder is not the title.
- **Structure.** Every element is closed, in order. Comments are dropped,
  and a doctype is not accepted.
- **Size.** What a template writes is at most 256 KiB.

A template is checked when the themes directory is read, for a page with
every part shown and the options of the theme at their defaults, and the
result is on the theme page. It is compiled again for each set of values a
page is rendered with, once, and the result is kept. If it is not accepted
for some set, for example because a branch for a value of an option writes
an element that is not accepted, pages with those values have the markup of
the application, the log has a warning, and the theme page lists the file
with the reason.

### What the check does not cover

- A template can write any text and any link, including links to other
  sites. Confirming the templates of a theme is trusting it with that.
- A template can leave a page hard to use, for example by hiding a part with
  the `hidden` attribute on an element around it. [Safe mode](theme-files.md#safe-mode)
  shows the page without the theme, and withdrawing the confirmation shows
  it without the templates.

## Changes while the instance runs

The templates of the selected theme are followed like its other files: one
that is added, edited or removed is read by the next page. See
[changes while the instance runs](theme-files.md#changes-while-the-instance-runs).

## Verification

Covered by Go tests:

- Settings: the setting for the site and the confirmation for a theme are
  stored in `config.yml` and can each be taken back; a name that is no theme
  name and an unreadable file are refused.
- Use: a template is used only when the site allows templates and those of
  the theme are confirmed; a preview comes before the confirmation; no
  template is used with `FLARE_DISABLE_CSP` or in safe mode.
- Compilation: a template is compiled once for each set of values and kept,
  up to a limit; each value it can branch on is part of the set; an edited
  template is compiled anew; a template that is not accepted for some values
  leaves the page to the application and is listed until it is edited.
- Rendering: the parts are those the application renders, in the places the
  template gives them; the title is escaped; the page around the template is
  byte-identical to the page without one.
- Over HTTP: the whole way from allowing templates over confirming them to
  the three pages rendered with a template, each of the two taken back, an
  edit while the instance runs, the help page, safe mode and the preview;
  what a template is told about the page for two sets of settings.
- Writes of the settings by a session that was revoked after the request was
  let in store nothing.
