package model

import "fmt"

// HomepageOptions controls presentation without changing stored bookmark order.
type HomepageOptions struct {
	MobileBookmarkLayout string `yaml:"MobileBookmarkLayout,omitempty"`
}

// Validate rejects unsupported choices instead of accepting arbitrary CSS values.
func (o HomepageOptions) Validate() error {
	switch o.MobileBookmarkLayout {
	case "", "single", "compact":
		return nil
	}
	return fmt.Errorf("invalid mobile bookmark layout: %q", o.MobileBookmarkLayout)
}

// Classes resolves absent and invalid legacy options to the sequential default.
func (o *HomepageOptions) Classes() string {
	if o != nil && o.MobileBookmarkLayout == "compact" {
		return "mobile-bookmarks-compact "
	}
	return ""
}
