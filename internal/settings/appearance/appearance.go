package appearance

import (
	"html/template"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/clock"
	"github.com/soulteary/flare/internal/formvalue"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/pool"
	"github.com/soulteary/flare/internal/settings"
)

// pages are the pages the settings of the appearance are spread over. Each
// shows its own fields, and each takes a form at its own address.
func pages() []model.Page {
	return []model.Page{define.SettingPages.Modules, define.SettingPages.Clock, define.SettingPages.Links, define.SettingPages.Layout, define.SettingPages.Site}
}

func RegisterRouting(e *fiber.App) {
	for _, page := range pages() {
		name := page.Name
		e.Get(page.Path, auth.AuthRequired, func(c fiber.Ctx) error { return renderPage(c, name) })
		e.Post(page.Path, auth.AuthRequired, func(c fiber.Ctx) error { return updateAppearanceOptions(c, name) })
	}
	// The address of the page that held all of them leads to the layout,
	// which most of its fields were about. A form of that page, as an
	// earlier release rendered it, is still taken there.
	e.Get(define.SettingPages.Appearance.Path, auth.AuthRequired, func(c fiber.Ctx) error {
		return c.Redirect().Status(http.StatusFound).To(define.SettingPages.Layout.Path)
	})
	e.Post(define.SettingPages.Appearance.Path, auth.AuthRequired, func(c fiber.Ctx) error {
		return updateAppearanceOptions(c, define.SettingPages.Layout.Name)
	})
}

// submitted reads a field that is yes or no, and keeps the stored value for
// a form that does not have the field: a page has the fields of its own
// subject only, and saving it changes nothing about the others.
func submitted(c fiber.Ctx, field string, sent, stored bool) bool {
	if formvalue.Has(c, field) {
		return sent
	}
	return stored
}

