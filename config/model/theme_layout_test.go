package model

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func TestThemeLayoutOptionsAndValues(t *testing.T) {
	if len(themeLayoutValues) != len(ThemeLayoutOptions) {
		t.Fatalf("%d options with values, %d options listed", len(themeLayoutValues), len(ThemeLayoutOptions))
	}
	for _, option := range ThemeLayoutOptions {
		if !IsThemeLayoutOption(option) || len(themeLayoutValues[option]) < 2 {
			t.Errorf("option %s has no choice of values", option)
		}
	}
	// What a theme cannot specify: the text size is for the reader, and
	// the rest is behavior or content.
	for _, option := range []string{"NavigationTextSize", "IgnoreThemeLayout", "KeepLetterCase", "IconMode", "ShowApps", "desktopColumns", "", "Theme"} {
		if IsThemeLayoutOption(option) {
			t.Errorf("%q is not for a theme to specify", option)
		}
	}
	for option, rejected := range map[string][]string{
		LayoutDesktopColumns:       {"", "3", "7", "12", "06", "Auto", "auto ", "four"},
		LayoutHideAppDescriptions:  {"", "yes", "no", "1", "0", "True"},
		LayoutMobileBookmarkLayout: {"", "single", "Compact", "desktop"},
		LayoutTitleSize:            {"", "large", "larger", "Compact"},
		LayoutCategoryNavigation:   {"", "tab", "none", "Tabs"},
		LayoutBookmarkLayout:       {"", "masonry", "Groups"},
		"NavigationTextSize":       {"large", "standard"},
	} {
		for _, value := range rejected {
			if IsThemeLayoutValue(option, value) {
				t.Errorf("%s: %q should not be a value a theme can give", option, value)
			}
		}
	}
}

// Every value a theme can give is one the site accepts for the same option,
// so a layout in effect is always a layout the appearance form could save.
func TestThemeLayoutValuesAreSiteValues(t *testing.T) {
	for _, option := range ThemeLayoutOptions {
		for _, value := range themeLayoutValues[option] {
			effective := (*HomepageOptions)(nil).WithThemeLayout(map[string]string{option: value}, nil)
			if effective == nil {
				t.Fatalf("%s: %s was not applied", option, value)
			}
			if err := effective.Validate(); err != nil {
				t.Errorf("%s: %s is not accepted by the site: %v", option, value, err)
			}
		}
	}
	// The values show in the classes of the page as the same site settings do.
	layout := map[string]string{
		LayoutMobileBookmarkLayout: "compact", LayoutTitleSize: "prominent", LayoutHideAppDescriptions: "true",
		LayoutBookmarkLayout: "sections", LayoutDesktopColumns: "8", LayoutCategoryNavigation: "collapse",
	}
	effective := (*HomepageOptions)(nil).WithThemeLayout(layout, nil)
	want := HomepageOptions{BookmarkLayout: "sections", DesktopColumns: "8", CategoryNavigation: "collapse", MobileBookmarkLayout: "compact", TitleSize: "prominent", HideAppDescriptions: true}
	if *effective != want {
		t.Fatalf("got %+v", *effective)
	}
	if got := effective.Classes(); got != "bookmark-layout-sections desktop-columns-8 mobile-bookmarks-compact homepage-title-prominent hide-app-descriptions " {
		t.Fatalf("classes: %q", got)
	}
}

