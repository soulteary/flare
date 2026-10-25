# Baseline Metrics

本文说明 Flare 核心接口的基线和 Echo / Fiber 对比方式。默认构建仍使用 Echo v5；`fiber` 构建标签启用实验性的 Fiber v3 原生服务，当前候选将读取缓冲设为 **64 KiB**。读取尺寸的同轮测量见 [64 KiB 与 8 KiB / 1 MiB 比较报告](../metrics-reports/fiber-64kib-buffer-2026-10-25.md)，该报告采集于跳转路径优化之前。当前跳转诊断与独立复测见 [跳转路径报告](../metrics-reports/fiber-redirect-profile-2026-10-25.md)。

[8 KiB 历史报告](../metrics-reports/fiber-default-buffer-2026-10-25.md)和 [1 MiB 历史报告](../metrics-reports/fiber-comparison-2026-10-25.md)保留各自当时的配置和结果，不能直接代替当前 64 KiB 候选的测量。缓冲大小的差异应比较同源码、同轮运行的参照；读取尺寸报告中的旧路径结果不能直接代替后续路径优化版本的测量。

## 构建与启动

在仓库根目录执行：

```bash
# 默认 Echo
go run .
go build -o flare-echo .

# 实验性 Fiber
go run -tags fiber .
go build -tags fiber -o flare-fiber .
```

Fiber 主服务使用 fasthttp 原生监听器，路由、中间件和模板渲染均直接使用 Fiber。部分现有 HTTP 集成测试使用兼容入口；性能工具会选择原生服务入口。内嵌 favicon 和 SVG 资源仍通过 `net/http.ServeContent` 适配器保留条件请求与 Range 语义，下面五个压测场景不包含这些静态资源。

## 覆盖接口

- `GET /`
- `POST /`（搜索场景）
- `GET /bookmarks`
- `GET /applications`
- `GET /redir/url`（默认书签匹配后返回 HTTP 302）

基线使用相同的默认离线数据，关闭登录和请求日志，保留默认启用的引导页、编辑器等功能。离线模式排除了天气外部请求。登录、上传、大规模书签和静态资源不属于这五个场景的性能结论。

## 一键采集现有基线

脚本默认运行 Echo：

```bash
./scripts/baseline-metrics.sh
```

通过 `HTTP_STACK=fiber` 选择 Fiber：

```bash
HTTP_STACK=fiber ./scripts/baseline-metrics.sh
```

可调整请求数、并发数、预热次数和输出目录：

```bash
REQUESTS=5000 CONCURRENCY=16 WARMUP=200 \
  HTTP_STACK=fiber ./scripts/baseline-metrics.sh /tmp/flare-fiber-baseline
```

`HTTP_STACK` 仅接受 `echo` 或 `fiber`。不传输出目录时，结果保存在 `.benchmark/baseline-<timestamp>/`：

- `baseline-report.md`：包含所选 HTTP 栈、QPS、P50/P95/P99、allocs/op、B/op。
- `<scenario>.cpu.pprof`：CPU profile。
- `<scenario>.mem.pprof`：Heap profile。

`scenario` 为 `home`、`home-search`、`bookmarks`、`applications` 或 `redir-url`。

**原基线的服务端和压测客户端运行在同一进程。** allocs/op 和 B/op 是混合进程的分配差值；CPU 和 Heap profile 也同时包含客户端、服务端及压测工具开销，不能把这些数字直接当作服务端单请求成本。判断服务端资源收益，应使用下一节的独立进程测量。

## 重复交替对比与独立服务端资源

执行 Echo、当前 64 KiB Fiber，以及 8 KiB / 1 MiB Fiber 参照的完整对比：

```bash
python3 scripts/compare-http-stacks.py \
  --output .benchmark/http-comparison-64kib \
  --requests 2000 --warmup 100 --rounds 5 \
  --resource-requests 20000 --resource-warmup 1000 --resource-rounds 4 \
  --concurrency 8 --gomaxprocs 4 --compare-read-buffers --keepalive-connections 0
```

脚本使用同一 Go 工具链和工作负载，构建四个目标：`echo`、当前 64 KiB 的 `fiber`、`fiber_8kib` 和 `fiber_1mib`。各目标在多轮中轮换先后顺序，并输出各场景的中位数及完整逐轮数据：

- `comparison.md`：方法、现有基线对比、独立服务端资源、二进制体积和复现命令。
- `comparison.json`：逐轮原始指标、环境信息和响应校验结果。
- `echo-app`、`fiber-app` 及四个目标各自的 baseline 二进制：本次使用的构建产物。
- 各独立场景的服务端错误日志。

对比分为两部分：

1. **现有基线**：每个 HTTP 栈分别在一个进程内运行客户端和服务端，保留旧指标的采集口径。分配仍包含客户端。
2. **独立资源测量**：每个场景启动全新服务端进程，由单独的客户端进程施压；四个目标都使用同一个 Echo 构建的客户端。服务端 CPU 来自进程用户态与内核态耗时，分配及 GC 来自服务端运行时计数。独立控制端口采集快照，其少量固定开销会计入服务端计数。RSS 通过进程采样获得，同时保留预热后的 RSS 和进程生命周期峰值。

