package data

import (
	"maps"
	"os"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/model"
)

// The values chosen for the options of a theme are stored per theme, as a
// whole: a value that is given up leaves the file.
func TestSaveThemeOptions(t *testing.T) {
	t.Chdir(t.TempDir())
	original := "# The settings of this instance.\nTitle: Mine\nTheme: forest\nFutureKey: kept\n"
	if err := os.WriteFile("config.yml", []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	stored := func() map[string]map[string]string {
		t.Helper()
		options, _ := readThemeSettings(t)
		values := map[string]map[string]string{}
		for theme, settings := range options.ThemeSettings {
			values[theme] = settings.Options
		}
		return values
	}

	// Nothing to store for a theme nothing is stored for writes nothing.
	if err := SaveThemeOptions("forest", nil); err != nil {
		t.Fatal(err)
	}
	if err := SaveThemeOptions("forest", map[string]string{}); err != nil {
		t.Fatal(err)
	}
	if _, raw := readThemeSettings(t); raw != original {
		t.Fatalf("the file was written:\n%s", raw)
	}

	// Values of every kind: a word, a number, on and off, and a color.
	chosen := map[string]string{"density": "compact", "columns": "4", "frames": "true", "shadows": "false", "highlight": "#00ff88"}
	if err := SaveThemeOptions("forest", chosen); err != nil {
		t.Fatal(err)
	}
	if got := stored(); len(got) != 1 || !maps.Equal(got["forest"], chosen) {
		t.Fatalf("stored: %v", got)
	}
	_, raw := readThemeSettings(t)
	for _, line := range []string{"# The settings of this instance.", "Title: Mine", "FutureKey: kept", "ThemeSettings:", "forest:", "Options:", "density: compact", "highlight: '#00ff88'"} {
		if !strings.Contains(raw, line) {
			t.Errorf("config.yml lacks %q:\n%s", line, raw)
		}
	}
	// What is passed in is the caller's to change afterwards.
	chosen["density"] = "changed"
	if got := stored()["forest"]["density"]; got != "compact" {
		t.Fatalf("the stored value followed the caller: %q", got)
	}
	// What is read is the reader's to change.
	stored()["forest"]["density"] = "changed"
	if got := stored()["forest"]["density"]; got != "compact" {
		t.Fatalf("the stored value followed a reader: %q", got)
	}

	// Another theme has values of its own.
	if err := SaveThemeOptions("paired", map[string]string{"density": "normal"}); err != nil {
		t.Fatal(err)
	}
	if got := stored(); len(got) != 2 || got["paired"]["density"] != "normal" || len(got["forest"]) != 5 {
		t.Fatalf("stored: %v", got)
	}

	// The values of a theme are replaced as a whole: one that is no longer
	// chosen is gone from the file, also where the others stay.
	if err := SaveThemeOptions("forest", map[string]string{"density": "normal", "highlight": "#112233"}); err != nil {
		t.Fatal(err)
	}
	got := stored()
	if !maps.Equal(got["forest"], map[string]string{"density": "normal", "highlight": "#112233"}) || got["paired"]["density"] != "normal" {
		t.Fatalf("stored: %v", got)
	}
	if _, raw = readThemeSettings(t); strings.Contains(raw, "frames") || strings.Contains(raw, "columns") || strings.Contains(raw, "shadows") {
		t.Fatalf("a value that was given up is still in the file:\n%s", raw)
	}

	// Other settings are saved around them.
	if err := SaveThemeName("paired"); err != nil {
		t.Fatal(err)
	}
	if err := SaveThemeMode(model.ThemeModeDark); err != nil {
		t.Fatal(err)
	}
	options, _ := readThemeSettings(t)
	update := model.Application{Title: "Renamed", Homepage: options.Homepage,
		ThemeSettings: map[string]model.ThemeSettings{"forest": {IgnoreLayout: []string{model.LayoutTitleSize}}}}
	if err := SaveAppearance(update); err != nil {
		t.Fatal(err)
	}
	options, raw = readThemeSettings(t)
	if options.Title != "Renamed" || options.Theme != "paired" || options.ThemeMode != model.ThemeModeDark {
		t.Fatalf("the other settings: %+v", options)
	}
	forest := options.ThemeSettings["forest"]
	if !maps.Equal(forest.Options, map[string]string{"density": "normal", "highlight": "#112233"}) || len(forest.IgnoreLayout) != 1 || forest.IgnoreLayout[0] != model.LayoutTitleSize {
		t.Fatalf("the settings of the theme: %+v\n%s", forest, raw)
	}
	if options.ThemeSettings["paired"].Options["density"] != "normal" {
		t.Fatalf("the values of the other theme: %+v", options.ThemeSettings["paired"])
	}

	// Without values the stored ones are removed, and what else is stored
	// for the theme stays.
	if err := SaveThemeOptions("forest", nil); err != nil {
		t.Fatal(err)
	}
	options, raw = readThemeSettings(t)
	forest = options.ThemeSettings["forest"]
	if forest.Options != nil || len(forest.IgnoreLayout) != 1 {
		t.Fatalf("after removing the values: %+v\n%s", forest, raw)
	}
	if options.ThemeSettings["paired"].Options["density"] != "normal" || strings.Contains(raw, "highlight") {
		t.Fatalf("after removing the values:\n%s", raw)
	}
	for _, line := range []string{"# The settings of this instance.", "FutureKey: kept"} {
		if !strings.Contains(raw, line) {
			t.Errorf("config.yml lacks %q:\n%s", line, raw)
		}
	}
}

// A key of a later release in the settings of a theme stays in the file when
// the values of its options are replaced.
func TestSaveThemeOptionsKeepsWhatItDoesNotKnow(t *testing.T) {
	t.Chdir(t.TempDir())
	original := "Theme: forest\nThemeSettings:\n    forest:\n        IgnoreLayout: []\n        Later: kept\n        Options:\n            density: compact\n            frames: \"true\"\n    other:\n        Later: also kept\n"
	if err := os.WriteFile("config.yml", []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if err := SaveThemeOptions("forest", map[string]string{"density": "normal"}); err != nil {
		t.Fatal(err)
	}
	options, raw := readThemeSettings(t)
	if !maps.Equal(options.ThemeSettings["forest"].Options, map[string]string{"density": "normal"}) {
		t.Fatalf("stored: %+v", options.ThemeSettings["forest"])
	}
	for _, line := range []string{"Later: kept", "Later: also kept"} {
		if !strings.Contains(raw, line) {
			t.Errorf("config.yml lacks %q:\n%s", line, raw)
		}
	}
	if strings.Contains(raw, "frames") {
		t.Fatalf("a value that was given up is still in the file:\n%s", raw)
	}
}

// Values written into config.yml by hand are read as texts, whatever they
// are written as.
func TestThemeOptionsWrittenByHand(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("config.yml", []byte("Theme: forest\nThemeSettings:\n    forest:\n        Options:\n            frames: true\n            columns: 4\n            density: compact\n            highlight: \"#00ff88\"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	options, _ := readThemeSettings(t)
	want := map[string]string{"frames": "true", "columns": "4", "density": "compact", "highlight": "#00ff88"}
	if got := options.ThemeSettings["forest"].Options; !maps.Equal(got, want) {
		t.Fatalf("read: %v", got)
	}
}

func TestSaveThemeOptionsRejectsWhatIsNoValue(t *testing.T) {
	t.Chdir(t.TempDir())
	original := "Theme: forest\n"
	if err := os.WriteFile("config.yml", []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	many := map[string]string{}
	for _, id := range []string{"a", "b", "c", "d", "e", "f", "g", "h", "i"} {
		many[id] = "true"
	}
	for name, tc := range map[string]struct {
		theme  string
		chosen map[string]string
	}{
		"a theme name with a path":      {"../forest", map[string]string{"density": "compact"}},
		"a theme name in upper case":    {"Forest", map[string]string{"density": "compact"}},
		"no theme name":                 {"", map[string]string{"density": "compact"}},
		"an option in upper case":       {"forest", map[string]string{"Density": "compact"}},
		"an option with a space":        {"forest", map[string]string{"my option": "compact"}},
		"no option name":                {"forest", map[string]string{"": "compact"}},
		"an empty value":                {"forest", map[string]string{"density": ""}},
		"a value with a space":          {"forest", map[string]string{"density": "very compact"}},
		"a value in upper case":         {"forest", map[string]string{"density": "Compact"}},
		"a value with a line break":     {"forest", map[string]string{"density": "compact\nTheme: other"}},
		"a color of three digits":       {"forest", map[string]string{"highlight": "#0f8"}},
		"a color with a declaration":    {"forest", map[string]string{"highlight": "#00ff88;color:red"}},
		"an address":                    {"forest", map[string]string{"highlight": "url(//example.test/)"}},
		"more options than a theme has": {"forest", many},
	} {
		err := SaveThemeOptions(tc.theme, tc.chosen)
		if SaveFailureReasonOf(err) != SaveFailureInvalidInput {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, raw := readThemeSettings(t); raw != original {
		t.Fatalf("the file was written:\n%s", raw)
	}
}
