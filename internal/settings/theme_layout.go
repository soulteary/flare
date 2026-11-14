package settings

import (
	"strings"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
)

// The settings pages name a layout option and its values as the appearance
// form does, so a remark about the layout of a theme reads like the control
// it is about.
var layoutOptionLabels = map[string]string{
	model.LayoutMobileBookmarkLayout: "mobile_bookmark_layout",
	model.LayoutTitleSize:            "homepage_title_size",
	model.LayoutHideAppDescriptions:  "app_descriptions",
	model.LayoutCategoryNavigation:   "category_navigation",
	model.LayoutBookmarkLayout:       "bookmark_layout",
	model.LayoutDesktopColumns:       "desktop_columns",
}

var layoutValueLabels = map[string]map[string]string{
	model.LayoutMobileBookmarkLayout: {
		model.MobileBookmarkLayoutClassic:      "mobile_bookmark_classic",
		model.MobileBookmarkLayoutSingleColumn: "mobile_bookmark_single",
		model.MobileBookmarkLayoutCompact:      "mobile_bookmark_compact",
	},
	model.LayoutTitleSize:           {"standard": "size_standard", "compact": "size_compact", "prominent": "size_prominent"},
	model.LayoutHideAppDescriptions: {"false": "show", "true": "hide"},
	model.LayoutCategoryNavigation:  {"all": "category_navigation_all", "tabs": "category_navigation_tabs", "collapse": "category_navigation_collapse"},
	model.LayoutBookmarkLayout:      {"groups": "bookmark_layout_groups", "sections": "bookmark_layout_sections"},
	model.LayoutDesktopColumns:      {model.DesktopColumnsLegacy: "desktop_columns_default", "auto": "desktop_columns_auto"},
}

// LayoutOptionLabel returns the name of a layout option in an interface
// language.
func LayoutOptionLabel(locale, option string) string {
	if key, known := layoutOptionLabels[option]; known {
		return i18n.T(locale, key)
	}
	return option
}

// LayoutValueLabel returns the name of a value of a layout option in an
// interface language. A number of columns is its own name.
func LayoutValueLabel(locale, option, value string) string {
	if key, known := layoutValueLabels[option][value]; known {
		return i18n.T(locale, key)
	}
	return value
}

// LayoutOptionList names layout options in the order of the appearance page,
// as one text.
func LayoutOptionList(locale string, options []string) string {
	names := make([]string, 0, len(options))
	for _, option := range model.ThemeLayoutOptions {
		for _, listed := range options {
			if listed == option {
				names = append(names, LayoutOptionLabel(locale, option))
			}
		}
	}
	return strings.Join(names, i18n.T(locale, "list_separator"))
}
