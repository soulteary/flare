# Flare 0.6.13

Release date: 2026-11-02 (Asia/Shanghai). This maintenance patch contains kit dependency updates and Go quality reports since [0.6.12](https://github.com/soulteary/flare/releases/tag/0.6.12).

## Changes

- cli-kit v1.9.3, i18n-kit/v4 v4.0.4, version-kit/v4 v4.0.3 and required transitive dependencies.
- Current-version installation links and release-candidate metadata now use 0.6.13. Build version/commit/date continue to come from version-kit linker flags and the exact release tag; no second runtime version constant is introduced.
- No application configuration, authentication, storage schema, or platform support changes relative to 0.6.12.

## Upgrade and rollback / 升级与回退

Stop the instance and back up its complete working directory, including hidden security state, YAML files, and external environment/Secret configuration. Retain the old binary or image digest. Replace the executable or recreate the container with the same persistent mounts and effective settings. Verify login, anonymous access to public/private resources, editing, search, and restart persistence before restoring traffic.

升级前停止实例，备份完整工作目录（包含隐藏的安全状态）、YAML 和外部部署配置，并记录旧镜像摘要或保留旧二进制。沿用原持久化挂载与配置更新；验证登录、匿名访问的隐私边界、编辑、搜索和重启后的状态。

There is no new migration from 0.6.12. To roll back, stop the new instance and use the saved 0.6.12 binary/image with its matching backup. For older versions, read [security migration and rollback](login-private-upgrade.md): old releases may not enforce current per-resource privacy. This patch retains 0.6.12 defaults; it does not switch an existing private deployment to public.

0.6.12 升级无需新增迁移。回退时停止新版，使用保留的 0.6.12 程序或镜像及对应备份。更早版本须先阅读安全迁移指南，旧版本可能无法保持当前逐项隐私保护。

## Distribution

```sh
docker pull soulteary/flare:0.6.13
docker pull ghcr.io/soulteary/flare:0.6.13
```

Containers: linux/amd64, linux/arm64, linux/arm/v7, linux/arm/v6. Archives additionally support Linux 386 and macOS amd64/arm64. Pin the published digest for reproducible deployment. Release preflight verifies seven archives and four container platforms without publishing; the tag workflow publishes binaries and Docker Hub images, then mirrors and verifies GHCR images. Consult the actual workflow results rather than treating this note as a record of completed publication.
