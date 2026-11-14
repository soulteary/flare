package theme

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/pool"
	"github.com/soulteary/flare/internal/settings"
)

// actionSaveCustom is submitted by the custom color form and actionSaveMode
// by the form for light and dark. Theme cards submit only a theme name.
const (
	actionSaveCustom = "save-custom"
	actionSaveMode   = "save-mode"
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
		Theme      string `form:"theme"`
		Action     string `form:"action"`
		Background string `form:"custom-background"`
		Primary    string `form:"custom-primary"`
		Accent     string `form:"custom-accent"`
		Mode       string `form:"theme-mode"`
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
	if body.Action == actionSaveMode {
		// The mode is one of three constants once it is accepted, so nothing
		// of the request is retained.
		return updateThemeMode(c, body.Mode)
	}
	// ThemeCurrent survives the request, so retain an owned copy.
	theme := strings.Clone(body.Theme)
	if theme == model.ThemeCustomName {
		return selectCustomPalette(c)
	}
	// The themes directory is read first: a file added or corrected since the
	// page was shown can be selected, and one that went away cannot.
	define.ReloadUserThemes()
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

// updateThemeMode stores and applies the setting for light and dark.
func updateThemeMode(c fiber.Ctx, submitted string) error {
	mode := ""
	for _, known := range []string{model.ThemeModeAuto, model.ThemeModeLight, model.ThemeModeDark} {
		if submitted == known {
			mode = known
		}
	}
	if mode == "" {
		return c.Status(http.StatusBadRequest).SendString("Unknown theme mode")
	}
	if done, err := commitThemeChange(c, func() error { return data.SaveThemeMode(mode) }); done {
		return err
	}
	define.SetThemeMode(mode)
	return renderTheme(c, http.StatusOK, themeNotice{modeSaved: true})
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
	// modeSaved reports a stored setting for light and dark.
	modeSaved bool
}

func pageTheme(c fiber.Ctx) error {
	return renderTheme(c, http.StatusOK, themeNotice{})
}

func renderTheme(c fiber.Ctx, status int, notice themeNotice) error {
	themes := define.ThemePalettes
	// The page lists the theme files as they are now.
	define.ReloadUserThemes()
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
	pageTheme := define.GetPageTheme()
	m["PageAppearance"] = pageTheme.BodyStyle
	m["PageThemeRules"] = pageTheme.Rules
	m["SettingsURI"] = define.RegularPages.Settings.Path
	m["PageName"] = "Theme"
	m["SettingPages"] = define.SettingPages
	m["Themes"] = themes
	m["OptionTitle"] = options.Title
	fileThemes, problems, notices := define.UserThemes()
	m["ThemeFiles"] = themeFileCards(locale, fileThemes)
	m["ThemeFileLines"] = themeFileLines(locale, problems, notices)
	// The setting for light and dark is offered while it has an effect: on a
	// theme with different values for the two.
	mode, choice := define.ThemeModeState()
	m["ThemeMode"] = mode
	m["ThemeModeChoice"] = choice
	m["ThemeModeAction"] = actionSaveMode
	m["ThemeModeSaved"] = notice.modeSaved
	if selected, _ := define.ThemeSnapshot(); selected != "" && !define.IsKnownTheme(selected) {
		m["ThemeUnavailable"] = i18n.Format(locale, "theme_unavailable", map[string]any{"name": plainText(selected, maximumShownName)})
	}

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

// themeFileCard is a theme file in use, as the page shows it.
type themeFileCard struct {
	// Name is the name of the file without its extension, the value stored
	// in config.yml when the theme is selected.
	Name string
	// Title is the name to show, About the version, author and license the
	// file gives.
	Title string
	About string
	// Colors are the colors of the theme in light mode. Dark holds those of
	// dark mode when they are different.
	Colors model.Palette
	Dark   *model.Palette
}

func themeFileCards(locale string, themes []model.Theme) []themeFileCard {
	cards := make([]themeFileCard, 0, len(themes))
	for _, theme := range themes {
		about := make([]string, 0, 3)
		for _, text := range []string{theme.Version, theme.Author, theme.License} {
			if text != "" {
				about = append(about, text)
			}
		}
		card := themeFileCard{
			Name:   theme.Name,
			Title:  theme.DisplayName(locale),
			About:  strings.Join(about, " · "),
			Colors: theme.In(model.ThemeModeLight).Colors,
		}
		if dark := theme.In(model.ThemeModeDark).Colors; dark != card.Colors {
			card.Dark = &dark
		}
		cards = append(cards, card)
	}
	return cards
}

// themeFileLine is one remark about a file in the themes directory: why it
// is not in use, or which of its parts are not used.
type themeFileLine struct {
	File   string
	Text   string
	Detail string
}

// Longest file name and longest detail shown for a file. Both come from the
// themes directory; the page shows them as plain text of a bounded length.
const (
	maximumShownName   = 80
	maximumShownDetail = 240
)

func themeFileLines(locale string, problems []define.ThemeProblem, notices []define.ThemeNotice) []themeFileLine {
	lines := make([]themeFileLine, 0, len(problems)+len(notices))
	for _, problem := range problems {
		lines = append(lines, themeFileLine{
			File:   plainText(problem.File, maximumShownName),
			Text:   i18n.T(locale, "theme_file_problem_"+problem.Reason),
			Detail: plainText(problem.Subject, maximumShownDetail),
		})
	}
	for _, notice := range notices {
		parts := append(append([]string(nil), notice.Unused.Keys...), notice.Unused.Tokens...)
		lines = append(lines, themeFileLine{
			File:   plainText(notice.File, maximumShownName),
			Text:   i18n.T(locale, "theme_file_unused"),
			Detail: plainText(strings.Join(parts, ", "), maximumShownDetail),
		})
	}
	// One file can have a problem and unused parts; its lines stay together.
	sort.SliceStable(lines, func(i, j int) bool { return lines[i].File < lines[j].File })
	return lines
}

// plainText prepares a file name or a parser message for display: one line
// of valid text, cut to a number of characters.
func plainText(text string, limit int) string {
	text = strings.Join(strings.FieldsFunc(strings.ToValidUTF8(text, "?"), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsControl(r)
	}), " ")
	if utf8.RuneCountInString(text) <= limit {
		return text
	}
	return string([]rune(text)[:limit]) + "…"
}
