# 阶段一 Docker 与反向代理验收

执行日期：2026-10-27（Asia/Shanghai）。这是开发候选的实测记录，不代表正式版本或所有部署环境认证。

## 可复现入口

```sh
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -o /tmp/flare-candidate ./
python3 tests/security-deployment/acceptance.py --binary /tmp/flare-candidate
```

若验证正式 Dockerfile/UPX 产物，先用相同候选二进制作为 `flare` 构建 `docker/goreleaser/Dockerfile.arm64v8`，再给脚本增加 `--image 自建镜像 --use-image-binary`；否则默认复制输入二进制进入测试镜像。最终镜像 ID 为 `sha256:9d7acca30e84eae984b1f36a7c59164a379e87aa4344cbb014e6d1d401d7c79b`。

架构应与 Docker daemon 匹配。脚本仅使用已有镜像，不拉取、不发布；默认依赖 `alpine:3.24` 和 `ghcr.io/soulteary/flare:0.6.7`，可分别用 `--image`、`--old-image` 指定。还需要 Python 3、OpenSSL、可用的 Docker API。它给自己创建的容器/卷加随机前缀，最后仅清理这些资源；测试密码为独立固定测试值，勿用于真实部署。不要把真实目录或 Secret 传给脚本。

本机实际环境为 macOS arm64，Docker CLI 29.5.3，通过 Docker API 连接 Podman 5.8.3 / Linux 7.0.9 arm64 / crun 1.27.1。候选为静态 Linux arm64 二进制，SHA-256 为 `74e00c4c36014756d14b3751865233422b50cd8345e661b525a37bcc639923d4`。最终使用官方 Dockerfile 构建、UPX 压缩后的镜像内二进制完整运行退出码 0，12 组通过，脱敏机器结果见 [acceptance-result.json](../../tests/security-deployment/acceptance-result.json)。首次矩阵使用缓存镜像。随后按原始 `docker/goreleaser/Dockerfile.arm64v8` 完成官方结构构建与 UPX 3.95 压缩、压缩后二进制执行及完整矩阵；未改动原 Dockerfile 和依赖版本。这不构成供应链审计。

远端 VM 没有共享 macOS `/tmp`，直接 bind 本机二进制会失败；Docker attach 流也返回 TCP upgrade 500。脚本使用 `create`、`cp`、`start` 和显式 HTTP 探测，应用在真实 Linux 容器内执行。整目录持久化使用独立命名卷；逐文件挂载使用该测试卷的 Linux daemon 路径，分别挂载三个 YAML 为只读，安全目录另挂独立卷。这个办法验证 Docker 挂载语义，不证明 macOS Desktop 的文件共享实现。

## 已执行项目

- 先运行缓存中的 0.6.7 镜像，再停旧容器，使用相同数据卷和有效部署账号启动候选；候选使用同一账号密码登录。公开条目匿名可见，私有条目匿名隐藏、登录可见。
- 三个 YAML 升级前后 SHA-256 完全相同；没有新增内容字段或内容数据库迁移。
- 停止并重新创建候选容器，继续挂载原目录；旧会话继续访问私有条目，证明账号、实例及会话实际持久化。
- 三个 YAML 分别只读绑定，单独挂载 `.flare-security`；首次启动和重新创建容器均可登录。持有效 CSRF 提交设置写入明确失败，没有假报保存成功。
- 密码从单独只读文件绑定到 `/run/flare-pass` 输入；环境不含 `FLARE_PASS`，实际登录成功。
- 免登录加 `private: true` 停在失败状态，不暴露公私条目。
- 只读安全状态目录进入故障维护（HTTP 503）；小容量 Linux tmpfs 实际触发 ENOSPC，迁移进入故障维护或退出，不启动公开业务。脚本另外在同一 tmpfs 中运行内核写入探针确认 ENOSPC，启动日志仍保留通用恢复提示。此处是实际文件系统故障，不是模拟错误。
- Python 标准库实现实际 HTTPS 终止代理，使用临时自签证书和随机外部端口；HTTP 后端运行在容器。客户端通过 HTTPS 发出规范 Host 与允许别名 Host 的登录及退出请求，验证 Secure/HttpOnly Cookie。
- HTTPS 下有效 CSRF 与同源 Origin 退出成功；跨源 Origin、缺失 CSRF 均拒绝。
- 不受信任直接客户端伪造 `X-Forwarded-Host` / `X-Forwarded-Proto` 无法改变规范来源或降级 HTTPS；非法 Host 返回 400。
- 独立受信代理测试验证内部 Host 被改写时的规范 forwarded authority；错误主机、HTTP 降级、逗号混合协议、非法客户端地址均返回 400。该一次性测试后端信任全部源地址以覆盖实际 peer 判断，生产部署必须填写准确代理 IP/CIDR。
- 把升级前原始 YAML 放入新隔离数据卷，换回真正 0.6.7 镜像，以 `FLARE_VISIBILITY=PRIVATE` 启动；无 Cookie 请求不包含公开或私有内容。旧版不能逐项执行新版 `private`，不能省略全站保护再回滚。

