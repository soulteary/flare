# Flare documentation

The latest published stable release at this documentation update is **0.6.5**.
The editor, search and layout guides below describe features shipped in 0.6.5.
Later changes on `main` may describe development code.
Use the release-tagged documentation when deploying a pinned image; merging a
feature does not publish an image or move `latest`.

## Users

| Task | Guide | Availability |
| --- | --- | --- |
| Install or upgrade Docker | [Deployment repository](https://github.com/soulteary/docker-flare#快速上手) | Stable 0.6.5 |
| Set login and private access | [Configuration reference](configuration.md#access-models) | Stable; parser and request validation hardened in 0.6.5 |
| Import/export bookmarks | [Bookmark transfer](bookmark-transfer.md) | Since 0.6.2 |
| Configure clocks and zones | [Clock](clock.md) | Since 0.6.3 |
| Control access logs | [Logging](logging.md) | Since 0.6.4 |
| Use list/forms/batch editor | [Editor](editor.md), [screenshots](screenshots/README.md), [icons](editor-icons.md) | Since 0.6.5 |
| Configure external/hybrid search | [Search](search.md), [suggestions](search-hybrid.md) | Since 0.6.5 |
| Choose category tabs/collapse | [Category navigation](category-navigation.md) | Since 0.6.5 |
| Change mobile layout/text size | [Mobile layout](mobile-layout.md) | Since 0.6.5 |
| Configure desktop columns | [Columns and scale](desktop-columns-and-scale.md) | Since 0.6.5 |

## Operations and development

- [Configuration reference and precedence](configuration.md)
- [Weather refresh, stale data and retry behavior](weather.md) (since 0.6.5)
- [Backups and restore](backup-restore.md), [editor save recovery](editor-recovery.md)
- [Reverse proxies and deployment boundaries](reverse-proxy.md)
- [Troubleshooting and evidence collection](troubleshooting.md)
- [Build and test from a clean checkout](development.md)
- [Navigation cache and invalidation](navigation-cache.md) (since 0.6.5)
- [Performance decisions and current observation index](performance.md), [measurement methods](baseline-metrics.md)
- [Physical mobile acceptance](mobile-device-acceptance.md)
- [0.6.5 artifact verification and limitations](release-0.6.5-verification.md)
- [Unpublished 0.6.6 draft](release-0.6.6-draft.md), [candidate acceptance and remaining gates](release-0.6.6-preflight.md)
- [Release preparation checklist and limitations](release-readiness.md)
- [Historical feedback disposition](issue-triage.md)

The server-side offline switch does not prevent the browser from opening external
bookmarks or fetching configured remote images. Encoded links are not encryption,
a password, or per-bookmark authorization. Category tabs are presentation filters.
