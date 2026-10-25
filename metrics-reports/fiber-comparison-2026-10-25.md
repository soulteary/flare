# Flare Fiber 实验：2026-10-25

> 历史实验记录：本文的构建标签、默认框架及源码路径描述对应测量当时的实验阶段。当前默认已切换为 64 KiB Fiber；完整记录和复现边界见 [性能总览](../docs/performance.md)。原始数字与当时结论保持不变。

这版 Fiber 在本机测量中提高了吞吐、降低了每请求 CPU 开销，但增加了常驻内存和可执行文件体积。默认构建继续使用 Echo，`-tags fiber` 启用候选版本。

## 结果与判断

- **原有基线，默认客户端连接池：** 每场景 2,000 请求、100 次预热、并发 8，Echo/Fiber 交替运行 5 轮后取中位数。五场景吞吐提升 **11.0%–35.2%**。首页 **6,469 → 8,534 QPS（+31.9%）**，P95 **2.593 → 2.216 ms**；每轮配对的首页吞吐差异均为正，幅度约 24.6%–40.8%。
- **独立服务资源，默认连接池：** 每场景新服务进程，1,000 次预热、20,000 请求、并发 8、3 轮中位数。四个页面场景每请求 CPU 时间下降 **31.7%–49.0%**，分配字节下降约 **21.8%–27.6%**；分配次数只下降约 1%–2%。这些是目标服务进程指标，不含压测客户端。
- **内存增加：** 首页预热后 RSS **25.12 → 36.08 MiB**，负载期间采样峰值 **26.89 → 38.61 MiB**。默认连接池的其他场景也增加；跳转场景的采样峰值达到 54.17 MiB。单请求分配字节减少不等于常驻内存减少。
- **稳定空闲连接池的补充测量：** 相同并发、保留 8 条空闲连接，页面吞吐和 CPU 改善方向一致。首页独立服务吞吐 **4,777 → 6,768 QPS（+41.7%）**，CPU **526.3 → 295.3 µs/请求（−43.9%）**，采样峰值 RSS **26.66 → 37.20 MiB**。跳转的单轮吞吐差异有正有负，不宜把其中位数小幅改善视为稳定收益。两组是在不同时间运行，应各自比较栈内配对，不能直接比较两组绝对吞吐来判断连接池的效果。
- **未裁剪的应用二进制：** Echo **22.02 MiB**，Fiber **28.30 MiB**，约增加 **28.6%**；相同 Go 版本和默认编译参数。

候选为保留 Echo 对长请求头和长书签跳转 URL 的接收能力，将 Fiber 读取缓冲设为 1 MiB。fasthttp 按连接分配并复用这个缓冲，属于本次候选的实际成本。Fiber 的 GC 次数明显更少，伴随更高的保留堆和 RSS；较大的连接缓冲与复用池提高 GC 堆目标，是一个合理解释，但本次未通过堆/CPU profile 分离验证各因素的贡献。`HeapAlloc` 也包含尚未回收的垃圾，不能直接称为 GC 后的存活堆。

这些结果支持继续评估 Fiber 的吞吐与 CPU 收益，暂不支持“更省常驻内存”的判断。此轮主机是 Apple M5 / macOS / ARM64，Go 1.27.3、GOMAXPROCS=4，GOGC/GOMEMLIMIT 未设置；离线、关闭登录和请求日志、使用默认书签数据。NAS/Linux、外部天气访问、上传、大规模书签、静态资源及登录状态下的性能尚不在此结论范围内。

## 验证与复现

- 默认 Echo 与 `fiber` 两种构建运行完整 Go 测试及静态检查。
- Fiber 认证、日志、资源、原生服务、主题缓冲所有权、书签导入和基线工具的相关竞态检查通过。
- 原生监听器回归覆盖 HTTP 主路由、较大请求头/长书签 URL、TLS Secure Cookie 登录/登出，避免只在 HTTP 适配层测试造成误判。
- 基线会拒绝错误状态及截断响应；每轮独立测量比较状态、正文大小与散列、类型、CSP、跳转目标。仅归一化可见时钟文本与 MIME 大小写。
- 两组完整逐轮数据保存在相邻的 `fiber-comparison-2026-10-25.json`。下面分别保留两组完整方法与复现命令。
- 构建、脚本和指标口径参见 [baseline-metrics](../docs/baseline-metrics.md)。


## 原有客户端连接池（2 条空闲连接/主机）

- Generated: 2026-10-25T18:04:00+08:00 (Asia/Shanghai)
- Source: `3e09ee4522d43c62eef31d98d74e20744b478cb6` plus the experimental changes in this workspace
- Host: macOS-26.5.1-arm64-arm-64bit-Mach-O · go version go1.27.3 darwin/arm64 · GOMAXPROCS=4
- CPU: Apple M5 · logical CPUs: 10
- Client idle connections per host: 2 (original Go default)
- Frameworks: Echo v5.4.1; Fiber v3.6.1 / fasthttp v1.75.0

