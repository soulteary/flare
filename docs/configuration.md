# Configuration reference

Runtime switches use command-line flags > the application's working-directory
`.env` > process environment > defaults. Compose's project `.env` is interpolation
input and is not automatically forwarded into the container: use `environment`
or `env_file`. UI/YAML appearance and search preferences live in `config.yml`.
UI saves apply to subsequent requests. Stop writers before external YAML edits.
Restart after changing `apps.yml` or `bookmarks.yml`. Stable 0.6.5 also caches
`config.yml`; the unpublished 0.6.6 candidate checks that file on each configuration
read. See [cache boundaries](navigation-cache.md).
Recreate a container after changing its image, Compose environment, or log driver.

## Runtime switches

All rows require a process restart; CLI changes require recreating a container's
command. The default data working directory in published images is `/app`.

| Environment | CLI | Default | Values and meaning |
| --- | --- | --- | --- |
| `FLARE_PORT` | `--port`, `-p` | `5005` | Integer 1–65535; configure the container port and published mapping consistently |
| `FLARE_DISABLE_LOGIN` | `--disable_login`, `-l` | `true` | Boolean; false enables authentication for administration |
| `FLARE_VISIBILITY` | `--visibility`, `-s` | `DEFAULT` | `DEFAULT` (public navigation) or `PRIVATE` (authentication required); never use `0`/`1` |
| `FLARE_USER` | — | `flare` | Login username |
| `FLARE_PASS` | — | Generated | Login password; absent password is generated and shown in startup logs |
| `FLARE_EDITOR` | `--enable_editor`, `-e` | `true` | Boolean; expose editor and transfer routes |
| `FLARE_GUIDE` | `--guide`, `-g` | `true` | Boolean; expose guide |
| `FLARE_OFFLINE` | `--offline`, `-o` | `false` | Boolean; disable server-side weather/location activity |
| `FLARE_MINI_REQUEST` | `--mini_request`, `-m` | `false` | Boolean; inline icon SVGs, trade fewer HTTP requests for larger HTML |
| `FLARE_DEPRECATED_NOTICE` | `--enable_notice`, `-n` | `true` | Boolean; deprecated-feature notices |
| `FLARE_DISABLE_CSP` | `--disable_csp`, `-c` | `false` | Boolean; disabling the page script policy reduces browser protection |
| `FLARE_COOKIE_NAME` | `--cookie_name`, `-k` | `flare` | Session cookie prefix; includes internal listen port |
| `FLARE_COOKIE_SECRET` | `--cookie_secret`, `-r` | Secure initialized key | At least 32 bytes; persist automatically created `.flare-cookie-secret` |
| `FLARE_COOKIE_SECURE` | `--cookie_secure` | `false` | Boolean; force HTTPS-only cookies behind TLS termination; direct TLS is secure automatically |
| `FLARE_DEBUG` | — | Unset | Environment-only application log level; literal `on` enables DEBUG; independent of development assets |
| `FLARE_ACCESS_LOG` | `--access_log` | `errors` | `errors`, `all`, `off`; since 0.6.4; invalid mode warns and uses errors |

Booleans accept the parser's standard true/false and 1/0 forms. Prefer literal
`"true"`/`"false"` strings in Compose. Legacy `--nologin` and `--mr` are deprecated;
use the names above. `FLARE_DEBUG=on` enables application DEBUG logs but does not enable development asset mode.
Check `flare --help` for the executable you actually run.

Do not rely on an invalid configuration to select a safe access mode. Use explicit
valid values and verify anonymous access after startup. Kubernetes can inject
`FLARE_PORT=tcp://...`; explicitly set a numeric `FLARE_PORT` or disable Service
injection with `enableServiceLinks: false`. Inspect startup warnings and the actual
running version rather than assuming malformed values preserve access controls.

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

Use both privacy settings together. `PRIVATE` cannot authenticate a visitor when
login has been disabled. Hiding the settings button only changes presentation.
Encoding a link is Base64 plus redirect matching, not encryption or access control.
After changing access settings, test navigation and editor access in a fresh,
unauthenticated browser; do not infer protection from the settings UI alone.

## Application preferences

| File/block | Scope | Guide |
| --- | --- | --- |
| `apps.yml` / `bookmarks.yml` | Navigation entries and categories | [Editor](editor.md), [transfer](bookmark-transfer.md) |
| `config.yml` `Clock` | Display time, seconds, zone; since 0.6.3 | [Clock](clock.md) |
| `config.yml` `Search` | Engines/hybrid since 0.6.5; additional modes and save protections in the 0.6.6 candidate | [Search](search.md) |
| `config.yml` `Homepage` | Layout controls since 0.6.5; adaptive tablet columns in the 0.6.6 candidate | [Mobile](mobile-layout.md), [desktop](desktop-columns-and-scale.md) |
| `config.yml` `Footer` | Administrator-controlled HTML | Treat it as trusted site content, unlike ordinary bookmark text |

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

Per-entry `private` values are preserved for data compatibility; they do not
implement a per-entry access-control list. Use whole-site authentication for
private navigation, and never rely on a stored flag, link encoding, tabs or hidden
buttons to protect individual links on a public homepage.


Deprecated boolean alias resolution (`--nologin` and `--mr`) was repaired in
0.6.5. Both spellings now use normal last-occurrence precedence. Prefer canonical
`--disable_login` and `--mini_request`; do not assume legacy spellings enforce
access settings when running 0.6.4.
