package model

import "fmt"

// HomepageOptions controls presentation without changing stored bookmark order.
type HomepageOptions struct {
	MobileBookmarkLayout string `yaml:"MobileBookmarkLayout,omitempty"`
	NavigationTextSize   string `yaml:"NavigationTextSize,omitempty"`
	TitleSize            string `yaml:"TitleSize,omitempty"`
	HideAppDescriptions  bool   `yaml:"HideAppDescriptions,omitempty"`
}

// Validate rejects unsupported choices instead of accepting arbitrary CSS values.
func (o HomepageOptions) Validate() error {
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
