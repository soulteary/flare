![Flare banner: a colorful squirrel holding an acorn in an oil-painted landscape](.github/flare-banner.jpg)

# Flare

[![Go Test Coverage](./.github/coverage.svg)](./.github/go-test-report.md) [![Go Report Card](./.github/goreportcard.svg)](./.github/goreportcard-report.md)

Challenge all bookmarking apps and websites directories, Aim to Be a best performance monster.

🚧 **Code is being prepared and refactored, commits are slow.**

## Feature

**Simple**, **Fast**, **Lightweight** and super **Easy** to install and use.

- Written in Go (Golang) and a little Modern vanilla Javascript only.
- HTTP stack: [Echo](https://echo.labstack.com/) v5 by default; an experimental native [Fiber](https://gofiber.io/) v3 build is available with `-tags fiber`.
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

### Experimental Fiber build

The default build continues to use Echo. Select Fiber with the `fiber` build tag:

```sh
# Default Echo build
go run .
go build -o flare-echo .

# Experimental Fiber build
go run -tags fiber .
go build -tags fiber -o flare-fiber .
```

The Fiber application serves routes on its native fasthttp listener. It keeps
`Immutable` enabled and copies retained bound theme strings. The request reader
is configured to 64 KiB; request lines and headers exceeding its capacity can
receive HTTP 431. Request bodies are buffered with a 32 MiB global limit; bookmark
import routes retain their existing upload and commit limits. Embedded favicon
and SVG resources keep a `net/http.ServeContent` adapter for conditional and range
requests. These choices are part of the candidate being measured.

Run the existing baseline for either stack:

```sh
./scripts/baseline-metrics.sh
HTTP_STACK=fiber ./scripts/baseline-metrics.sh
```

For repeated alternating runs and separate server CPU, allocation, GC and RSS
measurements, use `python3 scripts/compare-http-stacks.py`. Add
`--compare-read-buffers` to compare Echo, the current 64 KiB Fiber candidate,
and same-source 8 KiB / 1 MiB Fiber references in the same run. The archived
read-buffer comparison used 5 baseline rounds and 4 resource rounds, so every
target occupied each of the four resource-run positions once. It measured the
redirect path before the subsequent path optimization.

The original baseline includes client allocations and uses the Go client's
default pool of 2 idle connections per host. A supplementary run with
`--keepalive-connections 8` measures a stable pool at concurrency 8. See
[baseline methodology and reproduction](docs/baseline-metrics.md) and the
[64 KiB comparison report](metrics-reports/fiber-64kib-buffer-2026-10-25.md).
The earlier [8 KiB report](metrics-reports/fiber-default-buffer-2026-10-25.md) and
[1 MiB report](metrics-reports/fiber-comparison-2026-10-25.md) describe historical
candidates.

For the redirect investigation, `scripts/analyze-redirect.py` accepts repeated
`--target label=existing-baseline-binary`, a common Echo `--client`, and an
`--output` directory. The current focused comparison uses 8 rounds of 100000
requests after 1000 warmup requests, concurrency 8 and GOMAXPROCS 4, with both
default and 8-connection idle pools. It rotates target order and measures server
CPU, allocations, GC and RSS in isolated processes. `--profiles-only` downloads
server-only CPU and heap profiles for diagnosis; profiled throughput and RSS are
excluded from performance conclusions. See the
[redirect diagnosis and comparison report](metrics-reports/fiber-redirect-profile-2026-10-25.md)
and [reproduction details](docs/baseline-metrics.md#跳转路径独立诊断与复测).
The candidate continues to use a 64 KiB reader.

### Container images

Published stable versions are available from both registries:

```sh
docker pull soulteary/flare:0.6.3
docker pull ghcr.io/soulteary/flare:0.6.3
```

Both images support `linux/amd64`, `linux/arm64`, `linux/arm/v7`, and
`linux/arm/v6`. The `latest` tag follows the current stable release. GHCR copies
the verified Docker Hub image by digest, so both registries contain the same
multi-platform image rather than separate builds. Existing containers still
need to be recreated after pulling an update.

See [docker-flare](https://github.com/soulteary/docker-flare) for deployment
examples and [container publishing](.github/CONTAINER-RELEASES.md) for the release
workflow, verification, and historical backfills.

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
