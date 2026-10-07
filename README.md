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
Multiple instances should use the same configured key or share the persistent
key file so that their session cookies remain compatible.

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
