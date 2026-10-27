# Flare 0.6.8 发布准备

状态：候选版本，尚未发布。当前稳定版仍为 0.6.7；本文不表示镜像、标签或 GitHub Release 已可用。准备日期：2026-10-27（Asia/Shanghai）。

## 发布说明草稿

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
3. 可先用候选二进制执行 `flare security migration-check --json`，按报告处理无效或矛盾的旧配置。
4. 更新并重建容器。有效旧登录配置会迁移；旧 Cookie 会失效一次，需要重新登录。新实例需显式初始化管理员。
5. 使用独立无 Cookie 窗口核验匿名访问，再重建一次容器核验安全状态持久化，通过后再开放公网。

免登录配置与私有条目并存时会暂停业务，需要明确处理配置。无法区分的新旧默认配置不会自动公开。YAML 锚点和别名不会被网页自动展开或重写。

回滚必须恢复升级前完整工作目录和外部部署配置。旧版不执行本次新增的逐项隐私保护；回滚后须核验匿名访问，避免将私有内容暴露到旧公开站点。

完整子路径、多副本和网络盘并发写不属于本阶段支持范围。实体 Safari、Via、Android、iPad 和树莓派验收结果不作未经验证的承诺。

## 发布检查清单

- [ ] 最终候选提交的 CI、Clock acceptance、CodeQL 和安全扫描完成并核验结果。
- [ ] 运行 `Release preflight (no publication)`，显式指定 `candidate_version=0.6.8`，确认七个平台归档、四个平台容器及浏览器验收通过。
- [ ] 将最终提交 SHA、工作流链接及验收限制记录在本文；文档或代码变更后重新验证最终候选。
- [ ] 合入发布准备改动，核验最终提交后再创建并推送无 `v` 前缀的 `0.6.8` 标签。标签推送会触发真实发布。
- [ ] 核验 GitHub Release 归档与校验和、Docker Hub/GHCR 四个平台版本及提交标签、匿名拉取和 `latest`。
- [ ] 以本文发布说明草稿及实际验证记录更新 GitHub Release；发布成功后再更新 README 和文档中的稳定版本及开发功能范围。

本地构建检查不等于容器或真实浏览器验收。发布机制见[容器发布说明](../.github/CONTAINER-RELEASES.md)。

## 本轮准备的验证记录

验证源码提交：`ecab4df64cfd31f6e6cb954cbb55a48f729a43d3`。以下结果对应该源码，不代表尚未提交的发布文档与工作流改动已经完成线上验证。

- 线上 [CI](https://github.com/soulteary/flare/actions/runs/40464614252)、[Clock acceptance](https://github.com/soulteary/flare/actions/runs/40464614229)、[CodeQL](https://github.com/soulteary/flare/actions/runs/40464614191)、[Security Scan](https://github.com/soulteary/flare/actions/runs/40464614243) 全部成功。
- 本地 Go 1.27.3 / macOS ARM64：`go vet ./...` 通过；`go test -race -count=1 -coverpkg=./... -coverprofile=... -covermode=atomic ./...` 通过，全包语句覆盖率 91.1%（`go tool cover` 汇总），超过 90% 门槛。
- 发布工具 Python 单元测试 26 项通过。
- `scripts/release-preflight.py --version 0.6.8` 完成生产资源生成一致性检查和七个平台的交叉编译、归档内容、版本/提交元数据及 SHA-256 校验。覆盖 Darwin amd64/arm64、Linux 386/amd64/arm64/armv6/armv7。
- 本地编辑器浏览器补验未执行完成：当前 Playwright 包所需的 Chromium headless shell 1234 未安装。不能据此声称本地浏览器通过；上述线上浏览器验收为独立证据。

本地产物与日志位于临时目录，未纳入仓库：`/tmp/flare-0.6.8-preflight-final-20261027/`、`/tmp/flare-0.6.8-race-coverage.log`、`/tmp/flare-0.6.8-coverage.out` 和 `/tmp/flare-0.6.8-release-tests.log`。正式发布前仍需最终提交的完整容器预检。
