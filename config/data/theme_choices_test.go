package data

import (
	"os"
	"slices"
	"strings"
	"testing"
)

// The themes visitors can choose are stored as a list, which a later save
// replaces and an empty one clears, and the rest of the file is kept.
func TestSaveThemeChoices(t *testing.T) {
	t.Chdir(t.TempDir())
	original := "# The settings of this instance.\nTitle: Mine\nTheme: forest\nFutureKey: kept\n"
	if err := os.WriteFile("config.yml", []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	// An empty list where none is stored writes nothing.
	if err := SaveThemeChoices(nil); err != nil {
		t.Fatal(err)
	}
	if _, raw := readThemeSettings(t); raw != original {
		t.Fatalf("the file was written:\n%s", raw)
	}
	for _, themes := range [][]string{{"tron", "forest", "custom"}, {"forest"}, {}} {
		if err := SaveThemeChoices(themes); err != nil {
			t.Fatal(err)
		}
		options, raw := readThemeSettings(t)
		if !slices.Equal(options.ThemeChoices, themes) {
			t.Fatalf("stored %v for %v", options.ThemeChoices, themes)
		}
		for _, line := range []string{"# The settings of this instance.", "Title: Mine", "Theme: forest", "FutureKey: kept", "ThemeChoices:"} {
			if !strings.Contains(raw, line) {
				t.Errorf("config.yml lacks %q:\n%s", line, raw)
			}
		}
	}
}

func TestSaveThemeChoicesRefusesWhatIsNoThemeName(t *testing.T) {
	t.Chdir(t.TempDir())
	original := "Theme: forest\nThemeChoices:\n    - tron\n"
	if err := os.WriteFile("config.yml", []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "Forest", "../forest", "a b", strings.Repeat("a", 80)} {
		if err := SaveThemeChoices([]string{"forest", name}); SaveFailureReasonOf(err) != SaveFailureInvalidInput {
			t.Fatalf("%q: %v", name, err)
		}
	}
	if _, raw := readThemeSettings(t); raw != original {
		t.Fatalf("the file was written:\n%s", raw)
	}
}

// A file that cannot be read is not written over.
func TestSaveThemeChoicesWithUnreadableConfiguration(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("config.yml", []byte("Title: [\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := SaveThemeChoices([]string{"forest"}); SaveFailureReasonOf(err) != SaveFailureConfigUnreadable {
		t.Fatalf("%v", err)
	}
}
