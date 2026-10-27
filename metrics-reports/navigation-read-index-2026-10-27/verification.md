# 验证记录

基线版本及候选生产文件哈希见 source-manifest.json；基准摘要见 report.md。

通过的检查：

```sh
go run build/build.go
go build -o /tmp/flare-read-index .
go test -race -count=1 -coverpkg=./... -coverprofile=/tmp/flare-read-index-coverage.out -covermode=atomic ./...
go tool cover -func=/tmp/flare-read-index-coverage.out
go vet ./...
golangci-lint run
git diff --check
```

最终全量竞态覆盖率 91.1%，lint 为 0 issues。全量测试输出见 go-test-race.txt。
资源生成没有产生额外跟踪文件差异；最终分类快速路径修改后重新构建并验收。

浏览器使用仓库 Playwright 与本机 `/Applications/Google Chrome.app/Contents/MacOS/Google Chrome`，
通过 `CHROMIUM_EXECUTABLE` 显式选择。分类脚本在最终实现上重跑通过；公开/私有搜索
脚本在只读索引接入后通过，随后分类快速路径由完整 race 和分类浏览器检查覆盖。

```sh
FLARE_BINARY=/tmp/flare-read-index CHROMIUM_EXECUTABLE='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' node tests/mobile/category-navigation.browser.cjs
NODE_PATH=tests/clock/node_modules FLARE_BINARY=/tmp/flare-read-index CHROMIUM_EXECUTABLE='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' node tests/search/hybrid.cjs
NODE_PATH=tests/clock/node_modules FLARE_SEARCH_PRIVATE_TEST=1 FLARE_BINARY=/tmp/flare-read-index CHROMIUM_EXECUTABLE='/Applications/Google Chrome.app/Contents/MacOS/Google Chrome' node tests/search/hybrid.cjs
```

分类输出：`PASS: script-free category links, search scope/no-results, subpage navigation and collapsible groups across five viewport widths`。
公开与私有搜索分别输出：`PASS: three modes, legacy upgrade, permanent/latest backups, unchanged save and settings switching`。

新增基准脚本经 Python AST 解析检查，报告本地链接和候选文件摘要检查通过。
没有测量完整 HTTP 性能或 RSS，也没有进行物理移动设备验收。
