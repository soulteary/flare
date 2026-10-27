# Flare 0.6.8

已于 2026-10-27 15:21（Asia/Shanghai）发布。[GitHub Release](https://github.com/soulteary/flare/releases/tag/0.6.8)。

本次更新在保留 YAML 内容配置的基础上，加入持久化单管理员账号、可撤销的服务端会话，以及分类和条目的隐私访问控制，并改善编辑器重新认证和导航缓存。

- 账号和会话：支持管理员初始化、应用管理或部署管理凭据、密码文件、密码恢复及退出全部会话。安全状态保存在 `.flare-security/`，普通重启保留有效会话。
- 隐私保护：分类和条目的 `private: true` 对匿名访问真正生效，导航、搜索、跳转、编辑器和导出统一检查访问权限。新实例默认需要建立管理员且全站私有。
- 编辑与导入导出：恢复隐私控件，保留草稿隐私设置，识别嵌套的重新认证错误并引导用户重新登录。
- 性能：合并缓冲读取中的持久会话检查，缓存请求设置和导航 HTML 片段，并恢复公开静态资源的缓存。
- 兼容与质量：继续读取 YAML 锚点、别名和合并键；含这些结构的文件在网页中只读。升级 fasthttp 和 minify，改进可复现候选构建、CI 缓存与浏览器超时诊断，将全包覆盖率门槛提高至 90%。

## 从 0.6.7 升级

请先阅读[升级与回滚指南](login-private-upgrade.md)、[账号指南](accounts.md)和[备份恢复](backup-restore.md)。

1. 停止实例，备份整个工作目录、隐藏文件、外部 Compose/.env/Secret，并记录旧镜像摘要或保留旧二进制。
2. 确认 `.flare-security/` 持久化且可写。只挂载三个 YAML 文件的部署必须额外挂载安全状态目录，或通过 `FLARE_STATE_DIR` 指定持久目录。
3. 可先用新版二进制执行 `flare security migration-check --json`，按报告处理无效或矛盾的旧配置。
4. 更新并重建容器。有效旧登录配置会迁移；旧 Cookie 会失效一次，需要重新登录。新实例需显式初始化管理员。
5. 使用独立无 Cookie 窗口核验匿名访问，再重建一次容器核验安全状态持久化，通过后再开放公网。

免登录配置与私有条目并存时会暂停业务，需要明确处理配置。无法区分的新旧默认配置不会自动公开。YAML 锚点和别名不会被网页自动展开或重写。

回滚必须恢复升级前完整工作目录和外部部署配置。旧版不执行本次新增的逐项隐私保护；回滚后须核验匿名访问，避免将私有内容暴露到旧公开站点。

完整子路径、多副本和网络盘并发写不属于本阶段支持范围。实体 Safari、Via、Android、iPad 和树莓派验收结果不作未经验证的承诺。

## 安装

```sh
docker pull soulteary/flare:0.6.8
docker pull ghcr.io/soulteary/flare:0.6.8
```

容器支持 `linux/amd64`、`linux/arm64`、`linux/arm/v7`、`linux/arm/v6`。归档另提供 Linux 386 及 macOS amd64/arm64。

## 发布验证

- 发布源码：`36d909159d2422a46934f5bb1396f14bccaa4f17`。
- [最终提交完整预检](https://github.com/soulteary/flare/actions/runs/40465694840)：七平台归档、四平台容器 HTTP/时区/重启、归档二进制浏览器测试。
- [CI](https://github.com/soulteary/flare/actions/runs/40465685461)、[浏览器验收](https://github.com/soulteary/flare/actions/runs/40465685454)、[CodeQL](https://github.com/soulteary/flare/actions/runs/40465685412)。
- 全包 race 测试与覆盖率门槛 90%；发布准备时本地覆盖率汇总为 91.1%。

实体设备测试范围见上文；桌面浏览器和 QEMU 容器验收不代表实体手机、Safari 或树莓派验收。

[正式发布工作流](https://github.com/soulteary/flare/actions/runs/40466217420)已成功。Docker Hub 和 GHCR 的版本标签及 `latest` 使用同一多平台摘要：

`sha256:4e01c92941b42cd28b8e5aa6b8db9f44af03a9ba03edee226a6fc6b05d80c24b`

GHCR 四个平台的提交/版本标签和匿名访问均已验证；七个平台的公开下载归档校验和、内容及版本/提交元数据均已核对。
