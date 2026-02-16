package theme

import (
	"net/http"

	"github.com/labstack/echo/v5"

	FlareData "github.com/soulteary/flare/config/data"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareAuth "github.com/soulteary/flare/internal/auth"
	FlarePool "github.com/soulteary/flare/internal/pool"
)

func RegisterRouting(e *echo.Echo) {
	e.GET(FlareDefine.SettingPages.Theme.Path, pageTheme, FlareAuth.AuthRequired)
	e.POST(FlareDefine.SettingPages.Theme.Path, updateThemes, FlareAuth.AuthRequired)
}

func updateThemes(c *echo.Context) error {
	var body struct {
		Theme string `form:"theme"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusForbidden, "提交数据缺失")
	}
	FlareData.UpdateThemeName(body.Theme)
	FlareDefine.UpdatePagePalettes()
	FlareDefine.ThemeCurrent = body.Theme
	FlareDefine.ThemePrimaryColor = FlareDefine.GetThemePrimaryColor(body.Theme)
	return pageTheme(c)
}

func pageTheme(c *echo.Context) error {
	themes := FlareDefine.ThemePalettes
	options := FlareData.GetAllSettingsOptions()
	locale := options.Locale
	if locale == "" {
		locale = "zh"
	}
	m := FlarePool.GetTemplateMap()
	defer FlarePool.PutTemplateMap(m)
	m["Locale"] = locale
	m["DebugMode"] = FlareDefine.AppFlags.DebugMode
	m["PageInlineStyle"] = FlareDefine.GetPageInlineStyle()
	m["PageAppearance"] = FlareDefine.GetAppBodyStyle()
	m["SettingsURI"] = FlareDefine.RegularPages.Settings.Path
	m["PageName"] = "Theme"
	m["SettingPages"] = FlareDefine.SettingPages
	m["Themes"] = themes
	m["OptionTitle"] = options.Title
	return c.Render(http.StatusOK, "settings.html", m)
}
