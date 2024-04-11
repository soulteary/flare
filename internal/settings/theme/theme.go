package theme

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"

	FlareData "github.com/soulteary/flare/config/data"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareModel "github.com/soulteary/flare/config/model"
	FlareAuth "github.com/soulteary/flare/internal/auth"
	FlareFn "github.com/soulteary/flare/internal/fn"
)

func RegisterRouting(router *gin.Engine) {
	router.GET(FlareDefine.SettingPages.Theme.Path, FlareAuth.AuthRequired, pageTheme)
	router.POST(FlareDefine.SettingPages.Theme.Path, FlareAuth.AuthRequired, updateThemes)
}

func updateThemes(c *gin.Context) {
	// only allow to change theme when custom theme is not set
	if FlareDefine.AppFlags.CustomTheme == "" {
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
	}
	pageTheme(c)
}

func pageTheme(c *gin.Context) {
	// themes := getThemePalettes()
	themes := FlareDefine.ThemePalettes
	options := FlareData.GetAllSettingsOptions()

	themeLocked := false
	themeLockedInEmbededTheme := false
	var themeSelected FlareModel.Theme

	if FlareDefine.AppFlags.CustomTheme != "" {
		themeLocked = true
		for _, theme := range themes {
			if theme.Name == FlareDefine.AppFlags.CustomTheme {
				themeLockedInEmbededTheme = true
				themeSelected = theme
				break
			}
		}
	} else {
		for _, theme := range themes {
			if theme.Name == FlareDefine.ThemeCurrent {
				themeSelected = theme
				break
			}
		}
	}

	customThemes := FlareFn.GetAllCustomThemes()
	customThemeAlived := len(customThemes) > 0
	fmt.Println(customThemes)

	c.HTML(
		http.StatusOK,
		"settings.html",
		gin.H{
			"DebugMode":       FlareDefine.AppFlags.DebugMode,
			"PageInlineStyle": FlareDefine.GetPageInlineStyle(),
			"PageAppearance":  FlareDefine.GetAppBodyStyle(),
			"SettingsURI":     FlareDefine.RegularPages.Settings.Path,

			"PageName":     "Theme",
			"SettingPages": FlareDefine.SettingPages,
			// "Themes":       themes.Themes,
			"Themes": themes,
			// 当前选择主题
			"ThemeSelected": themeSelected,

			"OptionTitle": options.Title,

			// 自定义主题
			"CustomThemeName":   FlareDefine.AppFlags.CustomTheme,
			"CustomThemes":      customThemes,
			"CustomThemeAlived": customThemeAlived,

			// 主题锁定
			"ThemeLocked": themeLocked,
			// 主题锁定在内置主题
			"ThemeLockedInEmbededTheme": themeLockedInEmbededTheme,
		},
	)
}
