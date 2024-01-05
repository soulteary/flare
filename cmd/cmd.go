package cmd

import (
	"log/slog"

	FlareData "github.com/soulteary/flare/internal/data"
	FlareLogger "github.com/soulteary/flare/internal/logger"
	FlareState "github.com/soulteary/flare/internal/state"
)

func Parse() {
	envs := ParseEnvFile(ParseEnvVars())
	flags := parseCLI(envs)

	log := FlareLogger.GetLogger()
	log.Info("程序服务端口", slog.Int(_KEY_PORT, flags.Port))
	log.Info("页面请求合并", slog.Bool(_KEY_MINI_REQUEST, flags.EnableMinimumRequest))
	log.Info("启用离线模式", slog.Bool(_KEY_ENABLE_OFFLINE, flags.EnableOfflineMode))
	if flags.DisableLoginMode {
		log.Info("已禁用登陆模式，用户可直接调整应用设置。")
	} else {
		log.Info("启用登陆模式，调整应用设置需要先进行登陆。")
		log.Info("当前内容整体可见性为：", slog.String(_KEY_VISIBILITY, flags.Visibility))

		if flags.UserIsGenerated {
			log.Info("用户未指定 `FLARE_USER`，使用默认用户名", slog.String("username", DEFAULT_USER_NAME))
		} else {
			log.Info("应用用户设置为", slog.String("username", flags.User))
		}

		if flags.PassIsGenerated {
			log.Info("用户未指定 `FLARE_PASS`，自动生成应用密码", slog.String("password", flags.Pass))
		} else {
			log.Info("应用登陆密码已设置为", slog.String("password", FlareData.MaskTextWithStars(flags.Pass)))
		}
	}

	FlareState.AppFlags = flags
	startDaemon(&flags)
}
