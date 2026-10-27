# 性能决策与测量索引

## 0.4.0 / 0.5.0 / 0.6.9 版本对照

[受控版本报告](../metrics-reports/versions-040-050-069-2026-10-27/report.md)记录同一 Go 1.27.3、Linux ARM64 环境中的六轮比较，涵盖首页、书签页、选择性搜索和末尾书签跳转，以及相同到达率的首页 CPU/延迟/RSS。0.5.0 与 0.4.0 总体接近，0.6.9 有明显计算收益，但小数据和相同低流量 RSS 更高，HTML 也更大。

[交互图表](../metrics-reports/versions-040-050-069-2026-10-27/index.html)需下载打开或本地提供 HTTP；[校验与重放](../metrics-reports/versions-040-050-069-2026-10-27/README.md)保留 144 个窗口、8,871,300 次请求、原始归档和摘要。它测量三个冻结版本的热态、无登录、无压缩路径，不代表当前 HEAD、原始发布包或完整浏览器体验。

## 当前源码开发记录（0.6.7 之后）

- [搜索与分类只读索引](../metrics-reports/navigation-read-index-2026-10-27/report.md)：取消整集合复制、复用搜索归一化与分类索引；另有[完整 HTTP 延迟/CPU/RSS 对照](../metrics-reports/navigation-read-index-2026-10-27/http-report.md)。

- [管理员读取鉴权复用](requirements/admin-performance-optimization.md)：请求内证明与持久化边界。
- [管理员 CPU/内存归因](requirements/admin-resource-analysis.md)：相同到达率、profile 与缓存保留成本。
- [会话事务 I/O 合并](requirements/transaction-io-optimization.md)：响应释放前持久化及完整 HTTP 计数。
- [导航 HTML 片段缓存](../metrics-reports/html-fragment-cache-2026-10-27/report.md)：身份隔离、容量限制、冷/热与原生 HTTP 测量；[当前机制与复现](navigation-cache.md)。
- [CI 审计](../metrics-reports/ci-audit-2026-10-27.md)、[同提交冷/热缓存对照](../metrics-reports/ci-cache-comparison-2026-10-27.md)及[CI 设计](../.github/CI-OPTIMIZATION.md)。

以上分别针对各自冻结的候选与环境，不代表当前 HEAD 已重新测量，百分比不可相加。
认证持久化协议已在后续优化中变化；以事务专项和当前代码为准。

## 历史 Fiber 迁移与稳定版优化

以下保留迁移时的配置、会话兼容和热点分析，不将当时的 Gorilla 会话或 YAML 热点描述当作当前认证实现。

默认服务已从 Echo 切换为原生 Fiber v3，读取缓冲保留 **64 KiB**。正常构建、容器和测试不再需要构建标签，生产依赖中移除 Echo。这里汇总迁移前的全部性能实验、取舍和复现入口；各轮原始数据与当时结论继续保留，不以最终选择回写历史数字。

## 完整记录

1. **1 MiB Fiber 与 Echo 初次比较**：[报告](../metrics-reports/fiber-comparison-2026-10-25.md)、[逐轮 JSON](../metrics-reports/fiber-comparison-2026-10-25.json)。五个场景、默认与 8 条空闲连接池；原基线 2000 请求/100 预热/5 轮，独立资源 20000 请求/1000 预热/3 轮。页面吞吐与 CPU 有收益，但首页 RSS 约从 27 MiB 增到 37–39 MiB，未裁剪二进制也更大。这不支持“Fiber 更省常驻内存”的结论。
2. **恢复默认 8 KiB**：[报告](../metrics-reports/fiber-default-buffer-2026-10-25.md)、[逐轮 JSON](../metrics-reports/fiber-default-buffer-2026-10-25.json)。每组同场比较 Echo、8 KiB 与同源码 1 MiB 参照，轮转三轮资源顺序。首页 RSS 相对 1 MiB 少约 18.5%–22.4%，但吞吐收益缩小，短基线部分场景不稳定；RSS 仍高于 Echo。
3. **64 KiB 折中**：[报告](../metrics-reports/fiber-64kib-buffer-2026-10-25.md)、[逐轮 JSON](../metrics-reports/fiber-64kib-buffer-2026-10-25.json)。Echo、8 KiB、64 KiB、1 MiB 四目标同轮比较，资源扩为四轮，使每个目标各占一次执行位置。首页 64 KiB RSS 接近 8 KiB，比 1 MiB 低约 17.6%–20.9%；未观察到相对 8 KiB 稳定的吞吐提升。保留 64 KiB 的理由是更大的请求行/请求头容量与接近 8 KiB 的内存，而非证明它是最优尺寸。
4. **跳转 CPU 与分配诊断及优化**：[报告](../metrics-reports/fiber-redirect-profile-2026-10-25.md)、[完整 JSON](../metrics-reports/fiber-redirect-profile-2026-10-25.json)、[CPU/heap 与原源码归档](../metrics-reports/fiber-redirect-profile-2026-10-25.tar.gz)。旧/新实现的 8 KiB、64 KiB 四目标，100000 请求/1000 预热/8 轮、两连接池，每个位置各两次；profile 与正式性能分开。缓存命中时延迟绝对路径解析后，64 KiB 跳转 QPS 提高约 32.1%/38.9%，CPU 少约 15.3%/17.6%，每请求分配字节少约 2.8%/3.8%。RSS 中位数变化 +0.21/−0.10 MiB，基本持平，没有新增长期缓存。优化后与 8 KiB 的吞吐差小于 1%。