### Method

Both builds use identical offline fixtures, disabled login and request logging, the same Go toolchain, concurrency, and client. The default configuration still enables the guide and editor. Fiber serves requests on its native fasthttp listener; the net/http compatibility adapter is used only by existing HTTP integration tests. Fiber enables Immutable and explicitly copies retained bound form strings. Its 1 MiB ReadBufferSize preserves large request headers/redirect URLs accepted by Echo; fasthttp allocates that reader buffer per active connection. Static favicon/SVG responses retain a ServeContent adapter and are outside these five scenarios.

Existing baseline: 5 alternating rounds, 2000 requests and 100 warmup requests per scenario, concurrency 8. Each stack runs in its own process, with its client and server in that same process as in the original harness. Allocation values in this section include the client. Cells show the median across rounds.

| Scenario | Echo QPS | Fiber QPS | QPS change | Echo P95 (ms) | Fiber P95 (ms) |
| --- | ---: | ---: | ---: | ---: | ---: |
| home | 6469 | 8534 | +31.9% | 2.593 | 2.216 |
| home-search | 6565 | 8640 | +31.6% | 2.558 | 2.205 |
| bookmarks | 8613 | 11647 | +35.2% | 1.366 | 1.062 |
| applications | 15785 | 20124 | +27.5% | 0.974 | 0.693 |
| redir-url | 25497 | 28306 | +11.0% | 0.556 | 0.396 |

### Isolated server resources

3 alternating rounds; fresh target process per scenario; 1000 warmup requests followed by 20000 measured requests, concurrency 8. The load generator is a separate process and always uses the Echo-built client binary for both targets. CPU is target-process user+system time from getrusage; allocations and GC are target-process runtime counters. Snapshots are served on a separate loopback control listener, whose small fixed overhead is amortized into the counters. RSS is the target process sampled with ps every 50 ms; it includes executable pages, runtime and heap. RSS after warmup and the lifetime high-water mark are retained in the raw data. Neither RSS nor retained heap is equivalent to bytes allocated per request.

| Scenario | Echo QPS | Fiber QPS | QPS change | Echo CPU (µs/req) | Fiber CPU (µs/req) | CPU change | Echo allocs/req | Fiber allocs/req | Echo KiB/req | Fiber KiB/req |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 5535 | 8204 | +48.2% | 444.9 | 244.3 | -45.1% | 2652.1 | 2619.8 | 206.4 | 160.2 |
| home-search | 5204 | 6280 | +20.7% | 465.8 | 285.7 | -38.7% | 2684.5 | 2637.0 | 208.6 | 163.1 |
| bookmarks | 6746 | 10448 | +54.9% | 392.6 | 200.2 | -49.0% | 2069.0 | 2044.5 | 159.6 | 123.4 |
| applications | 12859 | 15758 | +22.5% | 203.9 | 139.2 | -31.7% | 829.8 | 811.3 | 71.4 | 51.7 |
| redir-url | 22426 | 24684 | +10.1% | 76.9 | 63.4 | -17.6% | 374.5 | 356.2 | 21.7 | 21.9 |

