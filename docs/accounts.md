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

## Initialize 0.6.8 or current source

New installations default to login enabled and whole-site PRIVATE visibility
unless explicitly configured. Missing credentials, contradictory access settings
or damaged security state never silently enable anonymous access. A non-ready
instance serves a generic HTTP 503 page; inspect its terminal diagnostics.

Use the 0.6.8 release binary, or build following [development instructions](development.md). Run maintenance in
the same working directory and effective environment as the service, while the
service is stopped:

```sh
flare security migration-check --json
flare security init --username your_user
```

Initialization asks for a password twice, with hidden input on a terminal.
New passwords require 15–128 Unicode code points, at most 512 UTF-8 bytes, and
must pass the bundled weak/repeated-password checks. Existing nonempty deployment
passwords can migrate without this new-password strength requirement.
Alternatively, supply an initial `FLARE_PASS` or `FLARE_PASS_FILE` at startup.
The new startup path does not print generated passwords in logs.

### 从源码运行

如果 `go run .` 输出 `Flare security: new`，浏览器显示“Flare 尚未就绪”，说明需要先初始化管理员。在运行终端按 Ctrl+C 停止服务，然后在同一个项目目录、相同环境变量下执行：

```sh
go run . security init --username admin
go run .
```

按提示输入两次密码（终端不回显）。密码要求见上文。启动后访问 <http://localhost:5005>，使用 `admin` 和设置的密码登录；自定义端口时使用实际端口。后续启动无需再次初始化。

### Docker Compose 部署

在原 Compose 项目目录执行，下面假设服务名为 `flare`：

```sh
docker compose stop flare
docker compose run --rm --no-deps flare flare security init --username admin
docker compose up -d flare
```

临时容器会沿用该服务的数据挂载和环境变量，并交互式询问两次密码。使用原来的 Compose 文件、项目名及其他 Compose 参数；服务名不同则替换 `flare`。初始化命令成功后再启动服务并登录。

必须先配置持久化，例如服务包含 `volumes: ["./app:/app"]`。默认工作目录是 `/app`，账户状态保存于宿主机的 `./app/.flare-security/`。只挂载 YAML 文件时，应额外挂载安全目录，例如 `./security:/app/.flare-security`。自定义 `FLARE_STATE_DIR` 时须持久化对应目录。

### docker run 部署

下面假设容器名为 `flare`，原挂载为当前目录的 `./app:/app`，镜像为 `soulteary/flare:0.6.8`：

```sh
docker stop flare
docker run --rm -it \
  -v "$PWD/app:/app" \
  soulteary/flare:0.6.8 \
  flare security init --username admin
docker start flare
```

仅在初始化成功后执行最后一行。已有部署必须替换为原来的镜像版本、数据挂载、工作目录和相关环境变量（包括自定义 `FLARE_STATE_DIR`）；命名卷继续使用原卷，不能新建空卷代替。初始化完成后访问映射的服务端口登录。

不要通过 `docker exec` 在仍运行的服务中初始化：服务持有安全状态的独占锁。停止后使用共享原数据的临时容器维护。重建容器时保留安全目录，且不要通过删除该目录重置密码。

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
