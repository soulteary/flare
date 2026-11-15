# Flare 0.7.0

Release date: 2026-11-15 (Asia/Shanghai). This release contains the changes since [0.6.15](https://github.com/soulteary/flare/releases/tag/0.6.15). It adds themes that an instance brings itself: custom colors, theme files in a `themes` directory, and themes with a stylesheet, images and fonts. An instance that keeps a bundled theme looks as it did. Read [Behaviour changes](#behaviour-changes) before updating; when updating from 0.6.14 or earlier, read the [0.6.15 notes](release-0.6.15.md) as well, because the changes of that release apply too.

Where the guides say "current source" or "current main", they describe this release unless a later version is named.

## Changes

### Custom colors

- Settings → Theme has a form for one palette of the instance: the background, the primary and the accent color. It is stored as `ThemeCustom` in `config.yml` and selected with `Theme: custom`, and selecting a bundled theme keeps it. A color is written as `#RRGGBB`, and the primary color needs a contrast of at least 3:1 with the background ([#246](https://github.com/soulteary/flare/pull/246)).

See [themes and custom colors](theme-colors.md).

### Theme files

- A file `themes/<name>.yml` in the working directory, next to `config.yml`, is a theme: three colors, optionally the other style tokens, and a name to show. Settings → Theme lists the usable files, and one is selected like a bundled theme. The pages follow the file of the selected theme, so an edit shows on the next page load without a restart ([#250](https://github.com/soulteary/flare/pull/250)).
- A theme file can give values for light and for dark. The site setting `ThemeMode` leaves the choice to the device of the visitor (`auto`, the default) or fixes `light` or `dark` ([#251](https://github.com/soulteary/flare/pull/251)).
- A theme file can set layout options of the site: the phone and tablet layout, the title size, application descriptions, the category navigation, the bookmark layout and the desktop columns. The settings of the site are not rewritten, and the appearance page can turn the layout of a theme off, per option or for every theme ([#252](https://github.com/soulteary/flare/pull/252)).
- A theme file can declare up to eight options of its own, each a list of values, a switch or a color, which the administrator sets on the theme page ([#262](https://github.com/soulteary/flare/pull/262)).
- A theme can be based on another theme with `extends` and give only what it changes ([#263](https://github.com/soulteary/flare/pull/263)), and it can name the first release it works with under `requires` ([#258](https://github.com/soulteary/flare/pull/258)).
- A file is used as a whole or not at all. The theme page lists each file that is not in use with the reason and the part of the file. Keys, tokens and layout options that this release does not know are left out and listed, so a file written for a later release stays usable.

See [theme files](theme-files.md).

### Stylesheets, images and fonts

- A theme can be a directory, `themes/<name>/` with a `theme.yml` in it ([#256](https://github.com/soulteary/flare/pull/256)), and bring a stylesheet, `theme.css`, for the home page, the application and bookmark pages and the help page. The settings pages do not load it, so a theme can always be changed from there ([#257](https://github.com/soulteary/flare/pull/257)).
- A stylesheet is checked before it is used and loads nothing from another address. It can refer to the images and fonts in the `assets` directory of its theme and to nothing else; `@import` and every other address are refused. Assets are PNG, JPEG, GIF, WebP and AVIF images and WOFF2, WOFF, TrueType and OpenType fonts of up to 2 MiB each. SVG files are not accepted, because they can hold scripts and refer to other files ([#260](https://github.com/soulteary/flare/pull/260)).
- A theme file can lay one of the images of the theme behind the navigation pages under `background`, with the background color of the theme over it ([#261](https://github.com/soulteary/flare/pull/261)).

See [theme stylesheets](theme-stylesheets.md).

### Preview, download, import and safe mode

- Every theme on the theme page has a preview, which shows the home page with that theme without selecting it ([#254](https://github.com/soulteary/flare/pull/254)).
- The theme page hands a theme out and takes one in: a theme file as `<name>.yml` ([#255](https://github.com/soulteary/flare/pull/255)) and a theme that is a directory as `<name>.zip` ([#264](https://github.com/soulteary/flare/pull/264)). An import gets the checks of a theme of the directory, a refusal names the reason and the file, and of an archive only the files of a theme are written.
- Safe mode leaves the themes of the themes directory out: `?theme-safe-mode` in the address of a page for that response, and `FLARE_THEME_SAFE_MODE=true` or `--theme_safe_mode` for the instance ([#259](https://github.com/soulteary/flare/pull/259)).

### Style tokens and icons

- The stylesheets of the themed pages take the colors, the corner radius, the font family and a scale for the text sizes from twelve custom properties named `--flare-*`. The names are stable for the stylesheet of a theme and for markup in the footer. `--color-background`, `--color-primary` and `--color-accent` keep their values ([#248](https://github.com/soulteary/flare/pull/248), [#253](https://github.com/soulteary/flare/pull/253)).
- An icon of the bundled icon set is one file without a color, which the page references and fills with the primary color of the theme in use. Up to 0.6.15 there was one file per theme and icon ([#247](https://github.com/soulteary/flare/pull/247)).
- The bundled themes are theme files of the same format, embedded in the binary. Their names, order and colors are unchanged, and the download of one on the theme page is a starting point for a theme of one's own ([#249](https://github.com/soulteary/flare/pull/249)).

See [style tokens](theme-tokens.md).

### Documentation

- The upgrade guide describes the startup of 0.6.15 with private entries while login is disabled, and the proxy settings that became optional in that release ([#245](https://github.com/soulteary/flare/pull/245)).

## Behaviour changes

- **Existing appearance.** With a bundled theme, the home page, the application and bookmark pages and the help page are drawn as in 0.6.15; [verification scope](#verification-scope) says how that was compared. The source of these pages differs: a bundled icon is an `svg` element with a `use` reference instead of an `img` element, and the stylesheets read their values through the style tokens, which are computed on `body`. Two kinds of style in the footer therefore need a look, as does anything that reads the pages as HTML. A rule that selects the `img` element of a bundled icon no longer matches it. A rule that sets `--color-primary`, `--color-background` or `--color-accent` on an element inside the page no longer changes the colors below that element; set on `body` it works as before, and now colors the bundled icons too. Icons given by an address remain `img` elements. With `FLARE_MINI_REQUEST` a bundled icon stays an inline `svg` element, and its fill reads `--flare-icon` instead of `--color-primary`.
- **Icon addresses.** A bundled icon is served at `/assets/mdi/<name>.svg`. The addresses of 0.6.15, `/assets/mdi/<theme>-<name>.svg`, return 404. Pages are sent with `Cache-Control: no-store`, so no stored page refers to them; a link to such a file from elsewhere stops working.
- **Forced colors.** In a forced-colors mode, such as Windows high contrast, bundled icons are drawn in the system text color. As images they kept the color of the theme.
- **The `themes` directory.** A directory named `themes` next to `config.yml` is read from this release on. Regular files ending in `.yml` in it, and directories with a `theme.yml`, are taken as themes; one that is not usable is listed on the theme page and not used. Reading changes nothing in the directory. Without the directory nothing is read, and it is created by the first import on the theme page. Persist it and include it in backups with the rest of the working directory.
- **Who can add a theme.** Adding a theme is a change of the settings. It needs the administrator; on an instance with login disabled, which is the default, that is anyone who can reach the instance, as for every other setting. What such a change now includes is storing files: up to 64 entries in the themes directory, a theme file of up to 64 KiB, and for a theme that is a directory a stylesheet of up to 128 KiB, up to 32 assets of up to 2 MiB each and up to eight text documents of up to 64 KiB each. The stylesheet of a theme cannot run scripts or load from other addresses, but it can hide, move and restyle everything on the navigation pages and show text and images of its own. Install themes from sources you trust, and enable login on an instance that others can reach.
- **Uploads.** Importing an archive sends up to 24 MiB in one request. A reverse proxy with a smaller limit for request bodies answers before Flare does; nginx allows 1 MiB unless `client_max_body_size` is set.
- **Request parameters.** `?theme-preview=<name>` and `?theme-safe-mode` on the navigation pages change that one response, for the administrator. On an instance with login disabled every visitor can use them. They change nothing for other visitors and nothing in `config.yml`, and such a response is marked `private, no-store`.
- **New addresses.** `/theme/<name>/<hash>/theme.css` and `/theme/<name>/<hash>/assets/<file>` serve the stylesheet and the assets of a theme to whoever the pages are for, and may be kept by a browser for a year. A reverse proxy that passes only listed paths needs `/theme/`.
- **Theme names that name no theme.** A `Theme` value that names no theme renders the colors of the stylesheet, as in 0.6.15. This release also logs a warning at startup and names the theme on the theme page.
- **`config.yml`.** The new keys are `ThemeCustom`, `ThemeMode`, `ThemeSettings` and `Homepage.IgnoreThemeLayout`. Starting and serving pages do not rewrite the file, and selecting a bundled theme adds none of these keys. The first save of a setting records `AppVersion: 0.7.0` and keeps the previous file as `.flare-search-backups/first-upgrade-to-0.7.0.bak`.
- **Startup.** `FLARE_THEME_SAFE_MODE` and `--theme_safe_mode` are new and off by default. No other startup input changes.

## Upgrade and rollback / 升级与回退

Stop the instance and back up its complete working directory, including hidden security state, YAML files, and external environment/Secret configuration. Retain the old binary or image digest. Replace the executable or recreate the container with the same persistent mounts and effective settings. There is no data migration from 0.6.15, the security state format is unchanged, and existing sessions stay signed in. Afterwards verify, through every address you use: the home page with your theme and icons, sign-in, saving a setting, saving an editor change, and search. If themes will be imported, check that the `themes` directory ends up on the persistent mount.

升级前停止实例，备份完整工作目录（包含隐藏的安全状态）、YAML 和外部部署配置，并记录旧镜像摘要或保留旧二进制。沿用原持久化挂载与配置更新。从 0.6.15 升级无需数据迁移，安全状态格式不变，已登录的会话继续有效。升级后请通过你实际使用的每一个访问地址验证：首页的主题与图标、登录、保存设置、保存编辑器修改，以及搜索。如果要导入主题，请确认 `themes` 目录落在持久化挂载里。

To roll back, stop the new instance and restore the backup taken before the upgrade together with the saved 0.6.15 binary or image. 0.6.15 starts on files saved by 0.7.0 and serves the pages, but it refuses every settings save: `config.yml` records the version that last saved it, and 0.6.15 does not overwrite a newer file. It reads neither the `themes` directory nor the new keys, and with `Theme: custom` or the name of a theme file it renders the colors of the stylesheet. With the `config.yml` of before the upgrade restored, 0.6.15 saves again; the `themes` directory can stay where it is. For versions before 0.6.8, read [security migration and rollback](login-private-upgrade.md) first: old releases may not enforce current per-resource privacy.

回退时停止新版，恢复升级前的备份，并使用保留的 0.6.15 程序或镜像。0.6.15 可以在 0.7.0 保存过的文件上启动并提供页面，但拒绝保存任何设置：`config.yml` 会记录最后保存它的版本，0.6.15 不会覆盖更新版本写入的文件。它不读取 `themes` 目录，也不认识新增的配置项；`Theme` 为 `custom` 或某个主题文件的名字时，页面使用样式表的默认配色。恢复升级前的 `config.yml` 之后，0.6.15 可以再次保存；`themes` 目录可以原样保留。回退到 0.6.8 之前的版本须先阅读安全迁移指南，旧版本可能无法保持当前逐项隐私保护。

## Verification scope

Covered before release: the Go test suite with the race detector and the all-package coverage gate, the Chromium browser acceptance scripts, and the release preflight for seven archives and four container platforms.

Other engines: the 16 acceptance scripts for icons and themes were also run in the Firefox and WebKit builds that Playwright distributes. 29 of the 32 runs pass with nothing but the browser changed. The other three compare a text size as it is reported, and one of them moves to a link with the Tab key; they pass with that size rounded to a tenth of a pixel and, in WebKit, with the link reached by Option+Tab. Firefox reports the size as 11.2031px where Chromium and WebKit report 11.2px, with 0.6.15 as with this release.

Compared with 0.6.15: the home page, the application and bookmark pages and the help page were captured from both releases with the same data and compared as screenshots. In Chromium, 136 pages covering the 15 bundled themes, five layout settings, widths of 1280 and 390 pixels and both icon output modes are identical pixel for pixel; so are 24 pages each in Firefox and WebKit. The pages were captured at rest, without a pointer over an element and without focus. An upgrade in place and the return to 0.6.15 were run on one data directory, with login disabled and with login enabled.

Not covered: Safari itself and physical devices; themes written by others, since every theme used was written for a test; the import of a theme and the themes directory inside a container image, where the preflight checks HTTP, the time zone and a restart; a reverse proxy in front of an upload or of the `/theme/` addresses; and forced colors outside the emulation of a browser.

## Distribution

```sh
docker pull soulteary/flare:0.7.0
docker pull ghcr.io/soulteary/flare:0.7.0
```

Containers: linux/amd64, linux/arm64, linux/arm/v7, linux/arm/v6. Archives additionally support Linux 386 and macOS amd64/arm64. Pin the published digest for reproducible deployment. Release preflight verifies seven archives and four container platforms without publishing; the tag workflow publishes binaries and Docker Hub images, then mirrors and verifies GHCR images. Consult the actual workflow results rather than treating this note as a record of completed publication.
