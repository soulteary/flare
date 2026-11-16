package model

import "slices"

// Layout options a theme can specify. They have the names of the options
// under Homepage in config.yml and are listed in the order of the appearance
// page. Options that are not presentation, such as the text size, which is
// for the reader, are not among them.
const (
	LayoutMobileBookmarkLayout = "MobileBookmarkLayout"
	LayoutTitleSize            = "TitleSize"
	LayoutHideAppDescriptions  = "HideAppDescriptions"
	LayoutCategoryNavigation   = "CategoryNavigation"
	LayoutBookmarkLayout       = "BookmarkLayout"
	LayoutDesktopColumns       = "DesktopColumns"
)

// ThemeLayoutOptions lists the options a theme can specify.
var ThemeLayoutOptions = []string{
	LayoutMobileBookmarkLayout, LayoutTitleSize, LayoutHideAppDescriptions,
	LayoutCategoryNavigation, LayoutBookmarkLayout, LayoutDesktopColumns,
}

// DesktopColumnsLegacy is how a theme, like the appearance form, writes the
// empty value of DesktopColumns: the layout of releases before the option.
const DesktopColumnsLegacy = "legacy"

// themeLayoutValues lists the values a theme can give for each option.
var themeLayoutValues = map[string][]string{
	LayoutMobileBookmarkLayout: {MobileBookmarkLayoutClassic, MobileBookmarkLayoutSingleColumn, MobileBookmarkLayoutCompact},
	LayoutTitleSize:            {"standard", "compact", "prominent"},
	LayoutHideAppDescriptions:  {"false", "true"},
	LayoutCategoryNavigation:   {"all", "tabs", "collapse"},
	LayoutBookmarkLayout:       {"groups", "sections"},
	LayoutDesktopColumns:       {DesktopColumnsLegacy, "auto", "4", "5", "6", "8"},
}

// IsThemeLayoutOption reports whether a theme can specify an option.
func IsThemeLayoutOption(option string) bool {
	_, known := themeLayoutValues[option]
	return known
}

// IsThemeLayoutValue reports whether a theme can give a value for an option.
func IsThemeLayoutValue(option, value string) bool {
	return slices.Contains(themeLayoutValues[option], value)
}

// ThemeLayoutUnsupported lists the layout options a theme with templates
// does not support, in the order of the appearance page: those it does not
// name among the options it supports and does not specify itself. A theme
// that does not say what it supports has none.
func ThemeLayoutUnsupported(supported []string, layout map[string]string) []string {
	if supported == nil {
		return nil
	}
	var unsupported []string
	for _, option := range ThemeLayoutOptions {
		if _, specified := layout[option]; !specified && !slices.Contains(supported, option) {
			unsupported = append(unsupported, option)
		}
	}
	return unsupported
}

// ThemeSettings holds what the administrator has set for one theme.
type ThemeSettings struct {
	// IgnoreLayout lists the layout options of the theme that are not
	// applied. The site settings are used for them. The list is encoded
	// when it is empty, so that saving can clear a previous one.
	IgnoreLayout []string `yaml:"IgnoreLayout"`
	// Options holds the values chosen for the options of the theme, by the
	// ID of each option. A value is used only if the theme has the option
	// and the option can have the value; the option has its default
	// otherwise.
	Options map[string]string `yaml:"Options,omitempty"`
	// Templates records that the administrator has confirmed the templates
	// of the theme. They are used while it is set and the site allows
	// themes to use templates. It is encoded when it is off, so that saving
	// can withdraw the confirmation.
	Templates bool `yaml:"Templates"`
}

// ThemeLayoutApplied lists the options of a theme's layout that take effect,
// in the order of the appearance page: those the theme gives a value for and
// the administrator has not turned off, by the list for that theme or for
// every theme at once.
func (o *HomepageOptions) ThemeLayoutApplied(layout map[string]string, ignored []string) []string {
	if len(layout) == 0 || (o != nil && o.IgnoreThemeLayout) {
		return nil
	}
	var applied []string
	for _, option := range ThemeLayoutOptions {
		if value, specified := layout[option]; specified && IsThemeLayoutValue(option, value) && !slices.Contains(ignored, option) {
			applied = append(applied, option)
		}
	}
	return applied
}

// LayoutValues returns the options a theme can specify with the values they
// have, written as a theme writes them. An option without a usable value
// has its default.
func (o *HomepageOptions) LayoutValues() map[string]string {
	var options HomepageOptions
	if o != nil {
		options = *o
	}
	values := map[string]string{
		LayoutMobileBookmarkLayout: options.MobileBookmarkLayout,
		LayoutTitleSize:            options.TitleSize,
		LayoutHideAppDescriptions:  "false",
		LayoutCategoryNavigation:   options.CategoryNavigation,
		LayoutBookmarkLayout:       options.BookmarkLayout,
		LayoutDesktopColumns:       options.DesktopColumns,
	}
	if options.HideAppDescriptions {
		values[LayoutHideAppDescriptions] = "true"
	}
	for option, value := range values {
		if !IsThemeLayoutValue(option, value) {
			values[option] = themeLayoutValues[option][0]
		}
	}
	return values
}

// WithThemeLayout returns the layout in effect: the options of the site with
// the values a theme specifies in place, as far as they take effect. The
// receiver is returned when the theme changes nothing, and is never modified.
func (o *HomepageOptions) WithThemeLayout(layout map[string]string, ignored []string) *HomepageOptions {
	applied := o.ThemeLayoutApplied(layout, ignored)
	if len(applied) == 0 {
		return o
	}
	var result HomepageOptions
	if o != nil {
		result = *o
	}
	for _, option := range applied {
		value := layout[option]
		switch option {
		case LayoutMobileBookmarkLayout:
			result.MobileBookmarkLayout = value
		case LayoutTitleSize:
			result.TitleSize = value
		case LayoutHideAppDescriptions:
			result.HideAppDescriptions = value == "true"
		case LayoutCategoryNavigation:
			result.CategoryNavigation = value
		case LayoutBookmarkLayout:
			result.BookmarkLayout = value
		case LayoutDesktopColumns:
			result.DesktopColumns = value
			if value == DesktopColumnsLegacy {
				result.DesktopColumns = ""
			}
		}
	}
	return &result
}
