package theme

import (
	"net/http"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/pool"
)

func RegisterRouting(e *fiber.App) {
	e.Get(define.SettingPages.Theme.Path, auth.AuthRequired, pageTheme)
	e.Post(define.SettingPages.Theme.Path, auth.AuthRequired, updateThemes)
}

var updateMu sync.Mutex

func updateThemes(c fiber.Ctx) error {
	updateMu.Lock()
	defer updateMu.Unlock()
	var body struct {
		Theme string `form:"theme"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return c.Status(http.StatusForbidden).JSON("提交数据缺失")
	}
	// The form binder borrows PostArgs storage even when Fiber Immutable is enabled.
	// ThemeCurrent survives the request, so retain an owned copy.
	theme := strings.Clone(body.Theme)
	if !define.IsKnownTheme(theme) {
		return c.Status(http.StatusBadRequest).SendString("Unknown theme")
	}
	if auth.PersistentSecurity(c) && !auth.Revalidate(c) {
		return c.Status(http.StatusUnauthorized).SendString("AUTH_REQUIRED")
	}
	saved := false
	if err := auth.AuthorizedWrite(c, func() error { saved = data.UpdateThemeName(theme); return nil }); err != nil {
		return err
	}
	if !saved {
		return c.Status(http.StatusInternalServerError).SendString("Unable to save theme; previous theme remains active")
	}
	define.SetTheme(theme)
	return pageTheme(c)
}

func pageTheme(c fiber.Ctx) error {
	themes := define.ThemePalettes
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	locale := options.Locale
	if locale == "" {
		locale = "zh"
	}
	m := pool.GetTemplateMap()
	if auth.PersistentSecurity(c) {
		m["SecurityCSRFToken"] = auth.CSRFToken(c)
	}
	defer pool.PutTemplateMap(m)
	m["Locale"] = locale
	m["DebugMode"] = define.AppFlags.DebugMode
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	m["PageAppearance"] = define.GetAppBodyStyle()
	m["SettingsURI"] = define.RegularPages.Settings.Path
	m["PageName"] = "Theme"
	m["SettingPages"] = define.SettingPages
	m["Themes"] = themes
	m["OptionTitle"] = options.Title
	return c.Status(http.StatusOK).Render("settings.html", m)
}
