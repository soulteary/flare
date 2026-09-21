package cmd

import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	flags "github.com/spf13/pflag"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/logger"
	version "github.com/soulteary/version-kit/v4"
)

func Parse() model.Flags {
	envs := ParseEnvFile(ParseEnvVars())
	resolved := parseCLI(envs)

	log := logger.GetLogger()
	log.Info("程序服务端口", slog.Int(_KEY_PORT, resolved.Port))
	log.Info("页面请求合并", slog.Bool(_KEY_MINI_REQUEST, resolved.EnableMinimumRequest))
	log.Info("启用离线模式", slog.Bool(_KEY_ENABLE_OFFLINE, resolved.EnableOfflineMode))
	if resolved.DisableLoginMode {
		log.Info("已禁用登陆模式，用户可直接调整应用设置。")
	} else {
		log.Info("启用登陆模式，调整应用设置需要先进行登陆。")
		log.Info("当前内容整体可见性为：", slog.String(_KEY_VISIBILITY, resolved.Visibility))

		if resolved.UserIsGenerated {
			log.Info("用户未指定 `FLARE_USER`，使用默认用户名", slog.String("username", define.DEFAULT_USER_NAME))
		} else {
			log.Info("应用用户设置为", slog.String("username", resolved.User))
		}

		if resolved.PassIsGenerated {
			log.Info("用户未指定 `FLARE_PASS`，自动生成应用密码", slog.String("password", resolved.Pass))
		} else {
			log.Info("应用登陆密码已设置为", slog.String("password", data.MaskTextWithStars(resolved.Pass)))
		}
	}

	define.AppFlags = resolved
	return resolved
}

// ExecuteCLI handles --help and --version; returns true if the program should exit.
func ExecuteCLI(cliFlags *model.Flags, options *flags.FlagSet) (exit bool) {
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
	info := version.Default()
	programVersion := fmt.Sprintf("Flare v%s-%s %s/%s BuildDate=%s", info.Version, strings.ToUpper(info.Commit), runtime.GOOS, runtime.GOARCH, info.BuildDate)
	if echo {
		log := logger.GetLogger()
		log.Info("Flare - 🏂 Challenge all bookmarking apps and websites directories, Aim to Be a best performance monster.")
		log.Info("程序信息：",
			slog.String("version", info.Version),
			slog.String("commit", strings.ToUpper(info.Commit)),
			slog.String("GOGS/ARCH", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)),
			slog.String("date", info.BuildDate),
		)
	}
	return programVersion
}
