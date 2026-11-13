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

func GetAppBodyStyle() template.CSS {
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

// IsKnownTheme reports whether a theme can be selected: a built-in theme, or
// the custom theme once a palette has been stored for it.
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

// themePalette resolves a theme name. Callers hold themeMu.
func themePalette(theme string) (model.Palette, bool) {
	if theme == model.ThemeCustomName {
		return themeCustom, themeCustomSet
	}
	for _, item := range ThemePalettes {
		if item.Name == theme {
			return item.Colors, true
		}
	}
	return model.Palette{}, false
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

// applyTheme publishes one theme. Callers hold themeMu for writing.
func applyTheme(theme string) {
	themeGeneration++
	ThemeCurrent = theme
	ThemePrimaryColor = themePrimaryColor(theme)
	CACHE_APP_CURRENT_THEME_PRIMARY_COLOR = ThemePrimaryColor
	_CACHE_PAGE_BODY_THEME_NAME = emptyPageBodyStyle
	if palette, known := themePalette(theme); known {
		// #nosec G203 -- CSS values come from the bundled palettes or from the custom palette, which is stored only after Palette.Validate accepted it (#RRGGBB only).
		_CACHE_PAGE_BODY_THEME_NAME = template.CSS(`--color-background:` + palette.Background + `;--color-primary:` + palette.Primary + `;--color-accent:` + palette.Accent + `;`)
	}
}

// UpdatePagePalettes applies the theme stored in config.yml. The file can be
// edited by hand, so the custom palette is validated again here; a custom
// theme without a usable palette falls back to the first built-in theme
// instead of rendering unstyled pages.
func UpdatePagePalettes() {
	theme, custom := data.GetThemeSelection()
	themeMu.Lock()
	defer themeMu.Unlock()
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
}

func getDefaultThemePalettes() []model.Theme {
	return []model.Theme{
		{
			Name:   "blackboard",
			Colors: model.Palette{Background: "#1a1a1a", Primary: "#FFFDEA", Accent: "#5c5c5c"},
		},
		{
			Name:   "gazette",
			Colors: model.Palette{Background: "#F2F7FF", Primary: "#000000", Accent: "#5c5c5c"},
		},
		{
			Name:   "espresso",
			Colors: model.Palette{Background: "#21211F", Primary: "#D1B59A", Accent: "#4E4E4E"},
		},
		{
			Name:   "cab",
			Colors: model.Palette{Background: "#F6D305", Primary: "#1F1F1F", Accent: "#424242"},
		},
		{
			Name:   "cloud",
			Colors: model.Palette{Background: "#f1f2f0", Primary: "#35342f", Accent: "#37bbe4"},
		},
		{
			Name:   "lime",
			Colors: model.Palette{Background: "#263238", Primary: "#AABBC3", Accent: "#aeea00"},
		},
		{
			Name:   "white",
			Colors: model.Palette{Background: "#ffffff", Primary: "#222222", Accent: "#dddddd"},
		},
		{
			Name:   "tron",
			Colors: model.Palette{Background: "#242B33", Primary: "#EFFBFF", Accent: "#6EE2FF"},
		},
		{
			Name:   "blues",
			Colors: model.Palette{Background: "#2B2C56", Primary: "#EFF1FC", Accent: "#6677EB"},
		},
		{
			Name:   "passion",
			Colors: model.Palette{Background: "#f5f5f5", Primary: "#12005e", Accent: "#8e24aa"},
		},
		{
			Name:   "chalk",
			Colors: model.Palette{Background: "#263238", Primary: "#AABBC3", Accent: "#FF869A"},
		},
		{
			Name:   "paper",
			Colors: model.Palette{Background: "#F8F6F1", Primary: "#4C432E", Accent: "#AA9A73"},
		},
		{
			Name:   "neon",
			Colors: model.Palette{Background: "#091833", Primary: "#EFFBFF", Accent: "#ea00d9"},
		},
		{
			Name:   "pumpkin",
			Colors: model.Palette{Background: "#2d3436", Primary: "#EFFBFF", Accent: "#ffa500"},
		},
		{
			Name:   "onedark",
			Colors: model.Palette{Background: "#282c34", Primary: "#dfd9d6", Accent: "#98c379"},
		},
	}
}
