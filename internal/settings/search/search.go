package search

import (
	"net/http"

	"github.com/labstack/echo/v5"

	FlareData "github.com/soulteary/flare/config/data"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareAuth "github.com/soulteary/flare/internal/auth"
	FlarePool "github.com/soulteary/flare/internal/pool"
)

func RegisterRouting(e *echo.Echo) {
	e.GET(FlareDefine.SettingPages.Search.Path, pageSearch, FlareAuth.AuthRequired)
	e.POST(FlareDefine.SettingPages.Search.Path, updateSearchOptions, FlareAuth.AuthRequired)
}

func updateSearchOptions(c *echo.Context) error {
	var body struct {
		ShowSearchComponent     bool `form:"show-search-component"`
		DisabledSearchAutoFocus bool `form:"disabled-search-auto-focus"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusForbidden, "提交数据缺失")
	}
	FlareData.UpdateSearch(body.ShowSearchComponent, body.DisabledSearchAutoFocus)
	return pageSearch(c)
}

func pageSearch(c *echo.Context) error {
	options := FlareData.GetAllSettingsOptions()
	m := FlarePool.GetTemplateMap()
	defer FlarePool.PutTemplateMap(m)
	m["DebugMode"] = FlareDefine.AppFlags.DebugMode
	m["PageInlineStyle"] = FlareDefine.GetPageInlineStyle()
	m["PageName"] = "Search"
	m["PageAppearance"] = FlareDefine.GetAppBodyStyle()
	m["SettingPages"] = FlareDefine.SettingPages
	m["SettingsURI"] = FlareDefine.RegularPages.Settings.Path
	m["ShowSearchComponent"] = options.ShowSearchComponent
	m["DisabledSearchAutoFocus"] = options.DisabledSearchAutoFocus
	m["OptionTitle"] = options.Title
	return c.Render(http.StatusOK, "settings.html", m)
}
