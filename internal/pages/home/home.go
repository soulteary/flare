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
	"github.com/soulteary/flare/internal/themetemplate"
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
	registerThemeStylesheet(e)
	if define.AppFlags.Visibility != "PRIVATE" {
		e.Get(define.RegularPages.Home.Path, pageHome)
		e.Get(define.RegularPages.Help.Path, renderHelp)
		e.Post(define.RegularPages.Home.Path, pageSearch)
		e.Get(define.RegularPages.Applications.Path, pageApplication)
		e.Get(define.RegularPages.Bookmarks.Path, pageBookmark)
		e.Post(ThemeChoicePath, chooseTheme)
	} else {
		e.Get(define.RegularPages.Home.Path, auth.AuthRequired, pageHome)
		e.Get(define.RegularPages.Help.Path, auth.AuthRequired, renderHelp)
		e.Post(define.RegularPages.Home.Path, auth.AuthRequired, pageSearch)
		e.Get(define.RegularPages.Applications.Path, auth.AuthRequired, pageApplication)
		e.Get(define.RegularPages.Bookmarks.Path, auth.AuthRequired, pageBookmark)
		e.Post(ThemeChoicePath, auth.AuthRequired, chooseTheme)
	}
	registerSearchSuggestions(e)
}

func pageHome(c fiber.Ctx) error {
	return render(c, strings.TrimSpace(c.Query("search")))
}

