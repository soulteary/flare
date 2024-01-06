package FlareCMD

import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	flags "github.com/spf13/pflag"

	FlareData "github.com/soulteary/flare/config/data"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareModel "github.com/soulteary/flare/config/model"
	FlareLogger "github.com/soulteary/flare/internal/logger"
	"github.com/soulteary/flare/internal/version"
)

func Parse() FlareModel.Flags {
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
			log.Info("用户未指定 `FLARE_USER`，使用默认用户名", slog.String("username", FlareDefine.DEFAULT_USER_NAME))
		} else {
			log.Info("应用用户设置为", slog.String("username", flags.User))
		}

		if flags.PassIsGenerated {
			log.Info("用户未指定 `FLARE_PASS`，自动生成应用密码", slog.String("password", flags.Pass))
		} else {
			log.Info("应用登陆密码已设置为", slog.String("password", FlareData.MaskTextWithStars(flags.Pass)))
		}
	}

	FlareDefine.AppFlags = flags
	return flags
}

func ExcuteCLI(cliFlags *FlareModel.Flags, options *flags.FlagSet) (exit bool) {
	programVersion := GetVersion(false)
	if cliFlags.ShowHelp {
		fmt.Println(programVersion)
		fmt.Println()
		fmt.Println("支持命令：")
		options.PrintDefaults()
		return true
	}
	if cliFlags.ShowVersion {
		fmt.Println(version.Version)
		return true
	}
	return false
}

func GetVersion(echo bool) string {
	programVersion := fmt.Sprintf("Flare v%s-%s %s/%s BuildDate=%s", version.Version, strings.ToUpper(version.Commit), runtime.GOOS, runtime.GOARCH, version.BuildDate)
	if echo {
		log := FlareLogger.GetLogger()
		log.Info("Flare - 🏂 Challenge all bookmarking apps and websites directories, Aim to Be a best performance monster.")
		log.Info("程序信息：",
			slog.String("version", version.Version),
			slog.String("commit", strings.ToUpper(version.Commit)),
			slog.String("GOGS/ARCH", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)),
			slog.String("date", version.BuildDate),
		)
	}
	return programVersion
}
