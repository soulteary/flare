![Flare banner: a colorful squirrel holding an acorn in an oil-painted landscape](.github/flare-banner.jpg)

# Flare

[![Go Test Coverage](./.github/coverage.svg)](./.github/go-test-report.md) [![Go Report Card](./.github/goreportcard.svg)](./.github/goreportcard-report.md)

Challenge all bookmarking apps and websites directories, Aim to Be a best performance monster.

🚧 **Code is being prepared and refactored, commits are slow.**

## Feature

**Simple**, **Fast**, **Lightweight** and super **Easy** to install and use.

- Written in Go (Golang) and a little Modern vanilla Javascript only.
- HTTP stack: native [Fiber](https://gofiber.io/) v3 with a 64 KiB request reader.
- Doesn't depend on any database or any complicated framework.
- Single executable, no dependencies required, good docker support.
- You can choose whether to enable various functions according to your needs: offline mode, weather, editor, account, and so on.

### Bookmark import and export

The editor includes browser HTML and native Flare YAML import/export, with
preview, duplicate handling, conflict detection, and automatic pre-import backups.
See [Bookmark transfer](docs/bookmark-transfer.md) for usage and limits.

## ScreenShot

TBD

## Documentation

TBD

- Browse automatically generated program documentation:
    - `godoc --http=localhost:8080`

### Build and HTTP behavior

```sh
go run .
go build -o flare .
```

All normal builds, container builds and tests use Fiber on its native fasthttp
listener. The migration keeps Gorilla signed session cookies, direct TLS and
explicit proxy Secure-cookie behavior, `Immutable` request ownership, and the
existing editor upload limits. The 64 KiB reader limits request lines and
headers; requests exceeding its capacity can receive HTTP 431. Bodies have a
32 MiB global limit. Embedded favicon and SVG resources retain
`net/http.ServeContent` for conditional and range responses.

### Performance records

[Performance decisions and complete experiment index](docs/performance.md)
records the 1 MiB, default 8 KiB and 64 KiB comparisons, raw per-round data,
server CPU/allocation profiles, and redirect optimization. The final choice is
64 KiB: memory is close to 8 KiB while allowing larger request headers and URLs.
The measurements do not establish that Fiber uses less RSS than Echo.

Run the current Fiber baseline with `./scripts/baseline-metrics.sh`. The original
in-process baseline includes client allocations; separate server measurements
and profiling use `scripts/analyze-redirect.py`. Historical Echo/Fiber comparisons
are replayed from a frozen experiment commit by `scripts/compare-http-stacks.py`;
Echo is no longer part of the production dependency graph. See
[methods and reproduction](docs/baseline-metrics.md) for the two measurement scopes.

### Container images

Published stable versions are available from both registries:

```sh
docker pull soulteary/flare:0.6.4
docker pull ghcr.io/soulteary/flare:0.6.4
```

Both images support `linux/amd64`, `linux/arm64`, `linux/arm/v7`, and
`linux/arm/v6`. The `latest` tag follows the current stable release. GHCR copies
the verified Docker Hub image by digest, so both registries contain the same
multi-platform image rather than separate builds. Existing containers still
need to be recreated after pulling an update.

See [docker-flare](https://github.com/soulteary/docker-flare) for deployment
examples and [container publishing](.github/CONTAINER-RELEASES.md) for the release
workflow, verification, and historical backfills.

### Access logs and Docker log rotation

HTTP access logs now default to failed requests (returned errors and HTTP 4xx/5xx).
Set `FLARE_ACCESS_LOG=all` or `--access_log=all` to restore ordinary access logs,
or use `off` to disable access logs. Application startup and diagnostic logs keep
their existing levels. The `all` mode keeps the existing successful health-check,
static-resource and redirect filters.

Container deployments also need Docker log rotation to bound retained logs.
See [logging configuration and upgrade steps / 日志配置与升级说明](docs/logging.md)
for the `local` driver with `10m` files and a limit of 3 files, configuration
precedence, and rebuilding existing containers while preserving application data.

### Login configuration

Login is disabled by default. To enable it, set `FLARE_DISABLE_LOGIN=false`.
You can configure `FLARE_COOKIE_SECRET` with a random value of at least 32 bytes,
generated once with `openssl rand -hex 32`, in your environment or `.env` file.
The `--cookie_secret` flag is also supported. Command-line flags override `.env`,
which overrides environment variables. A suitable configured key takes precedence
over an automatically saved key.

If the configured key is empty, the legacy default `secret`, or shorter than 32
bytes after trimming surrounding whitespace, Flare initializes a secure key and
continues starting. It generates 32 random bytes, stores them as 64 hexadecimal
characters in `.flare-cookie-secret` beside `config.yml`, and reuses the saved key
on later starts. The file is readable and writable only by the application user.
Keep this file in your persistent data volume along with the other configuration
files. Flare does not rewrite your environment or `.env` file.

Startup warnings explain the reason for initialization and whether a new key was
saved or an existing key was reused. The key itself is never logged. Replacing a
previous key invalidates existing session cookies, so users must log in again.
Login-disabled deployments do not create or require a key file.

If the saved key cannot be read or validated, or a new key cannot be saved, Flare
uses a secure key for the current process and warns that it was not persisted.
The service remains available, but users must log in again after a restart.
If a key file has already been saved and can be read, Flare keeps using that key
even when syncing its directory fails. A separate warning explains that the file
may not survive a power loss; ordinary restarts and other instances reuse it.
Multiple instances should use the same configured key or share the persistent
key file so that their session cookies remain compatible.

#### Session cookies and HTTPS proxies

Session cookies use `HttpOnly`, `SameSite=Lax`, `Path=/`, and a 30-day lifetime.
Plain HTTP deployments work by default. Requests received directly over TLS
automatically use `Secure` cookies.

**For HTTPS terminated by a reverse proxy, set `FLARE_COOKIE_SECURE=true`.**
Flare's built-in server listens over HTTP, so this setting preserves the
`Secure` attribute when the browser uses HTTPS but the proxy connects to Flare
over HTTP. Clients must then access the application using HTTPS.

```env
FLARE_DISABLE_LOGIN=false
FLARE_COOKIE_SECURE=true
```

`FLARE_COOKIE_SECURE` defaults to `false` and is supported in the environment,
the `.env` file, and as `--cookie_secure=true` or `--cookie_secure=false`.
The same precedence applies: command-line flags override `.env`, which
overrides environment variables. A value of `false` still allows direct TLS
requests to use `Secure`. Flare does not infer this policy from untrusted
`X-Forwarded-Proto`, `Forwarded`, or similar request headers.

## Date, time and time zones

The optional live clock supports server, visitor-device and custom IANA time zones,
with a static no-JavaScript fallback. Existing configurations keep their original
static display. See [clock configuration and behavior / 日期时间配置说明](docs/clock.md).

## Directory

```bash
├── build                   build script
├── cmd                     user cli/env parser
├── config                  config for app
│   ├── data                    data for app running
│   ├── define                  define for app launch
│   └── model                   data model for app
├── docker                  docker
├── embed                   resource (assets, template) for web
├── internal
│   ├── auth                user login
│   ├── fn                  fn utils
│   ├── logger              logger
│   ├── misc
│   │   ├── deprecated
│   │   ├── health
│   │   └── redir
│   ├── pages
│   │   ├── editor
│   │   ├── guide
│   │   └── home
│   ├── resources           static resource after minify
│   ├── server
│   ├── settings
│   └── version
└── main.go
```
