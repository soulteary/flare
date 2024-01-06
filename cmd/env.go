package FlareCMD

import (
	"fmt"

	env "github.com/caarlos0/env/v6"

	FlareData "github.com/soulteary/flare/config/data"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareModel "github.com/soulteary/flare/config/model"
	FlareLogger "github.com/soulteary/flare/internal/logger"
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
		*targetPass = FlareData.GenerateRandomString(8)
		*isPassGenerate = true
	} else {
		*isPassGenerate = false
		*targetPass = password
	}
}

func ParseEnvVars() (stor FlareModel.Flags) {
	log := FlareLogger.GetLogger()

	// 1. init default values
	defaults := FlareDefine.DefaultEnvVars

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
		FlareDefine.DEFAULT_USER_NAME,
		&stor.UserIsGenerated,
		&stor.PassIsGenerated,
		&stor.DisableLoginMode,
	)

	// 4. merge
	stor.Port = defaults.Port
	stor.EnableGuide = defaults.EnableGuide
	stor.EnableDeprecatedNotice = defaults.EnableDeprecatedNotice
	stor.EnableMinimumRequest = defaults.EnableMinimumRequest
	stor.DisableLoginMode = defaults.DisableLoginMode
	stor.Visibility = defaults.Visibility
	stor.EnableOfflineMode = defaults.EnableOfflineMode
	stor.EnableEditor = defaults.EnableEditor
	stor.DisableCSP = defaults.DisableCSP

	return stor
}