func TestWithThemeLayout(t *testing.T) {
	site := HomepageOptions{
		BookmarkLayout: "sections", DesktopColumns: "4", CategoryNavigation: "tabs", MobileBookmarkLayout: "single-column",
		NavigationTextSize: "large", TitleSize: "prominent", HideAppDescriptions: true,
	}
	layout := map[string]string{LayoutDesktopColumns: "6", LayoutTitleSize: "compact", LayoutHideAppDescriptions: "false"}

	// The theme's values replace those of the site; the rest stays, the
	// text size among it.
	original := site
	effective := site.WithThemeLayout(layout, nil)
	want := site
	want.DesktopColumns, want.TitleSize, want.HideAppDescriptions = "6", "compact", false
	if effective == &site || *effective != want {
		t.Fatalf("got %+v", *effective)
	}
	if site != original {
		t.Fatalf("the settings of the site were changed: %+v", site)
	}
	if got := site.ThemeLayoutApplied(layout, nil); !slices.Equal(got, []string{LayoutTitleSize, LayoutHideAppDescriptions, LayoutDesktopColumns}) {
		t.Fatalf("applied options: %v", got)
	}

	// An option the administrator turned off keeps the value of the site.
	effective = site.WithThemeLayout(layout, []string{LayoutDesktopColumns, LayoutHideAppDescriptions})
	want = site
	want.TitleSize = "compact"
	if *effective != want {
		t.Fatalf("with two options turned off: %+v", *effective)
	}
	if got := site.ThemeLayoutApplied(layout, []string{LayoutDesktopColumns, LayoutHideAppDescriptions}); !slices.Equal(got, []string{LayoutTitleSize}) {
		t.Fatalf("applied options: %v", got)
	}

	// The site settings are returned as they are, without a copy, when the
	// theme changes nothing.
	off := site
	off.IgnoreThemeLayout = true
	for name, unchanged := range map[string]*HomepageOptions{
		"no layout":             site.WithThemeLayout(nil, nil),
		"empty layout":          site.WithThemeLayout(map[string]string{}, nil),
		"every option off":      site.WithThemeLayout(layout, ThemeLayoutOptions),
		"unknown option":        site.WithThemeLayout(map[string]string{"NavigationTextSize": "standard", "CardStyle": "icon"}, nil),
		"value that is unknown": site.WithThemeLayout(map[string]string{LayoutDesktopColumns: "12", LayoutTitleSize: "huge"}, nil),
	} {
		if unchanged != &site {
			t.Errorf("%s: a copy was made: %+v", name, unchanged)
		}
	}
	if got := off.WithThemeLayout(layout, nil); got != &off {
		t.Fatalf("the layout of themes is turned off: %+v", got)
	}
	if got := off.ThemeLayoutApplied(layout, nil); got != nil {
		t.Fatalf("applied while turned off: %v", got)
	}

	// A site without a Homepage block starts from the defaults.
	if got := (*HomepageOptions)(nil).WithThemeLayout(nil, nil); got != nil {
		t.Fatalf("nothing to apply to nothing: %+v", got)
	}
	effective = (*HomepageOptions)(nil).WithThemeLayout(map[string]string{LayoutCategoryNavigation: "tabs"}, nil)
	if effective == nil || *effective != (HomepageOptions{CategoryNavigation: "tabs"}) {
		t.Fatalf("from the defaults: %+v", effective)
	}

	// The layout of releases before the option is written as legacy and
	// stored as the empty value.
	effective = site.WithThemeLayout(map[string]string{LayoutDesktopColumns: DesktopColumnsLegacy}, nil)
	if effective.DesktopColumns != "" || strings.Contains(effective.Classes(), "desktop-columns") {
		t.Fatalf("legacy columns: %+v", *effective)
	}
}

