package cmd

import (
	"os"
	"regexp"
	"strings"

	"github.com/soulteary/cli-kit/configutil"
	"github.com/soulteary/cli-kit/flagutil"
	version "github.com/soulteary/version-kit"
	flags "github.com/spf13/pflag"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
)

var flagsMapTrimValue = regexp.MustCompile(`=.*`)

func GetCliFlags() (*model.Flags, *flags.FlagSet) {
	var cliFlags = new(model.Flags)
	options := flags.NewFlagSet("appFlags", flags.ContinueOnError)
	options.SortFlags = false

	// port
	options.IntVarP(&cliFlags.Port, _KEY_PORT, _KEY_PORT_SHORT, define.DEFAULT_PORT, "指定监听端口")
	// guide
	options.BoolVarP(&cliFlags.EnableGuide, _KEY_ENABLE_GUIDE, _KEY_ENABLE_GUIDE_SHORT, define.DEFAULT_ENABLE_GUIDE, "启用应用向导")
	// visibility
	options.StringVarP(&cliFlags.Visibility, _KEY_VISIBILITY, _KEY_VISIBILITY_SHORT, define.DEFAULT_VISIBILITY, "调整网站整体可见性")
	// mini_request
	options.BoolVarP(&cliFlags.EnableMinimumRequest, _KEY_MINI_REQUEST, _KEY_MINI_REQUEST_SHORT, define.DEFAULT_ENABLE_MINI_REQUEST, "使用请求最小化模式")
	options.BoolVar(&cliFlags.EnableMinimumRequest, _KEY_MINI_REQUEST_OLD, define.DEFAULT_ENABLE_MINI_REQUEST, "使用请求最小化模式")
	_ = options.MarkDeprecated(_KEY_MINI_REQUEST_OLD, "please use --"+_KEY_MINI_REQUEST+" instead")
	// offline
	options.BoolVarP(&cliFlags.EnableOfflineMode, _KEY_ENABLE_OFFLINE, _KEY_ENABLE_OFFLINE_SHORT, define.DEFAULT_ENABLE_OFFLINE, "启用离线模式")
	// disable_login
	options.BoolVarP(&cliFlags.DisableLoginMode, _KEY_DISABLE_LOGIN, _KEY_DISABLE_LOGIN_SHORT, define.DEFAULT_DISABLE_LOGIN, "禁用账号登陆")
	options.BoolVar(&cliFlags.DisableLoginMode, _KEY_DISABLE_LOGIN_OLD, define.DEFAULT_DISABLE_LOGIN, "禁用账号登陆")
	_ = options.MarkDeprecated(_KEY_DISABLE_LOGIN_OLD, "please use --"+_KEY_DISABLE_LOGIN+" instead")
	// 启用废弃日志警告
	options.BoolVarP(&cliFlags.EnableDeprecatedNotice, _KEY_ENABLE_DEPRECATED_NOTICE, _KEY_ENABLE_DEPRECATED_NOTICE_SHORT, define.DEFAULT_ENABLE_DEPRECATED_NOTICE, "启用废弃日志警告")
	options.BoolVarP(&cliFlags.EnableEditor, _KEY_ENABLE_EDITOR, _KEY_ENABLE_EDITOR_SHORT, define.DEFAULT_ENABLE_EDITOR, "启用编辑器")
	// 禁用 CSP
	options.BoolVarP(&cliFlags.DisableCSP, _KEY_DISABLE_CSP, _KEY_DISABLE_CSP_SHORT, define.DEFAULT_DISABLE_CSP, "禁用CSP")
	// 其他
	options.BoolVarP(&cliFlags.ShowVersion, "version", "v", false, "显示应用版本号")
	options.BoolVarP(&cliFlags.ShowHelp, "help", "h", false, "显示帮助")
	// Cookie
	options.StringVarP(&cliFlags.CookieName, _KEY_COOKIE_NAME, _KEY_COOKIE_NAME_SHORT, define.DEFAULT_COOKIE_NAME, "调整 Cookie 字段名称")
	options.StringVarP(&cliFlags.CookieSecret, _KEY_COOKIE_SECRET, _KEY_COOKIE_SECRET_SHORT, define.DEFAULT_COOKIE_SECRET, "调整 Cookie 密钥")

	_ = options.Parse(os.Args)

	return cliFlags, options
}

