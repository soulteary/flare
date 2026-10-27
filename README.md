![Flare banner: a colorful squirrel holding an acorn in an oil-painted landscape](.github/flare-banner.jpg)

# Flare

[![Go Test Coverage](./.github/coverage.svg)](./.github/go-test-report.md) [![Go Report Card](./.github/goreportcard.svg)](./.github/goreportcard-report.md) [![CI results](https://github.com/soulteary/flare/actions/workflows/ci.yml/badge.svg)](https://github.com/soulteary/flare/actions/workflows/ci.yml)
[![Go quality reports and source verification](https://img.shields.io/badge/Go-quality%20reports-00ADD8?logo=go&logoColor=white)](.github/GO-REPORTS.md)

A lightweight self-hosted bookmark dashboard and start page, written in Go.

The latest stable release is [**0.6.8**](https://github.com/soulteary/flare/releases/tag/0.6.8),
including persistent administrator accounts, revocable sessions, resource privacy
and navigation caching. Read the [upgrade requirements](docs/release-0.6.8.md) before updating.
See the [documentation index](docs/README.md) for version scope and usage.

## Features

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

## Screenshots

See the [deployment screenshots](https://github.com/soulteary/docker-flare#快速上手).
See the [English and Chinese editor screenshots](docs/screenshots/README.md) for
the editor shipped in 0.6.5; older deployment screenshots show the earlier UI.

## Documentation

Start with the [documentation index](docs/README.md), including configuration,
backup/restore, reverse proxies, development builds and release availability.

### Build and HTTP behavior

```sh
go run build/build.go
go build -o /tmp/flare .
/tmp/flare
```

Generate resources after a clean checkout or changes under `embed/`; see
[build and test instructions](docs/development.md).

Production and container builds use Fiber on its native fasthttp listener.
Native-listener regressions exercise that path; HTTP integration tests also use a
`net/http` compatibility adapter. Current source uses persistent server-side
authentication; the legacy Gorilla cookie adapter remains for compatibility tests
and embedders. See [accounts and private access](docs/accounts.md) for version scope.
Request ownership uses `Immutable`; editor upload limits remain enforced.
The 64 KiB reader limits request lines and headers; requests exceeding its capacity can receive HTTP 431. Bodies have a
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
docker pull soulteary/flare:0.6.8
docker pull ghcr.io/soulteary/flare:0.6.8
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

### Accounts and private access

**Version scope:** stable 0.6.8 includes persistent single-administrator accounts,
session revocation and per-entry/category privacy. Stable 0.6.7 retains the legacy
login model.
Use [the documentation index](docs/README.md) to choose the matching instructions.

For 0.6.7, use `FLARE_DISABLE_LOGIN=false` to protect administration and also
`FLARE_VISIBILITY=PRIVATE` to protect the whole site. Per-entry `private` flags
alone do not protect data in 0.6.7. Preserve `.flare-cookie-secret` and configure
`FLARE_COOKIE_SECURE=true` when HTTPS terminates at a reverse proxy.

For 0.6.8 and current source, new installations require administrator initialization and
use private-site defaults unless explicitly configured otherwise. Persist the
security directory as well as YAML. Read [accounts](docs/accounts.md),
[configuration](docs/configuration.md), [upgrade/rollback](docs/login-private-upgrade.md),
[backup/restore](docs/backup-restore.md) and [reverse proxies](docs/reverse-proxy.md)
before changing an existing deployment.

## Date, time and time zones

The optional live clock supports server, visitor-device and custom IANA time zones,
with a static no-JavaScript fallback. Existing configurations keep their original
static display. See [clock configuration and behavior / 日期时间配置说明](docs/clock.md).

## Project layout

```text
build/                Resource generation
cmd/                  Command-line and environment configuration
config/               Runtime models and configuration persistence
embed/                Web assets, templates and translations
internal/auth/        Authentication, accounts and request authorization
internal/security/    Persistent single-administrator security state
internal/requestsource/ Explicit external-origin and trusted-proxy policy
internal/bookmarks/   Bookmark import and export
internal/clock/       Clock rendering and time zones
internal/pages/       Homepage, editor and guide handlers
internal/resources/   Embedded assets, icons and templates
internal/search/      Search validation and local matching
internal/searchui/    Search presentation
internal/server/      HTTP routing and request protections
internal/settings/    Settings handlers
scripts/              Performance and release verification tools
tests/                Browser, container and release acceptance
docs/                 User, operator and development guides
metrics-reports/      Historical measurements and release evidence
main.go               Application entry point
```
