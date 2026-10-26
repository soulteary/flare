package model

import "fmt"

// HomepageOptions controls presentation without changing stored bookmark order.
// Encode zero values explicitly so a settings merge can reset previous choices.
type HomepageOptions struct {
	BookmarkLayout       string `yaml:"BookmarkLayout"`
	DesktopColumns       string `yaml:"DesktopColumns"`
	CategoryNavigation   string `yaml:"CategoryNavigation"`
	MobileBookmarkLayout string `yaml:"MobileBookmarkLayout"`
	NavigationTextSize   string `yaml:"NavigationTextSize"`
	TitleSize            string `yaml:"TitleSize"`
	HideAppDescriptions  bool   `yaml:"HideAppDescriptions"`
}

// Validate rejects unsupported choices instead of accepting arbitrary CSS values.
func (o HomepageOptions) Validate() error {
	switch o.BookmarkLayout {
	case "", "groups", "sections":
	default:
		return fmt.Errorf("invalid bookmark layout: %q", o.BookmarkLayout)
	}
	switch o.DesktopColumns {
	case "", "auto", "4", "5", "6", "8":
	default:
		return fmt.Errorf("invalid desktop columns: %q", o.DesktopColumns)
	}
	switch o.CategoryNavigation {
	case "", "all", "tabs", "collapse":
	default:
		return fmt.Errorf("invalid category navigation: %q", o.CategoryNavigation)
	}
	switch o.MobileBookmarkLayout {
	case "", "single", "compact":
	default:
		return fmt.Errorf("invalid mobile bookmark layout: %q", o.MobileBookmarkLayout)
	}
	switch o.NavigationTextSize {
	case "", "standard", "larger", "large":
	default:
		return fmt.Errorf("invalid navigation text size: %q", o.NavigationTextSize)
	}
	switch o.TitleSize {
	case "", "compact", "standard", "prominent":
	default:
		return fmt.Errorf("invalid title size: %q", o.TitleSize)
	}
	return nil
}

// Classes resolves absent and invalid legacy options to the sequential default.
func (o *HomepageOptions) Classes() string {
	if o == nil {
		return ""
	}
	classes := ""
	if o.BookmarkLayout == "sections" {
		classes += "bookmark-layout-sections "
	}
	switch o.DesktopColumns {
	case "auto", "4", "5", "6", "8":
		classes += "desktop-columns-" + o.DesktopColumns + " "
	}
	if o.MobileBookmarkLayout == "compact" {
		classes += "mobile-bookmarks-compact "
	}
	switch o.NavigationTextSize {
	case "larger", "large":
		classes += "navigation-text-" + o.NavigationTextSize + " "
	}
	switch o.TitleSize {
	case "compact", "prominent":
		classes += "homepage-title-" + o.TitleSize + " "
	}
	if o.HideAppDescriptions {
		classes += "hide-app-descriptions "
	}
	return classes
}
