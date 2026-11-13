package model

import "testing"

func TestHomepageLayoutChoices(t *testing.T) {
	// "single" is the value 0.6.5–0.6.14 stored for an untouched default.
	for value, want := range map[string]struct{ layout, classes string }{
		"":              {"classic", ""},
		"classic":       {"classic", ""},
		"single":        {"classic", ""},
		"single-column": {"single-column", "mobile-bookmarks-single-column "},
		"compact":       {"compact", "mobile-bookmarks-compact "},
	} {
		options := &HomepageOptions{MobileBookmarkLayout: value}
		if err := options.Validate(); err != nil {
			t.Fatal(err)
		}
		if got := options.MobileLayout(); got != want.layout {
			t.Fatalf("%q resolved to %q", value, got)
		}
		if got := options.Classes(); got != want.classes {
			t.Fatalf("%q produced classes %q", value, got)
		}
	}
	injected := &HomepageOptions{MobileBookmarkLayout: "injected"}
	if injected.Validate() == nil {
		t.Fatal("invalid layout accepted")
	}
	if injected.MobileLayout() != "classic" || injected.Classes() != "" {
		t.Fatal("invalid layout must fall back to the default")
	}
	var absent *HomepageOptions
	if absent.Classes() != "" || absent.MobileLayout() != "classic" {
		t.Fatal("configurations without homepage options must keep the classic layout")
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

func TestDesktopColumnChoices(t *testing.T) {
	for _, columns := range []string{"", "auto", "4", "5", "6", "8"} {
		options := HomepageOptions{DesktopColumns: columns}
		if err := options.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, columns := range []string{"3", "7", "999", "4; color:red"} {
		if (HomepageOptions{DesktopColumns: columns}).Validate() == nil {
			t.Fatalf("accepted %q", columns)
		}
	}
}

func TestBookmarkLayoutChoices(t *testing.T) {
	for _, layout := range []string{"", "groups", "sections"} {
		options := HomepageOptions{BookmarkLayout: layout}
		if err := options.Validate(); err != nil {
			t.Fatal(err)
		}
		want := ""
		if layout == "sections" {
			want = "bookmark-layout-sections "
		}
		if options.Classes() != want {
			t.Fatal(options.Classes())
		}
	}
	options := HomepageOptions{BookmarkLayout: "injected class"}
	if options.Validate() == nil || options.Classes() != "" {
		t.Fatal("invalid layout accepted")
	}
}
