package FlareDefine

import (
	FlareModel "github.com/soulteary/flare/config/model"
)

const (
	DEFAULT_PORT                     = 5005
	DEFAULT_ENABLE_GUIDE             = true
	DEFAULT_ENABLE_DEPRECATED_NOTICE = true
	DEFAULT_ENABLE_MINI_REQUEST      = false
	DEFAULT_DISABLE_LOGIN            = true
	DEFAULT_ENABLE_OFFLINE           = false
	DEFAULT_USER_NAME                = "flare"
	DEFAULT_ENABLE_EDITOR            = true
	DEFAULT_VISIBILITY               = "DEFAULT"
	DEFAULT_DISABLE_CSP              = false

	DEFAULT_COOKIE_NAME   = "flare"
	DEFAULT_COOKIE_SECRET = "secret"
)

// get default env config
func GetDefaultEnvVars() FlareModel.Envs {
	return FlareModel.Envs{
		Port:                   DEFAULT_PORT,
		EnableGuide:            DEFAULT_ENABLE_GUIDE,
		EnableDeprecatedNotice: DEFAULT_ENABLE_DEPRECATED_NOTICE,
		EnableMinimumRequest:   DEFAULT_ENABLE_MINI_REQUEST,
		DisableLoginMode:       DEFAULT_DISABLE_LOGIN,
		EnableOfflineMode:      DEFAULT_ENABLE_OFFLINE,
		EnableEditor:           DEFAULT_ENABLE_EDITOR,
		Visibility:             DEFAULT_VISIBILITY,
		DisableCSP:             DEFAULT_DISABLE_CSP,

		User: DEFAULT_USER_NAME,
		Pass: "",

		CookieName:   DEFAULT_COOKIE_NAME,
		CookieSecret: DEFAULT_COOKIE_SECRET,
	}
}

var DefaultEnvVars = GetDefaultEnvVars()

var AppFlags FlareModel.Flags

// VISIABLE Levels
// - "DEFAULT"
// - "PRIVATE"
var FLARE_VISIABLE = "PRIVATE"
