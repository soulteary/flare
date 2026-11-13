# Troubleshooting

> Scope: stable 0.6.8 and current source, with legacy 0.6.7 differences. Persistent
> security-state diagnostics apply to 0.6.8 and current main; see [accounts](accounts.md).

Record the exact release/source SHA, image digest, platform, OS, browser version,
redacted Compose/runtime configuration, exit code and relevant logs. Never paste
passwords, signing keys, session cookies or complete private bookmark exports.

| Symptom | Check |
| --- | --- |
| Private homepage remains public | Use both `FLARE_DISABLE_LOGIN=false` and `FLARE_VISIBILITY=PRIVATE`; `0` is not a visibility enum; test without existing cookies |
| Correct login returns to login form | Check actual version, browser cookies, public origin and Secure policy; stable 0.6.7 needs its signing key, current main needs the persistent security directory; inspect the account cookie-check flow |
| Current main returns 503 on every route | Read the not-ready page: 0.6.15 and later show the status there and, for a rejected configuration input, the input and its accepted values. Otherwise inspect terminal `new`, `attention_required` or `faulted` status; initialize credentials or correct the reported inputs/permissions; never delete committed state to bypass recovery |
| A section shows "Private entries exist while login is disabled" | `apps.yml` or `bookmarks.yml` contains private entries/categories while login is disabled, so that collection is withheld as a whole; the editor, import and export return `PRIVATE_CONTENT_REQUIRES_LOGIN`. Enable login with valid credentials, or remove the private flags from the file. PRIVATE visibility plus disabled login is still rejected at startup |
| Password/environment update does not change current-main password | Check local/deployment ownership on the account page; local mode deliberately ignores leftover deployment passwords |
| Security maintenance cannot acquire lock | Stop the daemon and use the same working directory, environment and state volume; only one process may hold the security store |
| Sessions behave unexpectedly after full restore | Stop the instance and follow [restore and revoke](backup-restore.md#complete-current-main-instance-restore); ordinary restart preserves unexpired sessions |
| K8s fails parsing `FLARE_PORT=tcp://...` | Explicitly set a numeric port or disable Service environment injection with `enableServiceLinks: false`; record effective environment without secrets |
| Root works, `/flare/` fails | Full subpath deployment is unsupported; use a dedicated hostname at `/` |
| Bookmarks missing or blank | Check YAML syntax and server logs, browser extensions such as uBlock, filters/categories and actual persisted files; do not save over unreadable data. A file that cannot be decoded no longer blocks startup: only its own section shows an error, and the startup and request logs name the file and, where available, the line of the problem |
| Redirect fails only for one service | Compare exact port/query/fragment, encoded mode and target-service Referer/authentication policy; do not assume every Unauthorized response is Flare |
| Weather unavailable | Check configured location, offline mode, upstream coverage and logs; browser device location/time zone does not choose weather city |
| Settings page reports that a change was not saved | Follow the cause shown on the page: `config.yml` changed or unreadable, written by a newer version, using YAML aliases, a symbolic link, or not writable. The server log line `settings were not saved` has the detail. In 0.6.6 to 0.6.14 an unexpected state of `.flare-search-backups/` (for example permissions widened by a copy) also blocks the save; current main repairs or sets the directory aside and continues, see [configuration backups](backup-restore.md#configuration-backups-in-066) |
| Old editor remains after upgrade | Confirm running version and digest, recreate container, refresh cached assets and old forms; keep unsaved data before reloading |
| Editor reports 409 | Preserve/export draft, reload current server document, review and explicitly retry; no automatic merge |
| Settings/editor saves return 403 even with login disabled | Read `error.reason` in the response or on the explanation page and look up the reason in [write-source troubleshooting](reverse-proxy.md#write-requests-rejected-even-with-login-disabled). Current source accepts writes behind TLS termination without extra settings; 0.6.8–0.6.14 report `ORIGIN_SCHEME_MISMATCH` there unless `FLARE_COOKIE_SECURE=true` is set. 0.6.7 reports `write_source_rejected`: preserve the original Host including the public port at the proxy. In 0.6.6 this can appear as an expired login |
| Editor reports incomplete recovery | Stop writers and follow the exact recovery path in the response; preserve both current files and recovery copies |
| ARM container repeatedly exits | Record architecture/image platform/digest, exit code, startup logs and memory limit; cross-compilation is not Raspberry Pi validation |

Use [physical-device acceptance](mobile-device-acceptance.md) for Safari/Via,
software keyboards, safe areas and native font enlargement. Desktop Chromium
emulation does not close these reports without real-device evidence.


Since 0.6.5, editor, favicon and generated MDI references use content versions to
bypass old fresh browser entries. Other unchanged unversioned guide assets may
still need one hard refresh if a pre-fix one-year response is already cached;
new revalidation headers cannot retroactively expire that existing entry.
