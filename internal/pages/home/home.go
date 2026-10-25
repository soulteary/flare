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
	searchcore "github.com/soulteary/flare/internal/search"
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
	registerSearchSuggestions(e)
}

func pageHome(c fiber.Ctx) error {
	return render(c, strings.TrimSpace(c.Query("search")))
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
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	query := c.FormValue("search")
	target := c.FormValue("target")
	// Missing target is the legacy POST contract, irrespective of the configured default.
	if target == "" {
		target = "local"
	}
	opts := searchcore.Normalize(options.Search)
	configMessage := ""
	if validationErr := searchcore.Validate(opts); validationErr != nil {
		opts = searchcore.Normalize(nil)
		configMessage = "External search configuration is invalid; local search remains available."
		if target != "local" {
			return renderSearch(c, "", query, "local", configMessage)
		}
	}
	result, err := searchcore.Resolve(opts, query, target)
	if err != nil {
		return renderSearch(c, "", query, target, err.Error())
	}
	if result.URL != "" {
		c.Set("Referrer-Policy", "no-referrer")
		c.Set("Cache-Control", "no-store")
		c.Set("Location", result.URL)
		return c.SendStatus(http.StatusSeeOther)
	}
	return renderSearch(c, result.Query, result.Query, result.Target, configMessage)
}

func pageBookmark(c fiber.Ctx) error {
	setCSPHeader(c)
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
	m["BodyClassName"] = homepageBodyClasses(options)
	m["SettingPages"] = define.SettingPages
	m["BookmarksURI"] = define.RegularPages.Bookmarks.Path
	m["ApplicationsURI"] = define.RegularPages.Applications.Path
	m["SettingsURI"] = define.RegularPages.Settings.Path
	m["Bookmarks"] = GenerateBookmarkNavigationTemplate(strings.TrimSpace(c.Query("search")), c.Query("category"), c.Path(), &options)
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
	m["BodyClassName"] = homepageBodyClasses(options)
	m["OptionTitle"] = options.Title
	m["OptionOpenAppNewTab"] = options.OpenAppNewTab
	m["OptionShowApps"] = options.ShowApps
	m["OptionHideSettingsButton"] = options.HideSettingsButton
	m["OptionHideHelpButton"] = options.HideHelpButton
	return c.Status(http.StatusOK).Render("home.html", m)
}

func render(c fiber.Ctx, filter string) error {
	return renderSearch(c, filter, filter, "", "")
}

func renderSearch(c fiber.Ctx, filter, query, target, searchError string) error {
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
	apps := GenerateApplicationsTemplate(filter, &options)
	selected := c.Query("category")
	if c.Method() == http.MethodPost {
		selected = c.FormValue("category")
	}
	m["BookmarkCategory"] = selected
	bookmarks := GenerateBookmarkNavigationTemplate(filter, selected, c.Path(), &options)
	m["Applications"] = apps
	m["Bookmarks"] = bookmarks
	if filter != "" {
		m["SearchNoResults"] = (!options.ShowApps || !strings.Contains(string(apps), `class="app-item"`)) && (!options.ShowBookmarks || !strings.Contains(string(bookmarks), `class="bookmark"`))
	}
	m["SearchKeyword"] = searchKeyword
	PopulateSearch(m, options, query, target, searchError)
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
	m["BodyClassName"] = homepageBodyClasses(options)
	return renderClockPage(c, m, options, true)
}

func renderClockPage(c fiber.Ctx, m map[string]any, options model.Application, greeting bool) error {
	if options.Locale == "" {
		options.Locale = "zh"
	}
	resolved := clock.Populate(m, options, time.Now(), greeting)
	populateHybridSearch(m, options)
	applyHomeScriptPolicy(c, resolved.Enhance, m["SearchHybrid"] == true)
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

// homepageBodyClasses keeps appearance and letter-case preferences consistent on navigation pages.
func homepageBodyClasses(options model.Application) string {
	classes := options.Homepage.Classes()
	if !options.KeepLetterCase {
		classes += "app-content-uppercase "
	}
	return classes
}