| Scenario | Echo P95 (ms) | Fiber P95 (ms) | Echo idle RSS (MiB) | Fiber idle RSS (MiB) | Echo sampled peak RSS (MiB) | Fiber sampled peak RSS (MiB) | Echo GC cycles | Fiber GC cycles |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 2.84 | 2.28 | 25.12 | 36.08 | 26.89 | 38.61 | 3965.00 | 341.00 |
| home-search | 3.00 | 2.76 | 24.88 | 36.36 | 26.94 | 39.05 | 3928.00 | 347.00 |
| bookmarks | 1.87 | 1.25 | 24.91 | 35.98 | 26.36 | 39.28 | 3163.00 | 264.00 |
| applications | 1.20 | 0.96 | 23.98 | 34.94 | 26.20 | 43.50 | 1141.00 | 110.00 |
| redir-url | 0.67 | 0.55 | 22.91 | 31.83 | 24.23 | 54.17 | 218.00 | 40.00 |

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
python3 scripts/compare-http-stacks.py --output .benchmark/http-comparison --requests 2000 --warmup 100 --rounds 5 --resource-requests 20000 --resource-warmup 1000 --resource-rounds 3 --concurrency 8 --gomaxprocs 4 --keepalive-connections 0
```

Full per-round metrics and metadata: `fiber-comparison-2026-10-25.json`, key `original_pool` (also `comparison.json` in the output directory).

## 补充连接池（8 条空闲连接/主机）

- Generated: 2026-10-25T18:05:38+08:00 (Asia/Shanghai)
- Source: `3e09ee4522d43c62eef31d98d74e20744b478cb6` plus the experimental changes in this workspace
- Host: macOS-26.5.1-arm64-arm-64bit-Mach-O · go version go1.27.3 darwin/arm64 · GOMAXPROCS=4
- CPU: Apple M5 · logical CPUs: 10
- Client idle connections per host: 8 (explicit pool)
- Frameworks: Echo v5.4.1; Fiber v3.6.1 / fasthttp v1.75.0

### Method

Both builds use identical offline fixtures, disabled login and request logging, the same Go toolchain, concurrency, and client. The default configuration still enables the guide and editor. Fiber serves requests on its native fasthttp listener; the net/http compatibility adapter is used only by existing HTTP integration tests. Fiber enables Immutable and explicitly copies retained bound form strings. Its 1 MiB ReadBufferSize preserves large request headers/redirect URLs accepted by Echo; fasthttp allocates that reader buffer per active connection. Static favicon/SVG responses retain a ServeContent adapter and are outside these five scenarios.

Existing baseline: 5 alternating rounds, 2000 requests and 100 warmup requests per scenario, concurrency 8. Each stack runs in its own process, with its client and server in that same process as in the original harness. Allocation values in this section include the client. Cells show the median across rounds.

| Scenario | Echo QPS | Fiber QPS | QPS change | Echo P95 (ms) | Fiber P95 (ms) |
| --- | ---: | ---: | ---: | ---: | ---: |
| home | 5993 | 7976 | +33.1% | 2.861 | 2.290 |
| home-search | 5886 | 8046 | +36.7% | 2.817 | 2.318 |
| bookmarks | 8026 | 10611 | +32.2% | 1.458 | 1.236 |
| applications | 14487 | 19528 | +34.8% | 1.003 | 0.749 |
| redir-url | 23657 | 27095 | +14.5% | 0.605 | 0.452 |

### Isolated server resources

3 alternating rounds; fresh target process per scenario; 1000 warmup requests followed by 20000 measured requests, concurrency 8. The load generator is a separate process and always uses the Echo-built client binary for both targets. CPU is target-process user+system time from getrusage; allocations and GC are target-process runtime counters. Snapshots are served on a separate loopback control listener, whose small fixed overhead is amortized into the counters. RSS is the target process sampled with ps every 50 ms; it includes executable pages, runtime and heap. RSS after warmup and the lifetime high-water mark are retained in the raw data. Neither RSS nor retained heap is equivalent to bytes allocated per request.

| Scenario | Echo QPS | Fiber QPS | QPS change | Echo CPU (µs/req) | Fiber CPU (µs/req) | CPU change | Echo allocs/req | Fiber allocs/req | Echo KiB/req | Fiber KiB/req |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 4777 | 6768 | +41.7% | 526.3 | 295.3 | -43.9% | 2651.9 | 2619.8 | 205.6 | 160.2 |
| home-search | 4513 | 6210 | +37.6% | 569.4 | 323.8 | -43.1% | 2684.5 | 2637.0 | 208.7 | 163.0 |
| bookmarks | 5721 | 7933 | +38.7% | 464.7 | 269.7 | -42.0% | 2068.9 | 2044.5 | 159.6 | 123.3 |
| applications | 9669 | 13740 | +42.1% | 266.0 | 164.6 | -38.1% | 829.8 | 811.2 | 71.5 | 51.5 |
| redir-url | 18561 | 20117 | +8.4% | 93.5 | 77.9 | -16.7% | 374.3 | 356.1 | 21.6 | 19.4 |

| Scenario | Echo P95 (ms) | Fiber P95 (ms) | Echo idle RSS (MiB) | Fiber idle RSS (MiB) | Echo sampled peak RSS (MiB) | Fiber sampled peak RSS (MiB) | Echo GC cycles | Fiber GC cycles |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 3.19 | 2.52 | 24.70 | 35.95 | 26.66 | 37.20 | 3926.00 | 341.00 |
| home-search | 3.23 | 2.64 | 25.08 | 36.41 | 27.02 | 37.53 | 3955.00 | 346.00 |
| bookmarks | 2.09 | 1.58 | 24.53 | 35.73 | 26.66 | 38.08 | 3129.00 | 264.00 |
| applications | 1.50 | 1.05 | 23.94 | 34.56 | 26.09 | 36.75 | 1141.00 | 110.00 |
| redir-url | 0.81 | 0.66 | 23.22 | 32.00 | 24.14 | 33.73 | 217.00 | 44.00 |

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
python3 scripts/compare-http-stacks.py --output .benchmark/http-comparison-keepalive-8 --requests 2000 --warmup 100 --rounds 5 --resource-requests 20000 --resource-warmup 1000 --resource-rounds 3 --concurrency 8 --gomaxprocs 4 --keepalive-connections 8
```

Full per-round metrics and metadata: `fiber-comparison-2026-10-25.json`, key `keepalive_pool_8` (also `comparison.json` in the output directory).
