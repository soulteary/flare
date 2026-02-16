package home

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	FlareData "github.com/soulteary/flare/config/data"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareModel "github.com/soulteary/flare/config/model"
	FlareAuth "github.com/soulteary/flare/internal/auth"
	FlareFn "github.com/soulteary/flare/internal/fn"
	FlarePool "github.com/soulteary/flare/internal/pool"
	FlareWeather "github.com/soulteary/flare/internal/settings/weather"
	weather "github.com/soulteary/funny-china-weather"
)

func init() {
	if FlareDefine.AppFlags.EnableOfflineMode {
		return
	}

	data := FlareData.GetAllSettingsOptions()

	if data.Location == "" && data.ShowWeather {
		log.Println("天气模块启用，当前应用尚未配置区域，尝试自动获取区域名称。")
		location, _ := weather.GetMyIPLocation()
		FlareData.UpdateWeatherAndLocation(data.ShowWeather, location)
	} else {
		FlareData.UpdateWeatherAndLocation(data.ShowWeather, data.Location)
	}

}

func RegisterRouting(e *echo.Echo) {
	if FlareDefine.AppFlags.Visibility != "PRIVATE" {
		e.GET(FlareDefine.RegularPages.Home.Path, pageHome)
		e.GET(FlareDefine.RegularPages.Help.Path, renderHelp)
		e.POST(FlareDefine.RegularPages.Home.Path, pageSearch)
		e.GET(FlareDefine.RegularPages.Applications.Path, pageApplication)
		e.GET(FlareDefine.RegularPages.Bookmarks.Path, pageBookmark)
	} else {
		e.GET(FlareDefine.RegularPages.Home.Path, pageHome, FlareAuth.AuthRequired)
		e.GET(FlareDefine.RegularPages.Help.Path, renderHelp, FlareAuth.AuthRequired)
		e.POST(FlareDefine.RegularPages.Home.Path, pageSearch, FlareAuth.AuthRequired)
		e.GET(FlareDefine.RegularPages.Applications.Path, pageApplication, FlareAuth.AuthRequired)
		e.GET(FlareDefine.RegularPages.Bookmarks.Path, pageBookmark, FlareAuth.AuthRequired)
	}
}

func pageHome(c *echo.Context) error {
	return render(c, "")
}

func renderHelp(c *echo.Context) error {
	options := FlareData.GetAllSettingsOptions()
	now := time.Now()
	configWeatherShow := true
	var weatherData FlareModel.Weather
	if !FlareDefine.AppFlags.EnableOfflineMode {
		_, weatherShow := FlareData.GetLocationAndWeatherShow()
		if weatherShow {
			weatherData = GetWeatherData()
		} else {
			configWeatherShow = weatherShow
		}
	}
	var days = [...]string{
		"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六",
	}
	if !FlareDefine.AppFlags.DisableCSP {
		c.Response().Header().Set("Content-Security-Policy", "script-src 'none'; object-src 'none'; base-uri 'none'; require-trusted-types-for 'script'; report-uri 'none';")
	}
	m := FlarePool.GetTemplateMap()
	defer FlarePool.PutTemplateMap(m)
	m["PageName"] = "Home"
	m["PageAppearance"] = FlareDefine.GetAppBodyStyle()
	m["SettingPages"] = FlareDefine.SettingPages
	m["DebugMode"] = FlareDefine.AppFlags.DebugMode
	m["PageInlineStyle"] = FlareDefine.GetPageInlineStyle()
	m["ShowWeatherModule"] = !FlareDefine.AppFlags.EnableOfflineMode && configWeatherShow
	m["Location"] = options.Location
	m["WeatherData"] = weatherData
	m["WeatherIcon"] = weather.GetSVGCodeByName(weatherData.ConditionCode)
	m["HeroDate"] = now.Format("2006年01月02日")
	m["HeroTime"] = now.Format("15:04:05")
	m["HeroDay"] = fmt.Sprintf(`%s`, days[now.Weekday()])
	m["Greetings"] = "帮助"
	m["BookmarksURI"] = FlareDefine.RegularPages.Bookmarks.Path
	m["ApplicationsURI"] = FlareDefine.RegularPages.Applications.Path
	m["SettingsURI"] = FlareDefine.RegularPages.Settings.Path
	m["Applications"] = GenerateHelpTemplate()
	m["SearchKeyword"] = template.HTML(" ")
	m["HasKeyword"] = false
	m["ShowSearchComponent"] = options.ShowSearchComponent
	m["DisabledSearchAutoFocus"] = true
	m["OptionTitle"] = options.Title
	m["OptionFooter"] = template.HTML(options.Footer)
	m["OptionOpenAppNewTab"] = options.OpenAppNewTab
	m["OptionOpenBookmarkNewTab"] = options.OpenBookmarkNewTab
	m["OptionShowTitle"] = options.ShowTitle
	m["OptionShowDateTime"] = options.ShowDateTime
	m["OptionShowApps"] = true
	m["OptionShowBookmarks"] = false
	m["OptionHideSettingsButton"] = options.HideSettingsButton
	m["OptionHideHelpButton"] = options.HideHelpButton
	return c.Render(http.StatusOK, "home.html", m)
}

