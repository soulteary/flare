package FlareCMD

import (
	"fmt"
	"os"
	"strings"

	FlareDefine "github.com/soulteary/flare/config/define"
	FlareModel "github.com/soulteary/flare/config/model"
	FlareFn "github.com/soulteary/flare/internal/fn"
	FlareLogger "github.com/soulteary/flare/internal/logger"
	"gopkg.in/ini.v1"
)

func CheckDotEnvFileExist(filePath string) bool {
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log := FlareLogger.GetLogger()
		log.Debug("默认的 .env 文件不存在，跳过解析。")
		return false
	}
	return true
}

func GetDotEnvFileStringOrDefault(envs *ini.File, key string, def string) string {
	value := strings.TrimSpace(envs.Section("").Key(key).String())
	if value == "" {
		log := FlareLogger.GetLogger()
		log.Debug(fmt.Sprintf("%s 配置文件值为空，使用默认值。", key))
		return def
	}
	return value
}

func GetDotEnvFileBoolOrDefault(envs *ini.File, key string, def bool) bool {
	value, err := envs.Section("").Key(key).Bool()
	if err != nil {
		log := FlareLogger.GetLogger()
		log.Debug(fmt.Sprintf("%s 配置文件值异常，使用默认值。", key))
		return def
	}
	return value
}

func ParseEnvFile(baseFlags FlareModel.Flags) FlareModel.Flags {
	log := FlareLogger.GetLogger()

	envPath := FlareFn.GetWorkDirFile(".env")

	if !CheckDotEnvFileExist(envPath) {
		return baseFlags
	}

	envs, err := ini.Load(envPath)
	if err != nil {
		log.Error("解析 .env 文件出错，请检查文件格式或程序是否具备文件读取权限。")
		log.Warn("程序将使用默认配置继续运行。")
		return baseFlags
	}

	err = envs.MapTo(&FlareModel.Envs{})
	if err != nil {
		log.Error("解析 .env 文件出错，请检查文件内容是否正确。")
		log.Warn("程序使用默认配置继续运行。")
		return baseFlags
	}

	port, err := envs.Section("").Key("FLARE_PORT").Int()
	if err != nil {
		log.Warn("FLARE_PORT 的值不是有效的数字，使用默认值。")
	} else {
		if port < 1 || port > 65535 {
			log.Warn("FLARE_PORT 的值不在有效范围内，使用默认值。")
		} else {
			baseFlags.Port = port
		}
	}

	user := GetDotEnvFileStringOrDefault(envs, "FLARE_USER", baseFlags.User)
	baseFlags.User = user

	defaults := FlareDefine.DefaultEnvVars

	if user == defaults.User {
		baseFlags.UserIsGenerated = false
	} else {
		baseFlags.UserIsGenerated = true
	}

	pass := GetDotEnvFileStringOrDefault(envs, "FLARE_PASS", baseFlags.Pass)
	baseFlags.Pass = pass
	if pass == defaults.Pass {
		baseFlags.UserIsGenerated = false
	} else {
		baseFlags.UserIsGenerated = true
	}

	baseFlags.DisableLoginMode = GetDotEnvFileBoolOrDefault(envs, "FLARE_DISABLE_LOGIN", baseFlags.DisableLoginMode)
	baseFlags.DisableCSP = GetDotEnvFileBoolOrDefault(envs, "FLARE_DISABLE_CSP", baseFlags.DisableCSP)
	baseFlags.EnableDeprecatedNotice = GetDotEnvFileBoolOrDefault(envs, "FLARE_DEPRECATED_NOTICE", baseFlags.EnableDeprecatedNotice)
	baseFlags.EnableMinimumRequest = GetDotEnvFileBoolOrDefault(envs, "FLARE_MINI_REQUEST", baseFlags.EnableMinimumRequest)
	baseFlags.EnableOfflineMode = GetDotEnvFileBoolOrDefault(envs, "FLARE_OFFLINE", baseFlags.EnableOfflineMode)
	baseFlags.EnableEditor = GetDotEnvFileBoolOrDefault(envs, "FLARE_EDITOR", baseFlags.EnableEditor)
	baseFlags.EnableGuide = GetDotEnvFileBoolOrDefault(envs, "FLARE_GUIDE", baseFlags.EnableGuide)
	baseFlags.Visibility = GetDotEnvFileStringOrDefault(envs, "FLARE_VISIBILITY", baseFlags.Visibility)
	baseFlags.CookieName = GetDotEnvFileStringOrDefault(envs, "FLARE_COOKIE_NAME", baseFlags.CookieName)
	baseFlags.CookieSecret = GetDotEnvFileStringOrDefault(envs, "FLARE_COOKIE_SECRET", baseFlags.CookieSecret)

	return baseFlags
}
