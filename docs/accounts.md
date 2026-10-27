# Accounts and private resources

> Scope: stable 0.6.8 and current source. These persistent-account and
> per-resource authorization features are not included in legacy 0.6.7.

## Stable 0.6.7

Stable 0.6.7 defaults to disabled login. Enable `FLARE_DISABLE_LOGIN=false` for
protected administration; add `FLARE_VISIBILITY=PRIVATE` for whole-site protection.
Its per-entry/category `private` fields do not enforce resource access. It uses
Gorilla signed cookies with a 30-day lifetime; preserve `.flare-cookie-secret`
or supply a strong `FLARE_COOKIE_SECRET`. Those legacy cookie settings are not
the current-main persistent-session mechanism. Old instructions and measurements
remain available through the [archive](archive/README.md).

## Current source: start without account initialization

Current source restores the 0.4.0 access defaults: login is disabled and navigation
is public (`FLARE_DISABLE_LOGIN=true`, `FLARE_VISIBILITY=DEFAULT`). A new instance
starts immediately and automatically prepares its persistent internal security
state; no administrator, password, or maintenance command is required. Anyone
who can reach the instance can browse and modify content when the editor is enabled.
Setting a password alone does not enable login: set `FLARE_DISABLE_LOGIN=false`.

Existing installations created with the previous private-default policy retain
that recorded policy unless explicitly overridden. This change does not silently
publish an already protected installation. The published 0.6.8 release used login
enabled and PRIVATE defaults for new installations; use a build containing this
change for the new behavior, rather than assuming an existing image has changed.

### CSRF in current source

Login and CSRF protect different things. In trusted mode, anyone who can reach
the instance may administer it, so management writes still require a valid
browser-bound CSRF token and accepted request origin. Disabling login does not
turn off those checks. A read-only navigation request does not need to submit a
token merely because its visitor is anonymous.

The homepage creates a token only while its search form is visible. With
`ShowSearchComponent: false`, homepage rendering does not issue or renew a
preauth cookie. Settings and editor forms obtain their own tokens independently,
so hiding search does not affect their write protection. A browser may still
send an existing cookie on later navigation requests.

### 从源码运行

按[开发说明](development.md)准备好环境后，在项目目录直接运行：

```sh
go run .
```

打开 <http://localhost:5005> 即可浏览、调整设置和编辑书签，无需初始化账户。
内部安全状态自动写入 `.flare-security/`，后续重启继续使用，不要删除。

### Docker Compose 部署

使用包含本次变更的镜像，并持久化整个 `/app`（服务配置中的
`volumes: ["./app:/app"]`），在原 Compose 项目目录运行：

```sh
docker compose up -d flare
```

服务名不同则替换 `flare`。默认即可访问映射的端口并编辑，内部状态自动保存到
宿主机的 `./app/.flare-security/`。只挂载 YAML 文件时，应另加
`./security:/app/.flare-security`；自定义 `FLARE_STATE_DIR` 时须持久化对应目录。

### docker run 部署

假设已将包含本次变更的镜像构建或标记为 `flare:local`：

```sh
docker run -d --name flare -p 5005:5005 \
  -v "$PWD/app:/app" \
  flare:local
```

无需另外启动初始化容器。已有部署继续使用原来的数据挂载、命名卷、工作目录
和相关环境变量；重建时不要换成空卷。默认工作目录是 `/app`。

## Optional: enable login

Enable `FLARE_DISABLE_LOGIN=false` to require authentication for management.
Keep `FLARE_VISIBILITY=DEFAULT` for public navigation, or set it to `PRIVATE`
to require login for the whole site. On first account setup, supply `FLARE_PASS`
or `FLARE_PASS_FILE` at startup to initialize automatically, without a separate
maintenance command. Startup does not print passwords or silently enable anonymous
access when login is requested but credentials are missing.

Alternatively, before the first start, initialize interactively from the same
working directory and effective environment, while the service is stopped:

```sh
go run . security init --username admin
FLARE_DISABLE_LOGIN=false go run .
```

A fresh interactive `security init` retains the protected private-default setup.
New passwords require 15–128 Unicode code points, at most 512 UTF-8 bytes, and
must pass the bundled weak/repeated-password checks. Terminal input is hidden
and asks twice. Existing nonempty deployment passwords can migrate without this
new-password strength requirement.

If the instance has already started in trusted mode, its internal state is already
initialized. To create a locally managed administrator, stop the service and use
`security set-credentials --mode local --username admin` instead of `security init`,
then enable login in the deployment configuration and restart.

For Docker Compose, the interactive first-account alternative is:

```sh
docker compose stop flare
docker compose run --rm --no-deps flare flare security init --username admin
docker compose up -d flare
```

Configure `FLARE_DISABLE_LOGIN: "false"` under the service's `environment` before
starting. For an already initialized trusted instance, replace the middle command
with `docker compose run --rm --no-deps flare flare security set-credentials --mode local --username admin`.
Use the original Compose files, project name, mounts and environment throughout.

