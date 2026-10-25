package model

import "testing"

func TestHomepageLayoutChoices(t *testing.T) {
	for _, value := range []string{"", "single", "compact"} {
		options := &HomepageOptions{MobileBookmarkLayout: value}
		if err := options.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	if (&HomepageOptions{MobileBookmarkLayout: "injected"}).Validate() == nil {
		t.Fatal("invalid layout accepted")
	}
	var absent *HomepageOptions
	if absent.Classes() != "" {
		t.Fatal("old configurations must keep sequential layout")
	}
	if (&HomepageOptions{MobileBookmarkLayout: "compact"}).Classes() != "mobile-bookmarks-compact " {
		t.Fatal("compact class missing")
	}
}

func TestHomepageReadabilityChoices(t *testing.T) {
	for _, size := range []string{"", "standard", "larger", "large"} {
		if err := (&HomepageOptions{NavigationTextSize: size}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, size := range []string{"", "compact", "standard", "prominent"} {
		if err := (&HomepageOptions{TitleSize: size}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, options := range []HomepageOptions{{NavigationTextSize: "bad"}, {TitleSize: "bad"}} {
		if options.Validate() == nil {
			t.Fatal("invalid choice accepted")
		}
	}
	options := &HomepageOptions{NavigationTextSize: "large", TitleSize: "compact", HideAppDescriptions: true}
	if options.Classes() != "navigation-text-large homepage-title-compact hide-app-descriptions " {
		t.Fatal(options.Classes())
	}
	if (&HomepageOptions{NavigationTextSize: "injected class"}).Classes() != "" {
		t.Fatal("arbitrary classes accepted")
	}
}

func TestCategoryNavigationModes(t *testing.T) {
	for _, mode := range []string{"", "all", "tabs", "collapse"} {
		if err := (HomepageOptions{CategoryNavigation: mode}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
	if (HomepageOptions{CategoryNavigation: "<invalid>"}).Validate() == nil {
		t.Fatal("invalid mode accepted")
	}
}
