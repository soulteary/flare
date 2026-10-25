# Fiber 默认读取缓冲复测：2026-10-25

Fiber 候选已恢复当前 v3.6.1 默认读取缓冲 **8 KiB（8192 字节）**。这次内存明显下降，但 1 MiB 版本的性能收益只保留了一部分；默认 8 KiB 的 RSS 仍高于同场 Echo。默认应用构建继续使用 Echo，`-tags fiber` 启用候选。

## 测量方式

保留上一轮参数：五个现有场景、并发 8、GOMAXPROCS=4、离线、关闭登录与请求日志、默认书签数据。原基线每场景 2,000 请求、100 次预热、5 轮；独立服务资源每场景 20,000 请求、1,000 次预热、3 轮。分别运行默认客户端池（2 条空闲连接/主机）和 8 条空闲连接池。主机为 Apple M5 / macOS / ARM64、Go 1.27.3，GOGC/GOMEMLIMIT 未设置。

每组同时构建和轮转运行 Echo、默认 8 KiB Fiber、同源码 1 MiB Fiber。1 MiB 参照通过 Go overlay **只增加 `ReadBufferSize: 1 << 20`**，不修改工作区配置；已核对替代源码只包含这一差异。原生目标回报实际缓冲大小，三个隔离轮次让每个版本各占一次首、中、尾位置。以下比较使用本轮参照，不直接相减历史报告。

## 首页结果

### 独立服务内存与 CPU

每格为 3 轮中位数。RSS 为负载期间每 50 ms 采样的进程峰值，可能漏掉短暂尖峰；CPU 为服务端用户态+内核态时间/请求，不包含客户端。

| 客户端空闲池 | 1 MiB RSS（MiB） | 8 KiB RSS（MiB） | RSS 降幅 | Echo RSS（MiB） | 1 MiB CPU（µs/请求） | 8 KiB CPU（µs/请求） | Echo CPU（µs/请求） |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 默认 2 条 | 38.89 | 30.17 | 22.4% | 28.17 | 330.1 | 510.8 | 682.6 |
| 8 条 | 37.41 | 30.48 | 18.5% | 26.45 | 349.5 | 502.0 | 638.6 |

首页采样峰值 RSS 相对 1 MiB 参照下降 **6.92–8.72 MiB（18.5%–22.4%）**。但默认 8 KiB 仍比 Echo 多约 **2.00–4.03 MiB**。四个页面场景中，默认池 RSS 降幅约 18%–39%，8 连接池约 19%–27%；默认池跳转场景的较大波动单列在完整结果中。

8 KiB 首页 CPU 时间相对 1 MiB 增加 **43.6%–54.8%**，仍比 Echo 少 **21.4%–25.2%**。这与较小缓冲后的 GC 频率上升相符，但没有用 profile 分离 GC、池复用及其他成本的贡献。

### 原有基线吞吐

以下为每个版本 5 轮 QPS 中位数之间的比值，沿用上一轮口径；这不同于逐轮配对百分比的中位数。吞吐保留 = 8 KiB QPS / 1 MiB QPS；相对 Echo 增益保留 = (8 KiB QPS − Echo QPS) / (1 MiB QPS − Echo QPS)。

| 客户端空闲池 | Echo QPS | 1 MiB QPS | 8 KiB QPS | 8 KiB 吞吐保留（相对 1 MiB） | 8 KiB 相对 Echo | 相对 Echo 增益保留 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 默认 2 条 | 4748 | 6409 | 5742 | 89.6% | +20.9% | 59.8% |
| 8 条 | 5132 | 6605 | 4895 | 74.1% | -4.6% | -16.0% |

默认池中，首页保留约 **89.6%** 的 1 MiB 基线吞吐；相对 Echo 的优势从 **35.0% 降至 20.9%**，约保留 **59.8%** 的增益。

8 连接池中，首页保留约 **74.1%** 的 1 MiB 基线吞吐；按各版本 QPS 中位数比较为 Echo 的 **−4.6%**。但逐轮配对变化为 **−23.4%、+20.5%、+3.7%、−6.0%、+27.5%**（中位数 +3.7%），因此不能把 −4.6% 或负的“增益保留比例”断言为稳定回退。更准确的判断是：短基线的吞吐优势明显缩小，部分场景不稳定。

