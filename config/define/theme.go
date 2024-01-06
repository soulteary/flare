package FlareDefine

// TODO：挪到合适地方，拆分 assets

import (
	"html/template"

	FlareData "github.com/soulteary/flare/config/data"
	FlareModel "github.com/soulteary/flare/config/model"
)

// 程序运行默认使用内置的主题配色
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
var _CACHE_PREV_THEME_NAME string

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
	theme := FlareData.GetThemeName()
	ThemeCurrent = theme
	ThemePrimaryColor = GetThemePrimaryColor(theme)
}

// 页面 body cssvar 样式
var _CACHE_PAGE_BODY_THEME_NAME template.CSS

func GetAppBodyStyle() template.CSS {
	return _CACHE_PAGE_BODY_THEME_NAME
}

func GetThemePrimaryColor(theme string) string {
	if _CACHE_PREV_THEME_NAME == theme {
		return CACHE_APP_CURRENT_THEME_PRIMARY_COLOR
	}
	for _, themePresent := range ThemePalettes {
		if themePresent.Name == theme {
			CACHE_APP_CURRENT_THEME_PRIMARY_COLOR = themePresent.Colors.Primary
			return CACHE_APP_CURRENT_THEME_PRIMARY_COLOR
		}
	}
	return CACHE_APP_CURRENT_THEME_PRIMARY_COLOR
}

const emptyPageBodyStyle = template.CSS(``)

func UpdatePagePalettes() {
	theme := FlareData.GetThemeName()
	for _, themePresent := range ThemePalettes {
		if themePresent.Name == theme {
			_CACHE_PAGE_BODY_THEME_NAME = template.CSS(`--color-background:` + themePresent.Colors.Background + `;--color-primary:` + themePresent.Colors.Primary + `;--color-accent:` + themePresent.Colors.Accent + `;`)
			return
		}
	}
	_CACHE_PAGE_BODY_THEME_NAME = emptyPageBodyStyle
}

func getDefaultThemePalettes() []FlareModel.Theme {
	return []FlareModel.Theme{
		{
			Name:   "blackboard",
			Colors: FlareModel.Palette{Background: "#1a1a1a", Primary: "#FFFDEA", Accent: "#5c5c5c"},
		},
		{
			Name:   "gazette",
			Colors: FlareModel.Palette{Background: "#F2F7FF", Primary: "#000000", Accent: "#5c5c5c"},
		},
		{
			Name:   "espresso",
			Colors: FlareModel.Palette{Background: "#21211F", Primary: "#D1B59A", Accent: "#4E4E4E"},
		},
		{
			Name:   "cab",
			Colors: FlareModel.Palette{Background: "#F6D305", Primary: "#1F1F1F", Accent: "#424242"},
		},
		{
			Name:   "cloud",
			Colors: FlareModel.Palette{Background: "#f1f2f0", Primary: "#35342f", Accent: "#37bbe4"},
		},
		{
			Name:   "lime",
			Colors: FlareModel.Palette{Background: "#263238", Primary: "#AABBC3", Accent: "#aeea00"},
		},
		{
			Name:   "white",
			Colors: FlareModel.Palette{Background: "#ffffff", Primary: "#222222", Accent: "#dddddd"},
		},
		{
			Name:   "tron",
			Colors: FlareModel.Palette{Background: "#242B33", Primary: "#EFFBFF", Accent: "#6EE2FF"},
		},
		{
			Name:   "blues",
			Colors: FlareModel.Palette{Background: "#2B2C56", Primary: "#EFF1FC", Accent: "#6677EB"},
		},
		{
			Name:   "passion",
			Colors: FlareModel.Palette{Background: "#f5f5f5", Primary: "#12005e", Accent: "#8e24aa"},
		},
		{
			Name:   "chalk",
			Colors: FlareModel.Palette{Background: "#263238", Primary: "#AABBC3", Accent: "#FF869A"},
		},
		{
			Name:   "paper",
			Colors: FlareModel.Palette{Background: "#F8F6F1", Primary: "#4C432E", Accent: "#AA9A73"},
		},
		{
			Name:   "neon",
			Colors: FlareModel.Palette{Background: "#091833", Primary: "#EFFBFF", Accent: "#ea00d9"},
		},
		{
			Name:   "pumpkin",
			Colors: FlareModel.Palette{Background: "#2d3436", Primary: "#EFFBFF", Accent: "#ffa500"},
		},
		{
			Name:   "onedark",
			Colors: FlareModel.Palette{Background: "#282c34", Primary: "#dfd9d6", Accent: "#98c379"},
		},
	}
}