独立部分的 `server_*` 资源指标只覆盖服务端进程；客户端报告的分配字段仍只描述客户端，不能混用。RSS 包含代码页、运行时和堆，与单请求分配字节数或存活堆大小含义不同；每 50 ms 采样的峰值也可能遗漏短暂尖峰。

每轮独立测量会检查 HTTP 状态、响应体大小与哈希、Content-Type、CSP 和跳转目标。跨栈校验会归一化可见时钟文本及 Content-Type 大小写，原始校验值仍写入 JSON。响应不一致时应先排查兼容性，再解读性能。

### 连接池口径

默认 `--keepalive-connections 0` 保留原基线的 `net/http` 默认连接池，每个目标主机最多保留 **2 条空闲连接**。这不是并发连接上限；并发数为 8 时，多余连接的关闭和重建会影响 QPS、CPU、分配及 RSS。尤其要结合 Fiber 每连接 reader 缓冲的成本看待结果。

可追加一组明确保留 8 条空闲连接的补充测量：

```bash
python3 scripts/compare-http-stacks.py \
  --output .benchmark/http-comparison-64kib-keepalive-8 \
  --requests 2000 --warmup 100 --rounds 5 \
  --resource-requests 20000 --resource-warmup 1000 --resource-rounds 4 \
  --concurrency 8 --gomaxprocs 4 --compare-read-buffers --keepalive-connections 8
```

两组结果应分别解读：默认 0 延续旧基线；显式 8 观察并发为 8 时连接池更稳定的情况。报告会记录实际设置。交替顺序和重复运行只能减少机器噪声，不能消除温度、后台进程和共享压测机器带来的影响。小幅差异应结合逐轮数据判断。

### 同轮对比 64 KiB、8 KiB 与 1 MiB

已归档的读取尺寸报告属于跳转路径优化前的测量。下面命令会构建当前工作区源码；如要重现归档结果，应使用其记录的源码版本和构建产物，不能将新旧路径结果直接相减来归因缓冲尺寸。

当前 Fiber 候选显式设置 `ReadBufferSize: 64 << 10`。`--compare-read-buffers` 在同一次实验中比较四个目标：Echo、当前 64 KiB Fiber、8 KiB Fiber 和 1 MiB Fiber。8 KiB 与 1 MiB 参照通过 Go overlay 仅替换当前源码中的 `ReadBufferSize` 值，其他源码和构建参数保持一致；工作区源码仍使用 64 KiB。overlay 配置与替代源码也保存在输出目录中。

两组连接池测量均保持每场景基线 **2000 次请求 / 100 次预热、5 轮**，独立资源 **20000 次请求 / 1000 次预热、4 轮**，并发 **8**、`GOMAXPROCS=4`。基线保留原来的 5 轮数量；独立资源改用 4 轮，使每个目标在四个执行位置各出现一次，减少固定先后位置对资源对比的影响。这里的轮次和上面的复现命令一致。

原生目标通过 `ReadBufferSize()` getter 上报实际生效字节数；`comparison.json` 的 `read_buffer_bytes` 元数据和每次独立测量记录都会保存该值，脚本会校验参照目标的缓冲大小。本次已核对三个 Fiber 目标分别为 65536、8192 和 1048576 字节。`comparison.md` 区分原单进程基线吞吐和独立服务端 CPU、分配、GC、RSS。同轮参照用于比较缓冲大小，历史报告继续保留历史含义。


## 跳转路径独立诊断与复测

当前读取缓冲仍为 **64 KiB**。针对 `redir-url` 的调查使用 `scripts/analyze-redirect.py`，将无 profile 的正式资源测量和启用 profile 的诊断分别运行。入口见 [跳转诊断与复测报告](../metrics-reports/fiber-redirect-profile-2026-10-25.md)，其结论应以保存的实际数据为准。

脚本接受可重复的 `--target label=现有baseline二进制`、一个通用 Echo baseline `--client` 以及 `--output`，不会替用户构建二进制。以下路径为占位示例，运行前须换成保存好的各版本 baseline 产物；这些目标需要支持 `-serve`，不是普通应用可执行文件。标签可自行命名，用于区分 Echo、8 KiB 参照、64 KiB 路径优化前后等版本。

本次正式复测的参数为 **8 轮、每轮 100000 次请求、1000 次预热、并发 8、`GOMAXPROCS=4`**：

```bash
python3 scripts/analyze-redirect.py \
  --target before_8kib=/path/to/fiber8-before-baseline \
  --target before_64kib=/path/to/fiber64-before-baseline \
  --target after_8kib=/path/to/fiber8-after-baseline \
  --target after_64kib=/path/to/fiber64-after-baseline \
  --client /path/to/echo-baseline \
  --output .benchmark/redirect-path-comparison \
  --pools 0 8 --rounds 8 \
  --resource-requests 100000 --resource-warmup 1000 \
  --concurrency 8 --gomaxprocs 4
```

