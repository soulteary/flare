package define

// TODO：挪到合适地方，拆分 assets

import (
	"html/template"
	"sync"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
)

// 程序运行默认使用内置的主题配色
var themeMu sync.RWMutex

var ThemePalettes = getDefaultThemePalettes()
var ThemeCurrent = ""
var ThemePrimaryColor = ""

func Init() {
	initPageInlineStyle()
	UpdatePagePalettes()
	initPagePrimaryColorCache()
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

func initPagePrimaryColorCache() {
	SetTheme(data.GetThemeName())
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

func IsKnownTheme(theme string) bool {
	for _, item := range ThemePalettes {
		if item.Name == theme {
			return true
		}
	}
	return false
}

func GetThemePrimaryColor(theme string) string {
	for _, item := range ThemePalettes {
		if item.Name == theme {
			return item.Colors.Primary
		}
	}
	return ThemePalettes[0].Colors.Primary
}

const emptyPageBodyStyle = template.CSS(``)

// SetTheme publishes the body style and icon colors together, after persistence.
func SetTheme(theme string) {
	themeMu.Lock()
	defer themeMu.Unlock()
	ThemeCurrent = theme
	ThemePrimaryColor = GetThemePrimaryColor(theme)
	CACHE_APP_CURRENT_THEME_PRIMARY_COLOR = ThemePrimaryColor
	_CACHE_PAGE_BODY_THEME_NAME = emptyPageBodyStyle
	for _, item := range ThemePalettes {
		if item.Name == theme {
			// #nosec G203 -- CSS values come exclusively from the bundled palette; the request only selects a known name.
			_CACHE_PAGE_BODY_THEME_NAME = template.CSS(`--color-background:` + item.Colors.Background + `;--color-primary:` + item.Colors.Primary + `;--color-accent:` + item.Colors.Accent + `;`)
			return
		}
	}
}

func UpdatePagePalettes() { SetTheme(data.GetThemeName()) }

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
