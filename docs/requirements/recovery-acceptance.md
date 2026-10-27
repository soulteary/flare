# 阶段一：迁移、回滚与故障恢复验收

执行日期：2026-10-27（Asia/Shanghai）。结果来自下述自动验收脚本及恢复演练，适用于记录的候选构建。候选构建尚未发布。

## 环境与可复现入口

原生 darwin/arm64，旧版本从本地 `0.6.7` 标签（`2f1a924f40202210bce0531d43e16e5ac8acd4fb`）的独立 `git archive` 构建，未修改工作区源码；旧二进制显式注入 Version=0.6.7，候选注入 Version=0.6.8-dev。夹具 config.yml 显式记录 AppVersion=0.6.7，避免使用两个 dev 版本掩盖版本兼容检查。

HTTP 验收使用临时工作目录、独立真实进程、回环 TCP、CookieJar，禁用 HTTP 代理与自动重定向；密码、Cookie、导航只使用人工夹具。测试结束清除临时目录，不接触现有部署。可复现：

```sh
python3 tests/security-recovery/acceptance.py /absolute/path/to/flare-0.6.7 /absolute/path/to/flare-candidate /tmp/recovery-results.json
GOCACHE=/tmp/flare-phase1-go-cache GOFLAGS=-buildvcs=false go test ./cmd ./internal/security -count=1
GOCACHE=/tmp/flare-phase1-go-cache GOFLAGS=-buildvcs=false go test -race ./cmd ./internal/security -count=1
```

最终重跑使用主候选SHA-256 `b0ffcb369d3f5f149baa46ced5d4ee37c9a1476a89c4ed7b4a61e02d12b072cf`，7组均通过。本轮 HTTP 原始汇总与二进制 SHA-256 保存于 [results-2026-10-27.json](../../tests/security-recovery/results-2026-10-27.json)。HTTP 测试脚本位于 [acceptance.py](../../tests/security-recovery/acceptance.py)。

## 已通过的真实进程场景

1. **旧公开站点自动升级**：旧0.6.7 + DEFAULT + 开启管理登录 + 短密码 short，旧首页可见 private 标记条目。新版本自动建立 deployment 管理员，不改三个 YAML 字节；匿名与旧 Cookie 均看不到私有条目，新登录后可见；正常重启保持新会话。
2. **旧整站私有自动升级**：同上但 PRIVATE，升级前后匿名均无私有内容；旧弱密码照常迁移，不突然要求用户改密，不需要迁移命令。
3. **可信免登录升级**：显式 FLARE_DISABLE_LOGIN=true、无私有标记，旧→新→新重启均可匿名浏览和进入编辑器；新状态已持久提交，保持原可信使用方式。
4. **兼容边界阻止公开回退**：旧内容 + 登录开启但无持久密码，以及免登录 + private 标记，两组新版本均在 `/`、`/editor`、`/health` 返回503；响应无私有哨兵，YAML摘要不变。
5. **本地恢复和来源切换**：deployment 默认 reset-password 拒绝（退出码2）；显式 --take-local-control 成功，旧 FLARE_PASS 仍存在也不覆盖本地新密码。新密码通过真实 HTTP 登录。
6. **损坏和完整恢复**：分别损坏 state.json、complete、installation.json、commit.json；分别删除 complete、installation.json，及同时删除 state.json+complete；每次均503，无自动重新建库。停止进程后恢复完整目录，可重新登录。
7. **Secret 来源故障与锁**：FLARE_PASS_FILE 正常迁移后，空文件/文件消失均503；恢复原文件后原有效会话和账号仍可用，没有把错误空密码写入库。运行中执行 maintenance revoke-all 退出码3，不能并发获取实例锁。

## 回滚的实际安全语义

公开与私有两组均执行：停止新进程→删除测试工作目录→恢复升级前完整目录副本→使用完全相同的旧参数和0.6.7二进制启动。三个 YAML 摘要逐一相等。

实测旧 Cookie 在恢复旧密钥后再次被0.6.7接受；新版拒绝旧 Cookie，并不能改变回滚旧版本的验证规则。DEFAULT 回滚后 private 条目重新出现在匿名响应，这是旧版已知行为，不能把这种回滚称为“保留逐条保护”。实测将旧版 FLARE_VISIBILITY 改为 PRIVATE 后匿名不再获得条目。

因此运维回滚必须同时恢复镜像/二进制、完整数据、原部署环境和外部凭据；如需继续保护任何私有内容，旧版必须整站 PRIVATE + 登录。安全事件恢复还必须轮换旧版 Cookie 密钥；内部 migration-backups 只有 YAML/旧密钥与摘要，并不是完整部署备份，也不恢复外部 Secret。

## 持久化故障注入

[security_recovery_process_test.go](../../cmd/security_recovery_process_test.go) 在测试子进程中调用实际迁移阶段函数，分别在迁移快照落盘、未提交 installation 元数据落盘、账号已初始化但 installation 未提交三个边界暂停，父进程发送真实 SIGKILL。重启自动完成迁移；YAML和旧密钥不改、首次有效备份不覆盖、已初始化实例ID不重复生成。

[recovery_process_test.go](../../internal/security/recovery_process_test.go) 只在测试二进制包装实际atomicWrite，在同步落盘后暂停，父进程SIGKILL。优化前协议每次事务都重写complete，历史实测为初始化/全部撤销各3个边界，共6个存储边界。当前优化协议仅初始化写complete，之后先校验已有不可变实例标记，缺失/改变即故障且不发布新日志；不再重写相同标记。当前测试明确覆盖初始化commit.json/state.json/complete三个边界、全部撤销commit.json/state.json两个边界，共5个存储边界，加上迁移阶段三个边界为当前8个；没有将已经不存在的撤销complete边界记为通过。当前完整security套件通过，日志幂等回放、凭据可用、撤销不复活及日志清除均保持。生产二进制无故障环境变量或杀进程入口。

