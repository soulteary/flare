package home

import (
	"crypto/tls"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/clock"
	"github.com/soulteary/flare/internal/fn"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/pool"
	"github.com/soulteary/flare/internal/settings/weather"
	weatherip "github.com/soulteary/funny-china-weather"
)

const _weatherLocationDetectTimeout = 5 * time.Second

const _cspValue = "script-src 'none'; object-src 'none'; base-uri 'none'; require-trusted-types-for 'script'; report-uri 'none';"

func setCSPHeader(c fiber.Ctx) {
	if !define.AppFlags.DisableCSP {
		c.Set("Content-Security-Policy", _cspValue)
	}
}

// InitWeatherIfNeeded loads settings and updates weather/location (e.g. auto-detect location). Call from server startup instead of relying on init().
// GetMyIPLocation is called with a timeout to avoid blocking startup when the network is slow.
func InitWeatherIfNeeded() {
	if define.AppFlags.EnableOfflineMode {
		return
	}
	opts, err := data.GetAllSettingsOptions()
	if err != nil {
		return
	}
	if opts.Location == "" && opts.ShowWeather {
		log.Println("天气模块启用，当前应用尚未配置区域，尝试自动获取区域名称。")
		location := getMyIPLocationWithTimeout(_weatherLocationDetectTimeout)
		if location != "" {
			data.UpdateWeatherAndLocation(opts.ShowWeather, location)
		} else {
			data.UpdateWeatherAndLocation(opts.ShowWeather, opts.Location)
		}
	} else {
		data.UpdateWeatherAndLocation(opts.ShowWeather, opts.Location)
	}
}

// getMyIPLocationWithTimeout runs GetMyIPLocation in a goroutine and returns the result or empty string on timeout/error.
func getMyIPLocationWithTimeout(timeout time.Duration) string {
	type result struct {
		location string
		err      error
	}
	done := make(chan result, 1)
	go func() {
		location, err := weatherip.GetMyIPLocation()
		done <- result{location: location, err: err}
	}()
	select {
	case r := <-done:
		if r.err == nil && r.location != "" {
			return r.location
		}
		return ""
	case <-time.After(timeout):
		log.Println("自动获取区域名称超时，将使用默认配置。")
		return ""
	}
}

func RegisterRouting(e *fiber.App) {
	if define.AppFlags.Visibility != "PRIVATE" {
		e.Get(define.RegularPages.Home.Path, pageHome)
		e.Get(define.RegularPages.Help.Path, renderHelp)
		e.Post(define.RegularPages.Home.Path, pageSearch)
		e.Get(define.RegularPages.Applications.Path, pageApplication)
		e.Get(define.RegularPages.Bookmarks.Path, pageBookmark)
	} else {
		e.Get(define.RegularPages.Home.Path, auth.AuthRequired, pageHome)
		e.Get(define.RegularPages.Help.Path, auth.AuthRequired, renderHelp)
		e.Post(define.RegularPages.Home.Path, auth.AuthRequired, pageSearch)
		e.Get(define.RegularPages.Applications.Path, auth.AuthRequired, pageApplication)
		e.Get(define.RegularPages.Bookmarks.Path, auth.AuthRequired, pageBookmark)
	}
}

func pageHome(c fiber.Ctx) error {
	return render(c, "")
}

func renderHelp(c fiber.Ctx) error {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	configWeatherShow := true
	var weatherData model.Weather
	if !define.AppFlags.EnableOfflineMode {
		_, weatherShow := data.GetLocationAndWeatherShow()
		if weatherShow {
			weatherData = GetWeatherData()
		} else {
			configWeatherShow = weatherShow
		}
	}
	locale := options.Locale
	if locale == "" {
		locale = "zh"
	}
	setCSPHeader(c)
	m := pool.GetTemplateMap()
	defer pool.PutTemplateMap(m)
	m["Locale"] = locale
	m["PageName"] = "Home"
	m["PageAppearance"] = define.GetAppBodyStyle()
	m["SettingPages"] = define.SettingPages
	m["DebugMode"] = define.AppFlags.DebugMode
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	m["ShowWeatherModule"] = !define.AppFlags.EnableOfflineMode && configWeatherShow
	m["Location"] = options.Location
	m["WeatherData"] = weatherData
	m["WeatherIcon"] = weatherip.GetSVGCodeByName(weatherData.ConditionCode)
	m["Greetings"] = i18n.T(locale, "page_help")
	m["BookmarksURI"] = define.RegularPages.Bookmarks.Path
	m["ApplicationsURI"] = define.RegularPages.Applications.Path
	m["SettingsURI"] = define.RegularPages.Settings.Path
	m["Applications"] = GenerateHelpTemplate()
	m["SearchKeyword"] = template.HTML(i18n.T(locale, "search_placeholder"))
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
	return renderClockPage(c, m, options, false)
}

