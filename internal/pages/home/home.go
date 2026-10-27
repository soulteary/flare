package home

import (
	"crypto/tls"
	"html/template"
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
	weatherui "github.com/soulteary/flare/internal/settings/weather"
)

const _cspValue = "script-src 'none'; object-src 'none'; base-uri 'none'; require-trusted-types-for 'script'; report-uri 'none';"

func setCSPHeader(c fiber.Ctx) {
	if !define.AppFlags.DisableCSP {
		c.Set("Content-Security-Policy", _cspValue)
	}
}

// InitWeatherIfNeeded migrates a uniquely resolvable existing location.
// Empty locations are selected explicitly in Settings, including IP suggestions.
func InitWeatherIfNeeded() {
	if define.AppFlags.EnableOfflineMode {
		return
	}
	opts, err := data.GetAllSettingsOptions()
	if err != nil || opts.WeatherLocation != nil || !opts.ShowWeather || opts.Location == "" {
		return
	}
	weatherui.MigrateLegacyLocation(opts.Location)
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
	m["ShowWeatherModule"] = !define.AppFlags.EnableOfflineMode && configWeatherShow && weatherData.Location != ""
	m["Location"] = options.Location
	m["WeatherData"] = weatherData
	m["WeatherIcon"] = weatherui.IconHTML(weatherData.ConditionCode)
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
	opts, _ := searchcore.Upgrade(options.Search)
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
	if selected := c.Query("category"); selected != "" && !readableCategory(selected, auth.CheckUserIsLogin(c)) {
		return fiber.ErrNotFound
	}
	setCSPHeader(c)
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	locale := options.Locale
	if locale == "" {
		locale = "zh"
	}
	requestURL := parseRequestURL(c)
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
	if options.ShowBookmarks {
		m["Bookmarks"], m["BookmarksError"] = generateBookmarkNavigationFor(strings.TrimSpace(c.Query("search")), c.Query("category"), c.Path(), &options, auth.CheckUserIsLogin(c), &requestURL)
	}
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
	requestURL := parseRequestURL(c)
	m := pool.GetTemplateMap()
	defer pool.PutTemplateMap(m)
	m["Locale"] = locale
	m["DebugMode"] = define.AppFlags.DebugMode
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	m["BookmarksURI"] = define.RegularPages.Bookmarks.Path
	m["ApplicationsURI"] = define.RegularPages.Applications.Path
	m["SettingsURI"] = define.RegularPages.Settings.Path
	if options.ShowApps {
		m["Applications"], m["ApplicationsError"] = generateApplicationsFor("", &options, auth.CheckUserIsLogin(c), &requestURL)
	}
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
	requested := c.Query("category")
	if c.Method() == http.MethodPost {
		requested = c.FormValue("category")
	}
	if requested != "" && !readableCategory(requested, auth.CheckUserIsLogin(c)) {
		return fiber.ErrNotFound
	}
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	administrator := auth.CheckUserIsLogin(c)
	appsView, bookmarksView, viewErr := data.LoadVisibleNavigationPair(options.ShowApps, options.ShowBookmarks, administrator)
	requestURL := parseRequestURL(c)
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
	m["ShowWeatherModule"] = !define.AppFlags.EnableOfflineMode && configWeatherShow && weatherData.Location != ""
	m["Location"] = options.Location
	m["WeatherData"] = weatherData
	m["WeatherIcon"] = weatherui.IconHTML(weatherData.ConditionCode)
	m["BookmarksURI"] = define.RegularPages.Bookmarks.Path
	m["ApplicationsURI"] = define.RegularPages.Applications.Path
	m["SettingsURI"] = define.RegularPages.Settings.Path
	var apps template.HTML
	if options.ShowApps {
		if viewErr != nil {
			m["ApplicationsError"] = navigationDataError("applications", options.Locale, viewErr)
		} else {
			apps, m["ApplicationsError"] = generateApplicationsSnapshot(filter, &options, appsView, &requestURL)
		}
	}
	selected := c.Query("category")
	if c.Method() == http.MethodPost {
		selected = c.FormValue("category")
	}
	m["BookmarkCategory"] = selected
	var bookmarks template.HTML
	if options.ShowBookmarks {
		if viewErr != nil {
			m["BookmarksError"] = navigationDataError("bookmarks", options.Locale, viewErr)
		} else {
			bookmarks, m["BookmarksError"] = generateBookmarkSnapshot(filter, selected, c.Path(), &options, bookmarksView, &requestURL)
		}
	}
	m["Applications"] = apps
	m["Bookmarks"] = bookmarks
	if filter != "" {
		m["SearchNoResults"] = searchHasNoResults(m, options, apps, bookmarks)
	}
	m["SearchKeyword"] = searchKeyword
	PopulateSearch(m, options, query, target, searchError)
	m["HasKeyword"] = hasKeyword
	m["ShowSearchComponent"] = options.ShowSearchComponent
	// Keep the script-free homepage while avoiding an unsolicited mobile keyboard.
	ua := strings.ToLower(c.Get("User-Agent"))
	mobile := isMobileUserAgent(ua)
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
	m["SecurityCSRFToken"] = auth.CSRFToken(c)
	if options.Locale == "" {
		options.Locale = "zh"
	}
	resolved := clock.Populate(m, options, time.Now(), greeting)
	populateHybridSearch(m, options)
	applyHomeScriptPolicy(c, resolved.Enhance, m["SearchHybrid"] == true)
	return c.Status(http.StatusOK).Render("home.html", m)
}

// parseRequestURL preserves the dynamic bookmark URL metadata without adapting the handler.
func parseRequestURL(c fiber.Ctx) fn.DynamicURL {
	path := url.URL{Path: c.Path()}
	r := http.Request{Host: string(c.Request().Host()), URL: &path, RequestURI: c.OriginalURL()}
	if c.RequestCtx().IsTLS() {
		r.TLS = &tls.ConnectionState{}
	}
	return fn.ParseRequestURLTo(&r)
}

// homepageBodyClasses keeps appearance and letter-case preferences consistent on navigation pages.
func homepageBodyClasses(options model.Application) string {
	classes := options.Homepage.Classes()
	if !options.KeepLetterCase {
		classes += "app-content-uppercase "
	}
	return classes
}

func searchHasNoResults(m map[string]any, options model.Application, apps, bookmarks template.HTML) bool {
	dataFailed := m["ApplicationsError"] != nil && m["ApplicationsError"] != "" || m["BookmarksError"] != nil && m["BookmarksError"] != ""
	return !dataFailed && (!options.ShowApps || !strings.Contains(string(apps), `class="app-item"`)) && (!options.ShowBookmarks || !strings.Contains(string(bookmarks), `class="bookmark"`))
}

func isMobileUserAgent(ua string) bool {
	return strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "ipad") || strings.Contains(ua, "iphone")
}

func readableCategory(id string, administrator bool) bool {
	readable, err := data.ReadableBookmarkCategory(id, administrator)
	return err == nil && readable
}
