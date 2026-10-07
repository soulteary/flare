# Flare

Challenge all bookmarking apps and websites directories, Aim to Be a best performance monster.

🚧 **Code is being prepared and refactored, commits are slow.**

## Feature

**Simple**, **Fast**, **Lightweight** and super **Easy** to install and use.

- Written in Go (Golang) and a little Modern vanilla Javascript only.
- HTTP stack: [Echo](https://echo.labstack.com/) v5.
- Doesn't depend on any database or any complicated framework.
- Single executable, no dependencies required, good docker support.
- You can choose whether to enable various functions according to your needs: offline mode, weather, editor, account, and so on.

## ScreenShot

TBD

## Documentation

TBD

- Browse automatically generated program documentation:
    - `godoc --http=localhost:8080`

### Login configuration

Login is disabled by default. To enable it, set `FLARE_DISABLE_LOGIN=false` and
configure `FLARE_COOKIE_SECRET` with a randomly generated value of at least 32
bytes. Generate a key once with `openssl rand -hex 32`, then save it in your
environment or `.env` file. The `--cookie_secret` command-line flag is also
supported. Command-line flags override `.env`, which overrides environment
variables.

When login is enabled, the server refuses to start with an empty or whitespace-only
key, the legacy default `secret`, or a key shorter than 32 bytes after trimming
surrounding whitespace. Login-disabled deployments do not require a key.

Keep the key stable across restarts. Changing it invalidates previously issued
session cookies. When upgrading a deployment that used the published default key,
replace it with a new random key before enabling login.

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
