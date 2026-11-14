package define

// TODO：挪到合适地方，拆分 assets

import (
	"html/template"
	"maps"
	"strings"
	"sync"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/logger"
)

// 程序运行默认使用内置的主题配色
var themeMu sync.RWMutex
var themeGeneration uint64

var ThemePalettes = getDefaultThemePalettes()

// The custom palette is kept apart from ThemePalettes: every entry of that
// list has a favicon generated at build time. themeCustom is only read when
// themeCustomSet is true, and it has passed Palette.Validate before being stored.
var themeCustom model.Palette
var themeCustomSet bool
var ThemeCurrent = ""
var ThemePrimaryColor = ""

// themeMode is the site setting that chooses between the light and the dark
// values of a theme, or leaves the choice to the device. Guarded by themeMu.
var themeMode = model.ThemeModeAuto

func Init() {
	initPageInlineStyle()
	UpdatePagePalettes()
}

// 页面内缓存
var _CACHE_PAGE_INLINE_STYLE template.CSS

// 用于mdi
var CACHE_APP_CURRENT_THEME_PRIMARY_COLOR string

func GetPageInlineStyle() template.CSS {
	return _CACHE_PAGE_INLINE_STYLE
}

func initPageInlineStyle() {
	if AppFlags.DebugMode {
		return
	}

	_CACHE_PAGE_INLINE_STYLE = template.CSS(PAGE_INLINE_STYLE)
}

// 页面 body cssvar 样式
var _CACHE_PAGE_BODY_THEME_NAME template.CSS

// The rules of the selected theme for a style element, used instead of the
// style attribute when the device chooses between light and dark.
var _CACHE_PAGE_THEME_RULES template.CSS

// PageTheme is what a themed page takes from the selected theme, read at one
// moment. A theme reaches the page in one of two ways. BodyStyle is the style
// attribute of the body: the custom properties of the theme. Rules is the
// content of a style element with the same properties for light and for dark,
// each in a media query; it is used when the theme has different values for
// the two and the site leaves the choice to the device. The one that is not
// used is empty.
type PageTheme struct {
	BodyStyle template.CSS
	Rules     template.CSS
}

// GetPageTheme returns the selected theme for one page. Every themed page
// calls it once per response, which makes it the place where a change of the
// selected theme's file is noticed.
func GetPageTheme() PageTheme {
	refreshSelectedTheme()
	themeMu.RLock()
	defer themeMu.RUnlock()
	return PageTheme{BodyStyle: _CACHE_PAGE_BODY_THEME_NAME, Rules: _CACHE_PAGE_THEME_RULES}
}

// ThemeTitle returns the name a theme file gives itself in an interface
// language, and the name of the theme where the file gives none or the theme
// is not a file.
func ThemeTitle(name, locale string) string {
	themeMu.RLock()
	defer themeMu.RUnlock()
	if definition, known := themeDefinition(name); known {
		return definition.DisplayName(locale)
	}
	return name
}

// GetPageThemeFor returns a theme for one page as GetPageTheme returns the
// selected one, without selecting it: a preview shows a single response with
// it, and every other response keeps the selected theme. It reports false
// for a name that is not a theme in use.
func GetPageThemeFor(name string) (PageTheme, bool) {
	themeMu.RLock()
	defer themeMu.RUnlock()
	definition, known := themeDefinition(name)
	if !known {
		return PageTheme{}, false
	}
	return pageThemeOf(definition, themeMode), true
}

// pageThemeOf writes a theme for a page under a site setting for light and
// dark: into the style attribute, or as rules when the device chooses.
func pageThemeOf(theme model.Theme, mode string) PageTheme {
	shown, dark, deviceChooses := themeInMode(theme, mode)
	if deviceChooses {
		// #nosec G203 -- CSS values come from theme files accepted by ThemeFile.Validate.
		return PageTheme{Rules: template.CSS(themeRules(shown, dark))}
	}
	// #nosec G203 -- CSS values come from theme files accepted by ThemeFile.Validate or from the custom palette, which is stored only after Palette.Validate accepted it.
	return PageTheme{BodyStyle: template.CSS(themeStyle(shown))}
}

// GetAppBodyStyle returns the style attribute of the page body for the
// selected theme. It is empty when the theme is written as rules instead.
func GetAppBodyStyle() template.CSS {
	return GetPageTheme().BodyStyle
}

// ThemeSnapshot returns a consistent theme name/color pair for one resource.
func ThemeSnapshot() (string, string) {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return ThemeCurrent, ThemePrimaryColor
}

// ThemeRenderSnapshot also detects an ABA theme change during HTML generation.
func ThemeRenderSnapshot() (string, string, uint64) {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return ThemeCurrent, ThemePrimaryColor, themeGeneration
}

// IsKnownTheme reports whether a theme can be selected: a built-in theme, a
// theme file in use, or the custom theme once a palette has been stored for it.
func IsKnownTheme(theme string) bool {
	themeMu.RLock()
	defer themeMu.RUnlock()
	_, known := themePalette(theme)
	return known
}