// GetFlagsMaps returns a set of flag names that appear in os.Args (supports -x, --x, -x=val, --x=val).
// Used by tests. For unregistered flags (e.g. debug) use flagutil.HasFlagInOSArgs.
func GetFlagsMaps() map[string]bool {
	keys := make(map[string]bool)
	if len(os.Args) <= 1 {
		return keys
	}
	for _, key := range os.Args[1:] {
		var name string
		if len(key) >= 2 && key[:2] == "--" {
			name = flagsMapTrimValue.ReplaceAllString(key[2:], "")
		} else if len(key) >= 1 && key[:1] == "-" {
			name = flagsMapTrimValue.ReplaceAllString(key[1:], "")
		}
		if name != "" {
			keys[name] = true
		}
	}
	return keys
}

// CheckFlagsExists returns true if any of keys is present in dict.
// Returns false if dict is nil.
func CheckFlagsExists(dict map[string]bool, keys []string) bool {
	if dict == nil {
		return false
	}
	for _, key := range keys {
		if dict[key] {
			return true
		}
	}
	return false
}

func parseCLI(baseFlags model.Flags) model.Flags {
	cliFlags, fs := GetCliFlags()
	exit := ExecuteCLI(cliFlags, fs)
	if exit {
		os.Exit(0)
	}
	GetVersion(true)

	// Resolve from CLI when flag is set, else keep base (env + envfile). Empty envKey = no env re-read.
	port, err := configutil.ResolvePortPflag(fs, _KEY_PORT, "", baseFlags.Port)
	if err != nil {
		port = define.DEFAULT_PORT
	}
	baseFlags.Port = port

	baseFlags.EnableMinimumRequest = configutil.ResolveBoolPflag(fs, _KEY_MINI_REQUEST, "", baseFlags.EnableMinimumRequest)
	baseFlags.DisableLoginMode = configutil.ResolveBoolPflag(fs, _KEY_DISABLE_LOGIN, "", baseFlags.DisableLoginMode)
	baseFlags.DisableCSP = configutil.ResolveBoolPflag(fs, _KEY_DISABLE_CSP, "", baseFlags.DisableCSP)

	visibility, err := configutil.ResolveEnumPflag(fs, _KEY_VISIBILITY, "", baseFlags.Visibility, []string{"DEFAULT", "PRIVATE"}, false)
	if err != nil {
		visibility = define.DEFAULT_VISIBILITY
	}
	baseFlags.Visibility = strings.ToUpper(visibility)

	baseFlags.EnableOfflineMode = configutil.ResolveBoolPflag(fs, _KEY_ENABLE_OFFLINE, "", baseFlags.EnableOfflineMode)
	baseFlags.EnableDeprecatedNotice = configutil.ResolveBoolPflag(fs, _KEY_ENABLE_DEPRECATED_NOTICE, "", baseFlags.EnableDeprecatedNotice)
	baseFlags.EnableGuide = configutil.ResolveBoolPflag(fs, _KEY_ENABLE_GUIDE, "", baseFlags.EnableGuide)
	baseFlags.EnableEditor = configutil.ResolveBoolPflag(fs, _KEY_ENABLE_EDITOR, "", baseFlags.EnableEditor)

	baseFlags.CookieName = configutil.ResolveStringPflag(fs, _KEY_COOKIE_NAME, "", baseFlags.CookieName, true)
	baseFlags.CookieSecret = configutil.ResolveStringPflag(fs, _KEY_COOKIE_SECRET, "", baseFlags.CookieSecret, true)

	// Forcibly disable debug mode in non-development mode
	if !version.Default().IsDev() {
		baseFlags.DebugMode = false
	} else {
		baseFlags.DebugMode = flagutil.HasFlagInOSArgs("D") || flagutil.HasFlagInOSArgs("debug")
	}

	return baseFlags
}
