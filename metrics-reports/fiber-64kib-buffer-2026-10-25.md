# Fiber 64 KiB 读取缓冲复测：2026-10-25

> 历史实验记录：本文的构建标签、默认框架及源码路径描述对应测量当时的实验阶段。当前默认已切换为 64 KiB Fiber；完整记录和复现边界见 [性能总览](../docs/performance.md)。原始数字与当时结论保持不变。

64 KiB 可以作为请求头容量与资源占用的折中：本次内存接近 8 KiB，四个页面场景的 CPU 时间比 8 KiB 略少，但没有稳定的吞吐提升。相对 1 MiB，所有场景的采样峰值 RSS 都下降，CPU 时间则都增加。因此，这次结果支持它作为折中候选，不能认定它是三个尺寸中性能最好的选择。

工作区 `-tags fiber` 候选现为 **64 KiB（65536 字节）**，默认应用构建继续使用 Echo。8 KiB 和 1 MiB 都是本次同源码参照；以下不对历史两轮的绝对数值直接相减。

## 测量方式

沿用五个场景（首页、首页搜索、书签、应用、跳转）、并发 8、GOMAXPROCS=4、离线、关闭登录与请求日志、默认书签数据。原基线每场景 2000 请求、100 次预热、5 轮；独立资源测量每场景 20000 请求、1000 次预热。分别测量默认客户端池（每主机最多保留 2 条空闲连接）和显式 8 条空闲连接池。主机为 Apple M5 / macOS 26.5.1 / ARM64、Go 1.27.3，GOGC/GOMEMLIMIT 未设置。

本轮增加到四个目标：Echo、64 KiB Fiber、8 KiB Fiber、1 MiB Fiber。每轮工作负载不变；独立资源轮数由上一轮的 3 改为 **4**，让每个目标在首、次、三、末位置各出现一次。原基线保留 5 轮，各目标的四种位置出现次数相差不超过一次。轮转顺序保存在 JSON。

参照通过 Go overlay **只替换 `ReadBufferSize` 字段值**，已逐字核对替代源码与当前源码只有这一差异；构建哈希和源码哈希均已核对。全部原生运行回报的缓冲分别为 8192、65536、1048576 字节。两组各 100 条原基线、80 条独立资源记录，响应预检均无差异，全部测量成功完成。

## 首页的内存与 CPU

每格为独立服务 4 轮中位数。RSS 是负载期间每 50 ms 采样的进程峰值，包含代码页、运行时和堆，可能漏掉短暂尖峰；CPU 是服务端用户态与内核态时间之和除以请求数，不含客户端。

| 客户端空闲池 | Echo RSS（MiB） | 8 KiB RSS | 64 KiB RSS | 1 MiB RSS | 64 KiB 对 8 KiB 的 RSS 差 | 64 KiB 对 1 MiB 的 RSS 降幅 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 默认 2 条 | 27.63 | 31.15 | 30.75 | 38.89 | -0.40 MiB | 20.9% |
| 8 条 | 27.36 | 30.52 | 31.22 | 37.91 | +0.70 MiB | 17.6% |

| 客户端空闲池 | Echo CPU（µs/请求） | 8 KiB CPU | 64 KiB CPU | 1 MiB CPU | 64 KiB 对 8 KiB 的 CPU 变化 | 64 KiB 对 1 MiB 的 CPU 变化 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 默认 2 条 | 784.0 | 579.6 | 516.9 | 474.1 | -10.8% | +9.0% |
| 8 条 | 964.3 | 701.2 | 677.4 | 524.3 | -3.4% | +29.2% |

首页 64 KiB 的 RSS 与 8 KiB 相差 **−0.40 / +0.70 MiB**，可视为接近；相对 1 MiB 少 **6.69–8.14 MiB（17.6%–20.9%）**，仍比 Echo 多 **3.12–3.86 MiB**。CPU 时间相对 8 KiB 少 **3.4%–10.8%**，相对 1 MiB 多 **9.0%–29.2%**。这不是“内存更少、性能也全面更好”的结果。

