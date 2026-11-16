# Theme templates

> Availability: not in a release yet. 0.7.0 does not read the templates of a
> theme.

A [theme that is a directory](theme-files.md#a-theme-as-a-directory) can bring
templates of two kinds:

- **Page templates** for the home page, the application page and the
  bookmark page. A page template decides where the parts of a page stand and
  what stands between them. The parts themselves, such as the search form,
  the list of applications and the footer, are rendered by the application.
- **Item templates** for an application, a bookmark and a group of
  bookmarks. An [item template](#item-templates) decides the markup of one
  entry. Its name, its address and its icon are filled in by the application.

Templates are turned off until the administrator allows them and confirms
those of the selected theme.

This page is the reference. [Writing a theme with templates](theme-template-guide.md)
walks through a complete theme, says what to change for each reason a
template is not used, and has a list to go through before a theme is passed
on.

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
            app.html
            bookmark.html
            group.html
```

- The templates are the files `home.html`, `applications.html` and
  `bookmarks.html` for the pages, and `app.html`, `bookmark.html` and
  `group.html` for the items, in the directory `templates` beside
  `theme.yml`. Other files of that directory are not read. A theme that is a
  single file brings no templates; it can have those of
  [a theme it is based on](#a-theme-based-on-another).
- A template is a regular file of up to 64 KiB in UTF-8. A link is not
  followed.
- Each file is optional. The application page and the bookmark page take
  `home.html` from a theme that has no template of their own for them, and a
  theme without `home.html` leaves the home page to the application. What a
  theme has no item template for is written with the markup of the
  application.
- A template that is [not accepted](#what-a-template-can-contain) counts as
  absent, and the theme stays in use: the theme page lists the file with the
  reason and what it is about. This differs from a stylesheet, which a theme
  is not used without.
- An [archive](#archives) of a theme holds its templates.

## A theme based on another

A theme with [`extends`](theme-files.md#a-theme-based-on-another) is shown
with the templates of the themes below it for what it brings no template
for. The themes are looked at one after another, the theme itself first and
then each theme below it, the nearest first:

| For | The template is |
| --- | --- |
| The home page | `home.html` of the first theme that has it |
| The application page | Of the first theme that has `applications.html` or `home.html`: its `applications.html`, and its `home.html` otherwise |
| The bookmark page | The same with `bookmarks.html` |
| An application, a bookmark, a group | `app.html`, `bookmark.html`, `group.html` of the first theme that has it, each by itself |

Where no theme has a template, the markup of the application is used.

```text
themes/
    zelda/
        theme.yml
        templates/
            home.html
            applications.html
            app.html
    night/
        theme.yml            extends: zelda
        templates/
            applications.html
            bookmark.html
```

Here `night` is shown with its own `applications.html` and `bookmark.html`,
and with `home.html` and `app.html` of `zelda`. Its bookmark page takes
`home.html` of `zelda`. The `applications.html` of `zelda` is not used for
`night`.

- A theme that brings `home.html` uses none of the page templates below it:
  its `home.html` stands in for its other pages first.
- A template is compiled for the theme the page is shown with: `.Theme.Options`
  holds the options of that theme with the values chosen for it.
- `asset` names an asset of the theme that brings the template. A template
  of `night` cannot name an image among the assets of `zelda`, and a
  template of `zelda` that is used for `night` still names the assets of
  `zelda`.
- A template that is [not accepted](#what-a-template-can-contain) counts as
  absent here as well: what it was for is looked for in the themes below. A
  template of a theme below is checked again with the options of the theme
  above where it is used for that theme, and is not looked at where the
  theme above replaces it.
- The theme page lists a template of a theme below with the name of that
  theme, as `zelda/home.html`. The confirmation of a theme is about all the
  templates it is shown with. It is kept for that theme alone: confirming
  `night` does not confirm `zelda`, and `night` needs its own confirmation
  where `zelda` is confirmed.
- A bundled theme brings no templates.

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
| The help page and the guide | Always the markup of the application, for the page and for the entries: the help page describes the default interface, and the script of the guide finds its elements by it |
| The bar that selects a category | Where categories are selected one at a time, the bar is the application's, above the group of the theme |
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

## Item templates

```html
<!-- templates/app.html -->
<div class="tile">
  <a href="{{.Href}}" title="{{.Name}}">
    {{if .Has.Icon}}{{template "flare/icon"}}{{else}}<span class="letter">{{.Initial}}</span>{{end}}
    <strong>{{.Name}}</strong>
    {{if .Has.Description}}<small>{{.Description}}</small>{{else}}<small>{{.Host}}</small>{{end}}
  </a>
</div>
```

```html
<!-- templates/bookmark.html -->
<li class="mark"><a href="{{.Href}}">{{template "flare/icon"}}<span>{{.Name}}</span></a></li>
```

```html
<!-- templates/group.html -->
{{if .Collapsible}}
<details class="pack"><summary>{{.Name}}</summary><ul>{{template "flare/items"}}</ul></details>
{{else}}
<section class="pack">{{if .Has.Name}}<h3>{{.Name}}</h3>{{end}}<ul>{{template "flare/items"}}</ul></section>
{{end}}
```

| File | For | Can show | Can branch on |
| --- | --- | --- | --- |
| `app.html` | An application | `.Name`, `.Description`, `.Host`, `.Initial`, the address as `.Href`, the icon as `{{template "flare/icon"}}` | `.Has.Icon`, `.Has.Description`, `.NewTab` |
| `bookmark.html` | A bookmark | `.Name`, `.Host`, `.Initial`, `.Href`, the icon | `.Has.Icon`, `.NewTab` |
| `group.html` | A group of bookmarks: a category with its bookmarks | `.Name`, the bookmarks as `{{template "flare/items"}}` | `.Has.Name`, `.Collapsible` |

Every item template can also branch on `.Site.Lang` and
`.Theme.Options.<id>` and call `T` and `asset`, as a page template can.

**What is filled in**

| Field | Value |
| --- | --- |
| `.Name` | The name of the entry or of the category |
| `.Description` | The description of an application, and nothing where `.Has.Description` is false |
| `.Host` | The host the address leads to, without the port, for an `http` or `https` address; nothing for any other |
| `.Initial` | The first character of the name, as it is written |
| `.Href` | The address the application links to. With encoded links turned on it is the encoded address |
| `flare/icon` | The icon as the application writes it: an `svg` that refers to a bundled icon, or an `img` for an icon given as an address. Nothing for an entry without an icon |
| `flare/items` | The bookmarks of the group, each an `li` |

- The fields are filled in after the template has run, as text: a name is
  escaped wherever it stands. A template cannot compute with a field or
  branch on it. It can place it in the content of an element and in the
  attributes `title`, `alt`, `aria-label` and `data-*`.
- `.Href` is the whole `href` of one link and can stand nowhere else. The
  link gets `rel` and `target` from the application, by the setting for
  opening entries in a new window; what the template writes for them is
  dropped.

**What a template has to hold**

- **An application and a bookmark** have exactly one link to the entry, and
  show `.Name` in the content of an element. An entry can then be opened and
  is read out under every theme.
- **A bookmark** is one `li`, with everything else inside it.
- **A group** places `flare/items` once, directly in a `ul` or an `ol`. A
  bookmark is an item of a list whichever of the two templates a theme
  brings, so each fits the markup of the application for the other.
- **A group shows `.Name`** in the content of an element where it has one.
  The bookmarks of a site without categories are one group without a name,
  and `.Has.Name` is false for it.
- **A group that is opened and closed**, where `.Collapsible` is true, is
  written with `details` and `summary`, which open and close without script.
  The setting for collapsed categories does not collapse the results of a
  search, so `.Collapsible` is false there.
- An item template writes at most 64 KiB.

**What the application decides**

- **Hidden descriptions.** With the layout option that hides the
  descriptions of applications, `.Has.Description` is false and
  `.Description` is empty for every application.
- **An application without a description.** The markup of the application
  shows the address in its place. A template is given `.Host` for that.
- **Icons.** `.Has.Icon` is false for an entry without an icon. The size of
  an icon is for the stylesheet of the theme: the rules of the application
  for icons select by its own markup around them.
- **Layout options.** Options that act on the markup of the entries, such as
  the bookmark layouts and the columns, select by the classes of the
  application and have no effect on entries a template writes. A theme says
  which options work with its templates; see
  [layout options and templates](#layout-options-and-templates).

An item template is checked for every combination of what an entry has:
eight for an application, four for a bookmark and three for a group. One that
is not accepted for any of them is not used at all.

## Archives

The [archive](theme-files.md#archives) of a theme holds its templates, and
an import stores them.

- **Download.** The archive holds the template files as a reading of the
  directory reads them: the six names, regular files of up to 64 KiB in
  UTF-8. A template that is not accepted is in the archive as well. Other
  files of `templates` are not.
- **Import.** The six files of `templates` are stored. Every other entry of
  that directory is left out and named on the page, like any entry that is
  no file of a theme.
- **A template that cannot be read**, because it is larger than 64 KiB or
  not UTF-8, is a reason not to take the archive in, with the file named.
- **A template that is not accepted** is stored with the theme, as it could
  be written into the directory. The theme is in use without it and the
  theme page lists it. Whether a template is accepted can depend on the
  themes below, which an archive does not hold.
- **An import asks for the confirmation again.** What was confirmed are the
  templates that were there. An import of a theme, as an archive or as a
  theme file, withdraws the confirmation of that theme and of every theme
  that is based on it, and the page names them. The pages have the markup of
  the application until the templates are confirmed again. The
  confirmations of other themes stay.
- A theme of a name that was confirmed earlier and removed since is asked
  for again in the same way when it is imported.
- A release up to 0.7.0 leaves the templates of an archive out and names
  them among the entries that are no files of a theme.

## Layout options and templates

The layout options of the appearance settings are made for the markup of
the application. With the templates of a theme, some of them have no effect
any more. A theme lists the options that work with its templates under
`supports` in `theme.yml`, by the names they have under
[`layout`](theme-files.md#layout):

```yaml
contract: 1
tokens:
  background: "#1b2a22"
  primary: "#f2f2f2"
  accent: "#8fd19e"
supports:
  - TitleSize
  - CategoryNavigation
  - HideAppDescriptions
```

While the templates of the selected theme are in use, the appearance page
says of every other layout option that the theme does not support it. The
names are `MobileBookmarkLayout`, `TitleSize`, `HideAppDescriptions`,
`CategoryNavigation`, `BookmarkLayout` and `DesktopColumns`.

- **Nothing is turned off.** The remark is information for the
  administrator. The control stays, the setting is stored and handed to the
  templates as before, and it applies with another theme. Whether an option
  has an effect is decided by the templates and the stylesheet of the theme.
- **A theme that does not say.** A theme without `supports` has no such
  remark for any option. `supports: []` names none, so every option has the
  remark.
- **Options the theme sets.** An option the theme gives a value under
  `layout`, or through one of its [options](theme-files.md#options), counts
  as supported and has the remark about that value.
- **Only with templates in use.** Where the pages have the markup of the
  application, every option works and nothing is remarked: templates turned
  off for the site, not confirmed for the theme, or not used by the
  instance.
- **On the theme page.** The card of a theme with templates names the
  options its templates do not support, before the theme is selected.
- **A theme based on another** has the list of its file, or else that of the
  theme below, whose templates it has where it brings none.
- A name this release does not have is left out and listed on the theme
  page. A `supports` that is no list is not accepted, and the theme is not
  used.
- What the application decides for an item template is listed
  [above](#item-templates): hidden descriptions and the modes of the
  categories reach a template as values, so a theme that uses them can list
  `HideAppDescriptions` and `CategoryNavigation`.

## What a template can contain

A template is not run for a request. It is run once for a set of values, with
placeholders in place of the title of the site or the fields of an entry, and
what it writes is read as HTML, checked and written out again as text with
holes for the fields and the parts. A page and an entry are rendered by
filling the holes. The check is therefore about what the template writes,
whichever way it is written.

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
- **Fields.** `.Site.Title` and the fields of an entry can stand in the
  content of an element and in the attributes `title`, `alt`, `aria-label`
  and `data-*`. A template cannot compute with one: what a template makes of
  a placeholder is not the value.
- **Structure.** Every element is closed, in order. Comments are dropped,
  and a doctype is not accepted.
- **Size.** What a page template writes is at most 256 KiB, and an item
  template 64 KiB.

A template is checked when the themes directory is read, with the options of
the theme at their defaults, and the result is on the theme page: a page
template for a page with every part shown, an item template for every
combination of what an entry has. It is compiled again for each set of values
a page is rendered with, once, and the result is kept. If it is not accepted
for some set, for example because a branch for a value of an option writes
an element that is not accepted, pages and entries with those values have the
markup of the application, the log has a warning, and the theme page lists
the file with the reason.

### What the check does not cover

- A template can write any text and any link, including links to other
  sites. Confirming the templates of a theme is trusting it with that.
- A template can leave a page hard to use, for example by hiding a part with
  the `hidden` attribute on an element around it. [Safe mode](theme-files.md#safe-mode)
  shows the page without the theme, and withdrawing the confirmation shows
  it without the templates.

## Changes while the instance runs

The templates of the selected theme are followed like its other files: one
that is added, edited or removed is read by the next page. So are the
templates of the themes below it. See
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
- Item templates: what each kind renders for its sets of values; templates
  that are refused, each with its reason; a theme that brings one, two or all
  three; the entries over HTTP with names that need escaping, both settings
  for a new window, hidden descriptions, an option of the theme, every mode
  of the categories, a site without categories, encoded links and a search
  with and without results; the help page, safe mode, a preview and visitors.
- A theme based on another: what each theme of a chain is shown with for
  every page and item; the assets of a template below; whose confirmation
  counts; templates that are not accepted in the theme, in the theme below,
  and only with the options of the theme above.
- Layout options: the ways to write `supports`, names of another release,
  what a theme on another has; the remarks of the appearance page with
  templates turned off, allowed, confirmed and withdrawn.
- Archives: an archive with the six templates and other entries of that
  directory; templates that are not accepted and templates that cannot be
  read; a download taken in again as the same theme; an import over a
  confirmed theme with a theme above it.
- The guide: the example of [the guide](theme-template-guide.md) is read
  from that page and accepted, for each value of its option.

In a browser:

`FLARE_BINARY=/absolute/path/to/flare node tests/theme/templates.browser.cjs`
goes the way of the administrator with the example theme of the guide:

- The theme page lists the templates of the theme and the two that are not
  accepted, one with a `script` and one with an `onclick`.
- Templates are allowed for the site, previewed, and confirmed for the
  theme; each step is checked on the pages.
- The home page has the parts where the template places them and the
  entries as the item templates write them, with a name that needs
  escaping, an entry without an icon and one without a description. The
  stylesheet of the theme lays them out.
- The clock runs, a group is opened and closed, and a search shows the
  entries that match, inside the page of the theme.
- The application page and the bookmark page take the template of the home
  page. Nothing of the templates that are not accepted runs.
- The option of the theme changes the page, the appearance page names the
  layout options that are not supported, and safe mode shows the page of
  the application.
- An edit of a template is read by the next page.
- A theme based on the theme is confirmed by itself and shown with the
  templates of both.
- The download holds the templates. Imported over the theme, it asks for
  the confirmation of both themes again.
- Withdrawing the confirmation, and turning the setting off, each give the
  pages of the application.
- No request leaves the instance, and the pages log no error.

The script runs in Chromium. Other browsers and real devices are not
covered by it.
