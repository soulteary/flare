package main

import (
	"fmt"
	"github.com/soulteary/flare/cmd"
	"github.com/soulteary/flare/internal/server"
	"os"
)

func main() {
	if handled, code := cmd.ExecuteSecurityCommand(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); handled {
		os.Exit(code)
	}
	flags := cmd.Parse()
	prepared := cmd.PrepareSecurity(&flags)
	if prepared.Store != nil {
		defer prepared.Store.Close()
	}
	if prepared.Status != "ready" {
		fmt.Fprintln(os.Stderr, "Flare security:", prepared.Status, prepared.Message)
		if prepared.Status == "new" {
			fmt.Fprintln(os.Stderr, `首次使用请先停止服务（源码运行按 Ctrl+C），再初始化管理员：
  源码：在原工作目录执行 go run . security init --username admin，然后 go run .
  二进制：flare security init --username admin，然后重新启动服务
  Docker Compose（服务名 flare）：
    docker compose stop flare
    docker compose run --rm --no-deps flare flare security init --username admin
    docker compose up -d flare
密码需输入两次，终端不回显；要求 15–128 个字符且不能是弱密码。
维护命令必须使用原数据目录、挂载和环境变量；持久化 .flare-security/ 或 FLARE_STATE_DIR。
完整步骤（含 docker run）：docs/accounts.md`)
		} else {
			fmt.Fprintln(os.Stderr, "请根据上方诊断检查配置、目录权限或完整备份；维护前先停止服务，保留原数据目录、挂载和环境变量。不要删除安全状态目录。参见 docs/accounts.md 和 docs/backup-restore.md。")
		}
	}
	server.StartDaemon(&flags, prepared.Store)
}
