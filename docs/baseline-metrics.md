# 基线采集与独立服务端分析

当前默认构建使用原生 Fiber v3，读取缓冲 **64 KiB**；Echo 已从生产实现与依赖中移除。各轮测试记录、原始数据与最终取舍见 [性能记录总览](performance.md)。历史报告中的 Echo 默认构建和 `-tags fiber` 描述对应当时的实验阶段。

## 构建与接口

```sh
go run build/build.go
go build -o .benchmark/flare .
go build -o .benchmark/fiber-baseline ./tools/baseline
```

五个默认场景为 `home`（GET /）、`home-search`（POST /，search=example）、`bookmarks`、`applications` 和 `redir-url`。跳转目标是默认数据中的 https://link.example.com，预期 HTTP 302；其余场景预期 HTTP 200。错误状态、读响应失败或截断响应会使测量失败。

原生服务回归与 HTTP 适配层测试分开：主服务和基线走原生 Fiber 监听器；`NewRouter` 的 net/http 接口用于已有集成测试/嵌入兼容。favicon/SVG 的 ServeContent 适配不在这五个性能场景中。

## 原有单进程基线

```sh
./scripts/baseline-metrics.sh
# 可指定新输出目录和工作量
REQUESTS=2000 CONCURRENCY=8 WARMUP=100 ./scripts/baseline-metrics.sh .benchmark/current-baseline
```

脚本现在默认且仅支持 Fiber。输出各场景的 Markdown 指标及 CPU/heap profile。每个场景的客户端与服务端在同一进程，`allocs/op`、`B/op`、CPU 和 heap profile 都可能包含客户端与工具开销，不能当作纯服务端单请求成本。

手动指定单个场景：

```sh
go run ./tools/baseline -scenario home -requests 2000 -concurrency 8 -warmup 100 -json
go run ./tools/baseline -scenario redir-url -requests 2000 -concurrency 8 -warmup 100 -cpuprofile .benchmark/redirect.cpu.pprof -memprofile .benchmark/redirect.heap
```

支持 `-scenario all|home|home-search|bookmarks|applications|redir-url`、`-markdown`、`-json`、`-cpuprofile`、`-memprofile`、`-keepalive-connections`、`-serve`、`-url`。`-cpuprofile`/`-memprofile` 用于单进程测量；独立服务端 profile 使用下文的控制口。

## 独立跳转测量

当前版本可以同时作为目标和统一客户端。`scripts/analyze-redirect.py` 接受现有的 baseline 可执行文件，不接受普通应用可执行文件；目标需要支持 `-serve`，不是另行构建框架适配器。

```sh
python3 scripts/analyze-redirect.py \
  --target fiber64=.benchmark/fiber-baseline \
  --client .benchmark/fiber-baseline \
  --output .benchmark/current-redirect \
  --pools 0 8 --rounds 8 \
  --resource-requests 100000 --resource-warmup 1000 \
  --concurrency 8 --gomaxprocs 4
```

比较多版本时重复 `--target label=/path/to/baseline`。本次正式跳转优化使用旧/新实现的 8 KiB、64 KiB 四个目标，8 轮让每个版本各占四个执行位置两次。四个目标统一使用同一个 Echo 构建的历史客户端，其负载实现与当前客户端相同；新测量也可以统一使用当前 Fiber 构建的客户端，须在报告中标明客户端哈希。

每个场景/轮次启动全新目标进程，预热后用独立客户端施压；记录目标进程用户态+内核态 CPU、运行时分配/GC、预热后 RSS、每 50 ms 采样的峰值 RSS 与生命周期峰值。独立控制端口快照有小额固定开销，计入目标计数。客户端输出中的分配字段仍只描述客户端，必须使用 `server_*` 字段判断服务端成本。

`results.json` 保存逐轮数据、顺序、二进制哈希、环境与响应验证。比较会检查状态、正文大小/哈希、Content-Type、CSP 和 Location；只归一化时钟文本与 MIME 大小写。先解决响应差异，再解读性能。

### Linux RSS 与 PID 命名空间

Linux 的 `getrusage(RUSAGE_SELF).Maxrss` 可能包含启动器在 fork/exec 之间继承的驻留内存，不能直接把历史 `lifetime_peak_rss_bytes` 当作目标程序自身的峰值。独立诊断中，仅让启动器额外分配 80 MiB，未改变的目标程序 `Maxrss` 就从约 18 MiB 升到 97 MiB，而目标 `/proc` 的 `VmRSS`／`VmHWM` 仍约 18.5 MiB。

