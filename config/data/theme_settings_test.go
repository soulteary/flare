package data

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/model"
)

func readThemeSettings(t *testing.T) (model.Application, string) {
	t.Helper()
	options, err := GetAllSettingsOptions()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("config.yml")
	if err != nil {
		t.Fatal(err)
	}
	return options, string(raw)
}

// The appearance form stores which layout options of a theme are turned
// off. The list is kept per theme, and a list that becomes empty replaces
// the one in the file.
func TestSaveAppearanceStoresIgnoredThemeLayout(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("config.yml", []byte("Title: Mine\nTheme: forest\nHomepage:\n  DesktopColumns: \"4\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	base := func() model.Application {
		options, _ := readThemeSettings(t)
		return model.Application{Title: options.Title, Homepage: options.Homepage}
	}

	// A theme nothing was turned off for gets no entry.
	update := base()
	update.ThemeSettings = map[string]model.ThemeSettings{"forest": {IgnoreLayout: []string{}}}
	if err := SaveAppearance(update); err != nil {
		t.Fatal(err)
	}
	if options, raw := readThemeSettings(t); options.ThemeSettings != nil || strings.Contains(raw, "ThemeSettings") {
		t.Fatalf("an empty choice was stored:\n%s", raw)
	}

	// The options are stored once each, in the order of the appearance page.
	update = base()
	update.ThemeSettings = map[string]model.ThemeSettings{"forest": {IgnoreLayout: []string{model.LayoutDesktopColumns, model.LayoutTitleSize, model.LayoutDesktopColumns}}}
	if err := SaveAppearance(update); err != nil {
		t.Fatal(err)
	}
	options, raw := readThemeSettings(t)
	if got := options.ThemeSettings["forest"].IgnoreLayout; !slices.Equal(got, []string{model.LayoutTitleSize, model.LayoutDesktopColumns}) {
		t.Fatalf("stored: %v\n%s", got, raw)
	}
	for _, line := range []string{"ThemeSettings:", "forest:", "IgnoreLayout:", "- TitleSize", "- DesktopColumns", "Title: Mine"} {
		if !strings.Contains(raw, line) {
			t.Errorf("config.yml lacks %q:\n%s", line, raw)
		}
	}

	// Another theme has a list of its own, and a save without the field
	// leaves both.
	update = base()
	update.ThemeSettings = map[string]model.ThemeSettings{"paired": {IgnoreLayout: []string{model.LayoutCategoryNavigation}}}
	if err := SaveAppearance(update); err != nil {
		t.Fatal(err)
	}
	if err := SaveAppearance(base()); err != nil {
		t.Fatal(err)
	}
	options, _ = readThemeSettings(t)
	if len(options.ThemeSettings) != 2 || !slices.Equal(options.ThemeSettings["paired"].IgnoreLayout, []string{model.LayoutCategoryNavigation}) ||
		!slices.Equal(options.ThemeSettings["forest"].IgnoreLayout, []string{model.LayoutTitleSize, model.LayoutDesktopColumns}) {
		t.Fatalf("stored: %+v", options.ThemeSettings)
	}

	// Clearing the list of one theme empties it in the file and keeps the other.
	update = base()
	update.ThemeSettings = map[string]model.ThemeSettings{"forest": {IgnoreLayout: nil}}
	if err := SaveAppearance(update); err != nil {
		t.Fatal(err)
	}
	options, raw = readThemeSettings(t)
	if len(options.ThemeSettings["forest"].IgnoreLayout) != 0 || len(options.ThemeSettings["paired"].IgnoreLayout) != 1 {
		t.Fatalf("after clearing: %+v\n%s", options.ThemeSettings, raw)
	}
	if strings.Contains(raw, "- TitleSize") || strings.Contains(raw, "- DesktopColumns") {
		t.Fatalf("the cleared list is still in the file:\n%s", raw)
	}

	// What a caller gets is its own: changing it does not change what the
	// next caller reads.
	options.ThemeSettings["paired"].IgnoreLayout[0] = "changed"
	options.ThemeSettings["added"] = model.ThemeSettings{}
	again, _ := readThemeSettings(t)
	if len(again.ThemeSettings) != 2 || again.ThemeSettings["paired"].IgnoreLayout[0] != model.LayoutCategoryNavigation {
		t.Fatalf("a caller changed the cached settings: %+v", again.ThemeSettings)
	}
}

func TestSaveAppearanceRejectsOtherThemeSettings(t *testing.T) {
	t.Chdir(t.TempDir())
	raw := "Title: Mine\nTheme: forest\n"
	if err := os.WriteFile("config.yml", []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	for name, settings := range map[string]map[string]model.ThemeSettings{
		"option that is not layout":  {"forest": {IgnoreLayout: []string{"NavigationTextSize"}}},
		"unknown option":             {"forest": {IgnoreLayout: []string{model.LayoutTitleSize, "CardStyle"}}},
		"text that is not an option": {"forest": {IgnoreLayout: []string{"TitleSize: compact"}}},
		"name that is not a theme":   {"../config": {IgnoreLayout: []string{model.LayoutTitleSize}}},
		"name in upper case":         {"Forest": {IgnoreLayout: []string{model.LayoutTitleSize}}},
		"empty name":                 {"": {IgnoreLayout: []string{model.LayoutTitleSize}}},
	} {
		err := SaveAppearance(model.Application{Title: "Mine", ThemeSettings: settings})
		if SaveFailureReasonOf(err) != SaveFailureInvalidInput {
			t.Errorf("%s: got %v", name, err)
		}
		if saved, readErr := os.ReadFile("config.yml"); readErr != nil || string(saved) != raw {
			t.Errorf("%s: config.yml changed: %q %v", name, saved, readErr)
		}
	}
}

// The switch for every theme is part of the Homepage block. It is written
// out when it is off as well, so that saving can turn it off again.
func TestIgnoreThemeLayoutIsStoredWithTheHomepageOptions(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("config.yml", []byte("Title: Mine\nHomepage:\n  DesktopColumns: \"4\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, off := range []bool{true, false, true} {
		options, _ := readThemeSettings(t)
		homepage := *options.Homepage
		homepage.IgnoreThemeLayout = off
		if err := SaveAppearance(model.Application{Title: "Mine", Homepage: &homepage}); err != nil {
			t.Fatal(err)
		}
		options, raw := readThemeSettings(t)
		if options.Homepage.IgnoreThemeLayout != off || options.Homepage.DesktopColumns != "4" {
			t.Fatalf("stored %+v, want the switch %v\n%s", *options.Homepage, off, raw)
		}
	}
}
