package home

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	FlareData "github.com/soulteary/flare/config/data"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareModel "github.com/soulteary/flare/config/model"
	FlareAuth "github.com/soulteary/flare/internal/auth"
	FlareFn "github.com/soulteary/flare/internal/fn"
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

func RegisterRouting(router *gin.Engine) {

	if FlareDefine.AppFlags.Visibility != "PRIVATE" {
		router.GET(FlareDefine.RegularPages.Home.Path, pageHome)
		router.GET(FlareDefine.RegularPages.Help.Path, renderHelp)
		router.POST(FlareDefine.RegularPages.Home.Path, pageSearch)

		router.GET(FlareDefine.RegularPages.Applications.Path, pageApplication)
		router.GET(FlareDefine.RegularPages.Bookmarks.Path, pageBookmark)
	} else {
		router.GET(FlareDefine.RegularPages.Home.Path, FlareAuth.AuthRequired, pageHome)
		router.GET(FlareDefine.RegularPages.Help.Path, FlareAuth.AuthRequired, renderHelp)
		router.POST(FlareDefine.RegularPages.Home.Path, FlareAuth.AuthRequired, pageSearch)

		router.GET(FlareDefine.RegularPages.Applications.Path, FlareAuth.AuthRequired, pageApplication)
		router.GET(FlareDefine.RegularPages.Bookmarks.Path, FlareAuth.AuthRequired, pageBookmark)
	}
}

func pageHome(c *gin.Context) {
	render(c, "")
}

func renderHelp(c *gin.Context) {
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
		"星期日",
		"星期一",
		"星期二",
		"星期三",
		"星期四",
		"星期五",
		"星期六",
	}

	if !FlareDefine.AppFlags.DisableCSP {
		c.Header("Content-Security-Policy", "script-src 'none'; object-src 'none'; base-uri 'none'; require-trusted-types-for 'script'; report-uri 'none';")
	}

	c.HTML(
		http.StatusOK,
		"home.html",
		gin.H{
			"PageName":       "Home",
			"PageAppearance": FlareDefine.GetAppBodyStyle(),
			"SettingPages":   FlareDefine.SettingPages,

			"DebugMode":       FlareDefine.AppFlags.DebugMode,
			"PageInlineStyle": FlareDefine.GetPageInlineStyle(),

			"ShowWeatherModule": !FlareDefine.AppFlags.EnableOfflineMode && configWeatherShow,
			"Location":          options.Location,
			"WeatherData":       weatherData,
			"WeatherIcon":       weather.GetSVGCodeByName(weatherData.ConditionCode),

			"HeroDate":  now.Format("2006年01月02日"),
			"HeroTime":  now.Format("15:04:05"),
			"HeroDay":   fmt.Sprintf(`%s`, days[now.Weekday()]),
			"Greetings": "帮助",

			"BookmarksURI":    FlareDefine.RegularPages.Bookmarks.Path,
			"ApplicationsURI": FlareDefine.RegularPages.Applications.Path,
			"SettingsURI":     FlareDefine.RegularPages.Settings.Path,
			"Applications":    GenerateHelpTemplate(),
			"SearchKeyword":   template.HTML(" "),
			"HasKeyword":      false,

			// SearchProvider          string // 默认的搜索引擎
			"ShowSearchComponent":     options.ShowSearchComponent,
			"DisabledSearchAutoFocus": true,

			"OptionTitle":              options.Title,
			"OptionFooter":             template.HTML(options.Footer),
			"OptionOpenAppNewTab":      options.OpenAppNewTab,
			"OptionOpenBookmarkNewTab": options.OpenBookmarkNewTab,
			"OptionShowTitle":          options.ShowTitle,
			"OptionShowDateTime":       options.ShowDateTime,
			// help 界面强制展示 Apps 模块，隐藏书签模块
			"OptionShowApps":           true,
			"OptionShowBookmarks":      false,
			"OptionHideSettingsButton": options.HideSettingsButton,
			"OptionHideHelpButton":     options.HideHelpButton,
		},
	)
}

func pageSearch(c *gin.Context) {

	type UpdateBody struct {
		Search string `form:"search"`
	}

	var body UpdateBody
	if c.ShouldBind(&body) != nil {
		render(c, "")
		return
	}

	search := strings.TrimSpace(body.Search)
	if len(search) > 50 {
		render(c, "")
		return
	}

	render(c, search)
}

var _CACHE_WEATHER_DATA FlareModel.Weather

func GetWeatherData() (data FlareModel.Weather) {
	location, weatherShow := FlareData.GetLocationAndWeatherShow()
	if location != "" && weatherShow {
		updateWeatherData(location)
	}
	return _CACHE_WEATHER_DATA
}

// 每五分钟更新一次数据
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

	// 单一词语模式
	if count == 1 {
		if len(words[0]) > 0 {
			return words[0]
		}
		return defaultWord
	}

	hour, _, _ := time.Now().Clock()
	// 早晨
	if hour >= 5 && hour <= 10 {
		if len(words[0]) > 0 {
			return words[0]
		}
	}
	// 中午
	if hour >= 11 && hour <= 13 {
		if len(words[1]) > 0 {
			return words[1]
		}
	}
	// 下午
	if hour >= 14 && hour <= 18 {
		if len(words[2]) > 0 {
			return words[2]
		}
	}
	// 晚上
	if len(words[3]) > 0 {
		return words[3]
	}

	return defaultWord
}

