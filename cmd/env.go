package cmd

import (
	"fmt"
	"os"
	"strings"

	env "github.com/caarlos0/env/v6"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/logger"
)

func InitAccountFromEnvVars(
	username string, password string, targetUser *string, targetPass *string, defaultName string,
	isUserGenerate *bool, isPassGenerate *bool, disableLogin *bool) {

	if username == "" {
		*targetUser = defaultName
		*isUserGenerate = true
	} else {
		*isUserGenerate = false
		*targetUser = username
	}

	if password == "" {
		*targetPass = data.GenerateRandomString(8)
		*isPassGenerate = true
	} else {
		*isPassGenerate = false
		*targetPass = password
	}
}

func ParseEnvVars() (stor model.Flags) {
	log := logger.GetLogger()

	// 1. init default values
	defaults := define.DefaultEnvVars

	// 2. overwrite with user input
	if err := env.Parse(&defaults); err != nil {
		log.Error(fmt.Sprintf("%+v\n", err))
		return
	}

	// 3. update username and password
	InitAccountFromEnvVars(
		defaults.User,
		defaults.Pass,
		&stor.User,
		&stor.Pass,
		define.DEFAULT_USER_NAME,
		&stor.UserIsGenerated,
		&stor.PassIsGenerated,
		&stor.DisableLoginMode,
	)

	// 4. merge
	// env.Parse treats an empty environment value as unset; validate an explicit
	// empty access mode as well, without changing other configuration parsing.
	if value, ok := os.LookupEnv("FLARE_ACCESS_LOG"); ok {
		defaults.AccessLog = value
	}
	stor.AccessLog = normalizeAccessLog(defaults.AccessLog, "FLARE_ACCESS_LOG")
	stor.Port = defaults.Port
	stor.EnableGuide = defaults.EnableGuide
	stor.EnableDeprecatedNotice = defaults.EnableDeprecatedNotice
	stor.EnableMinimumRequest = defaults.EnableMinimumRequest
	stor.DisableLoginMode = defaults.DisableLoginMode
	stor.Visibility = defaults.Visibility
	stor.EnableOfflineMode = defaults.EnableOfflineMode
	stor.EnableEditor = defaults.EnableEditor
	stor.DisableCSP = defaults.DisableCSP
	stor.CookieName = defaults.CookieName
	stor.CookieSecret = defaults.CookieSecret
	stor.CookieSecure = defaults.CookieSecure

	return stor
}

func normalizeAccessLog(value, source string) string {
	switch mode := strings.ToLower(strings.TrimSpace(value)); mode {
	case "errors", "all", "off":
		return mode
	default:
		logger.GetLogger().Warn(fmt.Sprintf("%s 的访问日志模式无效，使用 errors；可选值：errors、all、off。", source))
		return define.DEFAULT_ACCESS_LOG
	}
}