func pageSearch(c *echo.Context) error {
	var body struct {
		Search string `form:"search"`
	}
	if err := c.Bind(&body); err != nil {
		return render(c, "")
	}
	search := strings.TrimSpace(body.Search)
	if len(search) > 50 {
		return render(c, "")
	}
	return render(c, search)
}

var _CACHE_WEATHER_DATA FlareModel.Weather

func GetWeatherData() (data FlareModel.Weather) {
	location, weatherShow := FlareData.GetLocationAndWeatherShow()
	if location != "" && weatherShow {
		updateWeatherData(location)
	}
	return _CACHE_WEATHER_DATA
}

func updateWeatherData(location string) {
	timestamp := time.Now().Unix()
	if (_CACHE_WEATHER_DATA.Expires < timestamp) || (location != _CACHE_WEATHER_DATA.Location) {
		data, _, err := FlareWeather.GetWeatherInfo(location)
		if err == nil {
			_CACHE_WEATHER_DATA.ConditionCode = data.ConditionCode
			_CACHE_WEATHER_DATA.ConditionText = data.ConditionText
			_CACHE_WEATHER_DATA.Degree = data.Degree
			_CACHE_WEATHER_DATA.ExternalLastUpdate = data.ExternalLastUpdate
			_CACHE_WEATHER_DATA.Humidity = data.Humidity
			_CACHE_WEATHER_DATA.IsDay = data.IsDay
			_CACHE_WEATHER_DATA.Expires = data.Expires
			_CACHE_WEATHER_DATA.Location = location
		}
	}
}

func getGreeting(greeting string) string {
	words := strings.Split(greeting, ";")
	count := len(words)
	defaultWord := "你好"
	if count == 1 {
		if len(words[0]) > 0 {
			return words[0]
		}
		return defaultWord
	}
	hour, _, _ := time.Now().Clock()
	if hour >= 5 && hour <= 10 && len(words[0]) > 0 {
		return words[0]
	}
	if hour >= 11 && hour <= 13 && len(words[1]) > 0 {
		return words[1]
	}
	if hour >= 14 && hour <= 18 && len(words[2]) > 0 {
		return words[2]
	}
	if len(words) > 3 && len(words[3]) > 0 {
		return words[3]
	}
	return defaultWord
}

func pageBookmark(c *echo.Context) error {
	options := FlareData.GetAllSettingsOptions()
	FlareFn.ParseRequestURL(c.Request())
	m := FlarePool.GetTemplateMap()
	defer FlarePool.PutTemplateMap(m)
	m["DebugMode"] = FlareDefine.AppFlags.DebugMode
	m["PageInlineStyle"] = FlareDefine.GetPageInlineStyle()
	m["PageName"] = "书签"
	m["SubPage"] = true
	m["PageAppearance"] = FlareDefine.GetAppBodyStyle()
	m["SettingPages"] = FlareDefine.SettingPages
	m["BookmarksURI"] = FlareDefine.RegularPages.Bookmarks.Path
	m["ApplicationsURI"] = FlareDefine.RegularPages.Applications.Path
	m["SettingsURI"] = FlareDefine.RegularPages.Settings.Path
	m["Bookmarks"] = GenerateBookmarkTemplate("", &options)
	m["OptionTitle"] = options.Title
	m["OptionOpenBookmarkNewTab"] = options.OpenBookmarkNewTab
	m["OptionShowBookmarks"] = options.ShowBookmarks
	m["OptionHideSettingsButton"] = options.HideSettingsButton
	m["OptionHideHelpButton"] = options.HideHelpButton
	return c.Render(http.StatusOK, "home.html", m)
}