## 首页的吞吐与延迟

以下为原单进程基线的 5 轮 QPS 中位数，保留旧基线口径。吞吐保留 = 64 KiB QPS / 1 MiB QPS。比值采用各版本中位数计算，不是逐轮配对百分比的中位数。

| 客户端空闲池 | Echo QPS | 8 KiB QPS | 64 KiB QPS | 1 MiB QPS | 64 KiB 对 8 KiB | 64 KiB 吞吐保留（对 1 MiB） |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 默认 2 条 | 6333 | 7125 | 6995 | 8314 | -1.8% | 84.1% |
| 8 条 | 2946 | 3757 | 3526 | 3947 | -6.2% | 89.3% |

| 客户端空闲池 | 8 KiB P95（ms） | 64 KiB P95 | 1 MiB P95 |
| --- | ---: | ---: | ---: |
| 默认 2 条 | 2.459 | 2.477 | 2.154 |
| 8 条 | 3.649 | 3.708 | 3.517 |

两组原基线的首页 64 KiB QPS 比 8 KiB 低 **1.8% / 6.2%**，保留 1 MiB 的 **84.1% / 89.3%** 吞吐；P95 略高于 8 KiB。这些是描述性中位数，不能据此断言小幅差异一定是稳定回退。

较长的独立服务测试与原基线的进程结构不同，应分别查看：

| 客户端空闲池 | Echo 独立 QPS | 8 KiB 独立 QPS | 64 KiB 独立 QPS | 1 MiB 独立 QPS | 64 KiB 对 8 KiB | 64 KiB 吞吐保留（对 1 MiB） |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 默认 2 条 | 3418 | 4230 | 4101 | 4036 | -3.0% | 101.6% |
| 8 条 | 2657 | 3283 | 3287 | 3892 | +0.1% | 84.5% |

独立测试中，首页 64 KiB 相对 8 KiB 为 **−3.0% / +0.1%**，也没有显示稳定的吞吐提升。CPU 时间下降不等于 QPS 按相同比例提高。两组连接池在不同时间测量，机器状态变化明显，不能跨组比较绝对 QPS 来判断连接池设置优劣。

## 全场景取舍与当前判断

- 相对 8 KiB，64 KiB 的 RSS 差异为 **−0.40～+1.81 MiB**，没有显示额外的内存节省；四个页面场景的 CPU 时间均下降，范围约 **3.4%～11.9%**，独立吞吐则随场景变化。
- 跳转场景是反例：默认池 / 8 条池中，64 KiB 相对 8 KiB 的 CPU 时间分别 **+14.4% / +6.4%**，独立吞吐分别 **−15.7% / −4.0%**。因此不能只依据页面 CPU 结果声称全面更快。
- 相对 1 MiB，64 KiB 在全部场景的 RSS 下降 **17.6%～46.8%**，CPU 时间全部增加 **9.0%～29.2%**。完整数据中默认池 1 MiB 跳转 RSS 尤其高，应结合逐轮值看待，不能把最大降幅推广到首页。
- 64 KiB 提供比 8 KiB 更大的请求行及请求头容量，内存仍接近 8 KiB；若需要容纳更长的书签跳转 URL 或请求头，它是一个可考虑的折中。仅凭本轮性能结果，没有充分依据认为它优于 8 KiB；三个采样尺寸也不足以确定最优值。

## 配置边界与验证

- 当前 Fiber 显式配置 `ReadBufferSize: 64 << 10`；容量超限仍可能返回 HTTP 431。64 KiB 是连接读取缓冲，不能直接当作可用 URL 长度或请求头净容量。
- 原生监听回归覆盖实际 64 KiB 配置、预算内请求头与书签 URL 成功、超预算 HTTP 431、主路由，以及 TLS Cookie 登录/登出。
- Echo 与 Fiber 的完整测试、两种构建的静态检查，以及 Fiber 服务与基线工具的竞态测试均通过。
- `Immutable`、主题字符串复制、上传限制、模板缓冲池和静态资源适配策略沿用上一版；本次仅改变读取缓冲并扩充对比脚本。
- 本机测试存在后台负载、温度和调度噪声，未计算置信区间。资源顺序均衡只能降低固定执行位置的影响，不能消除时间漂移；小幅差异不宜外推到生产、NAS 或 Linux。

