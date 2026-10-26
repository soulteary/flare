# Flare documentation

The latest published stable release is **0.6.6**. Search configuration protections,
weather location selection, adaptive tablet columns, updated MDI icons with legacy
compatibility and themed favicons are included in 0.6.6.
Use the release-tagged documentation when deploying a pinned image; merging a
feature does not publish an image or move `latest`.

## Users

| Task | Guide | Availability |
| --- | --- | --- |
| Install or upgrade Docker | [Deployment repository](https://github.com/soulteary/docker-flare#快速上手) | Stable 0.6.6 |
| Set login and private access | [Configuration reference](configuration.md#access-models) | Stable; parser and request validation hardened in 0.6.5 |
| Import/export bookmarks | [Bookmark transfer](bookmark-transfer.md) | Since 0.6.2 |
| Configure clocks and zones | [Clock](clock.md) | Since 0.6.3 |
| Control access logs | [Logging](logging.md) | Since 0.6.4 |
| Use list/forms/batch editor | [Editor](editor.md), [screenshots](screenshots/README.md), [icons](editor-icons.md) | Since 0.6.5 |
| Configure external/hybrid search | [Search](search.md), [suggestions](search-hybrid.md) | Engines/hybrid since 0.6.5; additional modes and save protections in 0.6.6 |
| Choose category tabs/collapse | [Category navigation](category-navigation.md) | Since 0.6.5 |
| Change mobile layout/text size | [Mobile layout](mobile-layout.md) | Controls since 0.6.5; adaptive tablet columns in 0.6.6 |
| Configure desktop columns | [Columns and scale](desktop-columns-and-scale.md) | Since 0.6.5 |

## Operations and development

- [Configuration reference and precedence](configuration.md)
- [Weather refresh and retries](weather.md) (since 0.6.5); location picker and migration in 0.6.6
- [Backups and restore](backup-restore.md), [editor save recovery](editor-recovery.md)
- [Reverse proxies and deployment boundaries](reverse-proxy.md)
- [Troubleshooting and evidence collection](troubleshooting.md)
- [Build and test from a clean checkout](development.md)
- [Interface language and translation development](i18n.md)
- [Navigation cache and invalidation](navigation-cache.md) (since 0.6.5)
- [Performance decisions and current observation index](performance.md), [measurement methods](baseline-metrics.md)
- [Physical mobile acceptance](mobile-device-acceptance.md)
- [0.6.5 artifact verification and limitations](release-0.6.5-verification.md)
- [0.6.6 release notes](release-0.6.6.md), [published artifact verification](release-0.6.6-verification.md)
- [Historical feedback disposition](issue-triage.md)

The server-side offline switch does not prevent the browser from opening external
bookmarks or fetching configured remote images. Encoded links are not encryption,
a password, or per-bookmark authorization. Category tabs are presentation filters.