较长的独立服务测试中，首页 8 KiB 相对 1 MiB 的吞吐保留分别为 **74.2% / 77.6%**，相对 Echo 仍为 **+19.5% / +11.3%**。这些与原基线的进程结构不同，应分别解读，不能将 89.6% 套用于所有性能指标。

## 当前配置与边界

- 工作区 Fiber 不再覆盖 `ReadBufferSize`，原生服务上报 8192；1 MiB 只存在于参照构建的 overlay。
- 请求行及请求头超出默认读取容量可能返回 HTTP 431。原生回归已覆盖预算内成功、超预算 431、主路由与 TLS Cookie 登录/登出。
- `Immutable`、主题字符串复制、上传限制、模板缓冲池、静态资源适配策略均延续上一版。
- 内存下降的比较对象是同场 1 MiB Fiber；本次不支持默认 Fiber 比 Echo 更省常驻内存的判断。
- 本机重复测量存在后台负载、温度和调度噪声，未计算置信区间。小幅结果尤其不应外推到 NAS/Linux 或生产容量。

## 数据与复现

相邻的 `fiber-default-buffer-2026-10-25.json` 保存两组各 75 条基线记录、45 条独立资源记录、响应校验、实际缓冲、构建哈希及环境信息。两个缓冲的标准指标和每场景复现命令保留如下；使用 `--compare-read-buffers` 可再次生成同源码参照。历史 1 MiB 报告保留在 [上一轮](fiber-comparison-2026-10-25.md)。

## 完整结果：默认客户端连接池（2 条空闲连接/主机）

- Generated: 2026-10-25T18:18:45+08:00 (Asia/Shanghai)
- Source: `3e09ee4522d43c62eef31d98d74e20744b478cb6` plus the experimental changes in this workspace
- Host: macOS-26.5.1-arm64-arm-64bit-Mach-O · go version go1.27.3 darwin/arm64 · GOMAXPROCS=4
- CPU: Apple M5 · logical CPUs: 10
- Client idle connections per host: 2 (original Go default)
- Frameworks: Echo v5.4.1; Fiber v3.6.1 / fasthttp v1.75.0
- Effective Fiber read buffer: 8192 bytes (reported by the native target)

### Method

Both builds use identical offline fixtures, disabled login and request logging, the same Go toolchain, concurrency, and client. The default configuration still enables the guide and editor. Fiber serves requests on its native fasthttp listener; the net/http compatibility adapter is used only by existing HTTP integration tests. Fiber enables Immutable and explicitly copies retained bound form strings. It uses the installed Fiber default read buffer; the effective size is recorded above. Request lines and headers exceeding that buffer can receive HTTP 431, so this candidate no longer preserves Echo’s larger header capacity. Static favicon/SVG responses retain a ServeContent adapter and are outside these five scenarios.

Existing baseline: 5 alternating rounds, 2000 requests and 100 warmup requests per scenario, concurrency 8. Each stack runs in its own process, with its client and server in that same process as in the original harness. Allocation values in this section include the client. Cells show the median across rounds.

| Scenario | Echo QPS | Fiber QPS | QPS change | Echo P95 (ms) | Fiber P95 (ms) |
| --- | ---: | ---: | ---: | ---: | ---: |
| home | 4748 | 5742 | +20.9% | 3.205 | 2.869 |
| home-search | 4702 | 5739 | +22.1% | 3.194 | 2.833 |
| bookmarks | 5917 | 7672 | +29.7% | 2.055 | 1.469 |
| applications | 11729 | 12511 | +6.7% | 1.297 | 1.195 |
| redir-url | 17416 | 18238 | +4.7% | 0.820 | 0.791 |

### Isolated server resources

3 alternating rounds; fresh target process per scenario; 1000 warmup requests followed by 20000 measured requests, concurrency 8. The load generator is a separate process and always uses the Echo-built client binary for all targets. CPU is target-process user+system time from getrusage; allocations and GC are target-process runtime counters. Snapshots are served on a separate loopback control listener, whose small fixed overhead is amortized into the counters. RSS is the target process sampled with ps every 50 ms; it includes executable pages, runtime and heap. RSS after warmup and the lifetime high-water mark are retained in the raw data. Neither RSS nor retained heap is equivalent to bytes allocated per request.