func renderHelp(c fiber.Ctx) error {
	options, err := requestSettings(c)
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	configWeatherShow := true
	var weatherData model.Weather
	if !define.AppFlags.EnableOfflineMode {
		weatherShow := options.ShowWeather
		if weatherShow {
			weatherData = weatherForSettings(options)
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
	setPageTheme(c, m, locale)
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

// pageSearch must remain read-only: the write-source protection does not
// diagnose the browser source of this POST route.
func pageSearch(c fiber.Ctx) error {
	options, err := requestSettings(c)
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
	if selected := c.Query("category"); selected != "" && !requestReadableCategory(c, selected, auth.CheckUserIsLogin(c)) {
		return fiber.ErrNotFound
	}
	setCSPHeader(c)
	options, err := requestSettings(c)
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
	setPageTheme(c, m, locale)
	m["BodyClassName"] = homepageBodyClasses(options)
	m["SettingPages"] = define.SettingPages
	m["BookmarksURI"] = define.RegularPages.Bookmarks.Path
	m["ApplicationsURI"] = define.RegularPages.Applications.Path
	m["SettingsURI"] = define.RegularPages.Settings.Path
	if options.ShowBookmarks {
		m["Bookmarks"], m["BookmarksError"] = generateBookmarkNavigationForRead(requestThemeItems(c, options, locale), strings.TrimSpace(c.Query("search")), c.Query("category"), c.Path(), &options, auth.CheckUserIsLogin(c), auth.BufferedReadTentative(c), &requestURL)
		if auth.BufferedReadTentative(c) && m["BookmarksError"] != "" {
			return c.Status(http.StatusInternalServerError).SendString("navigation unavailable")
		}
	}
	m["OptionTitle"] = options.Title
	m["OptionOpenBookmarkNewTab"] = options.OpenBookmarkNewTab
	m["OptionShowBookmarks"] = options.ShowBookmarks
	m["OptionHideSettingsButton"] = options.HideSettingsButton
	m["OptionHideHelpButton"] = options.HideHelpButton
	usePageTemplate(c, m, options, themetemplate.PageBookmarks)
	return c.Status(http.StatusOK).Render("home.html", m)
}

func pageApplication(c fiber.Ctx) error {
	options, err := requestSettings(c)
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
		m["Applications"], m["ApplicationsError"] = generateApplicationsForRead(requestThemeItems(c, options, locale), "", &options, auth.CheckUserIsLogin(c), auth.BufferedReadTentative(c), &requestURL)
		if auth.BufferedReadTentative(c) && m["ApplicationsError"] != "" {
			return c.Status(http.StatusInternalServerError).SendString("navigation unavailable")
		}
	}
	m["PageName"] = i18n.T(locale, "page_apps")
	m["SubPage"] = true
	setPageTheme(c, m, locale)
	m["BodyClassName"] = homepageBodyClasses(options)
	m["OptionTitle"] = options.Title
	m["OptionOpenAppNewTab"] = options.OpenAppNewTab
	m["OptionShowApps"] = options.ShowApps
	m["OptionHideSettingsButton"] = options.HideSettingsButton
	m["OptionHideHelpButton"] = options.HideHelpButton
	usePageTemplate(c, m, options, themetemplate.PageApplications)
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
	if requested != "" && !requestReadableCategory(c, requested, auth.CheckUserIsLogin(c)) {
		return fiber.ErrNotFound
	}
	options, err := requestSettings(c)
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	administrator := auth.CheckUserIsLogin(c)
	appsView, bookmarksView, viewErr := data.LoadNavigationViews(options.ShowApps, options.ShowBookmarks, administrator, auth.BufferedReadTentative(c))
	if auth.BufferedReadTentative(c) && viewErr != nil {
		return c.Status(http.StatusInternalServerError).SendString("navigation unavailable")
	}
	appsErr, bookmarksErr := viewErr, viewErr
	if viewErr != nil && options.ShowApps && options.ShowBookmarks {
		// The pair is selected under one lock for consistency. When that fails,
		// read each collection on its own so that one unreadable file does not
		// also hide the other module.
		appsView, _, appsErr = data.LoadNavigationViews(true, false, administrator, false)
		_, bookmarksView, bookmarksErr = data.LoadNavigationViews(false, true, administrator, false)
	}
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
		weatherShow := options.ShowWeather
		if weatherShow {
			weatherData = weatherForSettings(options)
		} else {
			configWeatherShow = weatherShow
		}
	}
	setCSPHeader(c)
	m := pool.GetTemplateMap()
	defer pool.PutTemplateMap(m)
	m["Locale"] = locale
	m["PageName"] = "Home"
	setPageTheme(c, m, locale)
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
	// How many entries each list shows, for a search without results.
	appsShown, bookmarksShown := 0, 0
	items := requestThemeItems(c, options, locale)
	if options.ShowApps {
		if appsErr != nil {
			m["ApplicationsError"] = navigationDataError("applications", options.Locale, appsErr)
		} else {
			apps, appsShown = navigationFragmentShown(items, appsView, &options, true, filter, "", c.Path(), &requestURL)
		}
	}
	selected := c.Query("category")
	if c.Method() == http.MethodPost {
		selected = c.FormValue("category")
	}
	m["BookmarkCategory"] = selected
	var bookmarks template.HTML
	if options.ShowBookmarks {
		if bookmarksErr != nil {
			m["BookmarksError"] = navigationDataError("bookmarks", options.Locale, bookmarksErr)
		} else {
			bookmarks, bookmarksShown = navigationFragmentShown(items, bookmarksView, &options, false, filter, selected, c.Path(), &requestURL)
		}
	}
	m["Applications"] = apps
	m["Bookmarks"] = bookmarks
	if filter != "" {
		m["SearchNoResults"] = searchHasNoResults(m, options, appsShown, bookmarksShown)
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
	usePageTemplate(c, m, options, themetemplate.PageHome)
	return renderClockPage(c, m, options, true)
}

func renderClockPage(c fiber.Ctx, m map[string]any, options model.Application, greeting bool) error {
	// Trusted-mode navigation and search are read-only. Management forms
	// obtain their own CSRF tokens independently of this renderer.
	if options.ShowSearchComponent && !define.AppFlags.DisableLoginMode {
		m["SecurityCSRFToken"] = auth.CSRFToken(c)
	}
	if options.Locale == "" {
		options.Locale = "zh"
	}
	resolved := clock.Populate(m, options, time.Now(), greeting)
	populateHybridSearch(m, options)
	if assets := guideAssetsURI(c); assets != "" {
		m["GuideAssetsURI"] = assets
		applyGuideScriptPolicy(c, resolved.Enhance, m["SearchHybrid"] == true)
	} else {
		applyHomeScriptPolicy(c, resolved.Enhance, m["SearchHybrid"] == true)
	}
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

// themePreview is what the page says while it previews a theme.
type themePreview struct {
	// Name is the theme, as the theme settings take it to select it.
	Name string
	Text string
	Use  string
	Back string
	// Action is where selecting the theme is submitted, and where the way
	// back leads.
	Action string
}

// themeSafeMode is the notice of a page that is shown in safe mode, with the
// way back to the theme settings. A page that shows the templates of the
// selected theme before they are confirmed has a notice of the same form.
type themeSafeMode struct {
	Text   string
	Back   string
	Action string
}

// setPageTheme hands the page what it takes from the theme: the selected
// theme, or the one this response previews. A preview changes nothing but
// this response.
func setPageTheme(c fiber.Ctx, m map[string]any, locale string) {
	theme := define.GetPageTheme()
	if requestSafeMode(c) {
		theme = define.GetPageThemeWithoutDirectory()
		// Like a preview, the response differs from what the address
		// shows to anyone else.
		c.Set("Cache-Control", "private, no-store")
		m["ThemeSafeMode"] = themeSafeMode{
			Text:   i18n.T(locale, "theme_safe_mode_page"),
			Back:   i18n.T(locale, "theme_preview_back"),
			Action: define.SettingPages.Theme.Path,
		}
	} else if name := requestPreview(c); name != "" {
		if previewed, known := define.GetPageThemeFor(name); known {
			theme = previewed
			title := define.ThemeTitle(name, locale)
			if name == model.ThemeCustomName {
				title = i18n.T(locale, "theme_custom")
			}
			// Selecting the previewed theme is a write to the settings and
			// needs the token of the session, on every page a preview can
			// be opened on.
			if _, set := m["SecurityCSRFToken"]; !set && auth.PersistentSecurity(c) {
				m["SecurityCSRFToken"] = auth.CSRFToken(c)
			}
			// The response differs from what the address shows to anyone
			// else, so nothing may keep a copy of it.
			c.Set("Cache-Control", "private, no-store")
			m["ThemePreview"] = themePreview{
				Name:   name,
				Text:   i18n.Format(locale, "theme_preview_notice", map[string]any{"name": title}),
				Use:    i18n.T(locale, "theme_preview_use"),
				Back:   i18n.T(locale, "theme_preview_back"),
				Action: define.SettingPages.Theme.Path,
			}
		}
	} else if name := requestChoice(c); name != "" {
		if chosen, known := define.GetPageThemeFor(name); known {
			theme = chosen
			// The response is the page of this visitor, which a cache
			// between the two must not hand to another. A response that
			// nothing may keep at all stays that.
			if c.GetRespHeader("Cache-Control") == "" {
				c.Set("Cache-Control", "private")
			}
		}
	}
	if requestChoices(c) {
		// The same address shows another theme to a visitor who chose one.
		c.Response().Header.Add("Vary", "Cookie")
	}
	m["PageAppearance"] = theme.BodyStyle
	m["PageThemeRules"] = theme.Rules
	m["PageThemeStylesheet"] = theme.Stylesheet
	m["PageThemeInherited"] = theme.Inherited
	m["PageThemeBackground"] = theme.Background
	m["PageThemeAttributes"] = theme.Attributes
}

// usePageTemplate hands the page the template its theme brings for it, where
// the theme has one and it is in use. The theme is the one the page is shown
// with: the selected one, or the one this response previews. Safe mode uses
// no template, and neither does the guide, whose script finds its elements
// by the markup of the application. It is called once everything the page
// shows is known, because a template can branch on which parts are shown.
func usePageTemplate(c fiber.Ctx, m map[string]any, options model.Application, kind themetemplate.PageKind) {
	if !options.ThemeTemplates || requestSafeMode(c) || guideAssetsURI(c) != "" {
		return
	}
	page := define.ThemePage{Kind: kind, Searching: m["HasKeyword"] == true}
	var unconfirmed bool
	page.Theme, page.Preview, unconfirmed = requestTemplateTheme(c, options)
	if locale, set := m["Locale"].(string); set {
		page.Lang = locale
	}
	page.T = func(key string) string { return i18n.T(page.Lang, key) }
	page.Modules = pageModules(m)
	fragment := define.ThemePageTemplate(options, page)
	if fragment == nil {
		return
	}
	m[themetemplate.PageData] = fragment
	if unconfirmed {
		// Visitors see this page without the templates, so the response
		// says what it is and nothing may keep a copy of it.
		c.Set("Cache-Control", "private, no-store")
		m["ThemeSafeMode"] = themeSafeMode{
			Text:   i18n.T(page.Lang, "theme_templates_preview_page"),
			Back:   i18n.T(page.Lang, "theme_preview_back"),
			Action: define.SettingPages.Theme.Path,
		}
	}
}

// requestTemplateTheme returns the theme whose templates a response uses,
// whether the response is a preview of it, and whether it is the selected
// theme shown before its templates are confirmed.
func requestTemplateTheme(c fiber.Ctx, options model.Application) (string, bool, bool) {
	if theme := requestPreview(c); theme != "" {
		return theme, true, false
	}
	// A theme the visitor chose is shown as it is when it is selected: with
	// its templates once they are confirmed, and never as a preview.
	if theme := requestChoice(c); theme != "" {
		return theme, false, false
	}
	theme, _ := define.ThemeSnapshot()
	// The selected theme is what the page shows anyway, so it is no
	// preview, with one exception: the administrator can look at its
	// templates before confirming them.
	unconfirmed := !options.ThemeSettings[theme].Templates
	return theme, unconfirmed && c.Query(previewParameter) == theme && auth.CheckUserIsLogin(c), unconfirmed
}

// pageModules says which parts a page shows, by the name of each part
// without its prefix, from what the page hands to its template. A part is
// shown under the condition its own markup has.
func pageModules(m map[string]any) map[string]bool {
	return map[string]bool{
		"search":         m["ShowSearchComponent"] == true,
		"clock":          m["OptionShowDateTime"] == true,
		"greeting":       m["OptionShowTitle"] == true,
		"weather":        m["OptionShowTitle"] == true && m["ShowWeatherModule"] == true,
		"subpage-header": m["SubPage"] == true,
		"apps":           m["OptionShowApps"] == true,
		"bookmarks":      m["OptionShowBookmarks"] == true,
		"toolbar":        m["OptionHideSettingsButton"] != true || m["OptionHideHelpButton"] != true,
		"footer":         m["OptionFooter"] != nil && m["OptionFooter"] != template.HTML(""),
	}
}

// homepageBodyClasses keeps appearance and letter-case preferences consistent on navigation pages.
func homepageBodyClasses(options model.Application) string {
	classes := options.Homepage.Classes()
	if !options.KeepLetterCase {
		classes += "app-content-uppercase "
	}
	return classes
}

// searchHasNoResults reports a search that shows no entry in any list the
// page has. It goes by the number of entries each list shows, not by their
// markup, which is the theme's to decide. A list that could not be read is
// no list without results.
func searchHasNoResults(m map[string]any, options model.Application, appsShown, bookmarksShown int) bool {
	dataFailed := m["ApplicationsError"] != nil && m["ApplicationsError"] != "" || m["BookmarksError"] != nil && m["BookmarksError"] != ""
	return !dataFailed && (!options.ShowApps || appsShown == 0) && (!options.ShowBookmarks || bookmarksShown == 0)
}

func isMobileUserAgent(ua string) bool {
	return strings.Contains(ua, "mobile") || strings.Contains(ua, "android") || strings.Contains(ua, "ipad") || strings.Contains(ua, "iphone")
}

func readableCategory(id string, administrator bool) bool {
	readable, err := data.ReadableBookmarkCategory(id, administrator)
	return err == nil && readable
}
