package cmd

import (
	"fmt"
	"log/slog"
	"runtime"
	"strings"

	flags "github.com/spf13/pflag"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/logger"
	version "github.com/soulteary/version-kit/v4"
)

func Parse() model.Flags {
	inputs, sources, inputErr := captureSecurityInputs()
	envs := ParseEnvFile(ParseEnvVars())
	envs.RuntimeInputs, envs.InputSources = inputs, sources
	if inputErr != nil {
		envs.ConfigError = inputErr.Error()
	}
	resolved := parseCLI(envs)

	log := logger.GetLogger()
	log.Info("程序服务端口", slog.Int(_KEY_PORT, resolved.Port))
	log.Info("页面请求合并", slog.Bool(_KEY_MINI_REQUEST, resolved.EnableMinimumRequest))
	log.Info("启用离线模式", slog.Bool(_KEY_ENABLE_OFFLINE, resolved.EnableOfflineMode))
	log.Info("访问策略将由安全启动检查确认；不会自动输出密码")

	if len(resolved.RuntimeInputs) == 0 {
		resolved.RuntimeInputs = nil
		resolved.InputSources = nil
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
