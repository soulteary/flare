package FlareCMD

import (
	"log/slog"
	"os"
	"path"

	FlareDefine "github.com/soulteary/flare/config/define"
	FlareModel "github.com/soulteary/flare/config/model"
	FlareLogger "github.com/soulteary/flare/internal/logger"
	"gopkg.in/ini.v1"
)

func ParseEnvFile(baseFlags FlareModel.Flags) FlareModel.Flags {
	log := FlareLogger.GetLogger()

	workDir, _ := os.Getwd()
	envPath := path.Join(workDir, ".env")

	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		log.Debug("默认的 .env 文件不存在，跳过解析。")
		return baseFlags
	}

	envs, err := ini.Load(envPath)
	if err != nil {
		log.Error("解析 .env 文件出错，请检查文件格式或程序是否具备文件读取权限。", slog.Any("error", err))
		os.Exit(1)
		return baseFlags
	}

	defaults := FlareDefine.GetDefaultEnvVars()

	err = envs.MapTo(&defaults)

	if envs.Section("").Key("FLARE_PASS") != nil {
		baseFlags.User = defaults.Pass
		baseFlags.UserIsGenerated = false
		baseFlags.PassIsGenerated = false
	}

	if err != nil {
		log.Error("解析 .env 文件出错，请检查文件内容是否正确。", slog.Any("error", err))
		os.Exit(1)
	} else {
		baseFlags.Port = defaults.Port
		baseFlags.EnableGuide = defaults.EnableGuide
		baseFlags.EnableDeprecatedNotice = defaults.EnableDeprecatedNotice
		baseFlags.EnableMinimumRequest = defaults.EnableMinimumRequest
		baseFlags.EnableOfflineMode = defaults.EnableOfflineMode
		baseFlags.EnableEditor = defaults.EnableEditor
		baseFlags.DisableCSP = defaults.DisableCSP
		baseFlags.Visibility = defaults.Visibility
		baseFlags.DisableLoginMode = defaults.DisableLoginMode
		baseFlags.User = defaults.User
		baseFlags.Pass = defaults.Pass
	}

	return baseFlags
}
