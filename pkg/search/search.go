package search

import (
	"net/http"

	"github.com/gin-gonic/gin"

	FlareData "github.com/soulteary/flare/data"
	FlareAuth "github.com/soulteary/flare/pkg/auth"
	FlareState "github.com/soulteary/flare/state"
)

func RegisterRouting(router *gin.Engine) {

	router.GET(FlareState.SettingPages.Search.Path, FlareAuth.AuthRequired, pageHome)
	router.POST(FlareState.SettingPages.Search.Path, FlareAuth.AuthRequired, updateSearchOptions)

}

func pageHome(c *gin.Context) {

	render(c, "")

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

	render(c, "")
}

func render(c *gin.Context, testResult string) {
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
