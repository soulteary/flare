package theme

import (
	"net/http"

	"github.com/gin-gonic/gin"

	FlareData "github.com/soulteary/flare/config/data"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareAuth "github.com/soulteary/flare/internal/auth"
)

func RegisterRouting(router *gin.Engine) {
	router.GET(FlareDefine.SettingPages.Theme.Path, FlareAuth.AuthRequired, pageTheme)
	router.POST(FlareDefine.SettingPages.Theme.Path, FlareAuth.AuthRequired, updateThemes)
}

func updateThemes(c *gin.Context) {

	type UpdateThemeBody struct {
		Theme string `form:"theme"`
	}

	var body UpdateThemeBody
	if c.ShouldBind(&body) != nil {
		c.PureJSON(http.StatusForbidden, "提交数据缺失")
		return
	}

	FlareData.UpdateThemeName(body.Theme)
	FlareDefine.UpdatePagePalettes()

	// 中转变量
	FlareDefine.ThemeCurrent = body.Theme
	FlareDefine.ThemePrimaryColor = FlareDefine.GetThemePrimaryColor(body.Theme)

	pageTheme(c)
}

func pageTheme(c *gin.Context) {
	// themes := getThemePalettes()
	themes := FlareDefine.ThemePalettes
	options := FlareData.GetAllSettingsOptions()

	c.HTML(
		http.StatusOK,
		"settings.html",
		gin.H{
			"DebugMode":       FlareDefine.AppFlags.DebugMode,
			"PageInlineStyle": FlareDefine.GetPageInlineStyle(),
			"PageAppearance":  FlareDefine.GetAppBodyStyle(),
			"SettingsURI":     FlareDefine.RegularPages.Settings.Path,

			"PageName": "Theme",
			// 当前选择主题
			"SettingPages": FlareDefine.SettingPages,
			// "Themes":       themes.Themes,
			"Themes": themes,

			"OptionTitle": options.Title,
		},
	)
}
