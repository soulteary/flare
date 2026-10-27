# 阶段一首批实现与验收记录

日期：2026-10-27。状态：开发实现，可继续评审；不是发布签署。

关联：[需求 v1.2](login-private-access.md)、[实施规格 v1.1](login-private-access-phase1.md)、[升级与回滚指南](../login-private-upgrade.md)、[内部安全存储 ADR](security-storage-adr.md)。

## 本批范围

三个 agent 分别实现内部安全存储、内容/YAML 授权、认证 HTTP/来源策略；主 agent 接入启动、自动迁移、本地命令、路由与验证。没有引入内容数据库、多人模型或必填资源字段。

- YAML 继续是内容/设置事实来源，旧非空凭据及访问模式兼容迁移；自动迁移不改三个原文件字节。仅内部 `.flare-security/` 新增持久状态。
- 分类/条目 private 叠加，读取过滤覆盖首页、应用、分类、搜索、联想和跳转；新生成跳转为随机不透明句柄，旧编码跳转仍执行当前授权。
- 已有未知 YAML 字段保留；旧锚点/别名可安全读取，含别名的网页编辑/合并导入明确只读，避免展开覆盖。非法 private、循环别名及不完整分类引用拒绝。
- 账号 Argon2id、普通持久会话、当前/全部注销、重新认证、改密、明确密码来源切换、有限 Cookie 检查流程可用。
- 登录/账号/编辑/导入/设置采用会话或预认证浏览器绑定 CSRF；显式代理信任与规范来源校验；并发内容保存和会话撤销排序。
- 迁移完成标记及提交日志、原始快照、初始化/本地重设/来源切换/全部撤销命令已接入；损坏状态不能按旧默认重新开放。

## 路由与授权登记

以下登记针对 main 经过 PrepareSecurity 且显式传入 Store 的生产路径。无 Store 的旧会话适配路径保留给既有测试/嵌入兼容，不作为新部署入口。

- 启动未就绪：所有路由通用 503/no-store，不构建导航或回显详细故障，具体原因仅实例终端。
- `/`、`/applications`、`/bookmarks`、`/help`、搜索和建议：公开模式按可见条目生成；PRIVATE 整站认证。`/` 的 POST 是搜索读取，不修改文件。
- `/redir`、`/redir/url`：导航读取；PRIVATE 入口认证，id/go 匹配当前可见资源，混合 id+go 拒绝，响应 no-referrer/no-store。
- `/login`、`/account/cookie-check`：公开认证流程；登录写入需预认证 CSRF/来源/限流，Cookie 检查不授予身份，不带 token URL。登录成功有限跳转到原站内允许页面。
- `/settings/application`：登录启用时由独立账号页接管；无有效会话只显示通用登录与可选 LoginFooter，内部 Footer 不继承。
- `/logout`、`/logout-all`、`/account/reauthenticate`、`/account/password`、`/account/local`：会话、CSRF、来源及操作要求；可信免登录不注册账号操作。
- `/settings/theme`、`/settings/appearance`、`/settings/search`、`/settings/weather` 及天气查询/测试：沿用原路由，管理认证或明确可信网络；写入统一 CSRF，磁盘改动用授权写入锁排序。天气/时钟后台请求不续会话。
- `/editor`、`/editor/csrf`：管理读取；编辑写入带摘要、CSRF、会话重查；放宽公开需近期密码验证与确认，旧表单冲突 409。
- `/editor/transfer` 及 `/preview`、`/commit`、`/export`、`/backup`：管理授权；预览/提交 CSRF、绑定会话及文件摘要；导出/备份下载在输出前再次校验，登录模式要求近期验证与确认。
- `/guide`：PRIVATE 时认证；引导所用首页也沿同一权限路径，不直接读取原始 YAML。
- `/ping`、内置静态图标/样式/脚本、旧内置资源兼容入口：公开通用资源，不包含实例原始数据。安全目录和工作目录 YAML 不挂静态路由。

## 已有自动化与构建证据

开发环境：macOS arm64、Go 1.27.3。使用临时 GOCACHE；完整套件中需要本机临时监听端口的用例在相应执行权限下运行。

- `go test -p 1 ./...`：全量通过。
- 安全状态、启动/命令、认证、来源策略、内容保存、编辑器、导航与服务端八个核心包的 race 测试通过；不以原有普通测试替代 race。
- macOS arm64 原生二进制构建成功；Linux amd64/arm64 交叉构建成功。交叉构建不等于对应真机通过。
- 原生二进制在独立临时目录的真实 HTTP 演练通过：旧短密码自动迁移、父分类私有、登录 Cookie 往返、退出全部，原 bookmarks.yml 字节不变。客户端显式禁用系统代理，未访问外部服务。
- 需求编号/文档链接/格式检查通过。未创建提交、PR 或部署。

测试入口包括 `cmd/security_startup_test.go`、`cmd/security_command_test.go`、`internal/security/store_test.go`、`internal/auth/persistent_test.go`、`internal/server/phase1_security_test.go`、`internal/requestsource/policy_test.go`、`config/data/visibility_test.go`、`internal/pages/home/private_test.go` 及 YAML/编辑/导入测试。

## T01–T30 的签署边界

首批实现后的后续验收已补齐真实进程SIGKILL、完整旧目录回滚、状态损坏恢复、Docker目录/逐文件/只读挂载与真实ENOSPC、TLS/Nginx及三引擎无脚本浏览器证据。它们的准确环境、构建摘要、结果和剩余项目集中记录在[综合验收与T01–T30追踪](login-private-access-acceptance.md)，不再以本首批历史记录的待办作为当前状态。

专项依据：[部署与代理](deployment-acceptance.md)、[迁移与恢复](recovery-acceptance.md)、[规模与预算](scale-acceptance.md)、[浏览器](../../tests/security-browser/README.md)。Chromium156、Firefox157、WebKit27.2各8项，共24项桌面无脚本验收通过；移动viewport不代替手机实机。Linux386/ARMv6/ARMv7、darwinamd64及既有Linuxamd64/arm64、原生darwinarm64构建证据不等同于低配设备运行验收。

本次更新时30分钟持续压测、最终全量及race回归仍在收敛；低配ARM/NAS、手机实机、硬件断电及其他未测平台保持未完成。ADR的POSIX本地文件系统与单写实例支持边界不变，不宣称Windows原生或网络盘多副本支持。

本记录不降低P0，不用“测试全绿”宣称阶段一已经完成全部发布验收；发布签署仍以实施规格第13节及综合追踪中全部剩余条件为准。