func GetThemePrimaryColor(theme string) string {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return themePrimaryColor(theme)
}

// CurrentThemePalette returns the colors in use: those of the mode the site
// is set to, and the light ones when the device chooses. An unknown theme name
// has no palette of its own, so the first built-in palette is returned for it.
func CurrentThemePalette() model.Palette {
	themeMu.RLock()
	defer themeMu.RUnlock()
	if definition, known := themeDefinition(ThemeCurrent); known {
		shown, _, _ := themeInMode(definition, themeMode)
		return shown.Colors
	}
	return ThemePalettes[0].Colors
}

// ThemeModeState returns the site setting for light and dark, and whether the
// selected theme has different values for the two. The setting has an effect
// only on such a theme.
func ThemeModeState() (string, bool) {
	themeMu.RLock()
	defer themeMu.RUnlock()
	definition, known := themeDefinition(ThemeCurrent)
	if !known {
		return themeMode, false
	}
	return themeMode, themeHasModes(definition)
}

// SetThemeMode publishes the site setting for light and dark, after
// persistence. A value that is not a mode changes nothing.
func SetThemeMode(mode string) bool {
	if !model.IsThemeMode(mode) {
		return false
	}
	themeMu.Lock()
	defer themeMu.Unlock()
	if mode == themeMode {
		return true
	}
	themeMode = mode
	// The theme and its file are the same as before.
	stamp := selectedThemeStamp
	applyTheme(ThemeCurrent)
	selectedThemeStamp = stamp
	return true
}

// SelectedThemeLayout returns the name of the selected theme and the layout
// options it specifies. The bundled themes and custom colors specify none.
func SelectedThemeLayout() (string, map[string]string) {
	themeMu.RLock()
	defer themeMu.RUnlock()
	definition, _ := themeDefinition(ThemeCurrent)
	return ThemeCurrent, maps.Clone(definition.Layout)
}

// HomepageInEffect returns the layout options a page is rendered with: those
// of the site with the layout of the selected theme in place, as far as the
// administrator has not turned it off. It is the Homepage block of the given
// settings when the theme changes nothing. Like GetPageTheme it notices a
// change of the selected theme's file first, so that one page takes its
// layout and its tokens from the same reading of the file.
func HomepageInEffect(options model.Application) *model.HomepageOptions {
	refreshSelectedTheme()
	themeMu.RLock()
	name := ThemeCurrent
	themeMu.RUnlock()
	return HomepageInEffectFor(options, name)
}

// HomepageInEffectFor returns the layout options a page is rendered with
// under a given theme, selected or not: those of the site with the layout of
// that theme in place, as far as the administrator has not turned it off for
// that theme.
func HomepageInEffectFor(options model.Application, name string) *model.HomepageOptions {
	themeMu.RLock()
	definition, _ := themeDefinition(name)
	themeMu.RUnlock()
	return options.Homepage.WithThemeLayout(definition.Layout, options.ThemeSettings[name].IgnoreLayout)
}

// themeHasModes reports whether a theme looks different in light and in dark.
func themeHasModes(theme model.Theme) bool {
	return themeStyle(theme.In(model.ThemeModeLight)) != themeStyle(theme.In(model.ThemeModeDark))
}

// themeInMode resolves a theme for a site setting. It returns the theme as
// pages show it, the theme as they show it on a device that asks for dark,
// and whether the device chooses between the two. When the site is set to
// light or to dark, or the theme is the same in both, there is no choice and
// the two results are equal.
func themeInMode(theme model.Theme, mode string) (model.Theme, model.Theme, bool) {
	light, dark := theme.In(model.ThemeModeLight), theme.In(model.ThemeModeDark)
	switch mode {
	case model.ThemeModeLight:
		return light, light, false
	case model.ThemeModeDark:
		return dark, dark, false
	}
	if themeStyle(light) == themeStyle(dark) {
		return light, light, false
	}
	return light, dark, true
}

// themeDefinition resolves a theme name. Callers hold themeMu.
func themeDefinition(theme string) (model.Theme, bool) {
	if theme == model.ThemeCustomName {
		return model.Theme{Name: theme, Colors: themeCustom}, themeCustomSet
	}
	for _, item := range ThemePalettes {
		if item.Name == theme {
			return item, true
		}
	}
	item, known := userThemeState.themes[theme]
	return item, known
}

// themePalette returns the three colors of a theme. Callers hold themeMu.
func themePalette(theme string) (model.Palette, bool) {
	definition, known := themeDefinition(theme)
	return definition.Colors, known
}

// themePrimaryColor is GetThemePrimaryColor for callers that hold themeMu.
func themePrimaryColor(theme string) string {
	if definition, known := themeDefinition(theme); known {
		shown, _, _ := themeInMode(definition, themeMode)
		return shown.Colors.Primary
	}
	return ThemePalettes[0].Colors.Primary
}

const emptyPageBodyStyle = template.CSS(``)

// SetTheme publishes the body style and icon colors together, after persistence.
func SetTheme(theme string) {
	themeMu.Lock()
	defer themeMu.Unlock()
	applyTheme(theme)
}

