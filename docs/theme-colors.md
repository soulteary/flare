# Themes and custom colors

> Availability: the 15 bundled themes are available in every release. Custom
> colors are in current source and are not part of 0.6.15.

A theme is three colors: the page background, the primary color used for text
and icons, and the accent color used for group titles, descriptions and
borders. Settings → Theme lists the bundled themes; selecting one applies it to
the home page, the application and bookmark pages, the help page and the
settings pages. The editor, the import/export page and the sign-in page keep
their own colors.

## Custom colors

The **Custom colors** form on the same page takes one color for each of the
three roles. It always starts from the colors in use, so any theme, bundled or
custom, can be adjusted instead of starting from black. **Save and apply**
stores the colors and selects them as the theme; the page reloads in the new
colors, which makes saving the way to try a combination.

After the first save a **Custom colors** card appears next to the bundled
themes. Selecting a bundled theme keeps the saved colors, and the card returns
to them. Saving the form again replaces them; there is one custom set. To
change the saved colors, select the card first, then edit and save.

The primary color needs a contrast ratio of at least 3:1 against the
background, the WCAG threshold for large text and interface components. A
combination below it is rejected, because the page that would correct it would
be hard to read. The accent color is not checked.

Icons from the bundled icon set follow the primary color. Themed browser-tab
icons exist only for the bundled themes, so custom colors use the default icon.

## Bundled icons

> Availability: the behavior in this section is in current source and is not
> part of 0.6.15, which writes one icon file per theme with the color in it.

An icon from the bundled icon set is drawn in the primary color of the theme in
use. The page takes the color from the theme; the icon file,
`/assets/mdi/<name>.svg`, carries none. Selecting another theme or saving other
custom colors therefore changes the icons without requesting other files.
`FLARE_MINI_REQUEST` writes the icons into the page instead of referencing
files; they follow the primary color in the same way.

When the browser forces colors, for example under a Windows contrast theme,
bundled icons are drawn in the system text color. The forced page background
could otherwise leave the theme color without contrast.

Icons given as a URL are images and keep their own colors.

## Stored form

The selection and the colors are stored in `config.yml`:

```yaml
Theme: custom
ThemeCustom:
    Background: '#10241b'
    Primary: '#e8f1e4'
    Accent: '#f2b84b'
```

Colors are `#RRGGBB` values and have to be quoted: an unquoted `#` starts a
YAML comment and leaves the color empty.

`Theme` is read at startup. After changing it by hand, restart the instance, as
for the bundled theme names. The card on the settings page shows the
`ThemeCustom` block as it is in the file, and selecting the card applies that
block, so an edited block can also be applied from the page without a restart.

A `ThemeCustom` block that is not usable is treated as absent: a missing or
invalid color, too little contrast, or a value that is not a block at all. The
rest of `config.yml` stays readable and no card is shown. If `Theme` is
`custom` in that case, the instance starts with the first bundled theme,
`blackboard`, and logs a warning.

## Returning to an earlier release

Releases up to 0.6.15 do not know these keys. They start with such a
configuration and render the stylesheet's default colors while `Theme` is
`custom`.

A release with custom colors records its version in `config.yml` when it saves
settings, and earlier releases do not overwrite a configuration saved by a newer
one. This existing protection, described under
[application version tracking](search-hybrid.md#application-version-tracking),
also blocks choosing another theme on the settings page of the earlier release.
To go back, restore the `config.yml` copied before the upgrade. Setting `Theme`
to a bundled name by hand restores the colors, but the earlier release still
does not save settings until the file is restored.

## Verification

`go test ./config/model ./config/define ./config/data ./internal/settings/theme ./internal/resources/... ./internal/server -run 'Palette|Contrast|Custom|ThemeSelection' -count=1`
covers validation, storage, hand-edited blocks, the settings form, icons, the
favicon and the rendered pages.
`FLARE_BINARY=/absolute/path/to/flare node tests/settings/save.browser.cjs`
submits the form in Chromium through a proxy.