func TestThemeFileLayout(t *testing.T) {
	raw := validThemeFile + "layout:\n  DesktopColumns: 6\n  TitleSize: compact\n  HideAppDescriptions: true\n  CategoryNavigation: tabs\n  BookmarkLayout: sections\n  MobileBookmarkLayout: single-column\n"
	file, err := ParseThemeFile([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	theme := file.Theme("arranged")
	want := map[string]string{
		LayoutDesktopColumns: "6", LayoutTitleSize: "compact", LayoutHideAppDescriptions: "true",
		LayoutCategoryNavigation: "tabs", LayoutBookmarkLayout: "sections", LayoutMobileBookmarkLayout: "single-column",
	}
	if len(theme.Layout) != len(want) {
		t.Fatalf("layout: %+v", theme.Layout)
	}
	for option, value := range want {
		if theme.Layout[option] != value {
			t.Errorf("%s: got %q, want %q", option, theme.Layout[option], value)
		}
	}
	// The theme owns its layout, and a mode has none of its own.
	file.Layout[LayoutDesktopColumns] = "8"
	if theme.Layout[LayoutDesktopColumns] != "6" || theme.In(ThemeModeDark).Layout[LayoutDesktopColumns] != "6" {
		t.Fatal("the theme shares its layout with the file it was read from")
	}

	// Quoted and unquoted scalars are the same value.
	for _, written := range []string{"\"6\"", "6", "'6'"} {
		file, err = ParseThemeFile([]byte(validThemeFile + "layout:\n  DesktopColumns: " + written + "\n  HideAppDescriptions: \"false\"\n"))
		if err != nil || file.Layout[LayoutDesktopColumns] != "6" || file.Layout[LayoutHideAppDescriptions] != "false" {
			t.Fatalf("%s: %v %+v", written, err, file.Layout)
		}
	}
	for _, raw := range []string{validThemeFile, validThemeFile + "layout: {}\n", validThemeFile + "layout:\n"} {
		file, err = ParseThemeFile([]byte(raw))
		if err != nil || file.Theme("plain").Layout != nil {
			t.Fatalf("%q: %v %+v", raw, err, file.Layout)
		}
	}
}

// An option or a value this release does not have is left out and reported;
// the theme and the rest of its layout stay in use.
func TestThemeFileLayoutFromAnotherRelease(t *testing.T) {
	long := strings.Repeat("值", 40)
	raw := validThemeFile + "layout:\n  DesktopColumns: 12\n  CardStyle: icon\n  TitleSize: compact\n  BookmarkLayout: " + long + "\n  NavigationTextSize: large\n  HideAppDescriptions: yes\n"
	file, unused, err := ReadThemeFile([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Layout) != 1 || file.Layout[LayoutTitleSize] != "compact" {
		t.Fatalf("layout in use: %+v", file.Layout)
	}
	wantUnused := []string{
		"layout.BookmarkLayout: " + strings.Repeat("值", 32) + "…",
		"layout.CardStyle",
		"layout.DesktopColumns: 12",
		"layout.HideAppDescriptions: yes",
		"layout.NavigationTextSize",
	}
	if !slices.Equal(unused.Keys, wantUnused) {
		t.Fatalf("unused: %q", unused.Keys)
	}
	// Where every part has to be used, the same file is an error.
	if _, err = ParseThemeFile([]byte(raw)); !errors.Is(err, ErrThemeSyntax) {
		t.Fatalf("strict reading: %v", err)
	}

	for name, layout := range map[string]string{
		"a list":           "layout:\n  - DesktopColumns\n",
		"text":             "layout: compact\n",
		"a nested mapping": "layout:\n  DesktopColumns:\n    desktop: 6\n",
	} {
		if _, _, err = ReadThemeFile([]byte(validThemeFile + layout)); !errors.Is(err, ErrThemeSyntax) {
			t.Errorf("%s: got %v", name, err)
		}
	}

	// A theme that is checked without being read from a file has to hold
	// only options and values of this release.
	colors := map[string]string{TokenBackground: "#1a1a1a", TokenPrimary: "#FFFDEA", TokenAccent: "#5c5c5c"}
	var rejection *ThemeFileError
	for layout, subject := range map[string]string{"DesktopColumns=12": "layout.DesktopColumns", "CardStyle=icon": "layout.CardStyle"} {
		option, value, _ := strings.Cut(layout, "=")
		err = ThemeFile{Contract: 1, Tokens: colors, Layout: map[string]string{option: value, LayoutTitleSize: "compact"}}.Validate()
		if !errors.As(err, &rejection) || !errors.Is(err, ErrThemeSyntax) || rejection.Subject != subject {
			t.Errorf("%s: %v", layout, err)
		}
	}
}
