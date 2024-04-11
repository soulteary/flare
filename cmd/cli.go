package FlareCMD

import (
	"os"
	"regexp"
	"strings"

	"github.com/soulteary/flare/internal/version"
	flags "github.com/spf13/pflag"

	FlareDefine "github.com/soulteary/flare/config/define"
	FlareModel "github.com/soulteary/flare/config/model"
)

func GetCliFlags() (*FlareModel.Flags, *flags.FlagSet) {
	var cliFlags = new(FlareModel.Flags)
	options := flags.NewFlagSet("appFlags", flags.ContinueOnError)
	options.SortFlags = false

	// port
	options.IntVarP(&cliFlags.Port, _KEY_PORT, _KEY_PORT_SHORT, FlareDefine.DEFAULT_PORT, "指定监听端口")
	// guide
	options.BoolVarP(&cliFlags.EnableGuide, _KEY_ENABLE_GUIDE, _KEY_ENABLE_GUIDE_SHORT, FlareDefine.DEFAULT_ENABLE_GUIDE, "启用应用向导")
	// visibility
	options.StringVarP(&cliFlags.Visibility, _KEY_VISIBILITY, _KEY_VISIBILITY_SHORT, FlareDefine.DEFAULT_VISIBILITY, "调整网站整体可见性")
	// mini_request
	options.BoolVarP(&cliFlags.EnableMinimumRequest, _KEY_MINI_REQUEST, _KEY_MINI_REQUEST_SHORT, FlareDefine.DEFAULT_ENABLE_MINI_REQUEST, "使用请求最小化模式")
	options.BoolVar(&cliFlags.EnableMinimumRequest, _KEY_MINI_REQUEST_OLD, FlareDefine.DEFAULT_ENABLE_MINI_REQUEST, "使用请求最小化模式")
	_ = options.MarkDeprecated(_KEY_MINI_REQUEST_OLD, "please use --"+_KEY_MINI_REQUEST+" instead")
	// offline
	options.BoolVarP(&cliFlags.EnableOfflineMode, _KEY_ENABLE_OFFLINE, _KEY_ENABLE_OFFLINE_SHORT, FlareDefine.DEFAULT_ENABLE_OFFLINE, "启用离线模式")
	// disable_login
	options.BoolVarP(&cliFlags.DisableLoginMode, _KEY_DISABLE_LOGIN, _KEY_DISABLE_LOGIN_SHORT, FlareDefine.DEFAULT_DISABLE_LOGIN, "禁用账号登陆")
	options.BoolVar(&cliFlags.DisableLoginMode, _KEY_DISABLE_LOGIN_OLD, FlareDefine.DEFAULT_DISABLE_LOGIN, "禁用账号登陆")
	_ = options.MarkDeprecated(_KEY_DISABLE_LOGIN_OLD, "please use --"+_KEY_DISABLE_LOGIN+" instead")
	// 启用废弃日志警告
	options.BoolVarP(&cliFlags.EnableDeprecatedNotice, _KEY_ENABLE_DEPRECATED_NOTICE, _KEY_ENABLE_DEPRECATED_NOTICE_SHORT, FlareDefine.DEFAULT_ENABLE_DEPRECATED_NOTICE, "启用废弃日志警告")
	options.BoolVarP(&cliFlags.EnableEditor, _KEY_ENABLE_EDITOR, _KEY_ENABLE_EDITOR_SHORT, FlareDefine.DEFAULT_ENABLE_EDITOR, "启用编辑器")
	// 禁用 CSP
	options.BoolVarP(&cliFlags.DisableCSP, _KEY_DISABLE_CSP, _KEY_DISABLE_CSP_SHORT, FlareDefine.DEFAULT_DISABLE_CSP, "禁用CSP")
	// 其他
	options.BoolVarP(&cliFlags.ShowVersion, "version", "v", false, "显示应用版本号")
	options.BoolVarP(&cliFlags.ShowHelp, "help", "h", false, "显示帮助")
	// Cookie
	options.StringVarP(&cliFlags.CookieName, _KEY_COOKIE_NAME, _KEY_COOKIE_NAME_SHORT, FlareDefine.DEFAULT_COOKIE_NAME, "调整 Cookie 字段名称")
	options.StringVarP(&cliFlags.CookieSecret, _KEY_COOKIE_SECRET, _KEY_COOKIE_SECRET_SHORT, FlareDefine.DEFAULT_COOKIE_SECRET, "调整 Cookie 密钥")

	_ = options.Parse(os.Args)

	return cliFlags, options
}

