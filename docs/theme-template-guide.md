# Writing a theme with templates

This is a walk through one theme with templates, from an empty directory to
pages that are rendered with it. Every rule it uses is stated in full in
[theme templates](theme-templates.md), which is the reference; this page
shows the rules at work and says where to look when something is not
accepted.

Templates are read by releases after 0.7.0.

Read this page for:

- [The example](#the-example): the six files of a complete theme.
- [Trying it](#trying-it): how the theme gets onto the pages.
- [Working on a template](#working-on-a-template): the loop of editing and
  looking, and how to get back when a page is broken.
- [What the application keeps](#what-the-application-keeps): what a
  template cannot change, and what it has to lay out itself.
- [When a template is not used](#when-a-template-is-not-used): each reason
  the theme page gives, with what to change.
- [Before passing a theme on](#before-passing-a-theme-on): a list to go
  through.

## The example

The theme is named `harbor`. It puts the clock, the greeting and the weather
in a column beside the lists, shows applications as tiles and bookmarks as
plain lists, and has one option that turns the column off.

```text
themes/
    harbor/
        theme.yml
        theme.css
        templates/
            home.html
            app.html
            bookmark.html
            group.html
```

The blocks below are the whole theme. A test of the application reads them
from this page and checks that every template is accepted, and the browser
acceptance script for templates renders pages with them, so they are kept
working.

### The theme file

**`themes/harbor/theme.yml`**

```yaml
contract: 1
name: Harbor
version: 1.0.0
tokens:
  background: "#10241b"
  primary: "#e8f1e4"
  accent: "#f2b84b"
  radius: 10px
options:
  - id: column
    type: boolean
    default: true
    label: { en: Side column, zh: 侧栏 }
supports:
  - TitleSize
  - CategoryNavigation
  - HideAppDescriptions
```

- The tokens are required of every theme; see [theme files](theme-files.md).
- The option is the theme's own setting. The administrator chooses its
  value on the theme page, and the templates below branch on it.
- `supports` lists the layout options of the appearance settings that still
  work with these templates. The appearance page says of the others that
  the theme does not support them; see
  [layout options and templates](theme-templates.md#layout-options-and-templates).

### The page

**`themes/harbor/templates/home.html`**

```html
<div class="harbor{{if eq .Theme.Options.column "true"}} harbor-with-column{{end}}" data-page="{{.Page.Kind}}">
  {{if eq .Page.Kind "home"}}
  <header class="harbor-head">
    <p class="harbor-site">{{.Site.Title}}</p>
    {{template "flare/search"}}
  </header>
  {{end}}
  {{template "flare/subpage-header"}}
  <div class="harbor-body">
    {{if eq .Page.Kind "home"}}
    <aside class="harbor-column">
      {{template "flare/clock"}}
      {{template "flare/greeting"}}
      {{template "flare/weather"}}
    </aside>
    {{end}}
    <main class="harbor-lists">
      {{if ne .Page.Kind "bookmarks"}}{{template "flare/apps"}}{{end}}
      {{if ne .Page.Kind "applications"}}{{template "flare/bookmarks"}}{{end}}
    </main>
  </div>
  {{template "flare/toolbar"}}
  {{template "flare/footer"}}
</div>
```

- A page template places the parts of the page by name. The parts
  themselves are rendered by the application.
- This theme has no `applications.html` and no `bookmarks.html`, so
  `home.html` is also the template of those two pages. It branches on
  `.Page.Kind` for that: the search form, the clock, the greeting and the
  weather are placed on the home page only, and each of the other pages
  gets the list it is for. Which parts a page requires is in the table of
  [the parts](theme-templates.md#the-parts).
- A part renders nothing while the administrator has turned it off, so the
  template does not test for that.
- `.Site.Title` is the one value a page template can show.
- The class on the first element carries the value of the option to the
  stylesheet. A template can branch on a value; it cannot write a `style`
  attribute.

### An application

**`themes/harbor/templates/app.html`**

```html
<div class="harbor-tile">
  <a href="{{.Href}}" title="{{.Name}}">
    {{if .Has.Icon}}<span class="harbor-icon">{{template "flare/icon"}}</span>{{else}}<span class="harbor-letter">{{.Initial}}</span>{{end}}
    <strong>{{.Name}}</strong>
    {{if .Has.Description}}<small>{{.Description}}</small>{{else}}<small>{{.Host}}</small>{{end}}
  </a>
</div>
```

- An item template writes one entry. It holds exactly one link to
  `{{.Href}}`, and shows `{{.Name}}` as text.
- `.Has.Icon` and `.Has.Description` say what the entry has, so the
  template can show the first character of the name in place of a missing
  icon, and the host in place of a missing description.
- The application decides where the link opens and writes `rel` and
  `target` itself.

### A bookmark and a group

**`themes/harbor/templates/bookmark.html`**

```html
<li class="harbor-mark"><a href="{{.Href}}">{{if .Has.Icon}}{{template "flare/icon"}}{{end}}<span>{{.Name}}</span></a></li>
```

**`themes/harbor/templates/group.html`**

```html
{{if .Collapsible}}
<details class="harbor-group" open><summary>{{.Name}}</summary><ul>{{template "flare/items"}}</ul></details>
{{else}}
<section class="harbor-group">{{if .Has.Name}}<h3>{{.Name}}</h3>{{end}}<ul>{{template "flare/items"}}</ul></section>
{{end}}
```

- A bookmark is one `li`, because the bookmarks of a group are the items of
  a list, whichever of the two templates a theme brings.
- A group places its bookmarks with `{{template "flare/items"}}` directly in
  a `ul` or an `ol`.
- Where the categories are opened and closed, `.Collapsible` is true and
  the group has to be a `details` with its name in the `summary`. The
  browser opens and closes it without a script.
- A site without categories has one group without a name: `.Has.Name` is
  false there.

### The stylesheet

**`themes/harbor/theme.css`**

```css
.harbor { display: grid; gap: 24px; padding: 24px 0; }
.harbor-head { display: flex; flex-wrap: wrap; align-items: center; gap: 16px; }
.harbor-site { margin: 0; font-weight: 700; color: var(--flare-accent); }
.harbor-body { display: grid; gap: 24px; }
.harbor-column { display: none; }
.harbor-with-column .harbor-column { display: block; }
.harbor-tile { display: inline-block; width: 220px; margin: 0 12px 12px 0; vertical-align: top; }
.harbor-tile a { display: grid; gap: 4px; padding: 12px; border: 1px solid var(--flare-border); border-radius: var(--flare-radius); color: var(--flare-primary); text-decoration: none; }
.harbor-tile a:hover, .harbor-tile a:focus-visible { background: var(--flare-hover); }
.harbor-tile small { color: var(--flare-muted); overflow-wrap: anywhere; }
.harbor-icon svg, .harbor-icon img { width: 32px; height: 32px; }
.harbor-letter { display: inline-grid; place-items: center; width: 32px; height: 32px; border-radius: 50%; background: var(--flare-accent); color: var(--flare-background); font-weight: 700; }
.harbor-group ul { margin: 8px 0 16px; padding: 0; list-style: none; }
.harbor-group summary, .harbor-group h3 { margin: 0; font-size: 1rem; color: var(--flare-accent); }
.harbor-mark a { display: flex; align-items: center; gap: 8px; padding: 6px 0; color: var(--flare-primary); text-decoration: none; }
.harbor-mark svg, .harbor-mark img { width: 18px; height: 18px; }
@media (min-width: 900px) {
  .harbor-with-column .harbor-body { grid-template-columns: 260px minmax(0, 1fr); }
}
```

- The stylesheet lays out what the templates write. It uses the tokens of
  the theme through the custom properties, so the option values and the
  light and dark values of a theme keep working; see
  [theme stylesheets](theme-stylesheets.md).
- The rules for icons are needed: the application sizes icons by its own
  markup around them, which an item template replaces.
- The column is shown through the class the template writes for the option.

## Trying it

1. Put the directory into `themes/` beside `config.yml`. The theme is listed
   on Settings → Theme without a restart. Its card says which templates it
   brings.
2. Turn on **Allow themes to use templates** on that page. This is one
   setting for the site, off by default.
3. Look first: the preview of a theme that is not selected shows its
   templates before anything is confirmed. Add `?theme-preview=harbor` to
   the address of the home page, or use the preview link of the card.
4. Select the theme. The page then has a form that lists the template files
   and confirms them. Until it is submitted, visitors get the pages without
   the templates.
5. Open the home page, the application page and the bookmark page.

Templates are not used on an instance that runs with `FLARE_DISABLE_CSP`,
and the theme page says so in place of the setting. See
[where no template is used](theme-templates.md#where-no-template-is-used).

## Working on a template

- **Edit and reload.** The templates of the selected theme are read again
  by the next page after a file is added, edited or removed. No restart and
  no new confirmation is needed for an edit in the directory.
- **Read the theme page.** A template that is not accepted is listed there
  with its file, the reason and what the reason is about, such as the name
  of an element. The theme stays in use without that template.
- **A template can pass the check and fail for a setting.** The check runs
  with the options of the theme at their defaults. A branch for another
  value is compiled when a page is first rendered with it. If it is not
  accepted, that page has the markup of the application, the log has a
  warning, and the theme page lists the file. Try every value of every
  option of the theme.
- **Try what an entry can have.** An application with and without an icon
  and a description, a site with and without categories, each mode of the
  category navigation, a search with results and without.
- **Getting back.** A template can leave a page hard to use. Adding
  `?theme-safe-mode` to the address shows that page without the theme, and
  withdrawing the confirmation on the theme page shows every page without
  the templates. See [safe mode](theme-files.md#safe-mode).

## What the application keeps

A template arranges the page. It does not replace the application:

| Kept by the application | What that means for a theme |
| --- | --- |
| The parts: the search form, the clock, the greeting, the weather, the heading and container of each list, the toolbar, the footer | Their markup is not the theme's. A stylesheet can rely on the selectors listed under [what a stylesheet can rely on](theme-stylesheets.md#what-a-stylesheet-can-rely-on), inside the parts |
| Scripts | A template has none. The clock and the search keep their scripts, which find their elements by `id`; this is why a template cannot write an `id` |
| The address of an entry, where it opens, `rel` | A template places `{{.Href}}` and nothing else about the link |
| Which parts and entries are shown | Decided by the settings and by the search. A template is not told about an entry it is not rendering |
| The bar that selects a category, the help page, the guide, the settings | Always the markup of the application |

What a theme with templates has to do itself:

- **Lay out what stands between the parts.** With a page template, the
  elements the application writes around the clock, the greeting and the
  weather are not there: `#hero-container` and `#plugin-datetime`. The rules
  of the application that select through them do not apply, which are the
  row of the clock, the greeting and the weather, their line height, and
  the position of the weather values. The parts themselves, such as `#clock`
  and `#plugin-greetings`, are there.
- **Size the icons** of entries an item template writes.
- **Lay out the entries.** The layout options that select by the classes of
  the application, such as the number of columns and the bookmark layouts,
  have no effect on entries a template writes. List under `supports` what
  still works.

## When a template is not used

The theme page names the file and one of these reasons, after "The template
is not used:" for all but the two that begin with "Not read". The subject
it shows with the reason is in the last column.

| Reason on the theme page | What to change | Shown with it |
| --- | --- | --- |
| It is not UTF-8 text | Save the file as UTF-8 | |
| Not read: larger than 64 KiB | Keep the file within 64 KiB | |
| Not read: a link is not followed | Put the file itself into `templates` | |
| It is not written as a template | Correct the syntax of the template language, such as a missing `{{end}}` | The message of the parser |
| It could not be run | The template calls something that fails, such as `asset` with a name that is no asset of the theme | The message |
| It defines a name that begins with `flare/`, which names the parts of the application | Give a template of your own another name | The name |
| It writes more than a template can, in bytes | A page writes up to 256 KiB and an item up to 64 KiB | The size |
| Markup that is not read | Write well-formed HTML, without a doctype | |
| An element a template cannot hold | Use the [elements that are accepted](theme-templates.md#what-a-template-can-contain). There is no `script`, `style`, `form`, `svg` or `iframe` | The element |
| An attribute, or a value of one, that a template cannot hold | Remove `id`, `name`, `style` and attributes that begin with `on`; move styles to `theme.css` | The attribute |
| An element is not closed where it was opened | Close every element in order, in every branch | The element |
| A field stands where it cannot be filled in safely | Place a field in the content of an element, or in `title`, `alt`, `aria-label` or a `data-*` attribute | The field |
| The address can only be the whole `href` of a link | Write `href="{{.Href}}"` and nothing around it | |
| A field can be placed, and nothing else can be done with it | Do not compare, cut or combine `.Site.Title` or a field of an entry. Branch on `.Has.<field>` | The field |
| A link has an address of a kind that is not accepted | Use `http:`, `https:`, `mailto:` or an address without a scheme | The address |
| An address that loads something is not a file among the assets of the theme | Write `src` with `asset`, and put the file into `assets` | The address |
| A part of the application stands where it cannot, such as inside a link | Place a part in the content of an element that is not a link | The part |
| A part of the application that the page shows is not placed | Place every part the page requires, in every branch the page can take | The part |
| A part of the application is placed more than once | Place it once in each branch | The part |
| It needs exactly one link to the address | An item template has one `a` with `href="{{.Href}}"` | |
| A field that has to be shown as text is not | Show `{{.Name}}` in the content of an element, in every branch | The field |
| What it writes is not the one element it has to be | `bookmark.html` writes one `li` and nothing beside it | The element |
| An element it has to write is missing | A group that is opened and closed is a `details` with a `summary` | The element |

A template that is not accepted counts as absent. The page or the entry is
then written with the template of a theme below, where the theme is
[based on another](theme-templates.md#a-theme-based-on-another), or with the
markup of the application.

## Before passing a theme on

- The theme page lists no template of the theme as not accepted, with every
  option of the theme at each of its values.
- The three pages were looked at: the home page, the application page and
  the bookmark page, also in a narrow window.
- A search was run, with results and without.
- Entries without an icon and without a description were looked at, and a
  name with characters such as `&` and `<`.
- The three modes of the category navigation were tried, and a site without
  categories, if the theme brings `group.html`.
- `supports` lists the layout options that still work, and the appearance
  page was looked at with the theme selected.
- The keyboard reaches every link and the search field, and the focus is
  visible.
- The theme file names the release it needs under `requires`, if it relies
  on more than is documented.
- The [archive](theme-templates.md#archives) of the theme was downloaded
  and imported under another name, and looks the same.

Whoever installs the theme allows templates for the site and confirms the
templates of the theme. The form says what a template can do: it can write
any text and any link. It cannot run a script, load anything from another
site, or read anything but what is listed under
[what a template is given](theme-templates.md#what-a-template-is-given).

## Verification

- `config/define`: the files of the example are read from this page, stored
  as a theme and read by the application. Every template is accepted, for
  each value of the option, and the theme is in use.
- `tests/theme/templates.browser.cjs` renders pages with the example in a
  browser; see [theme templates](theme-templates.md#verification).