func pageSearch(c fiber.Ctx) error {
	var body struct {
		Search string `form:"search"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return render(c, "")
	}
	search := strings.TrimSpace(body.Search)
	if len(search) > 50 {
		return render(c, "")
	}
	return render(c, search)
}

var _CACHE_WEATHER_DATA model.Weather

func GetWeatherData() (result model.Weather) {
	location, weatherShow := data.GetLocationAndWeatherShow()
	if location != "" && weatherShow {
		updateWeatherData(location)
	}
	return _CACHE_WEATHER_DATA
}

func updateWeatherData(location string) {
	timestamp := time.Now().Unix()
	if (_CACHE_WEATHER_DATA.Expires < timestamp) || (location != _CACHE_WEATHER_DATA.Location) {
		data, _, err := weather.GetWeatherInfo(location)
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

func pageBookmark(c fiber.Ctx) error {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	locale := options.Locale
	if locale == "" {
		locale = "zh"
	}
	parseRequestURL(c)
	m := pool.GetTemplateMap()
	defer pool.PutTemplateMap(m)
	m["Locale"] = locale
	m["DebugMode"] = define.AppFlags.DebugMode
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	m["PageName"] = i18n.T(locale, "page_bookmarks")
	m["SubPage"] = true
	m["PageAppearance"] = define.GetAppBodyStyle()
	m["SettingPages"] = define.SettingPages
	m["BookmarksURI"] = define.RegularPages.Bookmarks.Path
	m["ApplicationsURI"] = define.RegularPages.Applications.Path
	m["SettingsURI"] = define.RegularPages.Settings.Path
	m["Bookmarks"] = GenerateBookmarkTemplate("", &options)
	m["OptionTitle"] = options.Title
	m["OptionOpenBookmarkNewTab"] = options.OpenBookmarkNewTab
	m["OptionShowBookmarks"] = options.ShowBookmarks
	m["OptionHideSettingsButton"] = options.HideSettingsButton
	m["OptionHideHelpButton"] = options.HideHelpButton
	return c.Status(http.StatusOK).Render("home.html", m)
}

func pageApplication(c fiber.Ctx) error {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	locale := options.Locale
	if locale == "" {
		locale = "zh"
	}
	parseRequestURL(c)
	m := pool.GetTemplateMap()
	defer pool.PutTemplateMap(m)
	m["Locale"] = locale
	m["DebugMode"] = define.AppFlags.DebugMode
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	m["BookmarksURI"] = define.RegularPages.Bookmarks.Path
	m["ApplicationsURI"] = define.RegularPages.Applications.Path
	m["SettingsURI"] = define.RegularPages.Settings.Path
	m["Applications"] = GenerateApplicationsTemplate("", &options)
	m["PageName"] = i18n.T(locale, "page_apps")
	m["SubPage"] = true
	m["PageAppearance"] = define.GetAppBodyStyle()
	m["OptionTitle"] = options.Title
	m["OptionOpenAppNewTab"] = options.OpenAppNewTab
	m["OptionShowApps"] = options.ShowApps
	m["OptionHideSettingsButton"] = options.HideSettingsButton
	m["OptionHideHelpButton"] = options.HideHelpButton
	return c.Status(http.StatusOK).Render("home.html", m)
}

func render(c fiber.Ctx, filter string) error {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	parseRequestURL(c)
	locale := options.Locale
	if locale == "" {
		locale = "zh"
	}
	hasKeyword := false
	searchKeyword := i18n.T(locale, "search_placeholder")
	if filter != "" {
		searchKeyword = i18n.Tf(locale, "search_result", filter)
		hasKeyword = true
	}
	configWeatherShow := true
	var weatherData model.Weather
	if !define.AppFlags.EnableOfflineMode {
		_, weatherShow := data.GetLocationAndWeatherShow()
		if weatherShow {
			weatherData = GetWeatherData()
		} else {
			configWeatherShow = weatherShow
		}
	}
	setCSPHeader(c)
	bodyClassName := ""
	if !options.KeepLetterCase {
		bodyClassName += "app-content-uppercase "
	}
	m := pool.GetTemplateMap()
	defer pool.PutTemplateMap(m)
	m["Locale"] = locale
	m["PageName"] = "Home"
	m["PageAppearance"] = define.GetAppBodyStyle()
	m["SettingPages"] = define.SettingPages
	m["DebugMode"] = define.AppFlags.DebugMode
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	m["ShowWeatherModule"] = !define.AppFlags.EnableOfflineMode && configWeatherShow
	m["Location"] = options.Location
	m["WeatherData"] = weatherData
	m["WeatherIcon"] = weatherip.GetSVGCodeByName(weatherData.ConditionCode)
	m["BookmarksURI"] = define.RegularPages.Bookmarks.Path
	m["ApplicationsURI"] = define.RegularPages.Applications.Path
	m["SettingsURI"] = define.RegularPages.Settings.Path
	m["Applications"] = GenerateApplicationsTemplate(filter, &options)
	m["Bookmarks"] = GenerateBookmarkTemplate(filter, &options)
	m["SearchKeyword"] = template.HTML(searchKeyword)
	m["HasKeyword"] = hasKeyword
	m["ShowSearchComponent"] = options.ShowSearchComponent
	// Keep the script-free homepage while avoiding an unsolicited mobile keyboard.
	ua := strings.ToLower(c.Get("User-Agent"))
	mobile := strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "ipad") || strings.Contains(ua, "iphone")
	c.Response().Header.Add("Vary", "User-Agent")
	m["DisabledSearchAutoFocus"] = options.DisabledSearchAutoFocus || mobile
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
	return renderClockPage(c, m, options, true)
}

func renderClockPage(c fiber.Ctx, m map[string]any, options model.Application, greeting bool) error {
	if options.Locale == "" {
		options.Locale = "zh"
	}
	resolved := clock.Populate(m, options, time.Now(), greeting)
	if resolved.Enhance && !define.AppFlags.DisableCSP {
		c.Set("Content-Security-Policy", clock.Policy())
	}
	return c.Status(http.StatusOK).Render("home.html", m)
}

// parseRequestURL preserves the dynamic bookmark URL metadata without adapting the handler.
func parseRequestURL(c fiber.Ctx) {
	path := url.URL{Path: c.Path()}
	r := http.Request{Host: string(c.Request().Host()), URL: &path, RequestURI: c.OriginalURL()}
	if c.RequestCtx().IsTLS() {
		r.TLS = &tls.ConnectionState{}
	}
	fn.ParseRequestURL(&r)
}
