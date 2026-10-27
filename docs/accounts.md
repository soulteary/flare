# Accounts and private resources

> Scope: current main development, after stable 0.6.7. These persistent-account
> and per-resource authorization features are not included in 0.6.7.

## Stable 0.6.7

Stable 0.6.7 defaults to disabled login. Enable `FLARE_DISABLE_LOGIN=false` for
protected administration; add `FLARE_VISIBILITY=PRIVATE` for whole-site protection.
Its per-entry/category `private` fields do not enforce resource access. It uses
Gorilla signed cookies with a 30-day lifetime; preserve `.flare-cookie-secret`
or supply a strong `FLARE_COOKIE_SECRET`. Those legacy cookie settings are not
the current-main persistent-session mechanism. Old instructions and measurements
remain available through the [archive](archive/README.md).

## Initialize current source

New installations default to login enabled and whole-site PRIVATE visibility
unless explicitly configured. Missing credentials, contradictory access settings
or damaged security state never silently enable anonymous access. A non-ready
instance serves a generic HTTP 503 page; inspect its terminal diagnostics.

Build following [development instructions](development.md). Run maintenance in
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
