package others

import (
	"html/template"
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/pool"
	version "github.com/soulteary/version-kit"
)

func RegisterRouting(e *echo.Echo) {
	e.GET(define.SettingPages.Others.Path, pageOthers)
}

func pageOthers(c *echo.Context) error {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.String(http.StatusInternalServerError, "config error")
	}
	locale := options.Locale
	if locale == "" {
		locale = "zh"
	}
	isLogined := false
	if !define.AppFlags.DisableLoginMode {
		isLogined = auth.CheckUserIsLogin(c)
	} else {
		isLogined = true
	}
	m := pool.GetTemplateMap()
	defer pool.PutTemplateMap(m)
	m["Locale"] = locale
	m["DebugMode"] = define.AppFlags.DebugMode
	m["DisableLoginMode"] = define.AppFlags.DisableLoginMode
	m["UserIsLogin"] = isLogined
	m["UserName"] = auth.GetUserName(c)
	m["LoginDate"] = auth.GetUserLoginDate(c)
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	m["PageAppearance"] = define.GetAppBodyStyle()
	m["SettingsURI"] = define.RegularPages.Settings.Path
	m["LoginURI"] = define.MiscPages.Login.Path
	m["LogoutURI"] = define.MiscPages.Logout.Path
	m["PageName"] = "Others"
	m["SettingPages"] = define.SettingPages
	m["OptionTitle"] = options.Title
	m["Version"] = version.Version
	m["BuildDate"] = version.BuildDate
	m["COMMIT"] = version.Commit
	m["OptionFooter"] = template.HTML(options.Footer)
	return c.Render(http.StatusOK, "settings.html", m)
}
