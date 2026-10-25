# Flare documentation

The latest published stable release at this documentation update is **0.6.4**.
Pages on `main` describe development code unless a released version is stated.
Use the release-tagged documentation when deploying a pinned image; merging a
feature does not publish an image or move `latest`.

## Users

| Task | Guide | Availability |
| --- | --- | --- |
| Install or upgrade Docker | [Deployment repository](https://github.com/soulteary/docker-flare#快速上手) | Stable 0.6.4 |
| Set login and private access | [Configuration reference](configuration.md#access-models) | Stable; development validation changes are noted there |
| Import/export bookmarks | [Bookmark transfer](bookmark-transfer.md) | Since 0.6.2 |
| Configure clocks and zones | [Clock](clock.md) | Since 0.6.3 |
| Control access logs | [Logging](logging.md) | Since 0.6.4 |
| Use list/forms/batch editor | [Editor](editor.md), [screenshots](screenshots/README.md), [icons](editor-icons.md) | Development, after 0.6.4 |
| Configure external/hybrid search | [Search](search.md), [suggestions](search-hybrid.md) | Development, after 0.6.4 |
| Choose category tabs/collapse | [Category navigation](category-navigation.md) | Development, after 0.6.4 |
| Change mobile layout/text size | [Mobile layout](mobile-layout.md) | Development options after 0.6.4 |
| Configure desktop columns | [Columns and scale](desktop-columns-and-scale.md) | Development, after 0.6.4 |

## Operations and development

- [Configuration reference and precedence](configuration.md)
- [Weather refresh, stale data and retry behavior](weather.md) (development)
- [Backups and restore](backup-restore.md), [editor save recovery](editor-recovery.md)
- [Reverse proxies and deployment boundaries](reverse-proxy.md)
- [Troubleshooting and evidence collection](troubleshooting.md)
- [Build and test from a clean checkout](development.md)
- [Navigation cache and invalidation](navigation-cache.md) (development)
- [Performance decisions and current observation index](performance.md), [measurement methods](baseline-metrics.md)
- [Physical mobile acceptance](mobile-device-acceptance.md)
- [Next release readiness](release-readiness.md)
- [Historical feedback disposition](issue-triage.md)

The server-side offline switch does not prevent the browser from opening external
bookmarks or fetching configured remote images. Encoded links are not encryption,
a password, or per-bookmark authorization. Category tabs are presentation filters.
