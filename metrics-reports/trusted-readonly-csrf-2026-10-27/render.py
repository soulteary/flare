"""Render summary tables from the verified paired statistics."""
import json
from pathlib import Path
ROOT=Path(__file__).resolve().parent
summary=json.loads((ROOT/'summary.json').read_text())
labels={'home':'首页 GET','search-get':'搜索 GET','search-last':'搜索 POST'}
def group_key(g):return (g['size'],['home','search-get','search-last'].index(g['scenario']))
def change(m):return f"{m['paired_change_median_pct']:+.1f}% [{m['paired_change_min_pct']:+.1f}%, {m['paired_change_max_pct']:+.1f}%]"
def pair(m,decimals=2):return f"{m['before_median']:.{decimals}f} → {m['after_median']:.{decimals}f}"
load=sorted([g for g in summary['groups'] if g['cookie_jar'] and g['phase']=='Load'],key=group_key)
fixed=sorted([g for g in summary['groups'] if g['cookie_jar'] and g['phase']=='Fixed'],key=group_key)
lines=['## 结果','',f"共 {summary['windows']} 个正式窗口、{summary['measured_requests']:,} 个测量请求；另有每窗口 200 个预热请求，不计入性能统计。每个请求检查状态与正文长度，按每 256 个请求抽查完整正文，并校验阶段前后响应。",'',
f"固定速率阶段共丢弃 {summary['fixed_dropped_requests']} 个调度请求，涉及 {summary['fixed_windows_with_drops']} 个窗口；逐轮调度 P95 最大值 {summary['fixed_schedule_lag_p95_max_ms']:.3f} ms。主要场景丢弃 3/43,200 个计划请求；不保存 Cookie 的补充组丢弃 61/16,000 个计划请求。丢弃不计为已完成请求，也不被隐藏。",'',
'以下 `前 → 后` 为各轮中位数，变化列为配对变化中位数及 `[最小值, 最大值]`。','',
'### 复用 Cookie：闭环吞吐与服务端 CPU','',
'| 书签数 / 场景 | 吞吐（请求/秒，前 → 后） | 吞吐变化 | CPU（微秒/请求，前 → 后） | CPU 变化 |',
'| --- | ---: | --- | ---: | --- |']
for g in load:
 m=g['metrics'];lines.append(f"| {g['size']} / {labels[g['scenario']]} | {pair(m['qps'],0)} | {change(m['qps'])} | {pair(m['cpu_us_per_request'])} | {change(m['cpu_us_per_request'])} |")
lines+=['','### 复用 Cookie：固定流量下延迟','',
'| 书签数 / 场景 | P50（ms，前 → 后） | P95（ms，前 → 后） | P95 配对变化 | P99（ms，前 → 后） |',
'| --- | ---: | ---: | --- | ---: |']
for g in fixed:
 m=g['metrics'];lines.append(f"| {g['size']} / {labels[g['scenario']]} | {pair(m['p50_ms'],3)} | {pair(m['p95_ms'],3)} | {change(m['p95_ms'])} | {pair(m['p99_ms'],3)} |")
lines+=['','### 复用 Cookie：固定流量 CPU 与闭环 RSS','',
'| 书签数 / 场景 | 固定流量 CPU（微秒/请求，前 → 后） | CPU 变化 | 闭环 RSS 样本中位数（MiB，前 → 后） | 每窗 RSS 采样峰值的中位数（MiB，前 → 后） |',
'| --- | ---: | --- | ---: | ---: |']
for g,f in zip(load,fixed):
 m=g['metrics'];v=f['metrics'];lines.append(f"| {g['size']} / {labels[g['scenario']]} | {pair(v['cpu_us_per_request'])} | {change(v['cpu_us_per_request'])} | {pair(m['rss_median_mib'])} | {pair(m['rss_peak_mib'])} |")
lines+=['','### 不保存 Cookie：补充实验','',
'每场景 4 对，仅 20 书签。不保存 Cookie 会使优化前反复创建 preauth 绑定，不代表有 Cookie 的连续访问；单列解释。','',
'| 场景 | 吞吐（请求/秒，前 → 后） | 吞吐变化 | CPU（微秒/请求，前 → 后） | CPU 变化 |',
'| --- | ---: | --- | ---: | --- |']
for g in sorted([g for g in summary['groups'] if not g['cookie_jar'] and g['phase']=='Load'],key=group_key):
 m=g['metrics'];lines.append(f"| {labels[g['scenario']]} | {pair(m['qps'],0)} | {change(m['qps'])} | {pair(m['cpu_us_per_request'])} | {change(m['cpu_us_per_request'])} |")
lines+=['','所有场景的完整 P50/P95/P99、RSS、HWM、CPU 占用核数、客户端 CPU 和调度延迟均保存在 JSON/CSV 中；表格只摘取主要指标。','']
text='\n'.join(lines)
report=(ROOT/'report.md').read_text()
start=report.find('## 结果\n')
end=report.index('## 测量设计')
if start<0:start=end
report=report[:start]+text+'\n'+report[end:]
(ROOT/'report.md').write_text(report)
