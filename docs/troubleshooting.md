# Troubleshooting

Record the exact release/source SHA, image digest, platform, OS, browser version,
redacted Compose/runtime configuration, exit code and relevant logs. Never paste
passwords, signing keys, session cookies or complete private bookmark exports.

| Symptom | Check |
| --- | --- |
| Private homepage remains public | Use both `FLARE_DISABLE_LOGIN=false` and `FLARE_VISIBILITY=PRIVATE`; `0` is not a visibility enum; test without existing cookies |
| Correct login returns to login form | Upgrade past the 0.6.0/0.6.1 HTTP session regression; use HTTPS with forced Secure cookies and persist the signing key |
| K8s fails parsing `FLARE_PORT=tcp://...` | Explicitly set a numeric port or disable Service environment injection with `enableServiceLinks: false`; record effective environment without secrets |
| Root works, `/flare/` fails | Full subpath deployment is unsupported; use a dedicated hostname at `/` |
| Bookmarks missing or blank | Check YAML syntax and server logs, browser extensions such as uBlock, filters/categories and actual persisted files; do not save over unreadable data |
| Redirect fails only for one service | Compare exact port/query/fragment, encoded mode and target-service Referer/authentication policy; do not assume every Unauthorized response is Flare |
| Weather unavailable | Check configured location, offline mode, upstream coverage and logs; browser device location/time zone does not choose weather city |
| Old editor remains after upgrade | Confirm running version and digest, recreate container, refresh cached assets and old forms; keep unsaved data before reloading |
| Editor reports 409 | Preserve/export draft, reload current server document, review and explicitly retry; no automatic merge |
| Editor reports incomplete recovery | Stop writers and follow the exact recovery path in the response; preserve both current files and recovery copies |
| ARM container repeatedly exits | Record architecture/image platform/digest, exit code, startup logs and memory limit; cross-compilation is not Raspberry Pi validation |

Use [physical-device acceptance](mobile-device-acceptance.md) for Safari/Via,
software keyboards, safe areas and native font enlargement. Desktop Chromium
emulation does not close these reports without real-device evidence.


Since 0.6.5, editor, favicon and generated MDI references use content versions to
bypass old fresh browser entries. Other unchanged unversioned guide assets may
still need one hard refresh if a pre-fix one-year response is already cached;
new revalidation headers cannot retroactively expire that existing entry.
