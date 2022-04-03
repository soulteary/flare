package theme

import (
	"net/http"

	"github.com/gin-gonic/gin"

	FlareData "github.com/soulteary/flare/data"
	FlareAuth "github.com/soulteary/flare/pkg/auth"
	FlareState "github.com/soulteary/flare/state"
)

func RegisterRouting(router *gin.Engine) {
	router.GET(FlareState.SettingPages.Theme.Path, FlareAuth.AuthRequired, pageTheme)
	router.POST(FlareState.SettingPages.Theme.Path, FlareAuth.AuthRequired, updateThemes)
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
	FlareState.UpdatePagePalettes()

	// 中转变量
	FlareState.ThemeCurrent = body.Theme
	FlareState.ThemePrimaryColor = FlareState.GetThemePrimaryColor(body.Theme)

	pageTheme(c)
}

func pageTheme(c *gin.Context) {
	// themes := getThemePalettes()
	themes := FlareState.ThemePalettes
	options := FlareData.GetAllSettingsOptions()

	c.HTML(
		http.StatusOK,
		"settings.html",
		gin.H{
			"DebugMode":       FlareState.AppFlags.DebugMode,
			"PageInlineStyle": FlareState.GetPageInlineStyle(),
			"PageAppearance":  FlareState.GetAppBodyStyle(),
			"SettingsURI":     FlareState.RegularPages.Settings.Path,

			"PageName": "Theme",
			// 当前选择主题
			"SettingPages": FlareState.SettingPages,
			// "Themes":       themes.Themes,
			"Themes": themes,

			"OptionTitle": options.Title,
		},
	)
}
