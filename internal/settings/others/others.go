package others

import (
	"html/template"
	"net/http"

	"github.com/labstack/echo/v5"

	FlareData "github.com/soulteary/flare/config/data"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareAuth "github.com/soulteary/flare/internal/auth"
	FlarePool "github.com/soulteary/flare/internal/pool"
	FlareVersion "github.com/soulteary/flare/internal/version"
)

func RegisterRouting(e *echo.Echo) {
	e.GET(FlareDefine.SettingPages.Others.Path, pageOthers)
}

func pageOthers(c *echo.Context) error {
	options := FlareData.GetAllSettingsOptions()
	isLogined := false
	if !FlareDefine.AppFlags.DisableLoginMode {
		isLogined = FlareAuth.CheckUserIsLogin(c)
	} else {
		isLogined = true
	}
	m := FlarePool.GetTemplateMap()
	defer FlarePool.PutTemplateMap(m)
	m["DebugMode"] = FlareDefine.AppFlags.DebugMode
	m["DisableLoginMode"] = FlareDefine.AppFlags.DisableLoginMode
	m["UserIsLogin"] = isLogined
	m["UserName"] = FlareAuth.GetUserName(c)
	m["LoginDate"] = FlareAuth.GetUserLoginDate(c)
	m["PageInlineStyle"] = FlareDefine.GetPageInlineStyle()
	m["PageAppearance"] = FlareDefine.GetAppBodyStyle()
	m["SettingsURI"] = FlareDefine.RegularPages.Settings.Path
	m["LoginURI"] = FlareDefine.MiscPages.Login.Path
	m["LogoutURI"] = FlareDefine.MiscPages.Logout.Path
	m["PageName"] = "Others"
	m["SettingPages"] = FlareDefine.SettingPages
	m["OptionTitle"] = options.Title
	m["Version"] = FlareVersion.Version
	m["BuildDate"] = FlareVersion.BuildDate
	m["COMMIT"] = FlareVersion.Commit
	m["OptionFooter"] = template.HTML(options.Footer)
	return c.Render(http.StatusOK, "settings.html", m)
}