## 数据与复现

相邻的 [原始 JSON](fiber-64kib-buffer-2026-10-25.json) 保存两组各 100 条原基线和 80 条独立资源记录、响应校验、实际缓冲、执行顺序、环境和构建哈希，键分别为 `original_pool`、`keepalive_pool_8`。下方保留全场景表格和完整复现命令。历史 [8 KiB 报告](fiber-default-buffer-2026-10-25.md)与 [1 MiB 报告](fiber-comparison-2026-10-25.md)继续保留历史含义。

## 完整结果：默认客户端连接池（2 条空闲连接/主机）

- Generated: 2026-10-25T18:35:30+08:00 (Asia/Shanghai)
- Source: `3e09ee4522d43c62eef31d98d74e20744b478cb6` plus the experimental changes in this workspace
- Host: macOS-26.5.1-arm64-arm-64bit-Mach-O · go version go1.27.3 darwin/arm64 · GOMAXPROCS=4
- CPU: Apple M5 · logical CPUs: 10
- Client idle connections per host: 2 (original Go default)
- Frameworks: Echo v5.4.1; Fiber v3.6.1 / fasthttp v1.75.0
- Effective Fiber read buffer: 65536 bytes (reported by the native target)

### Method

Both builds use identical offline fixtures, disabled login and request logging, the same Go toolchain, concurrency, and client. The default configuration still enables the guide and editor. Fiber serves requests on its native fasthttp listener; the net/http compatibility adapter is used only by existing HTTP integration tests. Fiber enables Immutable and explicitly copies retained bound form strings. Its effective read buffer is recorded above; buffer references are built from the same source. Request lines and headers exceeding that buffer can receive HTTP 431, so this candidate no longer preserves Echo’s larger header capacity. Static favicon/SVG responses retain a ServeContent adapter and are outside these five scenarios.

Existing baseline: 5 alternating rounds, 2000 requests and 100 warmup requests per scenario, concurrency 8. Each stack runs in its own process, with its client and server in that same process as in the original harness. Allocation values in this section include the client. Cells show the median across rounds.

| Scenario | Echo QPS | Fiber QPS | QPS change | Echo P95 (ms) | Fiber P95 (ms) |
| --- | ---: | ---: | ---: | ---: | ---: |
| home | 6333 | 6995 | +10.5% | 2.599 | 2.477 |
| home-search | 6426 | 6887 | +7.2% | 2.709 | 2.612 |
| bookmarks | 8722 | 10271 | +17.8% | 1.307 | 1.148 |
| applications | 15622 | 17992 | +15.2% | 0.936 | 0.792 |
| redir-url | 24237 | 25818 | +6.5% | 0.568 | 0.503 |

### Isolated server resources

4 alternating rounds; fresh target process per scenario; 1000 warmup requests followed by 20000 measured requests, concurrency 8. The load generator is a separate process and always uses the Echo-built client binary for all targets. CPU is target-process user+system time from getrusage; allocations and GC are target-process runtime counters. Snapshots are served on a separate loopback control listener, whose small fixed overhead is amortized into the counters. RSS is the target process sampled with ps every 50 ms; it includes executable pages, runtime and heap. RSS after warmup and the lifetime high-water mark are retained in the raw data. Neither RSS nor retained heap is equivalent to bytes allocated per request.