// updateAppearanceOptions stores what a form submits and shows the page the
// form came from again. Every field is optional: one that the form does not
// have keeps its stored value.
func updateAppearanceOptions(c fiber.Ctx, page string) error {
	var body struct {
		OptionTitle              string `form:"title"`
		OptionFooter             string `form:"footer"`
		OptionOpenAppNewTab      bool   `form:"open-app-newtab"`
		OptionOpenBookmarkNewTab bool   `form:"open-bookmark-newtab"`
		OptionShowTitle          bool   `form:"show-title"`
		OptionGreetings          string `form:"greetings"`
		OptionShowDateTime       bool   `form:"show-datetime"`
		OptionShowApps           bool   `form:"show-apps"`
		OptionShowBookmarks      bool   `form:"show-bookmarks"`
		HideSettingsButton       bool   `form:"hide-settings-button"`
		HideVersion              bool   `form:"hide-version"`
		HideHelpButton           bool   `form:"hide-help-button"`
		EnableEncryptedLink      bool   `form:"enable-encrypted-link"`
		IconMode                 string `form:"icon-mode"`
		KeepLetterCase           bool   `form:"keep-letter-case"`
		Locale                   string `form:"locale"`
		OptionCustomDay          string `form:"custom-day"`
		OptionCustomMonth        string `form:"custom-month"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return c.Status(http.StatusForbidden).JSON("提交数据缺失")
	}
	if body.Locale != "" {
		locale, ok := i18n.ParseLocale(body.Locale)
		if !ok {
			return c.Status(http.StatusBadRequest).SendString(i18n.T(settingsLocale(), "settings_invalid_locale"))
		}
		body.Locale = locale
	}
	existing, readErr := data.GetAllSettingsOptions()
	if readErr != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	var update model.Application
	if err := updateHomepageTypography(c, &update); err != nil {
		return c.Status(err.Code).SendString(err.Message)
	}

	if err := updateHomepageNavigation(c, &update); err != nil {
		return c.Status(err.Code).SendString(err.Message)
	}

	if err := updateClockOptions(c, &update); err != nil {
		return c.Status(err.Code).SendString(err.Message)
	}

	if err := updateThemeLayout(c, &update); err != nil {
		return c.Status(err.Code).SendString(err.Message)
	}

	// A text that is submitted empty is cleared; one that is not submitted
	// stays. The greetings field is rendered only while the title is shown.
	update.Title, update.Footer, update.Greetings = existing.Title, existing.Footer, existing.Greetings
	if formvalue.Has(c, "title") {
		update.Title = body.OptionTitle
	}
	if formvalue.Has(c, "footer") {
		update.Footer = body.OptionFooter
	}
	if formvalue.Has(c, "greetings") {
		update.Greetings = body.OptionGreetings
	}
	update.OpenAppNewTab = submitted(c, "open-app-newtab", body.OptionOpenAppNewTab, existing.OpenAppNewTab)
	update.OpenBookmarkNewTab = submitted(c, "open-bookmark-newtab", body.OptionOpenBookmarkNewTab, existing.OpenBookmarkNewTab)
	update.ShowTitle = submitted(c, "show-title", body.OptionShowTitle, existing.ShowTitle)
	update.ShowDateTime = submitted(c, "show-datetime", body.OptionShowDateTime, existing.ShowDateTime)
	update.ShowApps = submitted(c, "show-apps", body.OptionShowApps, existing.ShowApps)
	update.ShowBookmarks = submitted(c, "show-bookmarks", body.OptionShowBookmarks, existing.ShowBookmarks)
	update.HideSettingsButton = submitted(c, "hide-settings-button", body.HideSettingsButton, existing.HideSettingsButton)
	update.HideHelpButton = submitted(c, "hide-help-button", body.HideHelpButton, existing.HideHelpButton)
	update.HideVersion = submitted(c, "hide-version", body.HideVersion, existing.HideVersion)
	update.EnableEncryptedLink = submitted(c, "enable-encrypted-link", body.EnableEncryptedLink, existing.EnableEncryptedLink)
	update.KeepLetterCase = submitted(c, "keep-letter-case", body.KeepLetterCase, existing.KeepLetterCase)
	update.IconMode = existing.IconMode
	if formvalue.Has(c, "icon-mode") {
		update.IconMode = strings.ToUpper(body.IconMode)
		if update.IconMode != "DEFAULT" && update.IconMode != "FILLING" {
			update.IconMode = "DEFAULT"
		}
	}
	if body.Locale != "" {
		update.Locale = body.Locale
	}
	if auth.PersistentSecurity(c) && !auth.Revalidate(c) {
		return c.Status(http.StatusUnauthorized).SendString("AUTH_REQUIRED")
	}
	var saveErr error
	if err := auth.AuthorizedWrite(c, func() error { saveErr = data.SaveAppearance(update); return nil }); err != nil {
		return err
	}
	if saveErr != nil {
		failure := settings.ExplainSaveFailure(saveErr)
		return c.Status(failure.Status).SendString(failure.Text())
	}
	return renderPage(c, page)
}

// renderPage shows one of the pages. Each is handed the values of all of
// them, and its template shows those of its subject.
func renderPage(c fiber.Ctx, page string) error {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	// A configuration that names no way, or one that is not known, is shown
	// as the default, so that the field is always there to be set.
	IconModeFilling := options.IconMode == "FILLING"
	IconModeDefault := !IconModeFilling
	m := pool.GetTemplateMap()
	if auth.PersistentSecurity(c) {
		m["SecurityCSRFToken"] = auth.CSRFToken(c)
	}
	defer pool.PutTemplateMap(m)
	m["DebugMode"] = define.AppFlags.DebugMode
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	m["PageName"] = page
	pageTheme := define.GetPageTheme()
	m["PageAppearance"] = pageTheme.BodyStyle
	m["PageThemeRules"] = pageTheme.Rules
	m["SettingPages"] = define.SettingPages
	m["SettingsURI"] = define.RegularPages.Settings.Path
	m["OptionNavigationTextSize"] = "standard"
	m["OptionTitleSize"] = "standard"
	m["OptionHideAppDescriptions"] = false
	if options.Homepage != nil {
		if options.Homepage.NavigationTextSize == "larger" || options.Homepage.NavigationTextSize == "large" {
			m["OptionNavigationTextSize"] = options.Homepage.NavigationTextSize
		}
		if options.Homepage.TitleSize == "compact" || options.Homepage.TitleSize == "prominent" {
			m["OptionTitleSize"] = options.Homepage.TitleSize
		}
		m["OptionHideAppDescriptions"] = options.Homepage.HideAppDescriptions
	}
	m["OptionMobileBookmarkLayout"] = options.Homepage.MobileLayout()
	m["OptionTitle"] = options.Title
	m["OptionFooter"] = template.HTML(options.Footer)
	m["OptionOpenAppNewTab"] = options.OpenAppNewTab
	m["OptionOpenBookmarkNewTab"] = options.OpenBookmarkNewTab
	m["OptionShowTitle"] = options.ShowTitle
	m["OptionGreetings"] = options.Greetings
	m["OptionShowDateTime"] = options.ShowDateTime
	m["OptionShowApps"] = options.ShowApps
	m["OptionShowBookmarks"] = options.ShowBookmarks
	m["OptionHideSettingsButton"] = options.HideSettingsButton
	m["OptionHideHelpButton"] = options.HideHelpButton
	m["OptionHideVersion"] = options.HideVersion
	m["OptionEnableEncryptedLink"] = options.EnableEncryptedLink
	m["OptionKeepLetterCase"] = options.KeepLetterCase
	m["OptionIconModeDefault"] = IconModeDefault
	m["OptionIconModeFilling"] = IconModeFilling
	resolved := clock.Resolve(options.Clock, options.ShowDateTime)
	m["ClockDynamic"] = resolved.Options.Dynamic
	m["ClockSeconds"] = resolved.Options.ShowSeconds()
	m["ClockMode"] = resolved.Options.TimezoneMode
	m["ClockTimezone"] = resolved.Options.Timezone
	m["ClockServerZone"] = time.Local.String()
	m["ClockServerTime"] = time.Now().Format("2006-01-02 15:04:05 MST")
	// The page of the clock has a script, which previews the time, and the
	// policy that allows that script and no other.
	if page == define.SettingPages.Clock.Name && !define.AppFlags.DisableCSP {
		c.Set("Content-Security-Policy", clock.SettingsPolicy())
	}
	m["OptionLocale"] = options.Locale
	m["BookmarkLayout"] = "groups"
	if options.Homepage != nil && options.Homepage.BookmarkLayout == "sections" {
		m["BookmarkLayout"] = "sections"
	}
	m["DesktopColumnChoices"] = []string{"4", "5", "6", "8"}
	m["DesktopColumns"] = ""
	if options.Homepage != nil {
		m["DesktopColumns"] = options.Homepage.DesktopColumns
	}
	m["CategoryNavigation"] = "all"
	if options.Homepage != nil && options.Homepage.CategoryNavigation != "" {
		m["CategoryNavigation"] = options.Homepage.CategoryNavigation
	}
	m["Locale"] = options.Locale
	themeLayoutNotes(m, options.Locale, options)
	return c.Status(http.StatusOK).Render("settings.html", m)
}

func settingsLocale() string {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return "zh"
	}
	return options.Locale
}

func updateHomepageTypography(c fiber.Ctx, update *model.Application) *fiber.Error {
	// Copy existing options so older forms cannot erase newer preferences.
	layout, textSize, titleSize, hideDesc := formvalue.Get(c, "mobile-bookmark-layout"), formvalue.Get(c, "navigation-text-size"), formvalue.Get(c, "title-size"), formvalue.Get(c, "hide-app-descriptions")
	if layout != "" || textSize != "" || titleSize != "" || hideDesc != "" {
		existing, err := data.GetAllSettingsOptions()
		if err != nil {
			return fiber.NewError(http.StatusInternalServerError, "config error")
		}
		homepage := model.HomepageOptions{}
		if existing.Homepage != nil {
			homepage = *existing.Homepage
		}
		if layout != "" {
			// Forms rendered by 0.6.5–0.6.14 submit "single" for an untouched
			// default; store the current name of that default instead.
			if layout == model.MobileBookmarkLayoutLegacySingle {
				layout = model.MobileBookmarkLayoutClassic
			}
			homepage.MobileBookmarkLayout = layout
		}
		if textSize != "" {
			homepage.NavigationTextSize = textSize
		}
		if titleSize != "" {
			homepage.TitleSize = titleSize
		}
		if hideDesc != "" {
			if hideDesc != "0" && hideDesc != "1" {
				return fiber.NewError(http.StatusBadRequest, "invalid description visibility")
			}
			homepage.HideAppDescriptions = hideDesc == "1"
		}
		if err := homepage.Validate(); err != nil {
			return fiber.NewError(http.StatusBadRequest, err.Error())
		}
		update.Homepage = &homepage
	}

	return nil
}

func updateHomepageNavigation(c fiber.Ctx, update *model.Application) *fiber.Error {
	if mode, columns, bookmarkLayout := formvalue.Get(c, "category-navigation"), formvalue.Get(c, "desktop-columns"), formvalue.Get(c, "bookmark-layout"); mode != "" || columns != "" || bookmarkLayout != "" {
		existing, err := data.GetAllSettingsOptions()
		if err != nil {
			return fiber.NewError(http.StatusInternalServerError, "config error")
		}
		homepage := model.HomepageOptions{}
		if existing.Homepage != nil {
			homepage = *existing.Homepage
		}
		if update.Homepage != nil {
			homepage = *update.Homepage
		}
		if bookmarkLayout != "" {
			homepage.BookmarkLayout = bookmarkLayout
		}
		if mode != "" {
			homepage.CategoryNavigation = mode
		}
		if columns != "" {
			if columns == "legacy" {
				homepage.DesktopColumns = ""
			} else {
				homepage.DesktopColumns = columns
			}
		}
		if err := homepage.Validate(); err != nil {
			return fiber.NewError(http.StatusBadRequest, err.Error())
		}
		update.Homepage = &homepage
	}

	return nil
}

// Form fields about the layout the selected theme specifies. The form has
// them only while such a theme is selected.
const (
	// fieldThemeLayoutAll turns the layout of every theme off or on.
	fieldThemeLayoutAll  = "theme-layout-all"
	themeLayoutAllApply  = "apply"
	themeLayoutAllIgnore = "ignore"
	// fieldThemeLayoutTheme names the theme the boxes of the form are about,
	// and fieldThemeLayoutIgnore, followed by an option, is the box that
	// keeps the site setting for that option.
	fieldThemeLayoutTheme  = "theme-layout-theme"
	fieldThemeLayoutIgnore = "theme-layout-ignore-"
)

// updateThemeLayout reads which parts of the theme's layout the administrator
// turned off. A form without these fields, such as one rendered while another
// theme was selected, changes nothing about them.
func updateThemeLayout(c fiber.Ctx, update *model.Application) *fiber.Error {
	if all := formvalue.Get(c, fieldThemeLayoutAll); all != "" {
		if all != themeLayoutAllApply && all != themeLayoutAllIgnore {
			return fiber.NewError(http.StatusBadRequest, "invalid theme layout choice")
		}
		homepage := model.HomepageOptions{}
		if update.Homepage != nil {
			homepage = *update.Homepage
		} else {
			existing, err := data.GetAllSettingsOptions()
			if err != nil {
				return fiber.NewError(http.StatusInternalServerError, "config error")
			}
			if existing.Homepage != nil {
				homepage = *existing.Homepage
			}
		}
		homepage.IgnoreThemeLayout = all == themeLayoutAllIgnore
		update.Homepage = &homepage
	}
	// The boxes are in the form only while the layout of the theme applies.
	// Their absence then means that none is ticked; without the name of the
	// theme the form had no boxes, and the stored choice stays.
	theme := formvalue.Get(c, fieldThemeLayoutTheme)
	if theme == "" {
		return nil
	}
	if !model.IsThemeName(theme) {
		return fiber.NewError(http.StatusBadRequest, "invalid theme name")
	}
	ignored := []string{}
	for _, option := range model.ThemeLayoutOptions {
		if formvalue.Has(c, fieldThemeLayoutIgnore+option) {
			ignored = append(ignored, option)
		}
	}
	update.ThemeSettings = map[string]model.ThemeSettings{theme: {IgnoreLayout: ignored}}
	return nil
}

// themeLayoutNote is the remark next to a layout option the selected theme
// specifies.
type themeLayoutNote struct {
	// Field is the name of the box that keeps the site setting. It is
	// empty while the layout of themes is turned off as a whole, when the
	// option has no box of its own.
	Field   string
	Text    string
	UseMine string
	Ignored bool
}

// themeLayoutNotes describes the layout of the selected theme for the page:
// one remark for each option the theme specifies, by option, and a summary.
func themeLayoutNotes(m map[string]any, locale string, options model.Application) {
	name, layout := define.SelectedThemeLayout()
	specified := make([]string, 0, len(layout))
	notes := map[string]*themeLayoutNote{}
	off := options.Homepage != nil && options.Homepage.IgnoreThemeLayout
	for _, option := range model.ThemeLayoutOptions {
		value, set := layout[option]
		if !set || !model.IsThemeLayoutValue(option, value) {
			continue
		}
		specified = append(specified, option)
		params := map[string]any{"value": settings.LayoutValueLabel(locale, option, value)}
		note := &themeLayoutNote{UseMine: i18n.T(locale, "theme_layout_use_mine")}
		if off {
			note.Text = i18n.Format(locale, "theme_layout_note_off", params)
		} else {
			note.Text = i18n.Format(locale, "theme_layout_note", params)
			note.Field = fieldThemeLayoutIgnore + option
			note.Ignored = slices.Contains(options.ThemeSettings[name].IgnoreLayout, option)
		}
		notes[option] = note
	}
	// An option the templates of the theme do not support keeps its control:
	// the setting is stored, and applies with another theme.
	for _, option := range define.ThemeLayoutUnsupported(options) {
		notes[option] = &themeLayoutNote{Text: i18n.T(locale, "theme_layout_unsupported")}
	}
	// The template looks every option up in the remarks, so they are there
	// for a theme without a layout as well, and then empty.
	m["ThemeLayout"] = notes
	if len(specified) == 0 {
		return
	}
	m["ThemeLayoutOff"] = off
	m["ThemeLayoutTheme"] = name
	m["ThemeLayoutSummary"] = i18n.Format(locale, "theme_layout_help", map[string]any{"options": settings.LayoutOptionList(locale, specified)})
}

func updateClockOptions(c fiber.Ctx, update *model.Application) *fiber.Error {
	// Missing fields from older forms must not erase existing clock preferences.
	if formvalue.Get(c, "clock-timezone-mode") != "" {
		dynamic, seconds := formvalue.Get(c, "clock-dynamic"), formvalue.Get(c, "clock-seconds")
		if (dynamic != "0" && dynamic != "1") || (seconds != "0" && seconds != "1") {
			return fiber.NewError(http.StatusBadRequest, "invalid clock options")
		}
		showSeconds := seconds == "1"
		update.Clock = &model.ClockOptions{Dynamic: dynamic == "1", Seconds: &showSeconds, TimezoneMode: formvalue.Get(c, "clock-timezone-mode"), Timezone: strings.TrimSpace(formvalue.Get(c, "clock-timezone"))}
		if err := clock.Validate(*update.Clock); err != nil {
			return fiber.NewError(http.StatusBadRequest, err.Error())
		}
	}

	return nil
}