更早的 [Gin 基线](../metrics-reports/baseline-report-gin.md)与 [Echo v5 基线](../metrics-reports/baseline-report-echo-v5.md)也保留，属于各自时期的历史记录，不能和上述不同时间、环境的数字直接相减。

## 如何解读

全部四轮实验均在 Apple M5、macOS/ARM64、Go 1.27.3、并发 8、GOMAXPROCS=4 上运行，离线、关闭登录与请求日志，使用默认数据，GOGC/GOMEMLIMIT 未设置。它们不是 NAS/Linux、生产容量或所有用户配置的保证。

原单进程基线含客户端分配，独立资源指标只覆盖服务端进程；RSS、存活堆与每请求分配不能混用。报告中的比值由各版本中位数计算，不等于逐轮配对比值的中位数。两连接池分时测量，不能跨组比较绝对 QPS 来归因连接池效果。后台负载、调度和温度仍会影响结果，未计算置信区间。

跳转分配主要来自每请求完整 YAML 解码。CPU profile 还发现锁内 `Getwd` 开销；基线切换目录后 PWD 未同步会放大它，另用 PWD 正确/错误的微基准核对。读取缓冲按连接复用，不能把上轮全部差距解释为“64 KiB 每请求重分配”。优化没有新增解析缓存或 URL 索引，也没有放宽跳转白名单、YAML 校验和上传限额。

## 可复现的源码与历史边界

实验阶段源码、双栈实现和依赖冻结在提交 **`92a9c47771b1083ec5c46e1ea6104ea061c14d42`**。该提交保留了迁移实验源码；后续集成主分支的移动端/编辑器更新，并将 Fiber 设为唯一实现。原 JSON 中的 revision、源码/二进制哈希和路径描述测量当时的工作区，不应要求它们与重命名后的当前文件一致。

`python3 scripts/compare-http-stacks.py --compare-read-buffers` 会从该固定提交提取临时源码，在隔离的源码目录重放 Echo/Fiber 比较，避免生产模块继续携带 Echo。可用 `--reference-revision` 明确指定另一份兼容的实验提交。它衡量固定实验源码，不是当前主分支产品；当前产品单独测量见 [基线方法](baseline-metrics.md)。浅克隆若没有该提交，需先取回包含它的 PR 分支/提交。

固定提交保留最后的 64 KiB 配置及跳转路径优化；前三次读取尺寸记录发生在路径优化之前，初次报告也采用 1 MiB。若要重放某一历史阶段，须恢复对应缓冲与源码状态：跳转采样归档中的 `before-source/config/data/bookmark.go` 保存优化前读取流程。原始报告与 JSON 才是当时测量的证据，新运行应保存到新目录，不能覆盖历史记录。

## 最终配置与验证