同文件额外注入 ENOSPC：撤销未获持久化确认时返回故障，当前进程拒绝继续使用会话；重开恢复最后有效状态。此用例是系统调用错误注入，不能单独算作真实磁盘写满或真实断电验收。Docker实际只读卷/小容量文件系统场景由部署验收单独记录。

## 恢复验收发现并修复的问题

发现自动快照重试只比较同名文件内容：来源文件新增/消失时，会覆盖 initial/manifest.json 的原始文件集合。现改为将已完成 manifest 的文件集合和摘要一并比较，变化进入独立 retry 快照，原始 manifest 保持不变；新增/删除旧 Cookie 密钥两种回归测试通过。

同时在快照完成后同步 migration-backups 父目录与状态目录，保证新建备份目录入口在提交安装元数据前被要求持久化；原先只同步文件和叶目录不足以覆盖父目录入口。

## 本记录未验证的项目

- 真实设备断电、磁盘控制器缓存断电行为、文件系统损坏后的硬件恢复；SIGKILL与错误注入不能替代它们。
- NAS/NFS/SMB、多副本共写；产品明确不支持，不能用本地文件锁结果推断支持。
- 真机 Windows、Android/iOS浏览器、旧NAS CPU与低内存设备；不能把交叉编译或Linux容器当作这些设备通过。
- 大规模数据与持续压测由独立规模验收记录，Docker 重建和反向代理结果见独立部署记录。

## 只读预检补强

本轮还发现 migration-check 仅检查文件类型，可能将内容损坏的已迁移目录报告为 existing_installation。现使用纯读取的 security.Inspect 校验 envelope 校验和、格式、凭据参数、实例标记及提交日志；不创建目录、不获取锁、不重放日志、不推进时钟、不写入任何文件，报告中不输出账号散列或会话材料。缺失安装元数据但账号已存在也报告 faulted。

稳定损坏返回 status=faulted/退出码4；有效待回放日志返回 status=indeterminate/退出码3，须停服务再检查或交由正常启动恢复。检测到连续两次快照不一致同样 indeterminate，不能把活动写入过程直接判为损坏，也不能宣称活动进程上的只读检查具有原子快照保证。只有持锁启动才是最终运行校验。

新增预检测试覆盖有效状态、有效JSON但校验和错误、实例标记不匹配、损坏日志、有效待提交日志、缺失状态/标记/安装元数据及首次安装目录；检查前后逐文件比较，确保没有修复或变更副作用。运行时 Open/会话实现没有因本次预检补强而变化。

## 额外恢复输入边界

独立检查发现 installation.json 使用无界 ReadFile，错误配置为FIFO会阻塞启动，超大文件会放大内存占用；密码文件FIFO也会阻塞启动或维护命令。现安装元数据要求最终文件为普通文件、禁止末级符号链接、非阻塞打开、最多64KiB；密码文件非阻塞打开并要求解析后目标为普通文件，继续支持Kubernetes等投影Secret符号链接。对应FIFO、符号链接、超大元数据和正常投影Secret测试通过。

新增遗留部署缺失FLARE_PASS_FILE的本地恢复测试：首次503/attention后，停机reset-password以本地新密码完成迁移，保留带空格的旧用户名、DEFAULT模式和原始YAML；遗留缺失Secret来源不覆盖本地恢复凭据。

Docker真实只读状态卷及实际1024字节tmpfs写满已在[部署验收](deployment-acceptance.md)记录通过；tmpfs内核dd探针明确报告No space left on device。此结果仍不替代硬件断电测试。


## 新版完整恢复及IPv6最终补验

对上述最终主候选补齐整条黑盒流程：正常登录取得会话→停止进程→复制完整工作目录作为一致性备份→保留恢复前目录→恢复完整备份→停机执行security revoke-all→重启。恢复前Cookie不再获得私有内容，新密码仍可登录，来源仍为local，三个YAML摘要不变。此用例使用真实CLI和真实HTTP，不把各自独立测试拼接成流程通过。

同一最终进程通过原生IPv6回环 `http://[::1]:随机端口` 完成GET登录页、绑定CSRF、携带IPv6 Origin的POST登录及Cookie私有内容读取，验证括号authority与端口解析。它证明本机IPv6回环行为，不代表公网IPv6路由、反代或移动网络。


## 读取授权的持久化优化边界

新增请求内SessionProof：每个HTTP请求先经SessionSnapshot取得持久授权点，同一请求的重复GuardSession只检查实时会话记录、世代、撤销、已知故障及最新已落盘时间，不读取新的日历时间，也不把身份跨请求缓存。证明寿命按单调时钟最多一秒，超时重新走持久Session路径；成功响应尾部仍经持久检查，写入/导出仍使用新鲜授权。没有降低日历高水位的持久化频率来换性能。

专项测试验证50次重复guard无新增日历观察/写盘、每次新请求snapshot仍落盘、请求中撤销/故障/关闭/其他store证明拒绝、最新持久时间到期删除不复活、长请求重新落盘，以及最终检查遇到超过60秒回拨或持久化失败即失效。其HTTP集成与新性能结果由认证及规模验收分别记录；旧候选摘要的浏览器/压测结果不自动代表优化候选。
