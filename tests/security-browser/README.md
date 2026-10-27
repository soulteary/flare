# 阶段一真实桌面浏览器验收

执行日期：2026-10-27，使用独立临时数据及原生 macOS 候选二进制，每个引擎分别启动实例。脚本通过实际 HTML 表单提交和 Cookie 回跳执行，禁用页面 JavaScript。`page.evaluate` 只用于测试端设置字体大小、测量横向溢出，不是应用功能依赖。

```sh
FLARE_BINARY=/absolute/path/to/flare \
FLARE_PLAYWRIGHT_MODULE=/absolute/path/to/playwright-core \
FLARE_BROWSER_REPORT=/tmp/flare-browser-report.json \
node tests/security-browser/acceptance.cjs
```

不设置 `FLARE_PLAYWRIGHT_MODULE` 时默认复用 `tests/clock/node_modules/playwright`。本次实际使用 Playwright 1.64.1，与已安装的浏览器缓存匹配；默认 clock 模块是 1.62.1，若缓存缺少其对应版本会启动失败，不能当作产品失败或通过。可设置 `FLARE_BROWSER_ENGINE=chromium|firefox|webkit` 单独运行，或用 `FLARE_CHROMIUM_EXECUTABLE` 等显式选择可执行文件；结果记录是否覆盖过。

测试只访问 `127.0.0.1` 临时监听端口。浏览器进程移除继承的大小写 HTTP_PROXY、HTTPS_PROXY、ALL_PROXY、NO_PROXY；Firefox 设置 `network.proxy.type=0` 使用直连。早先人工配置“不可达代理加 loopback bypass”的运行在 Firefox 首次导航出现 `NS_ERROR_NET_RESET`，修正测试网络环境后通过；不修改产品代理或降低来源保护。

每个引擎覆盖 8 项：

1. 登录页与私有条目/分类/内部页脚隔离，登录页脚脚本被过滤。
2. 历史短密码登录及原生 Cookie 回跳。
3. 显式接管部署密码来源，另一浏览器上下文会话撤销。
4. 改密。
5. 重新认证。
6. 390px viewport、200% 字体下账号页无横向溢出。
7. 退出全部。
8. 三个 YAML 字节保留及状态文件不含明文密码。

以上流程均禁用页面 JavaScript。最终完整运行退出码 0，Chromium 156.0.8078.4、Firefox 157.0、WebKit 27.2 均通过 8 项（共 24 项），驱动均为 Playwright 1.64.1。候选 SHA-256：`b0ffcb369d3f5f149baa46ced5d4ee37c9a1476a89c4ed7b4a61e02d12b072cf`。最终结果及准确版本见 [acceptance-result.json](acceptance-result.json)。390px 测试仅为桌面引擎视口模拟，不能代替手机实机、iOS Safari、移动密码管理器或辅助技术验收。浏览器版本来自 Playwright `browser.version()`，WebKit 数字不是系统 Safari 版本。

## T11 外发与跳转补验（独立计数）

`outbound.cjs` 单独执行，原有 24 项结果保持不变。本组 Chromium 156.0.8078.4、Firefox 157.0、WebKit 27.2 各通过 5 项，共新增 15 项；候选与前组相同。完整拦截记录见 [outbound-result.json](outbound-result.json)。

```sh
FLARE_BINARY=/absolute/path/to/flare \
FLARE_PLAYWRIGHT_MODULE=/absolute/path/to/playwright-core \
FLARE_BROWSER_REPORT=/tmp/flare-outbound-report.json \
node tests/security-browser/outbound.cjs
```

使用公开条目、私有显式图标条目及私有无图标条目，分别设置唯一标题、描述、目标路径和图标哨兵。全部为合成测试数据：跨源目标是另外监听的 loopback HTTP sink，外部 favicon 请求由 Playwright route 中止。因为 Playwright 对已继续的原始请求的后续重定向目标未必重新调用 route，不能只依赖 route.abort 声称已阻止全部外发；本组用本地 sink 兜底捕获真实浏览器跳转，保证不访问互联网目的站。

新增五项：

1. 匿名实际公开首页、`/?search=PRIVATE` 搜索页及登录/账号入口的 DOM 不含私人标题、描述、URL、图标；捕获请求也不含私人哨兵。
2. 匿名旧 Base64 私有跳转实际返回 404，不产生私有目标请求。
3. 公开条目使用同源不透明句柄跳转；目的 sink 捕获的实际请求没有 Referer。
4. 管理员能看到显式私有图标，其请求没有 Referer；私有无图标条目不会触发外部 favicon 回退。
5. 私有目标在显式点击前没有请求；授权点击同源不透明句柄后，目的 sink 捕获实际请求且没有 Referer。

本组不声称覆盖服务端 SSRF/DNS 重绑定、全部协议/CDN、手机实机或“公开后变私有并重启”的全部历史变化；旧跳转权限及重排句柄的其他回归测试见 Go/HTTP 验收记录。它补的是原来缺少的真实浏览器网络与 Referer 证据。

## 编辑器与导入 JavaScript 回归

`editor-transfer.cjs` 使用真实 Chromium、隔离数据和生产模板，开启 JavaScript。覆盖编辑器初始化、默认私有选项、隐私提示翻译、分类继承和保存，以及默认私有导入预览、公开选项变更使旧预览失效、重新预览及提交。Clock acceptance CI 执行本组；可用上面的 `FLARE_BINARY` 和 `FLARE_PLAYWRIGHT_MODULE` 环境变量单独运行。