func pageApplication(c *echo.Context) error {
	options := FlareData.GetAllSettingsOptions()
	FlareFn.ParseRequestURL(c.Request())
	m := FlarePool.GetTemplateMap()
	defer FlarePool.PutTemplateMap(m)
	m["DebugMode"] = FlareDefine.AppFlags.DebugMode
	m["PageInlineStyle"] = FlareDefine.GetPageInlineStyle()
	m["BookmarksURI"] = FlareDefine.RegularPages.Bookmarks.Path
	m["ApplicationsURI"] = FlareDefine.RegularPages.Applications.Path
	m["SettingsURI"] = FlareDefine.RegularPages.Settings.Path
	m["Applications"] = GenerateApplicationsTemplate("", &options)
	m["PageName"] = "应用"
	m["SubPage"] = true
	m["PageAppearance"] = FlareDefine.GetAppBodyStyle()
	m["OptionTitle"] = options.Title
	m["OptionOpenAppNewTab"] = options.OpenAppNewTab
	m["OptionShowApps"] = options.ShowApps
	m["OptionHideSettingsButton"] = options.HideSettingsButton
	m["OptionHideHelpButton"] = options.HideHelpButton
	return c.Render(http.StatusOK, "home.html", m)
}

func render(c *echo.Context, filter string) error {
	options := FlareData.GetAllSettingsOptions()
	FlareFn.ParseRequestURL(c.Request())
	hasKeyword := false
	searchKeyword := " "
	if filter != "" {
		searchKeyword = "搜索结果: " + filter
		hasKeyword = true
	}
	now := time.Now()
	configWeatherShow := true
	var weatherData FlareModel.Weather
	if !FlareDefine.AppFlags.EnableOfflineMode {
		_, weatherShow := FlareData.GetLocationAndWeatherShow()
		if weatherShow {
			weatherData = GetWeatherData()
		} else {
			configWeatherShow = weatherShow
		}
	}
	var days = [...]string{
		"星期日", "星期一", "星期二", "星期三", "星期四", "星期五", "星期六",
	}
	if !FlareDefine.AppFlags.DisableCSP {
		c.Response().Header().Set("Content-Security-Policy", "script-src 'none'; object-src 'none'; base-uri 'none'; require-trusted-types-for 'script'; report-uri 'none';")
	}
	bodyClassName := ""
	if !options.KeepLetterCase {
		bodyClassName += "app-content-uppercase "
	}
	m := FlarePool.GetTemplateMap()
	defer FlarePool.PutTemplateMap(m)
	m["PageName"] = "Home"
	m["PageAppearance"] = FlareDefine.GetAppBodyStyle()
	m["SettingPages"] = FlareDefine.SettingPages
	m["DebugMode"] = FlareDefine.AppFlags.DebugMode
	m["PageInlineStyle"] = FlareDefine.GetPageInlineStyle()
	m["ShowWeatherModule"] = !FlareDefine.AppFlags.EnableOfflineMode && configWeatherShow
	m["Location"] = options.Location
	m["WeatherData"] = weatherData
	m["WeatherIcon"] = weather.GetSVGCodeByName(weatherData.ConditionCode)
	m["HeroDate"] = now.Format("2006年01月02日")
	m["HeroTime"] = now.Format("15:04:05")
	m["HeroDay"] = fmt.Sprintf(`%s`, days[now.Weekday()])
	m["Greetings"] = getGreeting(options.Greetings)
	m["BookmarksURI"] = FlareDefine.RegularPages.Bookmarks.Path
	m["ApplicationsURI"] = FlareDefine.RegularPages.Applications.Path
	m["SettingsURI"] = FlareDefine.RegularPages.Settings.Path
	m["Applications"] = GenerateApplicationsTemplate(filter, &options)
	m["Bookmarks"] = GenerateBookmarkTemplate(filter, &options)
	m["SearchKeyword"] = template.HTML(searchKeyword)
	m["HasKeyword"] = hasKeyword
	m["ShowSearchComponent"] = options.ShowSearchComponent
	m["DisabledSearchAutoFocus"] = options.DisabledSearchAutoFocus
	m["OptionTitle"] = options.Title
	m["OptionFooter"] = template.HTML(options.Footer)
	m["OptionOpenAppNewTab"] = options.OpenAppNewTab
	m["OptionOpenBookmarkNewTab"] = options.OpenBookmarkNewTab
	m["OptionShowTitle"] = options.ShowTitle
	m["OptionShowDateTime"] = options.ShowDateTime
	m["OptionShowApps"] = options.ShowApps
	m["OptionShowBookmarks"] = options.ShowBookmarks
	m["OptionHideSettingsButton"] = options.HideSettingsButton
	m["OptionHideHelpButton"] = options.HideHelpButton
	m["BodyClassName"] = template.HTMLAttr(bodyClassName)
	return c.Render(http.StatusOK, "home.html", m)
}
