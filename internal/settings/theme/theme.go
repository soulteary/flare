package theme

import (
	"errors"
	"net/http"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/pool"
	"github.com/soulteary/flare/internal/settings"
)

// actionSaveCustom is submitted by the custom color form. Theme cards submit
// only a theme name.
const actionSaveCustom = "save-custom"

func RegisterRouting(e *fiber.App) {
	e.Get(define.SettingPages.Theme.Path, auth.AuthRequired, pageTheme)
	e.Post(define.SettingPages.Theme.Path, auth.AuthRequired, updateThemes)
}

var updateMu sync.Mutex

func updateThemes(c fiber.Ctx) error {
	updateMu.Lock()
	defer updateMu.Unlock()
	var body struct {
		Theme      string `form:"theme"`
		Action     string `form:"action"`
		Background string `form:"custom-background"`
		Primary    string `form:"custom-primary"`
		Accent     string `form:"custom-accent"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return c.Status(http.StatusForbidden).JSON("提交数据缺失")
	}
	if body.Action == actionSaveCustom {
		// The form binder borrows PostArgs storage even when Fiber Immutable is enabled.
		// The palette survives the request, so retain owned copies.
		palette := model.Palette{
			Background: strings.Clone(body.Background),
			Primary:    strings.Clone(body.Primary),
			Accent:     strings.Clone(body.Accent),
		}
		return updateCustomPalette(c, palette.Normalized())
	}
	// ThemeCurrent survives the request, so retain an owned copy.
	theme := strings.Clone(body.Theme)
	if theme == model.ThemeCustomName {
		return selectCustomPalette(c)
	}
	if !define.IsKnownTheme(theme) {
		return c.Status(http.StatusBadRequest).SendString("Unknown theme")
	}
	if done, err := commitThemeChange(c, func() error { return data.SaveThemeName(theme) }); done {
		return err
	}
	define.SetTheme(theme)
	return pageTheme(c)
}

// commitThemeChange runs one settings write with the checks every theme change
// needs. It reports whether the response has already been written; the
// previous theme then remains active.
func commitThemeChange(c fiber.Ctx, save func() error) (bool, error) {
	if auth.PersistentSecurity(c) && !auth.Revalidate(c) {
		return true, c.Status(http.StatusUnauthorized).SendString("AUTH_REQUIRED")
	}
	var saveErr error
	if err := auth.AuthorizedWrite(c, func() error { saveErr = save(); return nil }); err != nil {
		return true, err
	}
	if saveErr != nil {
		failure := settings.ExplainSaveFailure(saveErr)
		return true, c.Status(failure.Status).SendString(failure.Text())
	}
	return false, nil
}

// updateCustomPalette validates, stores and applies the submitted colors. A
// rejected palette is shown again with the reason; nothing is saved.
func updateCustomPalette(c fiber.Ctx, palette model.Palette) error {
	if err := palette.Validate(); err != nil {
		return renderTheme(c, http.StatusBadRequest, themeNotice{submitted: &palette, rejected: err})
	}
	if done, err := commitThemeChange(c, func() error { return data.SaveThemeCustom(palette) }); done {
		return err
	}
	if err := define.SetCustomTheme(palette); err != nil {
		return err
	}
	return renderTheme(c, http.StatusOK, themeNotice{saved: true})
}

// selectCustomPalette applies the palette stored in config.yml. The file is
// the source of truth: it may have been edited since the palette was last
// applied, and the selection has to match what the next start will read.
func selectCustomPalette(c fiber.Ctx) error {
	options, readErr := data.GetAllSettingsOptions()
	if readErr != nil {
		failure := settings.ExplainSaveFailure(settings.ConfigUnreadable(readErr))
		return c.Status(failure.Status).SendString(failure.Text())
	}
	palette, stored := storedCustomPalette(options)
	if !stored {
		return c.Status(http.StatusBadRequest).SendString("Unknown theme")
	}
	if done, err := commitThemeChange(c, func() error { return data.SaveThemeName(model.ThemeCustomName) }); done {
		return err
	}
	if err := define.SetCustomTheme(palette); err != nil {
		return err
	}
	return pageTheme(c)
}

// storedCustomPalette returns the custom palette in the configuration if it
// can be used. A hand-edited block that fails validation is treated as absent.
func storedCustomPalette(options model.Application) (model.Palette, bool) {
	if options.ThemeCustom == nil {
		return model.Palette{}, false
	}
	palette := options.ThemeCustom.Normalized()
	if palette.Validate() != nil {
		return model.Palette{}, false
	}
	return palette, true
}

// themeNotice carries the outcome of a custom color submission to the page.
type themeNotice struct {
	// submitted keeps the colors of a rejected palette in the form.
	submitted *model.Palette
	rejected  error
	saved     bool
}

func pageTheme(c fiber.Ctx) error {
	return renderTheme(c, http.StatusOK, themeNotice{})
}

func renderTheme(c fiber.Ctx, status int, notice themeNotice) error {
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

	if custom, stored := storedCustomPalette(options); stored {
		m["ThemeCustom"] = custom
	}
	// The form starts from the colors in use, so that any theme, bundled or
	// custom, can be adjusted.
	form := define.CurrentThemePalette().Normalized()
	if notice.submitted != nil {
		form = keepColors(form, *notice.submitted)
	}
	m["ThemeCustomForm"] = form
	m["ThemeCustomAction"] = actionSaveCustom
	m["ThemeCustomSaved"] = notice.saved
	if notice.rejected != nil {
		m["ThemeCustomError"] = paletteErrorText(locale, notice.rejected)
	}
	return c.Status(status).Render("settings.html", m)
}

// keepColors overlays the submitted values that are colors. A color input
// cannot show anything else, so such a field keeps its previous color rather
// than echoing the rejected text back into the page.
func keepColors(form, submitted model.Palette) model.Palette {
	keep := func(current, value string) string {
		if model.IsHexColor(value) {
			return value
		}
		return current
	}
	return model.Palette{
		Background: keep(form.Background, submitted.Background),
		Primary:    keep(form.Primary, submitted.Primary),
		Accent:     keep(form.Accent, submitted.Accent),
	}
}

func paletteErrorText(locale string, err error) string {
	if errors.Is(err, model.ErrPaletteContrast) {
		return i18n.T(locale, "theme_custom_low_contrast")
	}
	return i18n.T(locale, "theme_custom_invalid_color")
}