// SetCustomTheme stores a custom palette and selects it. The colors are
// written into style attributes and icon markup without further escaping, so a
// palette that does not pass Palette.Validate is rejected and nothing changes.
func SetCustomTheme(palette model.Palette) error {
	palette = palette.Normalized()
	if err := palette.Validate(); err != nil {
		return err
	}
	themeMu.Lock()
	defer themeMu.Unlock()
	themeCustom, themeCustomSet = palette, true
	applyTheme(model.ThemeCustomName)
	return nil
}

// applyTheme publishes one theme. Callers hold themeMu for writing. The state
// of the theme's file is not known here; a caller that has read the themes
// directory records it afterwards, and otherwise the next page reads it.
func applyTheme(theme string) {
	themeGeneration++
	ThemeCurrent = theme
	selectedThemeStamp = themeStamp{}
	ThemePrimaryColor = themePrimaryColor(theme)
	CACHE_APP_CURRENT_THEME_PRIMARY_COLOR = ThemePrimaryColor
	_CACHE_PAGE_BODY_THEME_NAME, _CACHE_PAGE_THEME_RULES = emptyPageBodyStyle, emptyPageBodyStyle
	if definition, known := themeDefinition(theme); known {
		page := pageThemeOf(definition, themeMode)
		_CACHE_PAGE_BODY_THEME_NAME, _CACHE_PAGE_THEME_RULES = page.BodyStyle, page.Rules
	}
}

// themeRules writes a theme with different values for light and for dark as
// rules for a style element. Each set of values is complete and stands in a
// media query of its own, and the two queries exclude each other: with one
// rule that always applied, a token set only for light would remain in
// effect for dark. The rules set the same custom properties on the body as
// the style attribute does for other themes, so they have to follow the
// stylesheet that holds the defaults.
func themeRules(light, dark model.Theme) string {
	return `@media not all and (prefers-color-scheme:dark){body{` + themeStyle(light) +
		`}}@media (prefers-color-scheme:dark){body{` + themeStyle(dark) + `}}`
}

// themeStyle writes a theme as custom properties for the style attribute of
// the page body. The three colors keep the names the page has always set;
// stylesheets read them through the style tokens. Any other token the theme
// sets replaces the default the stylesheet derives.
func themeStyle(theme model.Theme) string {
	style := `--color-background:` + theme.Colors.Background + `;--color-primary:` + theme.Colors.Primary + `;--color-accent:` + theme.Colors.Accent + `;`
	for _, token := range model.ThemeTokenNames[3:] {
		if value, set := theme.Tokens[token]; set {
			style += `--flare-` + token + `:` + value + `;`
		}
	}
	return style
}

// UpdatePagePalettes applies the theme stored in config.yml and reads the
// theme files of the instance. config.yml can be edited by hand, so the custom
// palette is validated again here; a custom theme without a usable palette
// falls back to the first built-in theme instead of rendering unstyled pages.
func UpdatePagePalettes() {
	userThemeScanMu.Lock()
	defer userThemeScanMu.Unlock()
	// One reading of config.yml gives the selection, the mode and the
	// settings the layout of the theme is weighed against.
	options, readErr := data.GetAllSettingsOptions()
	if readErr != nil {
		options = model.Application{}
	}
	theme, custom, mode := options.Theme, options.ThemeCustom, options.ThemeMode
	stamp := statTheme(theme)
	scan := scanUserThemes()
	themeMu.Lock()
	defer themeMu.Unlock()
	userThemeState = scan
	themeMode = model.ThemeModeAuto
	if model.IsThemeMode(mode) {
		themeMode = mode
	} else if mode != "" {
		logger.GetLogger().Warn("ThemeMode in config.yml is not auto, light or dark; using auto", "value", mode)
	}
	themeCustom, themeCustomSet = model.Palette{}, false
	if custom != nil {
		palette := custom.Normalized()
		if err := palette.Validate(); err != nil {
			logger.GetLogger().Warn("ThemeCustom in config.yml is ignored", "reason", err.Error())
		} else {
			themeCustom, themeCustomSet = palette, true
		}
	}
	if theme == model.ThemeCustomName && !themeCustomSet {
		logger.GetLogger().Warn("Theme custom has no usable ThemeCustom palette; using a bundled theme", "theme", ThemePalettes[0].Name)
		theme = ThemePalettes[0].Name
	}
	applyTheme(theme)
	selectedThemeStamp = stamp
	definition, known := themeDefinition(theme)
	if !known && theme != "" {
		logger.GetLogger().Warn("Theme is not a bundled theme and has no usable theme file; pages use the colors of the stylesheet", "theme", theme)
	}
	// Hand-edited values under Homepage have no effect on these options
	// while the theme is selected, which is easy to miss in the file.
	if applied := options.Homepage.ThemeLayoutApplied(definition.Layout, options.ThemeSettings[theme].IgnoreLayout); len(applied) > 0 {
		logger.GetLogger().Info("The selected theme specifies layout options that replace those under Homepage; the appearance settings can turn them off", "theme", theme, "options", strings.Join(applied, ","))
	}
}