HTTPS 代理会先清除来访者的 forwarding headers，本组验证固定外部地址与保留 Host 的常用部署。受信头处理另通过实际容器请求覆盖。未把 Python 测试代理等同于 Nginx、Caddy 或 Traefik 产品认证。

## 实际 Nginx 与浏览器补验

增加 `tests/security-deployment/nginx.py` 和 `nginx-browser.cjs`。从 Docker Hub 拉取 `nginx:stable-alpine` 后，创建独立网络、容器及临时证书；Nginx 实际监听 TLS 端口，保留 `$http_host` 并清除来访者 forwarding headers，转发给候选容器 HTTP 端口。脚本退出时清理自己创建的容器与网络。

固定候选 SHA-256 `74e00c4c36014756d14b3751865233422b50cd8345e661b525a37bcc639923d4`，使用同一官方镜像内压缩二进制，用真实 Chromium 156.0.8078.4 和 Playwright 1.64.1、关闭 JavaScript，分别以规范 `https://localhost:随机端口` 和允许别名 `https://127.0.0.1:相同端口` 完成 HTML 原生表单登录、Cookie 回跳检查、私有内容读取及退出，均通过；浏览器发出的实际 Origin 与当前来源相符，登录/退出 POST 均为 303，Cookie 为 Secure/HttpOnly。记录见 [nginx-result.json](../../tests/security-deployment/nginx-result.json)。这是在修复全局 `Referrer-Policy: no-referrer` 导致原生表单 `Origin: null` 的问题后执行；显式构造 HTTP Origin 的前一组用例不能替代这个浏览器证据。

复现（Playwright 版本须与本机浏览器缓存匹配）：

```sh
FLARE_PLAYWRIGHT_MODULE=/absolute/path/to/playwright-core python3 tests/security-deployment/nginx.py --binary /absolute/path/to/linux-arm64/flare
```

Nginx 镜像内容 ID：`sha256:71f44768e348d88b5e3ea14a77e0e082792bc46d283de47ef08249bd7ae2e223`。测试使用自签证书并显式忽略证书错误，只验证 TLS 终止及应用行为，不验证公有 CA 证书配置。

## 尚需真实环境执行

- Docker Engine（当前为 Podman 兼容 API）、Docker Desktop 本机共享路径、rootless 和不同 UID/GID；Linux amd64 原生容器与 ARM 小设备。
- 用户实际 Compose、Nginx 配置、Caddy/Traefik、CDN/WAF 多跳链路及其 Header 清洗配置；真实证书续期和公网入口。
- NAS/NFS/SMB、真实介质掉电、宿主机崩溃及磁盘硬件故障。本阶段不支持多副本/网络盘并发写；容器停止、进程 SIGKILL 和 tmpfs 满不等价于物理断电。
- 手机实机浏览器、跨设备 Cookie 行为和实际密码管理器；桌面自动化或模拟移动 viewport 不能代替实机验收。

升级/回滚操作以[升级指南](../login-private-upgrade.md)为准。迁移与恢复的原生黑盒/崩溃注入证据另见对应验收记录，不能把本文件的正常 Docker 重建扩展为全部恢复场景通过。