- 原生 Fiber/fasthttp 为唯一服务实现，64 KiB reader，5/10/60 秒读取/写入/空闲超时。
- `Immutable` 与长期保留表单字符串复制保持数据所有权；Gorilla 签名 Cookie 与原会话兼容，保留 TLS/显式 Secure-cookie 策略。
- 请求行/请求头超出容量可能返回 HTTP 431；请求体全局 32 MiB，导入路由继续使用原有更小限额与临时文件清理。
- favicon/SVG 保留标准库 ServeContent 的条件/Range 语义；主路由与性能路径使用原生监听器。
- 统一 JSON 错误响应隐藏内部 5xx 详情，服务端日志保留原始错误；表单读取保留原 net/http 的正文/查询优先级、显式空值和重复字段行为。

5/10/60 秒是底层读取、写入、空闲超时配置。fasthttp 与 net/http 的写入 deadline 设置时机不同，不能将这些值解释为完全相同的处理器总耗时上限。

## 默认切换后的验收

这些检查验证迁移后的产品，不是新增的性能结论：

- 资源生成、默认无标签构建、完整 Go 测试与 `go vet ./...` 通过；`go test -race ./...` 全量通过。
- 原生 HTTP/TLS、Gorilla 会话、64 KiB/431、上传/临时文件、模板、缓存/Range、表单优先级及错误详情隐藏回归通过。
- 时钟 19 个逻辑用例与浏览器验收通过；移动端 6 种宽度、字体放大、编辑器保存/失败重试/空数据和 3 种桌面宽度通过。本机浏览器验收使用已有 Chromium 156，CI 使用锁定 Playwright 的浏览器。
- Linux `amd64`、`arm64`、`arm/v6`、`arm/v7`、`386` 的 CGO 禁用构建通过。这里是交叉编译验证，不是这些平台的性能测量。
- 历史重放短烟雾测试验证四个目标、五场景、响应一致性与 8/64 KiB/1 MiB 实际配置。入口拒绝复用来源不明的二进制或不兼容参考提交；短测数据不作为吞吐收益证据。

具体命令和采样范围见 [基线方法](baseline-metrics.md)。


## Optimizations shipped in 0.6.5

The following reports measure separate optimizations; their percentages must not
be added together or presented as a whole-service RSS guarantee:

- [Parsed snapshots and lazy indexes](../metrics-reports/navigation-snapshots-2026-10-26.md): warmed data reads, ownership/invalidation semantics and allocation costs.
- [Category rendering](../metrics-reports/navigation-render-2026-10-26.md): matching/grouping and disabled-module work with small and large fixtures.
- [Editor client](../metrics-reports/editor-client-2026-10-26.md): repeated rendering, dirty checks, listeners, initial-load tradeoffs and JS heap; not server RSS.
- [Resources and URL validation](../metrics-reports/resources-and-url-validation-2026-10-26.md): conditional requests, icon-cache paths and parser microbenchmarks.

The [combined isolated-process report](../metrics-reports/audit-resource-comparison-2026-10-26.md)
compares pre-audit main `39df281` with reviewed head `94d2c0a` (merged as
`a116b716`). It retains the initial MIME-registry memory regression and a separate
final rerun: CPU/allocation traffic falls, while RSS/retained memory remains mixed.
It does not establish uniformly lower memory or whole-page browser latency.

Read each report's exact source/fixture, raw rounds and limitations. Snapshot and
index caches deliberately retain some data to reduce repeated decoding and GC
work; fewer allocated bytes per request do not alone prove a smaller idle process.

## Immutable resource metadata and navigation parsing

The resource follow-up shipped in 0.6.5 prepares strong content validators once per
immutable embedded file and avoids file opens/reads for warmed conditional and
HEAD responses. It retains conditional/range semantics, versions favicon/MDI
references, canonicalizes icon-cache names, and removes repeated navigation URL
parsing. Mutable generated SVG validators still derive from actual current bytes.
See the [microbenchmark report and raw rounds](../metrics-reports/resources-and-url-validation-2026-10-26.md).
Those allocation/time measurements are not a process CPU or RSS reduction claim.

### Avoidable MIME-registry initialization

The first combined resource measurement exposed additional retained heap after
favicon access. The embedded metadata loader now uses explicit bundled types and
content sniffing instead of initializing the host MIME registry. See the
[diagnostic and unchanged historical context](../metrics-reports/favicon-mime-init-2026-10-26.md).
The effect depends on installed system MIME mappings; its isolated HeapAlloc
result is not a universal RSS or CPU claim.