For `docker run`, stop the existing service and use a temporary container with the
same image, mounts, working directory and relevant environment:

```sh
docker stop flare
docker run --rm -it -e FLARE_DISABLE_LOGIN=false \
  -v "$PWD/app:/app" \
  flare:local flare security init --username admin
```

Use `security set-credentials --mode local --username admin` for existing trusted
state. Only after success, recreate the service with login enabled, preserving its
original data. `docker start` alone cannot change an existing container's environment.
Do not run maintenance with `docker exec` while the service holds the state lock.

Contradictory access settings, private content in trusted mode, and damaged or
unwritable security state still block startup with HTTP 503. Check terminal or
container logs; do not delete security state to bypass the error.

Persist `.flare-security/` (or `FLARE_STATE_DIR`) on writable local storage. The
process holds an exclusive lock; stop it before maintenance commands. Only a
single writer on a local POSIX filesystem is supported. Sharing cookie keys does
not enable multiple replicas; network-disk concurrent writers and native Windows
are outside this implementation's support boundary.

## Access models and private inheritance

- Login enabled + `DEFAULT`: public navigation contains public resources only;
  the single administrator can view all resources and use administration.
- Login enabled + `PRIVATE`: all navigation requires the administrator session.
- Login disabled + `DEFAULT`: trusted-network mode allows anonymous management.
  It must not contain private categories or entries.
- Login disabled + `PRIVATE`, invalid security input, or private resources in
  trusted mode: startup is blocked until the operator corrects the source.

`private: true` on a bookmark category restricts all its children. An entry's
`private: false` cannot override its category. Missing/false values retain public
meaning where the parent permits it. This also applies to private application
entries, which have no categories. Homepage, search/suggestions and redirects
filter current visibility. Hidden buttons and encoded URLs do not grant privacy.
Unknown or inaccessible requested categories return 404 in current source.

For example, in `bookmarks.yml` (lowercase YAML keys):

```yaml
categories:
  - id: internal
    title: Internal services
    private: true
links:
  - name: Admin dashboard
    link: https://admin.example.com/
    category: internal
    private: false
```

The entry remains private because its category is private. Stop writers before
manual YAML edits and restart after editing navigation files. This example alone
does not protect an old 0.6.7 public deployment.

Private navigation icons may still contact a configured remote host when an
administrator views them. Offline mode is not a browser network isolation policy.

## Password ownership and account actions

Open Settings → Application (`/settings/application`) for the account page.
It shows the current password source, site/access diagnostics and account actions.

`deployment` mode follows the effective deployment username/password source.
Changing that source and restarting applies the credentials and revokes old
sessions; restarting with identical credentials does not repeatedly revoke them.
To change the password in the application, verify the existing password and
explicitly take local control. In `local` mode, leftover `FLARE_PASS` no longer
overrides the application's password. Change back only with the stopped-service
`security set-credentials --mode deployment` command and the intended deployment
source. Do not merely change `FLARE_CREDENTIAL_MODE` on an initialized instance.

Sessions are stored by token digest, with a 30-minute idle limit and a 12-hour
absolute limit. Eligible foreground activity renews idle time; background
weather/clock work does not. Reauthentication rotates the token without extending
the original absolute limit. Recent password verification lasts 10 minutes for
sensitive operations. Ordinary restarts preserve unexpired sessions. Sign out
revokes the current session; Sign out all, password changes and password-source
switches revoke the relevant old sessions.

Cookies are host-only, HttpOnly, SameSite=Lax and Path=/; current session cookies
are browser-session cookies, distinct from server-side expiry. Direct TLS uses
Secure. For HTTPS termination, configure the [proxy policy](reverse-proxy.md).

## Recovery and maintenance

All modifying maintenance commands require a stopped service and the same state
directory and effective inputs. `--state-dir PATH` can select a maintenance state
directory. Password input may use `--password-file PATH`; there is no plaintext
password argument.

- `security migration-check --json`: read-only inspection. Read `status` even
  after exit 0; `new` or `attention_required` is not a ready deployment. Exit 2
  means input/usage error, 3 indeterminate/lock-related inspection, 4 state/output
  failure; retain the diagnostic when investigating.
- `security init --username NAME`: initialize an administrator.
- `security reset-password`: reset a local password; taking over deployment
  ownership requires explicit `--take-local-control`.
- `security set-credentials --mode local|deployment`: explicit ownership switch.
- `security revoke-all`: invalidate sessions after a complete instance restore.
- `security adopt-legacy-defaults`: explicitly retain a verified old default
  trusted deployment; never use it to bypass a private-content conflict.

Never delete the security directory to recover a password. Missing or damaged
committed state requires permission repair or a complete consistent restore.
See [backup/restore](backup-restore.md) and [upgrade/rollback](login-private-upgrade.md).

`LoginFooter` is an optional public login-page footer in `config.yml`: text or
filtered HTTP(S) links. It does not inherit the internal `Footer`, and does not
allow scripts, images or iframes. Keep private information out of this public text.