新比较使用预热后的 `VmRSS`、负载期间采样峰值，以及 exec 后地址空间的 `VmHWM`；原始 `Maxrss` 仍保留但不用于比较。PID 命名空间与 `/proc` 视图不一致时，必须同时核对父进程 `PPid` 和目标 `NSpid`，不能直接用子进程的本地 PID 读取 `/proc/PID`，以免测到其他进程。历史报告和原始数据保持不变，新的诊断与限制见[整进程资源比较](../metrics-reports/audit-resource-comparison-2026-10-26.md)。

### 连接池口径

默认 `--pools 0 8`：0 保留 Go 客户端每主机最多 2 条空闲连接的原口径，8 显式保留 8 条。空闲连接数量不是并发上限，并发为 8 时默认池可能发生关闭/重建。两组应各自比较版本间的差异，不能跨时间用绝对 QPS 判断连接池优劣。

## 独立 CPU 与分配 profile

```sh
python3 scripts/analyze-redirect.py \
  --target fiber64=.benchmark/fiber-baseline \
  --client .benchmark/fiber-baseline \
  --output .benchmark/current-redirect-profiles \
  --pools 0 8 --profiles-only \
  --resource-warmup 1000 --profile-requests 200000 --profile-seconds 15 \
  --concurrency 8 --gomaxprocs 4
```

`--profiles-only` 跳过正式性能阶段，通过独立回环控制口下载 `/debug/pprof/profile?seconds=N` 和前后 `/debug/pprof/heap?gc=1`。它们仅存在于 `tools/baseline -serve`，不在应用服务中开放。CPU/heap 来自服务端进程，未混入独立客户端，仍有 profiler 自身的小额成本。

采样率保持 Go 默认值。固定 CPU 窗口不保证与有限请求负载严格等长；heap 前后在同一进程内，通过 `alloc_space` 或 `alloc_objects` 做差分：

```sh
go tool pprof -top .benchmark/fiber-baseline /path/to/cpu.pprof
go tool pprof -top -alloc_space -base /path/to/before.heap .benchmark/fiber-baseline /path/to/after.heap
```

直接查看 heap 默认展示存活对象，不能代表总分配。强制 GC 会影响 sync.Pool，profile 会改变成本，因此其 QPS/RSS 不作性能结论，须用未启用 profile 的独立测量验证。RSS 包含代码页、运行时和堆，与分配字节或单张 HeapAlloc 快照含义不同。

## 历史 Echo/Fiber 重放

双栈源码与依赖冻结在 `92a9c47771b1083ec5c46e1ea6104ea061c14d42`，不再留在生产包中。当前比较入口提取该提交的普通源码文件到临时目录，调用归档中的原脚本；运行结束删除临时源码，保留指定输出目录的记录与构建物。

```sh
python3 scripts/compare-http-stacks.py \
  --output .benchmark/historical-read-buffers \
  --requests 2000 --warmup 100 --rounds 5 \
  --resource-requests 20000 --resource-warmup 1000 --resource-rounds 4 \
  --concurrency 8 --gomaxprocs 4 --compare-read-buffers --keepalive-connections 0
```

补充连接池使用另一个输出目录并设置 `--keepalive-connections 8`。可用 `--reference-revision` 指定另一份兼容的实验提交。默认快照为最后的 64 KiB/跳转优化阶段；重放更早记录须还原对应配置与优化前读取流程，详见 [历史边界](performance.md#可复现的源码与历史边界)。浅克隆需要先获取参考提交。

快照内 Echo/Fiber 使用同一工具链、数据、工作量与 Go overlay 缓冲参照。结果会标记参考提交及 `historical_reference`，不能把它称为当前主分支的产品测量。历史报告与 JSON 不应被新结果覆盖。

## 配置与验证

生产 Fiber 采用 `Immutable`，长期保留的主题字符串另行复制；读取缓冲 64 KiB，请求行/请求头超出预算可能返回 HTTP 431。请求体全局 32 MiB，导入预览保留文件 8 MiB 加额外 64 KiB 请求封装限制、提交 16 MiB 与临时文件清理。Gorilla 会话、TLS Cookie、静态资源 Range/条件请求和模板缓冲复用继续由回归覆盖。

```sh
go test ./...
go vet ./...
go test -race ./...
```

本机测量存在后台负载、温度和调度噪声。重复与轮转只能降低影响，不能消除时间漂移；未计算置信区间，小幅差异不可外推为生产容量保证。