func pageBookmark(c *gin.Context) {
	options := FlareData.GetAllSettingsOptions()
	FlareFn.ParseRequestURL(c.Request)

	c.HTML(
		http.StatusOK,
		"home.html",
		gin.H{
			"DebugMode":       FlareDefine.AppFlags.DebugMode,
			"PageInlineStyle": FlareDefine.GetPageInlineStyle(),

			"PageName": "书签",
			"SubPage":  true,

			"PageAppearance": FlareDefine.GetAppBodyStyle(),
			"SettingPages":   FlareDefine.SettingPages,

			"BookmarksURI":    FlareDefine.RegularPages.Bookmarks.Path,
			"ApplicationsURI": FlareDefine.RegularPages.Applications.Path,
			"SettingsURI":     FlareDefine.RegularPages.Settings.Path,

			"Bookmarks": GenerateBookmarkTemplate(""),

			"OptionTitle":              options.Title,
			"OptionOpenBookmarkNewTab": options.OpenBookmarkNewTab,
			"OptionShowBookmarks":      options.ShowBookmarks,
			"OptionHideSettingsButton": options.HideSettingsButton,
			"OptionHideHelpButton":     options.HideHelpButton,
		},
	)
}

func pageApplication(c *gin.Context) {
	options := FlareData.GetAllSettingsOptions()
	FlareFn.ParseRequestURL(c.Request)

	c.HTML(
		http.StatusOK,
		"home.html",
		gin.H{
			"DebugMode":       FlareDefine.AppFlags.DebugMode,
			"PageInlineStyle": FlareDefine.GetPageInlineStyle(),

			"BookmarksURI":    FlareDefine.RegularPages.Bookmarks.Path,
			"ApplicationsURI": FlareDefine.RegularPages.Applications.Path,
			"SettingsURI":     FlareDefine.RegularPages.Settings.Path,
			"Applications":    GenerateApplicationsTemplate(""),

			"PageName":       "应用",
			"SubPage":        true,
			"PageAppearance": FlareDefine.GetAppBodyStyle(),

			// "SettingPages": FlareState.SettingPages,

			"OptionTitle":              options.Title,
			"OptionOpenAppNewTab":      options.OpenAppNewTab,
			"OptionShowApps":           options.ShowApps,
			"OptionHideSettingsButton": options.HideSettingsButton,
			"OptionHideHelpButton":     options.HideHelpButton,
		},
	)
}

func render(c *gin.Context, filter string) {
	options := FlareData.GetAllSettingsOptions()
	FlareFn.ParseRequestURL(c.Request)

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
		"星期日",
		"星期一",
		"星期二",
		"星期三",
		"星期四",
		"星期五",
		"星期六",
	}

	if !FlareDefine.AppFlags.DisableCSP {
		c.Header("Content-Security-Policy", "script-src 'none'; object-src 'none'; base-uri 'none'; require-trusted-types-for 'script'; report-uri 'none';")
	}

	bodyClassName := ""
	if !options.KeepLetterCase {
		bodyClassName += "app-content-uppercase "
	}

	c.HTML(
		http.StatusOK,
		"home.html",
		gin.H{
			"PageName":       "Home",
			"PageAppearance": FlareDefine.GetAppBodyStyle(),
			"SettingPages":   FlareDefine.SettingPages,

			"DebugMode":       FlareDefine.AppFlags.DebugMode,
			"PageInlineStyle": FlareDefine.GetPageInlineStyle(),

			"ShowWeatherModule": !FlareDefine.AppFlags.EnableOfflineMode && configWeatherShow,
			"Location":          options.Location,
			"WeatherData":       weatherData,
			"WeatherIcon":       weather.GetSVGCodeByName(weatherData.ConditionCode),

			"HeroDate":  now.Format("2006年01月02日"),
			"HeroTime":  now.Format("15:04:05"),
			"HeroDay":   fmt.Sprintf(`%s`, days[now.Weekday()]),
			"Greetings": getGreeting(options.Greetings),

			"BookmarksURI":    FlareDefine.RegularPages.Bookmarks.Path,
			"ApplicationsURI": FlareDefine.RegularPages.Applications.Path,
			"SettingsURI":     FlareDefine.RegularPages.Settings.Path,
			"Applications":    GenerateApplicationsTemplate(filter),
			"Bookmarks":       GenerateBookmarkTemplate(filter),
			"SearchKeyword":   template.HTML(searchKeyword),
			"HasKeyword":      hasKeyword,

			// SearchProvider          string // 默认的搜索引擎
			"ShowSearchComponent":     options.ShowSearchComponent,
			"DisabledSearchAutoFocus": options.DisabledSearchAutoFocus,

			"OptionTitle":              options.Title,
			"OptionFooter":             template.HTML(options.Footer),
			"OptionOpenAppNewTab":      options.OpenAppNewTab,
			"OptionOpenBookmarkNewTab": options.OpenBookmarkNewTab,
			"OptionShowTitle":          options.ShowTitle,
			"OptionShowDateTime":       options.ShowDateTime,
			"OptionShowApps":           options.ShowApps,
			"OptionShowBookmarks":      options.ShowBookmarks,
			"OptionHideSettingsButton": options.HideSettingsButton,
			"OptionHideHelpButton":     options.HideHelpButton,
			"BodyClassName":            template.HTMLAttr(bodyClassName),
		},
	)
}
