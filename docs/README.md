# Flare documentation

## Choose your version

The latest published stable release is [0.6.7](https://github.com/soulteary/flare/releases/tag/0.6.7)
(verified 2026-10-27, Asia/Shanghai). It includes Chinese/English interfaces,
columns within bookmark categories, icon improvements and clearer write-source
errors. Use [release-tagged documentation](https://github.com/soulteary/flare/tree/0.6.7/docs)
when deploying that image. Merging a feature does not publish an image or move `latest`.

This working-tree documentation also describes **current main development**:
persistent single-administrator accounts/sessions, enforced per-entry/category
privacy and HTML fragment caching. These changes are not included in 0.6.7 and
have not received a new stable release or complete physical-device acceptance.
Guides below explicitly distinguish the two access models.

## Install and operate

- [Deployment examples](https://github.com/soulteary/docker-flare#快速上手) and [0.6.7 release notes](release-0.6.7.md)
- [Configuration, precedence and access models](configuration.md)
- [Accounts, initialization and private resources](accounts.md) — current main; includes stable-version differences
- [Development-version upgrade and rollback](login-private-upgrade.md)
- [Backups and restore](backup-restore.md), [editor recovery](editor-recovery.md)
- [Reverse proxies and deployment boundaries](reverse-proxy.md)
- [Troubleshooting](troubleshooting.md), [access logs and Docker log rotation](logging.md)

## Use and customize

- [Editor](editor.md), [icons](editor-icons.md), [versioned screenshots](screenshots/README.md)
- [Bookmark import/export](bookmark-transfer.md)
- [Search engines](search.md), [search modes and suggestions](search-hybrid.md)
- [Clock and time zones](clock.md), [weather and location selection](weather.md)
- [Category navigation](category-navigation.md), [mobile layout](mobile-layout.md), [desktop and category columns](desktop-columns-and-scale.md)
- [Interface language](i18n.md)

## Develop and verify

- [Build and test](development.md), [contributing](../CONTRIBUTING.md)
- [Navigation snapshots and HTML fragment cache](navigation-cache.md)
- [Performance decisions and reports](performance.md), [measurement methods](baseline-metrics.md)
- [Current security requirements, architecture and acceptance index](requirements/README.md)
- [CI ownership and cache design](../.github/CI-OPTIMIZATION.md), [Go report scopes](../.github/GO-REPORTS.md), [container publishing](../.github/CONTAINER-RELEASES.md)
- [Physical mobile acceptance](mobile-device-acceptance.md), [mobile test notes](../tests/mobile/README.md)
- [Historical releases, implementation records and feedback](archive/README.md)

The server-side offline switch does not prevent browser visits to external
bookmarks or remote images. Encoded links are not encryption or authorization.
Category tabs filter presentation; current-main resource privacy is enforced
separately by authentication and visibility checks.
