# Baseline Metrics

本目录文档用于固化 Flare 核心接口的基线指标采集方式。基准工具与主服务均基于 Echo v5 构建，压测结果反映当前 Web 栈表现。

## 覆盖接口

- `GET /`
- `POST /`（搜索场景）
- `GET /bookmarks`
- `GET /applications`
- `GET /redir/url`

## 一键采集

在仓库根目录执行：

```bash
./scripts/baseline-metrics.sh
```

可通过环境变量调整压测参数：

```bash
REQUESTS=5000 CONCURRENCY=16 WARMUP=200 ./scripts/baseline-metrics.sh
```

也可以指定输出目录：

```bash
./scripts/baseline-metrics.sh "/tmp/flare-baseline-$(date +%Y%m%d-%H%M%S)"
```

## 输出内容

默认会在 `.benchmark/baseline-<timestamp>/` 生成：

- `baseline-report.md`：包含 QPS、P50/P95/P99、allocs/op、B/op
- `<scenario>.cpu.pprof`：CPU profile
- `<scenario>.mem.pprof`：Heap profile

`scenario` 取值为：

- `home`
- `home-search`
- `bookmarks`
- `applications`
- `redir-url`

## 手动运行单场景

```bash
go run ./tools/baseline -scenario home -requests 2000 -concurrency 8 -warmup 100
```

可选参数：

- `-scenario`：`all|home|home-search|bookmarks|applications|redir-url`
- `-cpuprofile <path>`：导出 CPU profile
- `-memprofile <path>`：导出 Heap profile
- `-markdown`：以 Markdown 表格行输出，便于写入报告

## Profile 分析

查看热点函数（CPU）：

```bash
go tool pprof -top .benchmark/baseline-<timestamp>/home.cpu.pprof
```

查看内存分配热点（Heap）：

```bash
go tool pprof -top .benchmark/baseline-<timestamp>/home.mem.pprof
```
