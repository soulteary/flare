# Theme files

> Availability: in current source; not part of 0.6.15.

A theme can be added to an instance as a file. The file gives the three colors
of a [theme](theme-colors.md) and, if wanted, values for the other
[style tokens](theme-tokens.md) and a name to show. The bundled themes are
files of the same format. A theme can also be
[a directory](#a-theme-as-a-directory) with such a file in it.

## Location and name

Theme files are read from the `themes` directory in the working directory of
the instance, next to `config.yml`. In the container image that is
`/app/themes`.

```text
config.yml
apps.yml
bookmarks.yml
themes/
    forest.yml
```

The name of the file without `.yml` is the name of the theme. It is the value
of `Theme` in `config.yml` while the theme is selected, so renaming the file
of the selected theme leaves the selection without a theme.

- A name uses lower-case letters, digits and single hyphens between them, and
  has at most 32 characters: `forest`, `solarized-light`.
- The names of the bundled themes and `custom` cannot be used.
- A theme file is a regular file in the directory itself, or the `theme.yml`
  of [a theme that is a directory](#a-theme-as-a-directory). Links are not
  followed, and no other file of a subdirectory is read.
- A `.yaml` file is not read. It is reported on the theme page, because the
  extension of a theme file is `.yml`. Files with any other extension and
  hidden files are left alone.
- Up to 64 entries of the directory are looked at, in the order of their
  names: theme files, directories, and the `.yaml` files and links that are
  only reported. A theme file can have up to 64 KiB.

The directory is optional. Files in it are read, and nothing is written to it
except by an [import](#import-and-export) on the theme page.

### A theme as a directory

A theme can be a directory of the themes directory instead of a file. The
name of the directory is the name of the theme, and the file `theme.yml` in
it is its theme file, in the [format](#format) below.

```text
themes/
    forest.yml
    zelda/
        theme.yml
```

The directory is the form for a theme of more than one file. Of a directory,
`theme.yml` is read and nothing else, so a theme in a directory is the same
theme as its `theme.yml` would be as a file. It is listed, previewed and
selected by the name of the directory: `Theme: zelda`.

- The name of the directory has the rules of the name of a file without
  `.yml`, and cannot be the name of a bundled theme or `custom`.
- A directory without a `theme.yml` is not a theme and is left alone. One
  that holds a `theme.yaml` instead is reported, as a `.yaml` file is.
- The directory and its `theme.yml` have to be a directory and a regular
  file. A link in the themes directory, and a `theme.yml` that is a link, are
  reported on the theme page and not followed.
- A name has one theme. With both `forest.yml` and `forest/theme.yml`, the
  directory is the theme, and the file is reported and not read. That holds
  also when the `theme.yml` is not usable: the file does not stand in for it.

## Format

```yaml
contract: 1
name: Forest at dusk
version: "1.2"
author: Example Author
license: MIT
tokens:
  background: "#10241b"
  primary: "#e8f1e4"
  accent: "#f2b84b"
  muted: "#9db8a6"
  hover: "#ffffff1a"
  radius: 10px
  font-family: Georgia, serif
```

| Key | Required | Value |
| --- | --- | --- |
| `contract` | Yes | `1`, the version of this format |
| `tokens` | Yes | The style tokens the theme sets; `background`, `primary` and `accent` are required |
| `modes` | No | Other token values for light and for dark; see [light and dark](#light-and-dark) |
| `layout` | No | Layout options the theme specifies; see [layout](#layout) |
| `name` | No | The name to show, up to 48 characters; without it the name of the file is shown |
| `version`, `author`, `license` | No | Up to 64 characters each; shown with the theme on the settings page, without any other effect |

### Tokens

The keys under `tokens` are the names of the [style tokens](theme-tokens.md)
without the `--flare-` prefix. A token that is not set keeps the default
described there, so three colors make a complete theme.

| Token | Accepted value |
| --- | --- |
| `background`, `primary`, `accent`, `icon`, `muted`, `border`, `focus`, `inverse` | A `#RRGGBB` color |
| `hover` | A `#RRGGBB` color, or `#RRGGBBAA` with transparency |
| `radius` | `0`, or a whole number of pixels from `0px` to `32px` |
| `font-family` | Font family names separated by commas, without quotes, up to 160 bytes |
| `font-scale` | A number from `0.8` to `1.5` with at most two decimals |

- Colors have to be quoted: an unquoted `#` starts a YAML comment and leaves
  the color empty.
- The primary color needs a contrast ratio of at least 3:1 against the
  background, as for [custom colors](theme-colors.md#custom-colors).
- A font family name is one or more words of letters, digits, hyphens and
  underscores, such as `Helvetica Neue`, `-apple-system` or `思源黑体`. The
  names refer to fonts on the device of the visitor; a theme file does not
  load fonts.
- `font-scale` multiplies the text sizes of the navigation pages; `1.25`
  makes the text a quarter larger. What it applies to, and how it combines
  with the text size of the appearance page, is described with the
  [style tokens](theme-tokens.md#details).
- No other form of value is accepted: no color names or functions, no other
  units, no `var()` and no `url()`. The values are written into the style
  attribute of the page, and a value outside the forms above rejects the file.

### Light and dark

A theme can have different values for light and for dark. `modes` gives the
values that differ:

```yaml
contract: 1
tokens:
  background: "#16181d"
  primary: "#e6e1cf"
  accent: "#ffcc66"
  radius: 8px
modes:
  light:
    background: "#f6f3ea"
    primary: "#2b2a27"
    accent: "#b5651d"
```

- `modes.light` and `modes.dark` take the same tokens and values as `tokens`.
  A value given for a mode replaces the value of `tokens` in that mode;
  everything else stays as `tokens` has it. A theme that is dark as written
  needs only `modes.light`, and a light one only `modes.dark`.
- `tokens` stays a complete theme with its three colors. It is what a mode
  without values of its own uses, and what a release that does not know
  `modes` uses.
- In each mode the primary color needs the contrast of at least 3:1 against
  the background once the values of the mode are in place.
- Other names under `modes` are not used and are listed on the theme page.

Which of the two the pages show is a setting of the site. Settings → Theme
offers it under **Light and dark** while the selected theme has different
values for the two.

| Setting | `ThemeMode` in `config.yml` | The pages show |
| --- | --- | --- |
| Follow the device | `auto`, also without the key | The light values, and the dark ones on a device that asks for dark |
| Light | `light` | The light values |
| Dark | `dark` | The dark values |

- Following the device uses the `prefers-color-scheme` media feature of the
  browser. The page changes with the device without being loaded again.
- The setting belongs to the site, not to a theme. It is kept while another
  theme is selected, and it has no effect on a theme that is the same in both
  modes, such as the bundled themes and custom colors.
- A theme with one set of values in use writes it into the `style` attribute
  of `body`, as the bundled themes do. While the device chooses, the two sets
  are rules in a `style` element of the page head instead, each in a media
  query, and the attribute is empty.
- The card of such a theme on the theme page shows the light colors above the
  dark ones.

### Layout

A theme can specify how the home page, the application page and the bookmark
page are laid out. `layout` names options of the site and the value the theme
wants for each:

```yaml
layout:
  DesktopColumns: 6
  TitleSize: compact
  HideAppDescriptions: true
```

The options have the names they have under `Homepage` in `config.yml`.

| Option | Values | Described in |
| --- | --- | --- |
| `MobileBookmarkLayout` | `classic`, `single-column`, `compact` | [Mobile layout](mobile-layout.md) |
| `TitleSize` | `standard`, `compact`, `prominent` | [Mobile layout](mobile-layout.md) |
| `HideAppDescriptions` | `true`, `false` | [Mobile layout](mobile-layout.md) |
| `CategoryNavigation` | `all`, `tabs`, `collapse` | [Category navigation](category-navigation.md) |
| `BookmarkLayout` | `groups`, `sections` | [Desktop columns](desktop-columns-and-scale.md) |
| `DesktopColumns` | `legacy`, `auto`, `4`, `5`, `6`, `8` | [Desktop columns](desktop-columns-and-scale.md) |

`legacy` is the layout of releases before the option, which `config.yml`
stores as an empty value. A theme cannot specify anything else: the text size
is for the reader, and which modules a page shows, how links open, search,
clock and weather are for the administrator.

While the theme is selected, the options it specifies replace those of the
site, and the others stay as the site has them. The settings of the site are
not rewritten: selecting another theme brings its layout back, and an update
of the theme file changes the layout without anything to migrate. The card of
a theme on the theme page names the options it sets, so this is known before
the theme is selected. The bundled themes specify none.

Settings → Appearance says next to each of these options what the selected
theme sets, and can turn the layout of the theme off:

| On the page | Stored in `config.yml` | Effect |
| --- | --- | --- |
| **Use my setting** next to an option | `ThemeSettings.<theme>.IgnoreLayout`, a list of options | That option uses the setting of the site while this theme is selected |
| **Layout set by the theme** → **Use my settings for every option** | `Homepage.IgnoreThemeLayout: true` | No theme changes the layout |

```yaml
Homepage:
    DesktopColumns: "4"
    IgnoreThemeLayout: false
ThemeSettings:
    forest:
        IgnoreLayout:
            - DesktopColumns
```

A value under `Homepage` that is changed by hand has no effect on an option
while the theme's value applies. A line in the log at startup names the
options the selected theme replaces.

An option or a value that this release does not have is left out and listed
on the theme page, and the setting of the site applies to that option. The
rest of the layout, and the theme, stay in use.

### Names in several languages

`name` can give one text for each interface language instead of a single text:

```yaml
name:
  en: Forest at dusk
  zh: 暮色森林
```

The text for the [interface language](i18n.md) is shown. A regional code such
as `zh-CN` answers for its language. Without a text for the interface language
the English text is shown, and without one, the text of the first language in
the order of the codes.

### Files written for another release

Keys, tokens, modes and layout options that this release does not know are
not used, and neither is a layout value it does not have. The theme remains
usable, and the settings page lists the parts that were left out. A theme
file written for a later release therefore works with what this release
knows.

Everything this release does use has to be valid. A file with one value that
is not accepted is not used at all; the theme is not applied in part.

`contract` changes only when the format changes in a way an older release
cannot read. A file with another `contract` is not used.

## Selecting a theme file

Settings → Theme lists the usable files under **Theme files**, after the
bundled themes, with their names and colors. Selecting one stores its name in
`config.yml` and applies it to the pages a bundled theme applies to.

```yaml
Theme: forest
```

`Theme` can also be set by hand. It is read at startup, as for the bundled
theme names.

A theme file has no browser-tab icon of its own and uses the default icon, as
custom colors do. The **Custom colors** form starts from the three colors of
the theme in use, so the colors of a theme file can be adjusted there; the
result is a set of custom colors, without the other tokens of the file. For a
theme that follows the device, the form starts from its light colors.

## Preview

Every theme on the theme page has a **Preview** link: the bundled themes, the
custom colors and the theme files. It opens the home page shown with that
theme, with its tokens, its values for light and dark and its layout, without
selecting it.

- Only that response is shown with the theme. Other visitors, and the same
  page opened without the link, get the selected theme, and nothing is
  written to `config.yml`.
- A notice at the top of the page names the previewed theme. **Use this
  theme** selects it; **Back to the settings** leaves it unselected.
- The preview shows the file as it is at that moment. A theme file can be
  edited and its preview loaded again before the theme is selected.
- The address of a preview is the page with `?theme-preview=<name>`, on the
  home page, the application and bookmark pages and the help page. The
  parameter is honored for the administrator only and has no effect for a
  visitor. With login disabled there is no such difference.
- The browser-tab icon stays that of the selected theme.

## Import and export

The theme page hands a theme out as a theme file and takes a theme file in.
Both are for the administrator, and neither selects a theme.

**Download** next to a theme sends a theme file:

| Theme | File |
| --- | --- |
| A theme file | The file of the themes directory as it is, `<name>.yml` |
| A bundled theme | The file the theme is defined by, as `<name>-copy.yml` |
| Custom colors | A theme file with the three colors, as `custom-colors.yml` |

The name of a bundled theme and `custom` cannot be the name of a theme file,
so those two downloads have a name under which they can be imported. The
download of a bundled theme is a starting point for a theme of one's own.
A theme that is a directory has no download: it is not one file, and it is
copied as a directory.

**Import a theme file** stores an uploaded file in the themes directory:

- The name of the file is the name of the theme, with the rules under
  [location and name](#location-and-name): `<name>.yml`, and not the name of a
  bundled theme. A path the browser sends with the name is not used.
- The file gets the checks of every theme file. One that is not usable is
  refused with the reason, and nothing is written. An accepted file is stored
  as it was uploaded, with its comments and with parts for later releases.
- A theme file of the same name is replaced only with **Replace a theme file
  of the same name** ticked. Something else of that name in the directory, a
  folder or a link, is never replaced.
- A file is not imported under the name of a theme that is a directory, with
  the box ticked or not: the file would not be read, and an import does not
  replace a directory.
- A file can have up to 64 KiB, and a new file is refused while the directory
  holds the 64 entries that are looked at, because more are not read.
- The directory is created by the first import. A new file is readable and
  writable for the user the instance runs as; a replaced file keeps its
  permissions.

A theme is removed by deleting its file or its directory from the themes
directory.

## Changes while the instance runs

The pages follow the theme file of the selected theme. Each page load compares
the modification time and the size of that file with the values it was read
at, and reads the directory again when they differ. An edit therefore shows on
the next page load, without a restart. Both places a theme of the selected
name can be read from are compared, `<name>.yml` and `<name>/theme.yml`, so a
theme that is turned from a file into a directory, or back, is followed as
well. While a bundled theme or the custom colors are selected, no theme file
is looked at for a page.

The theme page reads the directory every time it is shown, so files that were
added, corrected or removed are listed as they are.

When the file of the selected theme is removed, or is edited into a file that
is not usable, the pages are not left with the previous version of the theme.
They use the colors of the stylesheet, and the theme page names the theme and,
for a file that is still there, the reason. Correcting the file or putting it
back applies the theme again on the next page load.

An edit that keeps both the size and the modification time of the file is not
noticed, except within two seconds of that time: during those, the directory
is read again on every page load.

## Files that are not used

The theme page lists each theme file that is not in use with the reason, in
the interface language, and with the part of the file the reason is about
where there is one. The theme file of a directory is listed with the
directory, as `zelda/theme.yml`.

| Reason | Cause |
| --- | --- |
| The extension has to be `.yml` | The file is named `.yaml`, or a directory holds a `theme.yaml` and no `theme.yml` |
| Name not accepted | Upper-case letters, spaces, underscores, other characters, or more than 32 characters, in the name of the file or of the directory |
| Name belongs to a bundled theme or the custom colors | For example `blackboard.yml`, `custom.yml` or `blackboard/theme.yml` |
| Not a regular file | A directory or a link named like a theme file, or a `theme.yml` that is one |
| A link is not followed | A link in the themes directory with a name that is not that of a theme file |
| The directory of the same name is the theme | A file `<name>.yml` beside a directory `<name>` with a `theme.yml` |
| Larger than 64 KiB | |
| More than 64 files and directories | Reported for the first entry that is not looked at; what comes after it is not looked at either |
| Cannot be read | For example missing read permission |
| Not a usable theme file | Not YAML, not a mapping, a key given twice, or a value of the wrong shape; the message of the parser is shown |
| `contract: 1` is required | `contract` is missing or has another value |
| Token value not accepted | Shown with the token, for example `tokens.radius` or `modes.light.radius` |
| Required color missing | Shown with the token, for example `tokens.accent` |
| Contrast below 3:1 | Between `primary` and `background`; shown with the mode, for example `modes.light`, when it is the values of a mode that lower it |
| Text not accepted | `name`, `version`, `author` or `license` is too long, has leading or trailing spaces, or contains control characters |

At startup, a selected theme that cannot be used is also written to the log as
a warning.

## Returning to an earlier release

Releases up to 0.6.15 do not read theme files. With the name of a theme file
in `Theme` they start and render the colors of the stylesheet. The protection
of a configuration saved by a newer release, described under
[returning to an earlier release](theme-colors.md#returning-to-an-earlier-release),
applies in the same way.

## Verification

`go test ./config/model ./config/define ./config/data ./internal/settings/... ./internal/resources/assets ./internal/server -run 'Theme|Homepage' -count=1`
covers the format, the files that are not used, selection, following the file
of the selected theme, light and dark, layout, the preview, import and export,
the favicon and the rendered pages, including that texts and names from the
directory are shown as text.
`FLARE_BINARY=/absolute/path/to/flare node tests/theme/files.browser.cjs`
selects a theme file in Chromium, edits it, breaks it, removes it and puts it
back, and checks the pages after each step.
`FLARE_BINARY=/absolute/path/to/flare node tests/theme/directories.browser.cjs`
selects a theme that is a directory, edits its `theme.yml`, puts a file of the
same name beside it, turns the theme into a file and back, and checks the
pages after each step.
`FLARE_BINARY=/absolute/path/to/flare node tests/theme/modes.browser.cjs`
loads the pages of a theme with light and dark values for a device that asks
for light and for one that asks for dark, under each of the three settings.
`FLARE_BINARY=/absolute/path/to/flare node tests/theme/preview.browser.cjs`
opens the preview of a theme file and of a bundled theme from the theme page,
checks that nothing is selected by it, and selects a theme from the notice.
`FLARE_BINARY=/absolute/path/to/flare node tests/theme/import-export.browser.cjs`
downloads a bundled theme, imports the file, has unusable files and an
existing name refused, replaces the file and downloads it again.
`FLARE_BINARY=/absolute/path/to/flare node tests/theme/layout.browser.cjs`
selects a theme that specifies layout, turns single options and the whole
layout off and on again on the appearance page, and checks the home page
after each step.
