package cmd

import (
	"github.com/soulteary/flare/pkg/logger"

	FlareData "github.com/soulteary/flare/internal/data"
	FlareState "github.com/soulteary/flare/internal/state"
)

func Parse() {

	envs := parseEnvFile(parseEnvVars())
	flags := parseCLI(envs)

	log := logger.GetLogger()
	log.Println()
	log.Println("程序服务端口", flags.Port)
	log.Println("页面请求合并", flags.EnableMinimumRequest)
	log.Println("启用离线模式", flags.EnableOfflineMode)
	if flags.DisableLoginMode {
		log.Println("已禁用登陆模式，用户可直接调整应用设置。")
	} else {
		log.Println("启用登陆模式，调整应用设置需要先进行登陆。")
		log.Println("当前内容整体可见性为：", flags.Visibility)

		if flags.UserIsGenerated {
			log.Println("用户未指定 `FLARE_USER`，使用默认用户名", _DEFAULT_USER_NAME)
		} else {
			log.Println("应用用户设置为", flags.User)

		}

		if flags.PassIsGenerated {
			log.Println("用户未指定 `FLARE_PASS`，自动生成应用密码", flags.Pass)
		} else {
			log.Println("应用登陆密码已设置为", FlareData.MaskTextWithStars(flags.Pass))
		}
	}

	FlareState.AppFlags = flags
	startDaemon(&flags)
}