默认测量两种连接池：0 延续原 Go 客户端每主机保留 2 条空闲连接的行为，8 显式保留 8 条空闲连接。每轮轮转目标顺序；上例四个目标在 8 轮中各占据每个执行位置两次。每次启动独立目标进程，由同一个 Echo 客户端施压，记录服务端 CPU、分配、GC 和 RSS；正式性能阶段不开启 profile。`results.json` 保存逐轮数据、执行顺序、二进制哈希、参数和响应校验信息。

### 仅提取独立服务端 profile

`--profiles-only` 跳过正式性能阶段，通过目标的独立回环控制端口下载 `/debug/pprof/profile?seconds=N` 和负载前后的 `/debug/pprof/heap?gc=1`。目标二进制须包含这些基线控制接口；普通应用服务不开放它们。例如，只诊断两个缓冲尺寸：

```bash
python3 scripts/analyze-redirect.py \
  --target fiber8_ref=/path/to/fiber8-baseline \
  --target fiber64=/path/to/fiber64-baseline \
  --client /path/to/echo-baseline \
  --output .benchmark/redirect-path-profiles \
  --pools 0 8 --profiles-only \
  --resource-warmup 1000 --profile-requests 200000 --profile-seconds 15 \
  --concurrency 8 --gomaxprocs 4
```

CPU 和 heap profile 都来自服务端进程，不包含独立客户端的调用栈；仍会包含控制接口及 profiler 自身的少量开销。保持 Go 默认采样率，CPU 使用定时采样窗口，heap 是采样数据。诊断目录保存 `cpu.pprof`、`before.heap`、`after.heap` 及热点摘要。

分配归因使用同一进程的前后快照做 **`alloc_space` 差分**；直接查看 heap 默认展示的是存活对象，不能代表负载期间的总分配。脚本同时生成 `alloc_space` 和 `alloc_objects` 的差分摘要。`gc=1` 会强制 GC 并影响缓冲池状态，profile 本身也会增加成本，因此 **profile 期间的 QPS、RSS 不用于性能结论**。热点用于解释原因，收益须回到独立运行、未启用 profile 的正式测量中验证。

## Fiber 候选版本的兼容性取舍

本次候选保留以下配置，性能数字包含这些成本：

- **`Immutable: true`**：为跨请求保留的请求数据提供稳定所有权。Fiber 的表单 binder 仍可能借用 PostArgs 缓冲，因此长期保存的主题名额外使用 `strings.Clone`；回归测试会真实复用请求缓冲区，检查主题不被后续请求改写。
- **读取缓冲 64 KiB**：显式设置 `ReadBufferSize: 64 << 10`。fasthttp 的连接读取缓冲也限制请求行及请求头容量，超出预算可能返回 HTTP 431；编码后较长的书签跳转 URL 和较大的 Cookie 等请求头需要考虑这一约束。此容量高于 Fiber 默认 8 KiB，仍低于 Echo 所用 `net/http` 默认约 1 MiB 的请求头上限。读取缓冲带来每连接固定成本，`--compare-read-buffers` 用同轮 8 KiB / 1 MiB 参照测量容量与资源成本的取舍。
- **请求体全局上限 32 MiB**：Fiber 在进入业务路由前缓冲请求体，并关闭 multipart 提前解析。书签预览继续使用标准库 multipart 解析器，保留文件 8 MiB、请求封装额外 64 KiB 的限制及临时文件清理；提交请求继续限制为 16 MiB。上传处理的缓冲方式与 Echo 不同，上传不在这五个性能场景中。
- **静态资源兼容性**：内嵌 favicon/SVG 使用 ServeContent 适配器；主服务及五个场景走原生 Fiber。

这组候选配置保留了上述业务兼容处理，同时采用 64 KiB 读取容量；请求容量和缓冲方式仍与 Echo 不同。测量不能直接代表其他 Fiber 配置、生产容量或 NAS/Linux 上的表现。

## 手动运行单场景

```bash
# Echo
go run ./tools/baseline -scenario home -requests 2000 -concurrency 8 -warmup 100

# Fiber
go run -tags fiber ./tools/baseline -scenario home -requests 2000 -concurrency 8 -warmup 100
```

可选参数：

- `-scenario`：`all|home|home-search|bookmarks|applications|redir-url`。
- `-cpuprofile <path>`：导出当前进程的 CPU profile。
- `-memprofile <path>`：导出当前进程的 Heap profile。
- `-markdown`：以 Markdown 表格行输出。
- `-json`：输出机器可读指标。
- `-keepalive-connections <n>`：每个主机保留的空闲连接数；默认 0 延续原默认连接池。
- `-serve`：仅启动原生目标服务及仅监听回环地址的资源控制端口。
- `-url <url>`：对独立目标施压；此模式下当前进程的分配指标仅包含客户端。

## Profile 分析

查看热点函数：

```bash
go tool pprof -top .benchmark/baseline-<timestamp>/home.cpu.pprof
go tool pprof -top .benchmark/baseline-<timestamp>/home.mem.pprof
```

以上 profile 来自现有单进程基线，分析时应区分客户端与服务端调用栈。