| Scenario | Echo QPS | Fiber QPS | QPS change | Echo CPU (µs/req) | Fiber CPU (µs/req) | CPU change | Echo allocs/req | Fiber allocs/req | Echo KiB/req | Fiber KiB/req |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 3418 | 4101 | +20.0% | 784.0 | 516.9 | -34.1% | 2652.0 | 2622.4 | 206.4 | 165.4 |
| home-search | 3416 | 4252 | +24.5% | 801.1 | 550.5 | -31.3% | 2684.6 | 2641.2 | 209.1 | 168.7 |
| bookmarks | 4280 | 5323 | +24.4% | 672.5 | 460.0 | -31.6% | 2068.4 | 2046.4 | 158.9 | 126.6 |
| applications | 7368 | 8552 | +16.1% | 379.7 | 287.3 | -24.3% | 829.8 | 812.2 | 71.4 | 52.5 |
| redir-url | 12003 | 10825 | -9.8% | 145.8 | 145.1 | -0.5% | 374.4 | 356.3 | 21.6 | 19.6 |

| Scenario | Echo P95 (ms) | Fiber P95 (ms) | Echo idle RSS (MiB) | Fiber idle RSS (MiB) | Echo sampled peak RSS (MiB) | Fiber sampled peak RSS (MiB) | Echo GC cycles | Fiber GC cycles |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 3.99 | 3.60 | 25.33 | 28.67 | 27.63 | 30.75 | 3915.50 | 1726.50 |
| home-search | 3.92 | 3.44 | 24.88 | 29.08 | 26.77 | 31.80 | 3893.50 | 1758.50 |
| bookmarks | 2.60 | 2.28 | 24.80 | 28.45 | 27.06 | 29.98 | 2972.50 | 1360.50 |
| applications | 1.95 | 1.62 | 24.11 | 27.53 | 26.25 | 29.53 | 1131.50 | 546.50 |
| redir-url | 1.20 | 1.27 | 23.14 | 25.34 | 24.05 | 26.91 | 215.50 | 233.00 |

### Same-session read buffer comparison

All Fiber variants use the same current source; Go overlays only replace ReadBufferSize. Each native target reports its effective size. All variants and Echo are rotated in the same session. Tables show per-stack medians; their ratios are not medians of paired per-round ratios. Small gains remain sensitive to host noise.

| Scenario | Echo baseline QPS | 8 KiB baseline QPS | 64 KiB baseline QPS | 1 MiB baseline QPS | Candidate vs 8 KiB | Candidate throughput retained vs 1 MiB |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 6333 | 7125 | 6995 | 8314 | -1.8% | 84.1% |
| home-search | 6426 | 7047 | 6887 | 8009 | -2.3% | 86.0% |
| bookmarks | 8722 | 9462 | 10271 | 11420 | +8.5% | 89.9% |
| applications | 15622 | 15900 | 17992 | 19265 | +13.2% | 93.4% |
| redir-url | 24237 | 24150 | 25818 | 27588 | +6.9% | 93.6% |

| Scenario | 8 KiB baseline P95 (ms) | 64 KiB baseline P95 (ms) | 1 MiB baseline P95 (ms) |
| --- | ---: | ---: | ---: |
| home | 2.459 | 2.477 | 2.154 |
| home-search | 2.436 | 2.612 | 2.382 |
| bookmarks | 1.196 | 1.148 | 1.177 |
| applications | 0.982 | 0.792 | 0.733 |
| redir-url | 0.584 | 0.503 | 0.429 |

| Scenario | 8 KiB peak RSS (MiB) | 64 KiB peak RSS (MiB) | 1 MiB peak RSS (MiB) | Candidate RSS change vs 8 KiB | Candidate RSS reduction vs 1 MiB |
| --- | ---: | ---: | ---: | ---: | ---: |
| home | 31.15 | 30.75 | 38.89 | -0.40 MiB | 20.9% |
| home-search | 29.99 | 31.80 | 39.79 | +1.81 MiB | 20.1% |
| bookmarks | 30.04 | 29.98 | 38.55 | -0.05 MiB | 22.2% |
| applications | 28.43 | 29.53 | 43.48 | +1.10 MiB | 32.1% |
| redir-url | 26.41 | 26.91 | 50.55 | +0.51 MiB | 46.8% |

