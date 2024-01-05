package search

import (
	"net/http"

	"github.com/gin-gonic/gin"

	FlareData "github.com/soulteary/flare/config/data"
	FlareState "github.com/soulteary/flare/config/state"
	FlareAuth "github.com/soulteary/flare/internal/auth"
)

func RegisterRouting(router *gin.Engine) {

	router.GET(FlareState.SettingPages.Search.Path, FlareAuth.AuthRequired, pageSearch)
	router.POST(FlareState.SettingPages.Search.Path, FlareAuth.AuthRequired, updateSearchOptions)

}

func updateSearchOptions(c *gin.Context) {

	type UpdateBody struct {
		ShowSearchComponent     bool `form:"show-search-component"`
		DisabledSearchAutoFocus bool `form:"disabled-search-auto-focus"`
	}

	var body UpdateBody
	if c.ShouldBind(&body) != nil {
		c.PureJSON(http.StatusForbidden, "提交数据缺失")
		return
	}

	FlareData.UpdateSearch(body.ShowSearchComponent, body.DisabledSearchAutoFocus)

	pageSearch(c)
}

func pageSearch(c *gin.Context) {
	options := FlareData.GetAllSettingsOptions()

	c.HTML(
		http.StatusOK,
		"settings.html",
		gin.H{
			"DebugMode":       FlareState.AppFlags.DebugMode,
			"PageInlineStyle": FlareState.GetPageInlineStyle(),

			"PageName":       "Search",
			"PageAppearance": FlareState.GetAppBodyStyle(),
			"SettingPages":   FlareState.SettingPages,
			"SettingsURI":    FlareState.RegularPages.Settings.Path,

			"ShowSearchComponent":     options.ShowSearchComponent,
			"DisabledSearchAutoFocus": options.DisabledSearchAutoFocus,

			"OptionTitle": options.Title,
		},
	)
}
