# PR #210 完整 HTTP 搜索与分类基线对照

2026-10-27，Apple M5，macOS arm64，Go 1.27.3，原生生产二进制。
测量对象为已合并的 [PR #210](https://github.com/soulteary/flare/pull/210)。
本报告补充[渲染旁路基准](report.md)，没有新增生产代码优化。

## 结论

相同到达率与完成量下，四个场景的服务端 CPU/请求均下降约 28%–43%。
匿名搜索与分类的 p95 分别下降约 28% 和 35%；已登录搜索约下降 7%。
已登录分类 p95 两轮一升一降，汇总只约下降 1.5%，不认为已证实稳定的尾延迟改善。
RSS 没有一致下降，不能将此前分配下降描述成常驻内存改善。

## 冻结版本与运行条件

- 基线：`e9cadb6245cc8bc28bf5cda06a55cf5af79dd00f`，用 Git archive 导出后原生构建。
- 候选：`01f3f0596b04d522d83fadbc93f6dc14ba0f5818`，PR #210 提交，构建时工作区干净。
- 基线 SHA-256：`e35fdcf2b86eaf206ce57b62299981c84ab884f89cc4095bece027fe1649f44e`。
- 候选 SHA-256：`e0332b46ce2268000117485ee39d3e5f2e0333ccd0caa0a00925f3268896c532`。
- 两版同一 Go 工具链、依赖和默认构建选项，`CGO_ENABLED=1`、darwin/arm64；
  archive 基线没有 VCS 元数据，候选含 VCS 信息。源码归因以冻结提交和二进制哈希为准。
- `GOMAXPROCS=4`，不设置 GOGC/GOMEMLIMIT；测量期间没有运行其他构建、测试或 profile。
  未锁定 CPU 频率，也未建立温度或系统后台任务的实验控制。
- 两版均开启登录、DEFAULT 可见性、离线模式及编辑器，使用真实持久化账户与会话。
  未设置 `FLARE_BASELINE=1`，保留生产路由的来源保护、认证和默认 errors 访问日志中间件。
  不使用 test-only profiler、可信免登录基线或替换认证逻辑。

## 数据与 HTTP 工作负载

每个窗口启动一个新进程及临时数据目录：10,000 个应用、10,000 个书签，合计 20,000 条，
各集合半数私有。书签分为 20 类，每类同时有公开和私有条目，无远端请求。
标题、主题、语言和布局固定，关闭天气及日期时间显示，类别布局为 tabs。

- 搜索：`GET /?search=Entry%20099`，匿名返回 80 张卡片、已登录返回 200 张卡片。
- 分类：`GET /bookmarks?category=c00`，匿名返回 250 张卡片、已登录返回 500 张卡片。
- 匿名仍开启登录功能，只是不携带登录 Cookie；已登录通过正常登录表单及 CSRF 建立会话。
- 20 次同路径预热后，进行实际 30 秒开放式固定到达率测量。最多四个读取在途，无等待队列。
  匿名 20 次/秒，每窗口 600 次；已登录 4 次/秒，每窗口 120 次。
- 每场景两轮，每轮两版顺序执行；第一轮 baseline→candidate，第二轮 candidate→baseline。
  匿名与已登录分别比较，不跨不同身份、到达率或输出量归因。
- 每个工作线程复用标准 HTTP/1.1 keep-alive 连接。延迟从请求开始到完整正文读完，
  包含真实路由、文件检查、渲染、最终认证、服务端输出及回环传输；不含客户端后续卡片解析。
  不包含 DNS、TLS、压缩、远程网络或浏览器资源与 DOM 处理。

**有效窗口共 16 个、实际计时读取 5,760 次，全部 HTTP 200、零调度/在途丢弃、零操作错误、零采样错误。**
每个响应检查实际渲染的卡片多重集合，不以搜索框回显判断私有内容。配对窗口还校验
完整响应长度分布、卡片摘要、fixture 摘要及完成量一致。两版每次搜索正文分别为匿名
57,805 字节、已登录 82,885 字节；分类分别为 66,687、95,937 字节。
所有已登录有效窗口在计时外执行 CSRF 退出与旧 Cookie 重放，重放仅能读取公开内容。

## 延迟与 CPU

以下各值为两个窗口对应指标的中位数（两样本时等于均值），不是合并请求后的百分位数。
箭头均为基线 → 候选。CPU 是服务端原生进程累计 CPU 时间差除以完成量，包含 GC，
不包含 Python 客户端、采样 ps 子进程和登录/预热/退出探针。它不等于墙钟延迟或瞬时 CPU%。

| 场景 | p50（ms） | p95（ms） | 服务端 CPU（ms/请求） | CPU 降幅 |
| --- | ---: | ---: | ---: | ---: |
| 匿名搜索 | 6.109 → 4.316 | 7.565 → 5.472 | 6.300 → 4.017 | 36.2% |
| 匿名分类 | 2.437 → 1.768 | 3.685 → 2.413 | 2.600 → 1.475 | 43.3% |
| 已登录搜索 | 33.850 → 31.627 | 36.960 → 34.254 | 12.167 → 8.250 | 32.2% |
| 已登录分类 | 30.083 → 28.647 | 32.730 → 32.238 | 7.083 → 5.083 | 28.2% |

已登录请求的最终持久化会话检查仍然存在；页面计算减少并不会同比减少整个请求等待时间。
本轮未对剩余时间做系统调用或调度归因，不从上表推导持久化 I/O 的精确占比。
固定到达率测量的完成速率由负载决定，不将它解释为最大吞吐量或饱和容量提升。

## RSS

窗口内每秒采样服务端 RSS。CPU 使用同次 `ps` 的累计 time，测量机器精度为 10 ms；
各有效窗口的 CPU 增量至少约 0.60 秒，仍须保留计数精度和运行波动的限制。
单独通过 `wait4` 记录**该服务进程自身**的内核 RSS 高水位，不混入客户端或 ps 子进程。
高水位包含启动、登录、预热和计时外权限探针，不能当作纯读取阶段的峰值。

以下也为两轮对应指标的中位数，单位 MiB：

| 场景 | 窗口 RSS 中位数 | 窗口采样峰值 | 进程生命周期内核峰值 |
| --- | ---: | ---: | ---: |
| 匿名搜索 | 72.89 → 72.23 | 72.96 → 72.23 | 72.96 → 72.23 |
| 匿名分类 | 73.05 → 73.08 | 73.12 → 73.13 | 73.12 → 73.13 |
| 已登录搜索 | 93.87 → 92.84 | 93.88 → 92.85 | 93.99 → 92.94 |
| 已登录分类 | 92.17 → 93.90 | 92.20 → 93.91 | 92.27 → 94.00 |

匿名分类 RSS 一轮下降、另一轮上升；已登录分类窗口 RSS 中位数汇总约增加 1.73 MiB。
不能只选有利窗口证明省内存，也不能仅凭 RSS 差异将全部增长归因于快照索引。
索引会保留额外数据，Go 分配器、池和运行状态也影响 RSS；本轮没有堆 profile、强制 GC
或长期内存稳定性实验，因此结论仅为：**有 CPU 收益，没有一致的 RSS 收益证据。**

## 每轮尾延迟与一致性限制

| 场景 / 轮次 | 完成量：基线 / 候选 | p95（ms） | p99（ms） | CPU（ms/请求） |
| --- | ---: | ---: | ---: | ---: |
| 匿名搜索 / 1 | 600 / 600 | 7.596 → 5.480 | 8.244 → 6.007 | 6.317 → 4.150 |
| 匿名搜索 / 2 | 600 / 600 | 7.535 → 5.463 | 8.377 → 5.934 | 6.283 → 3.883 |
| 匿名分类 / 1 | 600 / 600 | 3.758 → 2.424 | 5.105 → 3.161 | 2.617 → 1.500 |
| 匿名分类 / 2 | 600 / 600 | 3.613 → 2.403 | 4.397 → 2.966 | 2.583 → 1.450 |
| 已登录搜索 / 1 | 120 / 120 | 36.493 → 34.107 | 37.756 → 34.898 | 11.250 → 7.917 |
| 已登录搜索 / 2 | 120 / 120 | 37.428 → 34.400 | 38.587 → 35.828 | 13.083 → 8.583 |
| 已登录分类 / 1 | 120 / 120 | 32.593 → 32.833 | 34.613 → 189.155 | 7.167 → 5.000 |
| 已登录分类 / 2 | 120 / 120 | 32.867 → 31.643 | 274.327 → 33.879 | 7.000 → 5.167 |

已登录分类第一轮候选 p99 约 189 ms，第二轮基线约 274 ms，另一版本同轮约 34 ms。
这些尖峰真实保留，不因不利于结论而删除；本轮没有足够证据定位其原因。
因此不以汇总 p99 声称稳定的尾延迟改善，也不将两轮短窗口当作 NAS/Linux、
生产饱和、物理移动设备、长时间运行或断电恢复的验收；没有置信区间。

## 校准排除与数据来源

完整排除记录见 [exclusions.json](http/exclusions.json)：

1. 1 秒 / 2 次每秒的烟雾窗口仅验证脚本，不纳入性能结论。
2. 初始匿名 80 次/秒时，候选搜索发生两次 scheduler-late 丢弃（2400 offered、2398 admitted）；
   无服务端操作错误，整组 80 次/秒数据排除，两版均改为 20 次/秒。
3. 初始已登录 8 次/秒时，第二轮基线搜索出现一次 inflight-cap 丢弃（240 offered、239 admitted）；
   无服务端操作错误，所有 8 次/秒已登录窗口排除，两版均以 4 次/秒重测两轮。
4. 20 次/秒匿名的八个窗口均有效，保留；新增 `--identities` 选择参数后只重跑已登录。
   选择参数没有修改测量函数，原测量脚本以 [initial-runner.py.txt](http/initial-runner.py.txt)
   及匹配的 SHA-256 保留。原始 manifest 保留实际执行顺序，包含不参与汇总的校准窗口。

正式汇总仅采用：

- [initial-20rps/manifest.json](http/initial-20rps/manifest.json) 中八个 anonymous 窗口。
- [authenticated-4rps/manifest.json](http/authenticated-4rps/manifest.json) 中全部八个窗口。
- [http-summary.json](http-summary.json) 明确列出每个汇总值引用的原始文件。
- [calibration-80rps/manifest.json](http/calibration-80rps/manifest.json) 仅保留校准来源，不参与结论。

每个 JSON 包含延迟样本、资源时间序列、状态数、响应大小、CPU 差值、RSS、二进制与
fixture 哈希及权限断言。不保存会话 Cookie 或响应中的 CSRF 令牌。

## 复现与工具验证

使用 Python 标准库、macOS/Linux 的 ps 和 wait4；需要回环监听及子进程采样权限。
从仓库根目录构建两个冻结版本，保证 Git 对象可用：

```sh
mkdir /tmp/flare-http-baseline-source-new
git archive e9cadb6245cc8bc28bf5cda06a55cf5af79dd00f | tar -x -C /tmp/flare-http-baseline-source-new
go build -C /tmp/flare-http-baseline-source-new -o /tmp/flare-http-baseline-new .
mkdir /tmp/flare-http-candidate-source-new
git archive 01f3f0596b04d522d83fadbc93f6dc14ba0f5818 | tar -x -C /tmp/flare-http-candidate-source-new
go build -C /tmp/flare-http-candidate-source-new -o /tmp/flare-http-candidate-new .
```

候选复现采用 archive，VCS 构建元数据会与本次工作区二进制不同；记录新二进制哈希，
不要要求它等于本次哈希，也不要覆盖本次数据。再依次运行：

```sh
python3 tools/security-load/navigation_http.py --baseline /tmp/flare-http-baseline-new --candidate /tmp/flare-http-candidate-new --output /tmp/flare-http-anonymous-new --duration 30 --rounds 2 --identities anonymous --anonymous-rate 20 --authenticated-rate 4
python3 tools/security-load/navigation_http.py --baseline /tmp/flare-http-baseline-new --candidate /tmp/flare-http-candidate-new --output /tmp/flare-http-authenticated-new --duration 30 --rounds 2 --identities authenticated --anonymous-rate 20 --authenticated-rate 4
python3 -m unittest discover -s tools/security-load -p 'test_*.py'
```

输出目录必须不存在。停止其他构建、测试和压测，任何丢弃/错误窗口都不能直接拿来比较
每请求资源；记录校准失败，在同一新到达率下重测两版。16 项 Python 工具测试通过，
覆盖 fixture 可见性、卡片断言防回显误判、百分位数，以及既有调度和 CPU 时间解析。
测量期间没有修改生产源码；此前 PR #210 的 Go 安全与竞态验证仍见原验证记录。