| Scenario | 8 KiB CPU (µs/req) | 64 KiB CPU (µs/req) | 1 MiB CPU (µs/req) | 8 KiB isolated QPS | 64 KiB isolated QPS | 1 MiB isolated QPS |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 579.6 | 516.9 | 474.1 | 4229.8 | 4101.4 | 4036.2 |
| home-search | 602.6 | 550.5 | 440.5 | 3974.8 | 4251.7 | 4817.8 |
| bookmarks | 506.9 | 460.0 | 363.9 | 5071.5 | 5322.6 | 6172.3 |
| applications | 299.1 | 287.3 | 260.0 | 8423.5 | 8551.9 | 8921.4 |
| redir-url | 126.9 | 145.1 | 124.5 | 12848.2 | 10825.2 | 12277.0 |

### Limits and interpretation

- These are local macOS/ARM64 loopback tests, not NAS/Linux measurements or production capacity estimates.
- The original client keeps at most 2 idle connections per host, so concurrency 8 can cause connection churn. An explicit --keepalive-connections setting is recorded above; results include the costs of that connection pattern.
- Closed-loop throughput uses a fixed concurrency; the machine also runs the client. Alternating order reduces but does not eliminate host noise and thermal effects.
- The five original baseline scenarios use default fixtures. `redir-url` validates the existing default-fixture redirect to https://link.example.com and expects HTTP 302.
- Offline mode excludes weather network requests. Login, uploads, large collections and static assets are validated separately and are not performance claims from this run.
- Every isolated run validates the expected HTTP status (200 or 302), response body hash/size, content type, CSP and redirect destination. Content type casing and the visible clock time are normalized for cross-stack comparisons; raw preflight results are stored for cross-stack comparison.
- The compatibility-oriented candidate keeps Immutable enabled and standard-library multipart parsing; results do not represent an aggressively tuned Fiber microbenchmark.
- Medians are descriptive. Check raw per-round data before treating small differences as improvements.

### Binary size

- Echo application: 22.02 MiB; Fiber application: 28.30 MiB (same default build flags, no stripping).

### Reproduce

```sh
python3 scripts/compare-http-stacks.py --output .benchmark/http-comparison --requests 2000 --warmup 100 --rounds 5 --resource-requests 20000 --resource-warmup 1000 --resource-rounds 4 --concurrency 8 --gomaxprocs 4 --keepalive-connections 0 --compare-read-buffers
```

Full per-round metrics and metadata: `fiber-64kib-buffer-2026-10-25.json`, key `original_pool`; also `comparison.json` in the output directory.

## 完整结果：8 条空闲连接/主机

- Generated: 2026-10-25T18:41:20+08:00 (Asia/Shanghai)
- Source: `3e09ee4522d43c62eef31d98d74e20744b478cb6` plus the experimental changes in this workspace
- Host: macOS-26.5.1-arm64-arm-64bit-Mach-O · go version go1.27.3 darwin/arm64 · GOMAXPROCS=4
- CPU: Apple M5 · logical CPUs: 10
- Client idle connections per host: 8 (explicit pool)
- Frameworks: Echo v5.4.1; Fiber v3.6.1 / fasthttp v1.75.0
- Effective Fiber read buffer: 65536 bytes (reported by the native target)

### Method

Both builds use identical offline fixtures, disabled login and request logging, the same Go toolchain, concurrency, and client. The default configuration still enables the guide and editor. Fiber serves requests on its native fasthttp listener; the net/http compatibility adapter is used only by existing HTTP integration tests. Fiber enables Immutable and explicitly copies retained bound form strings. Its effective read buffer is recorded above; buffer references are built from the same source. Request lines and headers exceeding that buffer can receive HTTP 431, so this candidate no longer preserves Echo’s larger header capacity. Static favicon/SVG responses retain a ServeContent adapter and are outside these five scenarios.

Existing baseline: 5 alternating rounds, 2000 requests and 100 warmup requests per scenario, concurrency 8. Each stack runs in its own process, with its client and server in that same process as in the original harness. Allocation values in this section include the client. Cells show the median across rounds.

