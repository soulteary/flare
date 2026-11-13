package server

import (
	"html"
	"strings"
)

const notReadyHead = `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>Flare 尚未就绪 / Flare is not ready</title></head><body><main><h1>Flare 尚未就绪 / Flare is not ready</h1>
<p>现有内容未公开。具体原因记录在实例的终端输出或容器日志中。</p>
<p lang="en">Existing content is not being served. The instance terminal output or container log records the specific reason.</p>
`

const notReadyInput = `<h2>需要修正的配置 / Configuration to correct</h2>
<p>启动检查拒绝了下面这项配置。修正后重新启动实例（容器需要重新创建）；现有数据无需改动。可接受的写法见项目文档 docs/configuration.md。</p>
<p lang="en">Startup checks rejected the configuration input below. Correct it and restart the instance (recreate the container). Existing data does not need to change. See docs/configuration.md in the project documentation for accepted values.</p>
`

const notReadyNew = `<h2>已启用登录且尚未配置账户：初始化管理员 / Login is enabled and no account exists: initialize the administrator</h2>
<p>先停止服务，再在原工作目录、数据挂载和环境变量下执行初始化。密码需输入两次，终端不回显；要求 15–128 个字符且不能是弱密码。</p>
<p lang="en">Stop the service, then run the initialization with the original working directory, data mounts and environment. The password is entered twice without echo; it must be 15–128 characters and not a weak password.</p>
<h3>源码运行 / Running from source</h3><p>在运行终端按 Ctrl+C，然后执行： <span lang="en">Press Ctrl+C in the running terminal, then run:</span></p><pre>go run . security init --username admin
go run .</pre>
<h3>Docker Compose</h3><p>在原 Compose 目录执行（服务名以 flare 为例）： <span lang="en">Run in the original Compose directory (the service is named flare in this example):</span></p><pre>docker compose stop flare
docker compose run --rm --no-deps flare flare security init --username admin
docker compose up -d flare</pre>
<p>初始化成功并重启后，刷新页面，使用设置的账户登录。Docker 必须持久化 /app/.flare-security/（或自定义 FLARE_STATE_DIR）；挂载整个 /app 已包含该目录。</p>
<p lang="en">After initialization succeeds and the service restarts, reload this page and sign in with the account you set. Docker must persist /app/.flare-security/ (or a custom FLARE_STATE_DIR); mounting the whole of /app already includes it.</p>
<h2>已经初始化过？ / Already initialized?</h2>
<p>请根据实例终端或容器日志检查配置、目录权限和持久化状态。不要重复初始化或删除安全状态目录；损坏或缺失的已提交状态需要恢复完整备份。</p>
<p lang="en">Check the configuration, directory permissions and persistent state against the instance terminal or container log. Do not initialize again or delete the security state directory; committed state that is damaged or missing must be restored from a complete backup.</p>
`

const notReadyFaulted = `<h2>安全状态需要处理 / Security state needs attention</h2>
<p>内部安全状态无法打开或需要恢复。请根据实例终端或容器日志检查目录权限、实例锁和持久化状态。不要重复初始化或删除安全状态目录；损坏或缺失的已提交状态需要恢复完整备份。</p>
<p lang="en">The internal security state could not be opened or needs recovery. Check directory permissions, the instance lock and persistent storage against the instance terminal or container log. Do not initialize again or delete the security state directory; committed state that is damaged or missing must be restored from a complete backup.</p>
`

const notReadyAttention = `<h2>配置或数据需要处理 / Configuration or data needs attention</h2>
<p>请由实例管理员查看终端输出或容器日志，确认具体原因并按其提示处理。默认免登录模式会自动准备内部状态，无需手动初始化。不要重复初始化或删除安全状态目录。</p>
<p lang="en">The instance administrator should read the terminal output or container log for the specific reason and follow it. The default login-disabled mode prepares its internal state automatically and needs no manual initialization. Do not initialize again or delete the security state directory.</p>
`

const notReadyGuides = `<p>完整说明见项目文档 docs/accounts.md（含 docker run 步骤）和 docs/backup-restore.md。</p>
<p lang="en">Full instructions are in docs/accounts.md (including docker run steps) and docs/backup-restore.md in the project documentation.</p>
`

const notReadyFoot = `</main></body></html>`

// notReadyPage renders the only response served until startup checks pass.
//
// StartupMessage is deliberately not an input: it can quote file content. The
// diagnostic is fixed text produced by configuration input validation and is
// escaped regardless. Administrator initialization steps apply only to the
// "new" status. Site language settings are not available at this point, so the
// page carries both Chinese and English text.
func notReadyPage(status, diagnostic string) string {
	var page strings.Builder
	page.WriteString(notReadyHead)
	page.WriteString("<p>状态 / Status: <code>" + html.EscapeString(status) + "</code></p>\n")
	switch {
	case diagnostic != "":
		page.WriteString(notReadyInput)
		page.WriteString("<pre>" + html.EscapeString(diagnostic) + "</pre>\n")
	case status == "new":
		page.WriteString(notReadyNew + notReadyGuides)
	case status == "faulted":
		page.WriteString(notReadyFaulted + notReadyGuides)
	default:
		page.WriteString(notReadyAttention + notReadyGuides)
	}
	page.WriteString(notReadyFoot)
	return page.String()
}
