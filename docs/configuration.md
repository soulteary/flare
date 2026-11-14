# Configuration reference

> Scope: stable 0.6.8 and current source. Legacy 0.6.7 differences are explicit below;
> use [tagged docs](https://github.com/soulteary/flare/tree/0.6.7/docs) for an unchanged
> stable deployment. See [accounts](accounts.md) before initializing or migrating.

Runtime switches use command-line flags > the application's working-directory
`.env` > process environment > defaults. Compose's project `.env` is interpolation
input and is not automatically forwarded into the container: use `environment`
or `env_file`. UI/YAML appearance and search preferences live in `config.yml`.
UI saves apply to subsequent requests. Stop writers before external YAML edits.
Current source observes external edits to `apps.yml` and `bookmarks.yml` on the next
request; releases up to 0.6.14 require a restart after changing them. Stable 0.6.5
also caches `config.yml`; 0.6.6 checks that file on each configuration
read. See [cache boundaries](navigation-cache.md).
Recreate a container after changing its image, Compose environment, or log driver.

## Runtime switches

All rows require a process restart; CLI changes require recreating a container's
command. The default data working directory in published images is `/app`.
Current source restores the 0.4.0 defaults: login disabled and public navigation.
Internal security state is prepared automatically, including in trusted mode.
Previously recorded private defaults remain effective unless explicitly overridden.
Published 0.6.8 used private defaults for new installations; this behavior change
requires a build containing it. Login-enabled deployments still require persistent
credentials or initialization, and invalid security inputs still block startup.

| Environment | CLI | Default | Values and meaning |
| --- | --- | --- | --- |
| `FLARE_PORT` | `--port`, `-p` | `5005` | Integer 1–65535; configure the container port and published mapping consistently |
| `FLARE_DISABLE_LOGIN` | `--disable_login`, `-l` | `true` (current source) | Boolean; false requires administrator authentication; recorded private defaults are preserved; a value that is not a boolean blocks startup |
| `FLARE_VISIBILITY` | `--visibility`, `-s` | `DEFAULT` (current source) | `DEFAULT` (public visible resources) or `PRIVATE` (whole-site authentication), in any letter case; never use `0`/`1`; any other value blocks startup |
| `FLARE_USER` | — | `flare` | Login username |
| `FLARE_PASS` | — | No persistent password supplied | Initial/deployment password; current startup does not print a generated password; local mode retains stored credentials |
| `FLARE_PASS_FILE` | — | Unset | Current main: password file, conflicts with nonempty effective `FLARE_PASS`; see format below |
| `FLARE_STATE_DIR` | — | `.flare-security` | Current main: writable persistent security directory, relative to working directory unless absolute |
| `FLARE_CREDENTIAL_MODE` | — | Login disabled: `trusted`; new login account: `local`; migrated credentials: `deployment` | Current main: `local` or `deployment`; existing mode changes require explicit account/maintenance action |
| `FLARE_EXTERNAL_URL` | — | Unset | Current main: public HTTP(S) origin without credentials, query or application subpath; accepted as a write source when a proxy rewrites Host, and an HTTPS value enforces Secure cookies. Current source no longer rejects other Host values; 0.6.8–0.6.14 do |
| `FLARE_ALLOWED_HOSTS` | — | Unset | Current main: comma-separated host/port authorities; when set, any other Host (except the one in `FLARE_EXTERNAL_URL`) receives HTTP 400 |
| `FLARE_TRUSTED_PROXIES` | — | Unset | Current main: comma-separated peer IPs/CIDRs allowed to supply supported forwarding headers |
| `FLARE_EDITOR` | `--enable_editor`, `-e` | `true` | Boolean; expose editor and transfer routes |
| `FLARE_GUIDE` | `--guide`, `-g` | `true` | Boolean; expose the guide at `/guide`, a walkthrough rendered on the visitor's own homepage |
| `FLARE_OFFLINE` | `--offline`, `-o` | `false` | Boolean; disable server-side weather/location activity |
| `FLARE_MINI_REQUEST` | `--mini_request`, `-m` | `false` | Boolean; inline icon SVGs, trade fewer HTTP requests for larger HTML |
| `FLARE_DEPRECATED_NOTICE` | `--enable_notice`, `-n` | `true` | Boolean; deprecated-feature notices |
| `FLARE_DISABLE_CSP` | `--disable_csp`, `-c` | `false` | Boolean; disabling the page script policy reduces browser protection |
| `FLARE_COOKIE_NAME` | `--cookie_name`, `-k` | `flare` | Session cookie prefix; current main also includes the instance identity |
| `FLARE_COOKIE_SECRET` | `--cookie_secret`, `-r` | Legacy: secure initialized key | Stable/legacy signed-cookie setting (at least 32 bytes); does not replace current-main security state |
| `FLARE_COOKIE_SECURE` | `--cookie_secure` | `false` | Boolean; force HTTPS-only cookies behind TLS termination, after which pages opened over HTTP cannot sign in or save; direct TLS is secure automatically. Current source no longer needs it for writes behind TLS termination; 0.6.8–0.6.14 do |
| `FLARE_DEBUG` | — | Unset | Environment-only application log level; literal `on` enables DEBUG; independent of development assets |
| `FLARE_ACCESS_LOG` | `--access_log` | `errors` | `errors`, `all`, `off`; since 0.6.4; invalid mode warns and uses errors |

Releases through 0.6.14 accept only the standard true/false and 1/0 forms from
the environment and the command line, and treat a quoted environment value as
invalid. Current source reads booleans the same way from the environment, `.env`
and the command line: `true`/`false`, `1`/`0`, `yes`/`no` and `on`/`off` in any
letter case. Surrounding whitespace and one pair of matching quotes are ignored
for booleans, `FLARE_VISIBILITY` and `FLARE_CREDENTIAL_MODE`, so the Compose list
form `- FLARE_DISABLE_LOGIN="true"` is read as `true`. Legacy `--nologin` and
`--mr` are deprecated; use the names above. `FLARE_DEBUG=on` enables application DEBUG logs but does not enable development asset mode.
Check `flare --help` for the executable you actually run.

Do not rely on an invalid configuration to select a safe access mode. Use explicit
valid values and verify anonymous access after startup. Kubernetes can inject
`FLARE_PORT=tcp://...`; explicitly set a numeric `FLARE_PORT` or disable Service
injection with `enableServiceLinks: false`. Inspect startup warnings and the actual
running version rather than assuming malformed values preserve access controls.

In current source, a `FLARE_DISABLE_LOGIN`, `FLARE_VISIBILITY` or
`FLARE_CREDENTIAL_MODE` value that cannot be interpreted, and an unknown or
malformed command-line flag, keep the instance not ready (HTTP 503) instead of
selecting a default. The not-ready page and the startup log name the rejected
input, its source and the accepted values; neither repeats the supplied value.
Administrator initialization steps appear on that page only when login is enabled
and no administrator exists. Other boolean switches set through the environment
or `.env` that cannot be interpreted keep their default or earlier value and log
a warning.

## Access models

| Goal | `FLARE_DISABLE_LOGIN` | `FLARE_VISIBILITY` | Result |
| --- | --- | --- | --- |
| Trusted local, no sign-in | `true` | `DEFAULT` | Navigation and enabled administration available without sign-in |
| Public navigation, protected editor | `false` | `DEFAULT` | Navigation public; administration requires sign-in |
| Private navigation and editor | `false` | `PRIVATE` | Navigation and administration require sign-in |

Example private deployment, terminating HTTPS at a reverse proxy:

```yaml
environment:
  FLARE_DISABLE_LOGIN: "false"
  FLARE_VISIBILITY: "PRIVATE"
  FLARE_USER: "your_user"
  FLARE_PASS: "replace_with_a_strong_password"
  FLARE_COOKIE_SECURE: "true"
```

Use both privacy settings together. Current main rejects `PRIVATE` with disabled
login at startup. Private entries/categories in trusted anonymous mode do not stop
the instance: the collection that contains them is withheld as a whole until login
is enabled or the flags are removed; see [access models](accounts.md#access-models-and-private-inheritance).
It does not clear private flags or fall back to public access. Stable 0.6.7 does not enforce
per-resource flags: use whole-site PRIVATE to protect all resources. Hiding the
settings button only changes presentation. Encoded redirects are not encryption
or authorization; current main uses opaque handles and rechecks resource access
for both new and legacy redirects.
After changing access settings, test navigation and editor access in a fresh,
unauthenticated browser; do not infer protection from the settings UI alone.

## Application preferences

| File/block | Scope | Guide |
| --- | --- | --- |
| `apps.yml` / `bookmarks.yml` | Navigation entries and categories | [Editor](editor.md), [transfer](bookmark-transfer.md) |
| `config.yml` `Clock` | Display time, seconds, zone; since 0.6.3 | [Clock](clock.md) |
| `config.yml` `Search` | Engines/hybrid since 0.6.5; additional modes and save protections in 0.6.6 | [Search](search.md) |
| `config.yml` `Homepage` | Layout controls since 0.6.5; adaptive tablet columns in 0.6.6. `IgnoreThemeLayout` (current source) turns off the layout that themes specify | [Mobile](mobile-layout.md), [desktop](desktop-columns-and-scale.md), [layout of a theme](theme-files.md#layout) |
| `config.yml` `ThemeSettings` | Per theme: the layout options of the theme that are turned off (current source) | [Layout of a theme](theme-files.md#layout) |
| `config.yml` `Theme`, `ThemeCustom` | A bundled theme name, the name of a theme file (current source), or `custom` with three `#RRGGBB` colors in `ThemeCustom` (current source); read at startup | [Themes and custom colors](theme-colors.md), [theme files](theme-files.md) |
| `config.yml` `ThemeMode` | `auto`, `light` or `dark` for a theme with light and dark values (current source); read at startup | [Light and dark](theme-files.md#light-and-dark) |
| `themes/*.yml`, `themes/*/theme.yml` | Themes of the instance, one per file or per directory (current source); optional | [Theme files](theme-files.md) |
| `config.yml` `Footer` | Administrator-controlled internal HTML | Treat it as trusted site content, unlike ordinary bookmark text |
| `config.yml` `LoginFooter` | Current main: public login-page text/filtered HTTP(S) links | [Accounts](accounts.md); separate from internal Footer |

`config.yml` also supports `HideVersion: true` (default: `false`), available in
Settings → Appearance. It removes the application version, commit and build date
from the About section for all visitors. Saving takes effect on subsequent requests.
CLI version output and startup logs remain available for local diagnostics.

Search settings are not included in bookmark exports. Multiple processes sharing
YAML do not share in-process locks. Use one writer and normal filesystem backups.


## Navigation URL and icon policy (since 0.6.5)

Since 0.6.5, rendered navigation text/attributes are escaped. Supported schemes
are HTTP(S), relative links, ftp, ftps, mailto, tel, ssh, smb, rdp, vnc, magnet,
steam, obsidian and chrome-extension. Executable/document schemes and unsupported
protocols remain editable in stored data but render as `#`; do not assume an
arbitrary custom scheme is clickable. Icons accept bundled identifiers or
absolute HTTP(S) image URLs. The administrator-controlled Footer intentionally
supports HTML and must be treated as trusted site content.

## Dynamic bookmark placeholders

For a request to `http://nas.example:5005/bookmarks?search=docs`:

| Placeholder | Value |
| --- | --- |
| `{host}` | `nas.example:5005` |
| `{hostname}` | `nas.example` |
| `{origin}` | `http://nas.example:5005` |
| `{href}` | `http://nas.example:5005/bookmarks?search=docs` |
| `{pathname}` | `/bookmarks` |
| `{port}` | `5005` (or the scheme default when absent) |
| `{protocol}` | `http:` |

Use `{origin}/service` to follow the current authority, or explicit addresses when
services have different hosts/ports. `{host}` retains IPv6 brackets; `{hostname}`
is the unbracketed literal, so do not splice it into an IPv6 URL authority without
brackets. These values come from the request Flare receives; a TLS-terminating
HTTP backend still sees HTTP unless the server directly receives TLS. This is not
a general trusted-proxy feature or a two-address network-switching configuration.
Since 0.6.5, encoded redirects carry validated relative source context so path
placeholders continue to refer to the originating navigation page.

## Private resources and persistent state (current main)

`private: true` on an entry or bookmark category restricts it to the single
administrator; category restrictions cannot be relaxed by child `private: false`.
Missing/false fields retain existing meaning. See [accounts](accounts.md) for
sessions, password ownership and maintenance, and [upgrade/rollback](login-private-upgrade.md)
for migration and YAML anchor/alias read-only boundaries.

`FLARE_PASS_FILE` is a regular UTF-8 file (projected Secret symlinks are accepted).
Input is limited to 514 bytes; remove one terminal LF/CRLF only, preserving other
whitespace, with at most 512 password bytes after removal. A nonempty effective
`FLARE_PASS` cannot be combined with it. New-password strength rules and old
credential compatibility are described in the account guide.

Security state must remain writable and persistent even with read-only YAML.
Persist custom state directories and external password files separately if they
are outside the data volume. Only one local-filesystem writer is supported;
shared cookie secrets do not provide multi-replica support. See [backup/restore](backup-restore.md).

Deprecated boolean aliases `--nologin` and `--mr` use last-occurrence precedence
since 0.6.5. Prefer canonical `--disable_login` and `--mini_request`.