func GetFlagsMaps() map[string]bool {
	keys := make(map[string]bool)
	if len(os.Args) <= 1 {
		return keys
	}
	trimValue := regexp.MustCompile(`=.*`)
	for _, key := range os.Args[1:] {
		if key[:2] == "--" {
			keys[trimValue.ReplaceAllString(key[2:], "")] = true
		} else if key[:1] == "-" {
			keys[trimValue.ReplaceAllString(key[1:], "")] = true
		}
	}
	return keys
}

func CheckFlagsExists(dict map[string]bool, keys []string) bool {
	for _, key := range keys {
		if dict[key] {
			return true
		}
	}
	return false
}

func parseCLI(baseFlags FlareModel.Flags) FlareModel.Flags {
	userOptionsFromCLI, originFlags := GetCliFlags()
	exit := ExcuteCLI(userOptionsFromCLI, originFlags)
	if exit {
		os.Exit(0)
	}
	GetVersion(true)

	// 用于判断参数是否存在
	keys := GetFlagsMaps()

	cliFlags := userOptionsFromCLI

	if CheckFlagsExists(keys, []string{_KEY_PORT, _KEY_PORT_SHORT}) {
		baseFlags.Port = cliFlags.Port
	}

	if CheckFlagsExists(keys, []string{_KEY_MINI_REQUEST, _KEY_MINI_REQUEST_SHORT, _KEY_MINI_REQUEST_OLD}) {
		baseFlags.EnableMinimumRequest = cliFlags.EnableMinimumRequest
	}

	if CheckFlagsExists(keys, []string{_KEY_DISABLE_LOGIN, _KEY_DISABLE_LOGIN_SHORT, _KEY_DISABLE_LOGIN_OLD}) {
		baseFlags.DisableLoginMode = cliFlags.DisableLoginMode
	}

	if CheckFlagsExists(keys, []string{_KEY_DISABLE_CSP, _KEY_DISABLE_CSP_SHORT}) {
		baseFlags.DisableCSP = cliFlags.DisableCSP
	}

	if CheckFlagsExists(keys, []string{_KEY_VISIBILITY, _KEY_VISIBILITY_SHORT}) {
		baseFlags.Visibility = cliFlags.Visibility
		// 判断是否为白名单中的词，以及强制转换内容为大写
		if strings.ToUpper(cliFlags.Visibility) != FlareDefine.DEFAULT_VISIBILITY &&
			strings.ToUpper(cliFlags.Visibility) != "PRIVATE" {
			baseFlags.Visibility = FlareDefine.DEFAULT_VISIBILITY
		} else {
			baseFlags.Visibility = strings.ToUpper(cliFlags.Visibility)
		}
	} else {
		baseFlags.Visibility = strings.ToUpper(baseFlags.Visibility)
	}

	if CheckFlagsExists(keys, []string{_KEY_ENABLE_OFFLINE, _KEY_ENABLE_OFFLINE_SHORT}) {
		baseFlags.EnableOfflineMode = cliFlags.EnableOfflineMode
	}

	if CheckFlagsExists(keys, []string{_KEY_ENABLE_DEPRECATED_NOTICE, _KEY_ENABLE_DEPRECATED_NOTICE_SHORT}) {
		baseFlags.EnableDeprecatedNotice = cliFlags.EnableDeprecatedNotice
	}

	if CheckFlagsExists(keys, []string{_KEY_ENABLE_GUIDE, _KEY_ENABLE_GUIDE_SHORT}) {
		baseFlags.EnableGuide = cliFlags.EnableGuide
	}

	if CheckFlagsExists(keys, []string{_KEY_ENABLE_EDITOR, _KEY_ENABLE_EDITOR_SHORT}) {
		baseFlags.EnableEditor = cliFlags.EnableEditor
	}

	// 设置 Cookie 相关信息
	if CheckFlagsExists(keys, []string{_KEY_COOKIE_NAME, _KEY_COOKIE_NAME_SHORT}) {
		baseFlags.CookieName = cliFlags.CookieName
	}

	if CheckFlagsExists(keys, []string{_KEY_COOKIE_SECRET, _KEY_COOKIE_SECRET_SHORT}) {
		baseFlags.CookieSecret = cliFlags.CookieSecret
	}

	// Forcibly disable `debug mode` in non-development mode
	if strings.ToLower(version.Version) != "dev" {
		baseFlags.DebugMode = false
	} else {
		if keys["D"] || keys["debug"] {
			baseFlags.DebugMode = true
		}
	}

	return baseFlags
}
