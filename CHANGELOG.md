# Changelog

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

[0.6.14]: https://github.com/soulteary/flare/compare/0.6.13...0.6.14
[0.6.13]: https://github.com/soulteary/flare/compare/0.6.12...0.6.13
