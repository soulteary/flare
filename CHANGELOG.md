# Changelog

## [0.7.0] - 2026-11-15

### Added

- Custom theme colors: one palette of background, primary and accent color, set on the theme page, stored as `ThemeCustom` and selected with `Theme: custom`.
- Theme files: `themes/<name>.yml` in the working directory is a theme of the instance, with the three colors, optional style tokens, values for light and for dark, layout options and up to eight options of its own. The pages follow the file of the selected theme without a restart.
- The site setting `ThemeMode` (`auto`, `light`, `dark`) for themes with light and dark values, and `Homepage.IgnoreThemeLayout` and `ThemeSettings` to turn off the layout a theme specifies and to hold the values of its options.
- Themes as directories: `themes/<name>/theme.yml` with a checked stylesheet `theme.css` for the navigation pages, images and fonts in `assets`, and a background image.
- `extends`, which bases a theme on another, and `requires`, which names the first release a theme works with.
- A preview of every theme on the theme page, and download and import of themes there: a theme file as `.yml`, a theme that is a directory as `.zip`.
- Safe mode, which leaves the themes of the themes directory out: `?theme-safe-mode` for one page, `FLARE_THEME_SAFE_MODE` or `--theme_safe_mode` for the instance.
- Twelve style tokens, custom properties named `--flare-*`, for colors, corner radius, font family and text scale, with names that stay stable for theme stylesheets and footer markup.

### Changed

- Reference bundled icons from one colorless file per icon and fill them with the primary color of the page. `/assets/mdi/<theme>-<name>.svg` is no longer served.
- Read colors, corner radius and font through the style tokens in the stylesheets of the themed pages. With a bundled theme the navigation pages are drawn as in 0.6.15.
- Define the bundled themes as theme files in the format an instance uses; their names, order and colors are unchanged.
- Draw bundled icons in the system text color when the browser forces colors.
- Log a warning at startup when `Theme` names no usable theme.
- Align the upgrade guide with the startup behaviour of 0.6.15.

Read the [release and upgrade notes](docs/release-0.7.0.md) for the behaviour changes to review before updating.

## [0.6.15] - 2026-11-14

### Changed

- Accept same-origin browser writes behind a TLS-terminating reverse proxy without additional configuration: decide by `Sec-Fetch-Site`, fall back to the `Origin` host and port, and compare the scheme only under an explicit HTTPS policy.
- `FLARE_EXTERNAL_URL` no longer restricts the Host header; `FLARE_ALLOWED_HOSTS` does and includes the external authority.
- Keep settings saves working when their backup cannot be taken: repair backup permissions, set an untrusted backup directory aside under `.flare-search-backups.quarantined-<UTC timestamp>`, and continue with a logged warning.
- Restore the classic phone layout of 0.6.3 and earlier as the default; the 0.6.4 to 0.6.14 default remains available as `single-column`.
- Keep a sign-in for 30 days with a persistent session cookie, instead of 30 minutes of inactivity or 12 hours.
- Start with an undecodable `apps.yml` or `bookmarks.yml`, and with private entries while login is disabled: the affected module reports the condition, and private entries are still never served without login.
- Serve external edits to `apps.yml` and `bookmarks.yml` on the next request without a restart.
- Accept surrounding quotes, any letter case and `yes`/`no`/`on`/`off` for `FLARE_DISABLE_LOGIN`, `FLARE_VISIBILITY`, `FLARE_CREDENTIAL_MODE` and the boolean switches from every configuration source.

### Added

- Specific reasons for rejected request sources and authorities, with an explanation page in the site language for rejected page navigations.
- Localized causes for failed theme, appearance, search and weather saves.
- A not-ready page that names the rejected configuration input and shows administrator initialization steps only when they apply.
- Release candidates (`0.x.y-rc.N`) that are published without moving `latest`.

### Fixed

- Homepage search and its suggestions were rejected by the write-source check behind TLS-terminating proxies.
- Saving the appearance settings while the title was hidden cleared the greetings.
- The guide page was empty on private sites and behind a Host allow-list, and its markup was invalid elsewhere.
- Declining the editor's save confirmation reported "All changes saved".
- Log timestamps printed the seconds twice and omitted the minute.

Read the [release and upgrade notes](docs/release-0.6.15.md) for the behaviour changes to review before updating.

## [0.6.14] - 2026-11-13

### Security

- Require Go 1.27.4 in `go.mod` and build the manual base image from `golang:1.27.4-alpine3.24`. Go 1.27.4 contains security fixes to cgo, the go command, the compiler, the linker, and `net/http` (GO-2026-6859).

### Changed

- Update Fiber to v3.7.0 and fasthttp to v1.78.0, with gofiber/schema v1.9.3 and gofiber/utils/v2 v2.6.4.
- Update cli-kit to v1.9.4, i18n-kit/v4 to v4.0.6, memfs-kit to v1.1.2, and version-kit/v4 to v4.0.5.
- Update pflag to v1.1.0 and tdewolff/minify/v2 to v2.24.25.
- Refresh Go quality reports and align release documentation and candidate preflight metadata.
- Retain existing configuration formats, authentication defaults, session persistence, and supported archive/container platforms.

## [0.6.13] - 2026-11-02

### Changed

- Update cli-kit to v1.9.3, i18n-kit/v4 to v4.0.4, and version-kit/v4 to v4.0.3, with their required transitive dependencies and checksums.
- Refresh Go quality reports and align release documentation and candidate preflight metadata.
- Retain Go 1.27.3, existing configuration formats, authentication defaults, session persistence, and supported archive/container platforms.

Changes before 0.6.13 are recorded in [GitHub Releases](https://github.com/soulteary/flare/releases) and [historical release records](docs/archive/README.md).

[0.7.0]: https://github.com/soulteary/flare/compare/0.6.15...0.7.0
[0.6.15]: https://github.com/soulteary/flare/compare/0.6.14...0.6.15
[0.6.14]: https://github.com/soulteary/flare/compare/0.6.13...0.6.14
[0.6.13]: https://github.com/soulteary/flare/compare/0.6.12...0.6.13
