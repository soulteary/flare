package search

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/pool"
)

func RegisterRouting(e *echo.Echo) {
	e.GET(define.SettingPages.Search.Path, pageSearch, auth.AuthRequired)
	e.POST(define.SettingPages.Search.Path, updateSearchOptions, auth.AuthRequired)
}

func updateSearchOptions(c *echo.Context) error {
	var body struct {
		ShowSearchComponent     bool `form:"show-search-component"`
		DisabledSearchAutoFocus bool `form:"disabled-search-auto-focus"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusForbidden, "提交数据缺失")
	}
	data.UpdateSearch(body.ShowSearchComponent, body.DisabledSearchAutoFocus)
	return pageSearch(c)
}

func pageSearch(c *echo.Context) error {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.String(http.StatusInternalServerError, "config error")
	}
	locale := options.Locale
	if locale == "" {
		locale = "zh"
	}
	m := pool.GetTemplateMap()
	defer pool.PutTemplateMap(m)
	m["Locale"] = locale
	m["DebugMode"] = define.AppFlags.DebugMode
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	m["PageName"] = "Search"
	m["PageAppearance"] = define.GetAppBodyStyle()
	m["SettingPages"] = define.SettingPages
	m["SettingsURI"] = define.RegularPages.Settings.Path
	m["ShowSearchComponent"] = options.ShowSearchComponent
	m["DisabledSearchAutoFocus"] = options.DisabledSearchAutoFocus
	m["OptionTitle"] = options.Title
	return c.Render(http.StatusOK, "settings.html", m)
}