| Scenario | Echo QPS | Fiber QPS | QPS change | Echo P95 (ms) | Fiber P95 (ms) |
| --- | ---: | ---: | ---: | ---: | ---: |
| home | 2946 | 3526 | +19.7% | 4.187 | 3.708 |
| home-search | 2510 | 3380 | +34.7% | 4.983 | 4.038 |
| bookmarks | 3836 | 4709 | +22.8% | 2.773 | 2.532 |
| applications | 7074 | 8428 | +19.1% | 1.964 | 1.720 |
| redir-url | 10642 | 11601 | +9.0% | 1.288 | 1.140 |

### Isolated server resources

4 alternating rounds; fresh target process per scenario; 1000 warmup requests followed by 20000 measured requests, concurrency 8. The load generator is a separate process and always uses the Echo-built client binary for all targets. CPU is target-process user+system time from getrusage; allocations and GC are target-process runtime counters. Snapshots are served on a separate loopback control listener, whose small fixed overhead is amortized into the counters. RSS is the target process sampled with ps every 50 ms; it includes executable pages, runtime and heap. RSS after warmup and the lifetime high-water mark are retained in the raw data. Neither RSS nor retained heap is equivalent to bytes allocated per request.

| Scenario | Echo QPS | Fiber QPS | QPS change | Echo CPU (µs/req) | Fiber CPU (µs/req) | CPU change | Echo allocs/req | Fiber allocs/req | Echo KiB/req | Fiber KiB/req |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 2657 | 3287 | +23.7% | 964.3 | 677.4 | -29.7% | 2652.0 | 2622.6 | 206.4 | 165.9 |
| home-search | 2540 | 3153 | +24.1% | 1001.1 | 692.4 | -30.8% | 2684.5 | 2641.3 | 208.9 | 169.0 |
| bookmarks | 3938 | 4740 | +20.4% | 735.6 | 509.6 | -30.7% | 2068.1 | 2046.5 | 158.3 | 126.7 |
| applications | 7106 | 8028 | +13.0% | 389.2 | 304.8 | -21.7% | 829.8 | 812.2 | 71.5 | 52.6 |
| redir-url | 11223 | 11262 | +0.4% | 154.8 | 143.5 | -7.3% | 374.3 | 356.2 | 21.6 | 19.4 |

| Scenario | Echo P95 (ms) | Fiber P95 (ms) | Echo idle RSS (MiB) | Fiber idle RSS (MiB) | Echo sampled peak RSS (MiB) | Fiber sampled peak RSS (MiB) | Echo GC cycles | Fiber GC cycles |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 4.70 | 4.05 | 24.98 | 29.16 | 27.36 | 31.22 | 3908.50 | 1739.00 |
| home-search | 5.13 | 4.45 | 25.02 | 28.98 | 27.99 | 30.89 | 3810.50 | 1768.50 |
| bookmarks | 2.76 | 2.45 | 24.47 | 28.53 | 26.56 | 30.67 | 2922.50 | 1371.00 |
| applications | 1.88 | 1.69 | 24.14 | 27.52 | 25.83 | 29.57 | 1142.50 | 551.50 |
| redir-url | 1.26 | 1.23 | 23.23 | 25.21 | 24.19 | 26.30 | 214.50 | 234.00 |

### Same-session read buffer comparison

All Fiber variants use the same current source; Go overlays only replace ReadBufferSize. Each native target reports its effective size. All variants and Echo are rotated in the same session. Tables show per-stack medians; their ratios are not medians of paired per-round ratios. Small gains remain sensitive to host noise.

| Scenario | Echo baseline QPS | 8 KiB baseline QPS | 64 KiB baseline QPS | 1 MiB baseline QPS | Candidate vs 8 KiB | Candidate throughput retained vs 1 MiB |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 2946 | 3757 | 3526 | 3947 | -6.2% | 89.3% |
| home-search | 2510 | 3381 | 3380 | 3863 | -0.0% | 87.5% |
| bookmarks | 3836 | 4786 | 4709 | 4529 | -1.6% | 104.0% |
| applications | 7074 | 8757 | 8428 | 6403 | -3.8% | 131.6% |
| redir-url | 10642 | 12633 | 11601 | 11715 | -8.2% | 99.0% |

