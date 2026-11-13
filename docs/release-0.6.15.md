# Flare 0.6.15

Release date: 2026-11-14 (Asia/Shanghai). This release contains the changes since [0.6.14](https://github.com/soulteary/flare/releases/tag/0.6.14). It restores behaviour that 0.6.4 to 0.6.8 changed for existing deployments and fixes defects found along the way. Several defaults change; read [Behaviour changes](#behaviour-changes) before updating.

Where the guides say "current source" or "current main", they describe this release unless a later version is named.

## Changes

### Reverse proxies and write requests

- Saves, sign-in and search work behind a TLS-terminating reverse proxy without additional configuration. A browser write is accepted when the browser marks it `Sec-Fetch-Site: same-origin` (or `none`). When a browser does not send that header, the `Origin` host and port must match the Host header, the authority supplied by a trusted proxy, `FLARE_EXTERNAL_URL` or a `FLARE_ALLOWED_HOSTS` entry. The scheme is compared only to reject a page opened over HTTP under an explicit HTTPS policy. In 0.6.8 to 0.6.14 a default configuration behind TLS termination rejected every write with `ORIGIN_SCHEME_MISMATCH` ([#234](https://github.com/soulteary/flare/pull/234)).
- `FLARE_EXTERNAL_URL` no longer restricts the Host header. `FLARE_ALLOWED_HOSTS` does, and the external authority belongs to that list.
- Homepage search and its suggestions only read; the write-source check no longer applies to them.
- A rejection names a specific reason. A rejected page navigation receives an explanation page in the site language instead of JSON. HTTP 400 responses report `HOST_NOT_ALLOWED`, `HOST_INVALID` or a `FORWARDED_*` reason instead of `REQUEST_AUTHORITY_INVALID`.

See [reverse proxies](reverse-proxy.md).

### Settings

- A settings save no longer depends on its backup. Modes of `.flare-search-backups/` that allow group or other access are repaired. A backup directory that cannot be trusted is renamed to `.flare-search-backups.quarantined-<UTC timestamp>` and a new one is started; nothing is deleted. When no backup can be written the save continues and the condition is logged. In 0.6.6 to 0.6.14 each of these cases refused every settings save ([#238](https://github.com/soulteary/flare/pull/238)).
- The theme, appearance, search and weather pages state why a save failed, in the site language.
- Saving the appearance settings while the title is hidden no longer clears the greetings ([#239](https://github.com/soulteary/flare/pull/239)).

See [backup and restore](backup-restore.md).

### Phone and tablet layout

- The default returns to the presentation of 0.6.3 and earlier: three bookmark categories per row on phones and one line per link. The default of 0.6.4 to 0.6.14 remains available as "Single column"; "Compact" is unchanged ([#237](https://github.com/soulteary/flare/pull/237)).

See [mobile layout](mobile-layout.md).

### Accounts and sessions

- A sign-in lasts 30 days and survives a browser restart, as it did up to 0.6.7. In 0.6.8 to 0.6.14 a session ended after 30 minutes without activity or 12 hours. Sensitive operations still require a password verification within the last 10 minutes ([#232](https://github.com/soulteary/flare/pull/232)).

See [accounts](accounts.md).

### Navigation files

- An `apps.yml` or `bookmarks.yml` that cannot be decoded no longer keeps the instance from starting. The module backed by that file shows an error and the rest of the instance is served. Decode errors give the position of the offending entry ([#242](https://github.com/soulteary/flare/pull/242)).
- Private entries combined with disabled login no longer stop the instance. The collection that contains them is withheld from every page and endpoint, with a notice, until login is enabled or the private flags are removed.
- Edits made to `apps.yml` and `bookmarks.yml` outside Flare are served by the next request, as before 0.6.0. A restart is no longer needed ([#243](https://github.com/soulteary/flare/pull/243)).

See [configuration](configuration.md) and [navigation cache](navigation-cache.md).

### Startup inputs

- `FLARE_DISABLE_LOGIN`, `FLARE_VISIBILITY`, `FLARE_CREDENTIAL_MODE` and the boolean switches accept surrounding quotes, any letter case and `yes`/`no`/`on`/`off` from the environment, `.env` and the command line. A value that still cannot be interpreted keeps the instance not ready; the not-ready page and the log name the input and the accepted values ([#236](https://github.com/soulteary/flare/pull/236)).

### Other fixes

- The guide page is rendered within the visitor's request. It was empty on private sites and behind a Host allow-list, and its markup was invalid elsewhere ([#241](https://github.com/soulteary/flare/pull/241)).
- Declining the editor's save confirmation no longer reports "All changes saved" ([#240](https://github.com/soulteary/flare/pull/240)).
- Log timestamps print the minute (`HH:MM:SS.mmm`). They printed the seconds twice ([#235](https://github.com/soulteary/flare/pull/235)).

### Release process

- A release candidate (`0.x.y-rc.N`) can be published without moving `latest` ([#233](https://github.com/soulteary/flare/pull/233)). 0.6.15 itself is published as a stable version without a candidate. See [container publishing](../.github/CONTAINER-RELEASES.md).

## Behaviour changes

- **Write-source check.** For browsers that do not send `Sec-Fetch-Site` (released before 2023), the scheme is no longer compared. A page served over HTTP from the same hostname is not stopped by this check when it submits to the HTTPS site; the CSRF token and `SameSite=Lax` cookies still apply. HSTS at the proxy, or `FLARE_COOKIE_SECURE=true`, closes that gap.
- **Host restriction.** An instance that sets only `FLARE_EXTERNAL_URL` no longer rejects other Host values. Add `FLARE_ALLOWED_HOSTS` to keep the restriction. This matters most for login-disabled instances, where it is the defence against DNS rebinding.
- **Existing proxy settings.** `FLARE_COOKIE_SECURE`, `FLARE_EXTERNAL_URL`, `FLARE_ALLOWED_HOSTS` and `FLARE_TRUSTED_PROXIES` keep their meaning apart from the point above and can stay as they are. With `FLARE_COOKIE_SECURE=true`, or an HTTPS `FLARE_EXTERNAL_URL`, pages opened over plain HTTP still cannot sign in or save.
- **Session lifetime.** A stolen session cookie stays usable for up to 30 days instead of 12 hours. Sign out on shared devices; sign out, sign out all and a password change revoke sessions on the server. Sessions created before the upgrade keep their original deadline.
- **Layout.** Instances with no stored choice, or with the stored value `single`, change appearance at 768 CSS pixels and below. Versions 0.6.5 to 0.6.14 stored `single` whenever the appearance form was saved, so that value cannot express a choice. Select "Single column" in the appearance settings to keep the previous layout. Classic rows are about 28px high, below the 44px touch target of the single-column layout.
- **Settings backup.** A save can complete without a backup of the previous `config.yml`. Set-aside backup directories are kept and are not removed automatically.
- **Startup.** Undecodable navigation files and private entries with disabled login no longer return HTTP 503 on every route. Private entries are still never served without login. `flare security migration-check` is unchanged and still reports both conditions.
- **Quoted values.** A quoted boolean that used to be discarded now takes effect. For example `FLARE_EDITOR="false"`, quotes included, left the editor enabled before and disables it now.
- **Guide page.** `/guide` now carries a Content-Security-Policy and reflects the visitor's own view. Inline scripts placed in the footer no longer run there unless `FLARE_DISABLE_CSP` is set.
- **Responses and messages.** Failed theme, appearance and weather saves return 409, 400 or 500 depending on the cause instead of always 500. Rejected form submissions receive HTML. The texts of settings failures, startup diagnostics and navigation decode errors changed; anything that matches the old strings needs updating.
- **External edits.** Each read of a navigation collection performs one additional `stat`. An edit that keeps both the file size and a modification time older than two seconds is not detected.

## Upgrade and rollback / 升级与回退

Stop the instance and back up its complete working directory, including hidden security state, YAML files, and external environment/Secret configuration. Retain the old binary or image digest. Replace the executable or recreate the container with the same persistent mounts and effective settings. There is no data migration from 0.6.14 and the security state format is unchanged. Afterwards verify, through every address you use (for example the public HTTPS name and a LAN address): sign-in, saving a setting, saving an editor change, search, the phone layout, and anonymous access to public and private resources.

升级前停止实例，备份完整工作目录（包含隐藏的安全状态）、YAML 和外部部署配置，并记录旧镜像摘要或保留旧二进制。沿用原持久化挂载与配置更新。从 0.6.14 升级无需数据迁移，安全状态格式不变。升级后请通过你实际使用的每一个访问地址（例如公网 HTTPS 域名和内网地址）验证：登录、保存设置、保存编辑器修改、搜索、手机布局，以及匿名访问时公开与私有内容的边界。

To roll back, stop the new instance and restore the backup taken before the upgrade together with the saved 0.6.14 binary or image. Do not run 0.6.14 against files saved by 0.6.15: `config.yml` records the version that last saved it and 0.6.14 refuses to overwrite a newer file, and the stored layout values `classic` and `single-column` are unknown to it. For versions before 0.6.8, read [security migration and rollback](login-private-upgrade.md) first: old releases may not enforce current per-resource privacy.

回退时停止新版，恢复升级前的备份，并使用保留的 0.6.14 程序或镜像。不要让 0.6.14 直接读写 0.6.15 保存过的文件：`config.yml` 会记录最后保存它的版本，0.6.14 拒绝覆盖更新版本写入的文件，也不认识新的布局取值 `classic` 和 `single-column`。回退到 0.6.8 之前的版本须先阅读安全迁移指南，旧版本可能无法保持当前逐项隐私保护。

## Verification scope

Covered before release: the Go test suite with the race detector and the all-package coverage gate, the Chromium browser acceptance scripts, and the release preflight for seven archives and four container platforms.

Not covered: a real TLS-terminating proxy (the proxy cases use request headers and a local header-rewriting proxy), Firefox, WebKit and physical devices, and filesystems that do not store permission bits (simulated in tests only).

## Distribution

```sh
docker pull soulteary/flare:0.6.15
docker pull ghcr.io/soulteary/flare:0.6.15
```

Containers: linux/amd64, linux/arm64, linux/arm/v7, linux/arm/v6. Archives additionally support Linux 386 and macOS amd64/arm64. Pin the published digest for reproducible deployment. Release preflight verifies seven archives and four container platforms without publishing; the tag workflow publishes binaries and Docker Hub images, then mirrors and verifies GHCR images. Consult the actual workflow results rather than treating this note as a record of completed publication.