| Scenario | Echo QPS | Fiber QPS | QPS change | Echo CPU (µs/req) | Fiber CPU (µs/req) | CPU change | Echo allocs/req | Fiber allocs/req | Echo KiB/req | Fiber KiB/req |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 3738 | 4466 | +19.5% | 682.6 | 510.8 | -25.2% | 2652.1 | 2624.1 | 206.5 | 172.0 |
| home-search | 3788 | 4793 | +26.5% | 649.4 | 473.4 | -27.1% | 2684.6 | 2644.5 | 209.1 | 175.0 |
| bookmarks | 5215 | 5568 | +6.8% | 531.1 | 422.8 | -20.4% | 2068.8 | 2047.4 | 159.3 | 129.7 |
| applications | 8508 | 9543 | +12.2% | 316.1 | 255.5 | -19.1% | 829.8 | 812.5 | 71.4 | 53.0 |
| redir-url | 14407 | 16565 | +15.0% | 116.4 | 99.1 | -14.9% | 374.4 | 356.3 | 21.6 | 19.4 |

| Scenario | Echo P95 (ms) | Fiber P95 (ms) | Echo idle RSS (MiB) | Fiber idle RSS (MiB) | Echo sampled peak RSS (MiB) | Fiber sampled peak RSS (MiB) | Echo GC cycles | Fiber GC cycles |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 3.71 | 3.28 | 24.75 | 27.77 | 28.17 | 30.17 | 3974.00 | 2411.00 |
| home-search | 3.68 | 3.26 | 24.89 | 28.44 | 28.64 | 30.81 | 3943.00 | 2441.00 |
| bookmarks | 2.08 | 2.19 | 24.58 | 27.59 | 26.66 | 30.52 | 3062.00 | 1891.00 |
| applications | 1.64 | 1.50 | 25.23 | 26.61 | 26.30 | 28.77 | 1141.00 | 708.00 |
| redir-url | 1.04 | 0.83 | 22.98 | 25.17 | 24.47 | 26.39 | 216.00 | 192.00 |

### Same-session read buffer comparison

The reference is built from the same current source using a Go overlay that only adds ReadBufferSize: 1 << 20. The checkout stays on the default buffer. All three targets are interleaved in this session, and each isolated run reports its effective buffer size. CPU and RSS below are isolated target-process medians; throughput retention is default-buffer QPS divided by 1 MiB QPS. Throughput retained = QPS(default) / QPS(1 MiB). Advantage retained = [QPS(default) - QPS(Echo)] / [QPS(1 MiB) - QPS(Echo)]; negative values mean the candidate is slower than Echo. The advantage fraction is not reported when the reference gain is at most 1%, because its denominator is unstable. These are ratios of per-stack medians, not medians of paired per-round ratios; host noise can change the sign of small gains.

| Scenario | Baseline 1 MiB QPS | Baseline default QPS | Throughput retained | 1 MiB advantage over Echo | Default advantage over Echo | Advantage retained |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 6409 | 5742 | 89.6% | +35.0% | +20.9% | 59.8% |
| home-search | 6323 | 5739 | 90.8% | +34.5% | +22.1% | 64.0% |
| bookmarks | 9122 | 7672 | 84.1% | +54.2% | +29.7% | 54.8% |
| applications | 16735 | 12511 | 74.8% | +42.7% | +6.7% | 15.6% |
| redir-url | 22890 | 18238 | 79.7% | +31.4% | +4.7% | 15.0% |

