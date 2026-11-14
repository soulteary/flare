package define

// TODO：挪到合适地方，拆分 assets

import (
	"html/template"
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

// GetAppBodyStyle returns the style attribute of the page body for the
// selected theme. Every themed page calls it once per response, which makes it
// the place where a change of the selected theme's file is noticed.
func GetAppBodyStyle() template.CSS {
	refreshSelectedTheme()
	themeMu.RLock()
	defer themeMu.RUnlock()
	return _CACHE_PAGE_BODY_THEME_NAME
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

// CurrentThemePalette returns the colors in use. An unknown theme name has no
// palette of its own, so the first built-in palette is returned for it.
func CurrentThemePalette() model.Palette {
	themeMu.RLock()
	defer themeMu.RUnlock()
	if palette, known := themePalette(ThemeCurrent); known {
		return palette
	}
	return ThemePalettes[0].Colors
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
	if palette, known := themePalette(theme); known {
		return palette.Primary
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
	selectedThemeStamp = themeFileStamp{}
	ThemePrimaryColor = themePrimaryColor(theme)
	CACHE_APP_CURRENT_THEME_PRIMARY_COLOR = ThemePrimaryColor
	_CACHE_PAGE_BODY_THEME_NAME = emptyPageBodyStyle
	if definition, known := themeDefinition(theme); known {
		// #nosec G203 -- CSS values come from theme files accepted by ThemeFile.Validate or from the custom palette, which is stored only after Palette.Validate accepted it.
		_CACHE_PAGE_BODY_THEME_NAME = template.CSS(themeStyle(definition))
	}
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
	theme, custom := data.GetThemeSelection()
	stamp := statThemeFile(theme)
	scan := scanUserThemes()
	themeMu.Lock()
	defer themeMu.Unlock()
	userThemeState = scan
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
	if _, known := themeDefinition(theme); !known && theme != "" {
		logger.GetLogger().Warn("Theme is not a bundled theme and has no usable theme file; pages use the colors of the stylesheet", "theme", theme)
	}
}
