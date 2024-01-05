package FlareCMD

import (
	"fmt"
	"log/slog"
	"os"
	"regexp"
	"runtime"
	"strings"

	env "github.com/caarlos0/env/v6"
	"github.com/soulteary/flare/internal/version"
	flags "github.com/spf13/pflag"
	"gopkg.in/ini.v1"

	FlareData "github.com/soulteary/flare/config/data"
	FlareModel "github.com/soulteary/flare/config/model"
	FlareLogger "github.com/soulteary/flare/internal/logger"
)

func ParseEnvVars() (stor FlareModel.Flags) {
	log := FlareLogger.GetLogger()

	// 1. init default values
	defaults := FlareModel.Envs{
		Port:                   DEFAULT_PORT,
		EnableGuide:            DEFAULT_ENABLE_GUIDE,
		EnableDeprecatedNotice: DEFAULT_ENABLE_DEPRECATED_NOTICE,
		EnableMinimumRequest:   DEFAULT_ENABLE_MINI_REQUEST,
		DisableLoginMode:       DEFAULT_DISABLE_LOGIN,
		EnableOfflineMode:      DEFAULT_ENABLE_OFFLINE,
		EnableEditor:           DEFAULT_ENABLE_EDITOR,
		Visibility:             DEFAULT_VISIBILITY,
		DisableCSP:             DEFAULT_DISABLE_CSP,
	}

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
		DEFAULT_USER_NAME,
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

func ParseEnvFile(baseFlags FlareModel.Flags) FlareModel.Flags {
	log := FlareLogger.GetLogger()

	if _, err := os.Stat(".env"); os.IsNotExist(err) {
		log.Debug("默认的 .env 文件不存在，跳过解析。")
		return baseFlags
	}

	envs, err := ini.Load(".env")
	if err != nil {
		log.Error("解析 .env 文件出错，请检查文件格式或程序是否具备文件读取权限。", slog.Any("error", err))
		os.Exit(1)
		return baseFlags
	}

	defaults := FlareModel.EnvFile{
		Port:                   DEFAULT_PORT,
		EnableGuide:            DEFAULT_ENABLE_GUIDE,
		EnableDeprecatedNotice: DEFAULT_ENABLE_DEPRECATED_NOTICE,
		EnableMinimumRequest:   DEFAULT_ENABLE_MINI_REQUEST,
		DisableLoginMode:       DEFAULT_DISABLE_LOGIN,
		EnableOfflineMode:      DEFAULT_ENABLE_OFFLINE,
		EnableEditor:           DEFAULT_ENABLE_EDITOR,
		Visibility:             DEFAULT_VISIBILITY,
		DisableCSP:             DEFAULT_DISABLE_CSP,
	}

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

func parseCLI(baseFlags FlareModel.Flags) FlareModel.Flags {

	var cliFlags = new(FlareModel.Flags)
	options := flags.NewFlagSet("appFlags", flags.ContinueOnError)
	options.SortFlags = false

	// port
	options.IntVarP(&cliFlags.Port, _KEY_PORT, _KEY_PORT_SHORT, DEFAULT_PORT, "指定监听端口")
	// guide
	options.BoolVarP(&cliFlags.EnableGuide, _KEY_ENABLE_GUIDE, _KEY_ENABLE_GUIDE_SHORT, DEFAULT_ENABLE_GUIDE, "启用应用向导")
	// visibility
	options.StringVarP(&cliFlags.Visibility, _KEY_VISIBILITY, _KEY_VISIBILITY_SHORT, DEFAULT_VISIBILITY, "调整网站整体可见性")
	// mini_request
	options.BoolVarP(&cliFlags.EnableMinimumRequest, _KEY_MINI_REQUEST, _KEY_MINI_REQUEST_SHORT, DEFAULT_ENABLE_MINI_REQUEST, "使用请求最小化模式")
	options.BoolVar(&cliFlags.EnableMinimumRequest, _KEY_MINI_REQUEST_OLD, DEFAULT_ENABLE_MINI_REQUEST, "使用请求最小化模式")
	_ = options.MarkDeprecated(_KEY_MINI_REQUEST_OLD, "please use --"+_KEY_MINI_REQUEST+" instead")
	// offline
	options.BoolVarP(&cliFlags.EnableOfflineMode, _KEY_ENABLE_OFFLINE, _KEY_ENABLE_OFFLINE_SHORT, DEFAULT_ENABLE_OFFLINE, "启用离线模式")
	// disable_login
	options.BoolVarP(&cliFlags.DisableLoginMode, _KEY_DISABLE_LOGIN, _KEY_DISABLE_LOGIN_SHORT, DEFAULT_DISABLE_LOGIN, "禁用账号登陆")
	options.BoolVar(&cliFlags.DisableLoginMode, _KEY_DISABLE_LOGIN_OLD, DEFAULT_DISABLE_LOGIN, "禁用账号登陆")
	_ = options.MarkDeprecated(_KEY_DISABLE_LOGIN_OLD, "please use --"+_KEY_DISABLE_LOGIN+" instead")
	// 启用废弃日志警告
	options.BoolVarP(&cliFlags.EnableDeprecatedNotice, _KEY_ENABLE_DEPRECATED_NOTICE, _KEY_ENABLE_DEPRECATED_NOTICE_SHORT, DEFAULT_ENABLE_DEPRECATED_NOTICE, "启用废弃日志警告")
	options.BoolVarP(&cliFlags.EnableEditor, _KEY_ENABLE_EDITOR, _KEY_ENABLE_EDITOR_SHORT, DEFAULT_ENABLE_EDITOR, "启用编辑器")
	// 禁用 CSP
	options.BoolVarP(&cliFlags.DisableCSP, _KEY_DISABLE_CSP, _KEY_DISABLE_CSP_SHORT, DEFAULT_DISABLE_CSP, "禁用CSP")
	// 其他
	options.BoolVarP(&cliFlags.ShowVersion, "version", "v", false, "显示应用版本号")
	options.BoolVarP(&cliFlags.ShowHelp, "help", "h", false, "显示帮助")

	_ = options.Parse(os.Args)

	exit := ExcuteCLI(cliFlags, options)
	if exit {
		os.Exit(0)
	}
	GetVersion(true)

	// 用于判断参数是否存在
	keys := make(map[string]bool)
	trimValue := regexp.MustCompile(`=.*`)
	for _, key := range os.Args[1:] {
		if key[:2] == "--" {
			keys[trimValue.ReplaceAllString(key[2:], "")] = true
		} else if key[:1] == "-" {
			keys[trimValue.ReplaceAllString(key[1:], "")] = true
		}
	}

	if keys[_KEY_PORT] || keys[_KEY_PORT_SHORT] {
		baseFlags.Port = cliFlags.Port
	}

	if keys[_KEY_MINI_REQUEST] || keys[_KEY_MINI_REQUEST_SHORT] || keys[_KEY_MINI_REQUEST_OLD] {
		baseFlags.EnableMinimumRequest = cliFlags.EnableMinimumRequest
	}

	if keys[_KEY_DISABLE_LOGIN] || keys[_KEY_DISABLE_LOGIN_SHORT] || keys[_KEY_DISABLE_LOGIN_OLD] {
		baseFlags.DisableLoginMode = cliFlags.DisableLoginMode
	}

	if keys[_KEY_DISABLE_CSP] || keys[_KEY_DISABLE_CSP_SHORT] {
		baseFlags.DisableCSP = cliFlags.DisableCSP
	}

	if keys[_KEY_VISIBILITY] || keys[_KEY_VISIBILITY_SHORT] {
		baseFlags.Visibility = cliFlags.Visibility
		// 判断是否为白名单中的词，以及强制转换内容为大写
		if strings.ToUpper(cliFlags.Visibility) != DEFAULT_VISIBILITY &&
			strings.ToUpper(cliFlags.Visibility) != "PRIVATE" {
			baseFlags.Visibility = DEFAULT_VISIBILITY
		} else {
			baseFlags.Visibility = strings.ToUpper(cliFlags.Visibility)
		}
	} else {
		baseFlags.Visibility = strings.ToUpper(baseFlags.Visibility)
	}

	if keys[_KEY_ENABLE_OFFLINE] || keys[_KEY_ENABLE_OFFLINE_SHORT] {
		baseFlags.EnableOfflineMode = cliFlags.EnableOfflineMode
	}

	if keys[_KEY_ENABLE_DEPRECATED_NOTICE] || keys[_KEY_ENABLE_DEPRECATED_NOTICE_SHORT] {
		baseFlags.EnableDeprecatedNotice = cliFlags.EnableDeprecatedNotice
	}

	if keys[_KEY_ENABLE_GUIDE] || keys[_KEY_ENABLE_GUIDE_SHORT] {
		baseFlags.EnableGuide = cliFlags.EnableGuide
	}

	if keys[_KEY_ENABLE_EDITOR] || keys[_KEY_ENABLE_EDITOR_SHORT] {
		baseFlags.EnableEditor = cliFlags.EnableEditor
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