| Scenario | 1 MiB peak RSS (MiB) | Default peak RSS (MiB) | RSS reduction | 1 MiB CPU (µs/req) | Default CPU (µs/req) | CPU change | 1 MiB isolated QPS | Default isolated QPS |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 38.89 | 30.17 | 22.4% | 330.1 | 510.8 | +54.8% | 6017 | 4466 |
| home-search | 38.28 | 30.81 | 19.5% | 352.2 | 473.4 | +34.4% | 5548 | 4793 |
| bookmarks | 37.25 | 30.52 | 18.1% | 300.9 | 422.8 | +40.5% | 7041 | 5568 |
| applications | 46.86 | 28.77 | 38.6% | 224.2 | 255.5 | +14.0% | 10095 | 9543 |
| redir-url | 51.42 | 26.39 | 48.7% | 114.9 | 99.1 | -13.7% | 10891 | 16565 |

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
python3 scripts/compare-http-stacks.py --output .benchmark/http-comparison --requests 2000 --warmup 100 --rounds 5 --resource-requests 20000 --resource-warmup 1000 --resource-rounds 3 --concurrency 8 --gomaxprocs 4 --keepalive-connections 0 --compare-read-buffers
```

Full per-round metrics and metadata: `fiber-default-buffer-2026-10-25.json`, key `original_pool`; also `comparison.json` in the output directory.

## 完整结果：8 条空闲连接/主机

- Generated: 2026-10-25T18:21:47+08:00 (Asia/Shanghai)
- Source: `3e09ee4522d43c62eef31d98d74e20744b478cb6` plus the experimental changes in this workspace
- Host: macOS-26.5.1-arm64-arm-64bit-Mach-O · go version go1.27.3 darwin/arm64 · GOMAXPROCS=4
- CPU: Apple M5 · logical CPUs: 10
- Client idle connections per host: 8 (explicit pool)
- Frameworks: Echo v5.4.1; Fiber v3.6.1 / fasthttp v1.75.0
- Effective Fiber read buffer: 8192 bytes (reported by the native target)

### Method

Both builds use identical offline fixtures, disabled login and request logging, the same Go toolchain, concurrency, and client. The default configuration still enables the guide and editor. Fiber serves requests on its native fasthttp listener; the net/http compatibility adapter is used only by existing HTTP integration tests. Fiber enables Immutable and explicitly copies retained bound form strings. It uses the installed Fiber default read buffer; the effective size is recorded above. Request lines and headers exceeding that buffer can receive HTTP 431, so this candidate no longer preserves Echo’s larger header capacity. Static favicon/SVG responses retain a ServeContent adapter and are outside these five scenarios.

Existing baseline: 5 alternating rounds, 2000 requests and 100 warmup requests per scenario, concurrency 8. Each stack runs in its own process, with its client and server in that same process as in the original harness. Allocation values in this section include the client. Cells show the median across rounds.

| Scenario | Echo QPS | Fiber QPS | QPS change | Echo P95 (ms) | Fiber P95 (ms) |
| --- | ---: | ---: | ---: | ---: | ---: |
| home | 5132 | 4895 | -4.6% | 2.995 | 3.070 |
| home-search | 4717 | 4997 | +5.9% | 3.250 | 3.084 |
| bookmarks | 5944 | 6612 | +11.2% | 1.837 | 1.641 |
| applications | 10734 | 12814 | +19.4% | 1.341 | 1.160 |
| redir-url | 16237 | 17988 | +10.8% | 0.877 | 0.715 |

### Isolated server resources

3 alternating rounds; fresh target process per scenario; 1000 warmup requests followed by 20000 measured requests, concurrency 8. The load generator is a separate process and always uses the Echo-built client binary for all targets. CPU is target-process user+system time from getrusage; allocations and GC are target-process runtime counters. Snapshots are served on a separate loopback control listener, whose small fixed overhead is amortized into the counters. RSS is the target process sampled with ps every 50 ms; it includes executable pages, runtime and heap. RSS after warmup and the lifetime high-water mark are retained in the raw data. Neither RSS nor retained heap is equivalent to bytes allocated per request.

| Scenario | Echo QPS | Fiber QPS | QPS change | Echo CPU (µs/req) | Fiber CPU (µs/req) | CPU change | Echo allocs/req | Fiber allocs/req | Echo KiB/req | Fiber KiB/req |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 4075 | 4534 | +11.3% | 638.6 | 502.0 | -21.4% | 2652.1 | 2623.9 | 206.7 | 171.6 |
| home-search | 3861 | 4579 | +18.6% | 672.5 | 508.8 | -24.3% | 2684.6 | 2644.8 | 209.4 | 175.8 |
| bookmarks | 5267 | 6083 | +15.5% | 529.5 | 398.6 | -24.7% | 2068.8 | 2047.5 | 159.3 | 129.9 |
| applications | 9286 | 10348 | +11.4% | 289.8 | 235.9 | -18.6% | 829.9 | 812.5 | 71.5 | 52.9 |
| redir-url | 15958 | 15430 | -3.3% | 108.8 | 103.9 | -4.5% | 374.3 | 356.2 | 21.6 | 19.4 |

| Scenario | Echo P95 (ms) | Fiber P95 (ms) | Echo idle RSS (MiB) | Fiber idle RSS (MiB) | Echo sampled peak RSS (MiB) | Fiber sampled peak RSS (MiB) | Echo GC cycles | Fiber GC cycles |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 3.40 | 3.27 | 24.58 | 27.94 | 26.45 | 30.48 | 3990.00 | 2366.00 |
| home-search | 3.52 | 3.24 | 24.92 | 28.22 | 26.95 | 29.66 | 3974.00 | 2490.00 |
| bookmarks | 2.07 | 1.84 | 24.53 | 27.55 | 26.42 | 29.11 | 3048.00 | 1899.00 |
| applications | 1.50 | 1.37 | 24.03 | 26.50 | 26.11 | 28.23 | 1138.00 | 704.00 |
| redir-url | 0.94 | 0.91 | 22.88 | 25.33 | 24.52 | 26.31 | 215.00 | 189.00 |

### Same-session read buffer comparison

The reference is built from the same current source using a Go overlay that only adds ReadBufferSize: 1 << 20. The checkout stays on the default buffer. All three targets are interleaved in this session, and each isolated run reports its effective buffer size. CPU and RSS below are isolated target-process medians; throughput retention is default-buffer QPS divided by 1 MiB QPS. Throughput retained = QPS(default) / QPS(1 MiB). Advantage retained = [QPS(default) - QPS(Echo)] / [QPS(1 MiB) - QPS(Echo)]; negative values mean the candidate is slower than Echo. The advantage fraction is not reported when the reference gain is at most 1%, because its denominator is unstable. These are ratios of per-stack medians, not medians of paired per-round ratios; host noise can change the sign of small gains.

| Scenario | Baseline 1 MiB QPS | Baseline default QPS | Throughput retained | 1 MiB advantage over Echo | Default advantage over Echo | Advantage retained |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 6605 | 4895 | 74.1% | +28.7% | -4.6% | -16.0% |
| home-search | 5647 | 4997 | 88.5% | +19.7% | +5.9% | 30.1% |
| bookmarks | 8607 | 6612 | 76.8% | +44.8% | +11.2% | 25.1% |
| applications | 15773 | 12814 | 81.2% | +46.9% | +19.4% | 41.3% |
| redir-url | 20514 | 17988 | 87.7% | +26.3% | +10.8% | 40.9% |

| Scenario | 1 MiB peak RSS (MiB) | Default peak RSS (MiB) | RSS reduction | 1 MiB CPU (µs/req) | Default CPU (µs/req) | CPU change | 1 MiB isolated QPS | Default isolated QPS |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| home | 37.41 | 30.48 | 18.5% | 349.5 | 502.0 | +43.6% | 5840 | 4534 |
| home-search | 38.89 | 29.66 | 23.7% | 403.5 | 508.8 | +26.1% | 4876 | 4579 |
| bookmarks | 37.34 | 29.11 | 22.1% | 299.6 | 398.6 | +33.1% | 7221 | 6083 |
| applications | 38.47 | 28.23 | 26.6% | 205.9 | 235.9 | +14.5% | 10733 | 10348 |
| redir-url | 33.75 | 26.31 | 22.0% | 91.1 | 103.9 | +14.1% | 17129 | 15430 |

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
python3 scripts/compare-http-stacks.py --output .benchmark/http-comparison-keepalive-8 --requests 2000 --warmup 100 --rounds 5 --resource-requests 20000 --resource-warmup 1000 --resource-rounds 3 --concurrency 8 --gomaxprocs 4 --keepalive-connections 8 --compare-read-buffers
```

Full per-round metrics and metadata: `fiber-default-buffer-2026-10-25.json`, key `keepalive_pool_8`; also `comparison.json` in the output directory.