| Scenario | 8 KiB baseline P95 (ms) | 64 KiB baseline P95 (ms) | 1 MiB baseline P95 (ms) |
| --- | ---: | ---: | ---: |
| home | 3.649 | 3.708 | 3.517 |
| home-search | 4.213 | 4.038 | 3.622 |
| bookmarks | 2.242 | 2.532 | 2.660 |
| applications | 1.670 | 1.720 | 1.947 |
| redir-url | 1.078 | 1.140 | 1.045 |

| Scenario | 8 KiB peak RSS (MiB) | 64 KiB peak RSS (MiB) | 1 MiB peak RSS (MiB) | Candidate RSS change vs 8 KiB | Candidate RSS reduction vs 1 MiB |
| --- | ---: | ---: | ---: | ---: | ---: |
| home | 30.52 | 31.22 | 37.91 | +0.70 MiB | 17.6% |
| home-search | 30.31 | 30.89 | 38.05 | +0.58 MiB | 18.8% |
| bookmarks | 29.46 | 30.67 | 37.73 | +1.21 MiB | 18.7% |
| applications | 28.74 | 29.57 | 36.67 | +0.83 MiB | 19.4% |
| redir-url | 26.40 | 26.30 | 35.79 | -0.10 MiB | 26.5% |

| Scenario | 8 KiB CPU (µs/req) | 64 KiB CPU (µs/req) | 1 MiB CPU (µs/req) | 8 KiB isolated QPS | 64 KiB isolated QPS | 1 MiB isolated QPS |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 701.2 | 677.4 | 524.3 | 3283.5 | 3286.8 | 3891.8 |
| home-search | 739.1 | 692.4 | 586.2 | 3112.0 | 3153.1 | 3551.5 |
| bookmarks | 578.6 | 509.6 | 415.5 | 4340.2 | 4740.0 | 5357.3 |
| applications | 318.4 | 304.8 | 267.7 | 7808.9 | 8028.4 | 8677.1 |
| redir-url | 134.9 | 143.5 | 123.7 | 11734.0 | 11262.1 | 12482.5 |

### Limits and interpretation

- These are local macOS/ARM64 loopback tests, not NAS/Linux measurements or production capacity estimates.
- The original client keeps at most 2 idle connections per host, so concurrency 8 can cause connection churn. An explicit --keepalive-connections setting is recorded above; results include the costs of that connection pattern.
- Closed-loop throughput uses a fixed concurrency; the machine also runs the client. Alternating order reduces but does not eliminate host noise and thermal effects.
- The five original baseline scenarios use default fixtures. `redir-url` validates the existing default-fixture redirect to https://link.example.com and expects HTTP 302.
- Offline mode excludes weather network requests. Login, uploads, large collections and static assets are validated separately and are not performance claims from this run.
- Every isolated run validates the expected HTTP status (200 or 302), response body hash/size, content type, CSP and redirect destination. Content type casing and the visible clock time are normalized for cross-stack comparisons; raw preflight results are stored for cross-stack comparison.
- The compatibility-oriented candidate keeps Immutable enabled and standard-library multipart parsing; results do not represent an aggressively tuned Fiber microbenchmark.
- Medians are descriptive. Check raw per-round data before treating small differences as improvements.

### Binary size

- Echo application: 22.02 MiB; Fiber application: 28.30 MiB (same default build flags, no stripping).

### Reproduce

```sh
python3 scripts/compare-http-stacks.py --output .benchmark/http-comparison-keepalive-8 --requests 2000 --warmup 100 --rounds 5 --resource-requests 20000 --resource-warmup 1000 --resource-rounds 4 --concurrency 8 --gomaxprocs 4 --keepalive-connections 8 --compare-read-buffers
```

Full per-round metrics and metadata: `fiber-64kib-buffer-2026-10-25.json`, key `keepalive_pool_8`; also `comparison.json` in the output directory.
