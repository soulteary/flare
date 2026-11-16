package data

import (
	"os"
	"strings"
	"testing"
)

// The switch for templates of themes and the confirmation for one theme are
// stored in config.yml, and both can be taken back: they are written when
// they are off.
func TestSaveThemeTemplates(t *testing.T) {
	t.Chdir(t.TempDir())
	original := "# The settings of this instance.\nTitle: Mine\nTheme: forest\nFutureKey: kept\n"
	if err := os.WriteFile("config.yml", []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	if options, _ := readThemeSettings(t); options.ThemeTemplates {
		t.Fatal("templates are allowed before anything is stored")
	}

	// Withdrawing what was never confirmed writes nothing.
	if err := SaveThemeTemplatesConfirmed("forest", false); err != nil {
		t.Fatal(err)
	}
	if _, raw := readThemeSettings(t); raw != original {
		t.Fatalf("the file was written:\n%s", raw)
	}

	if err := SaveThemeTemplatesAllowed(true); err != nil {
		t.Fatal(err)
	}
	if err := SaveThemeTemplatesConfirmed("forest", true); err != nil {
		t.Fatal(err)
	}
	options, raw := readThemeSettings(t)
	if !options.ThemeTemplates || !options.ThemeSettings["forest"].Templates || len(options.ThemeSettings) != 1 {
		t.Fatalf("stored: %v %+v", options.ThemeTemplates, options.ThemeSettings)
	}
	for _, line := range []string{"# The settings of this instance.", "Title: Mine", "FutureKey: kept", "ThemeTemplates: true", "Templates: true"} {
		if !strings.Contains(raw, line) {
			t.Errorf("config.yml lacks %q:\n%s", line, raw)
		}
	}

	// The confirmation is kept with what else is stored for the theme, and
	// each of the two is taken back by itself.
	if err := SaveThemeOptions("forest", map[string]string{"density": "compact"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveThemeTemplatesAllowed(false); err != nil {
		t.Fatal(err)
	}
	options, _ = readThemeSettings(t)
	if options.ThemeTemplates || !options.ThemeSettings["forest"].Templates || options.ThemeSettings["forest"].Options["density"] != "compact" {
		t.Fatalf("after the switch is turned off: %v %+v", options.ThemeTemplates, options.ThemeSettings)
	}
	if err := SaveThemeTemplatesAllowed(true); err != nil {
		t.Fatal(err)
	}
	if err := SaveThemeTemplatesConfirmed("forest", false); err != nil {
		t.Fatal(err)
	}
	options, raw = readThemeSettings(t)
	if !options.ThemeTemplates || options.ThemeSettings["forest"].Templates || options.ThemeSettings["forest"].Options["density"] != "compact" {
		t.Fatalf("after the confirmation is withdrawn: %v %+v", options.ThemeTemplates, options.ThemeSettings)
	}
	if !strings.Contains(raw, "Templates: false") {
		t.Fatalf("a withdrawn confirmation is not written:\n%s", raw)
	}

	// Another theme is confirmed by itself.
	if err := SaveThemeTemplatesConfirmed("other", true); err != nil {
		t.Fatal(err)
	}
	options, _ = readThemeSettings(t)
	if options.ThemeSettings["forest"].Templates || !options.ThemeSettings["other"].Templates {
		t.Fatalf("two themes: %+v", options.ThemeSettings)
	}
}

func TestSaveThemeTemplatesRefusesWhatIsNoThemeName(t *testing.T) {
	t.Chdir(t.TempDir())
	original := "Theme: forest\n"
	if err := os.WriteFile("config.yml", []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"", "Forest", "../forest", "a b", strings.Repeat("a", 80)} {
		if err := SaveThemeTemplatesConfirmed(name, true); SaveFailureReasonOf(err) != SaveFailureInvalidInput {
			t.Fatalf("%q: %v", name, err)
		}
	}
	if _, raw := readThemeSettings(t); raw != original {
		t.Fatalf("the file was written:\n%s", raw)
	}
}

// A file that cannot be read is not written over.
func TestSaveThemeTemplatesWithUnreadableConfiguration(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("config.yml", []byte("Title: [\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, save := range []func() error{
		func() error { return SaveThemeTemplatesAllowed(true) },
		func() error { return SaveThemeTemplatesConfirmed("forest", true) },
	} {
		if err := save(); SaveFailureReasonOf(err) != SaveFailureConfigUnreadable {
			t.Fatalf("%v", err)
		}
	}
}

// The confirmations of several themes are withdrawn in one write, and
// nothing is written where none of them was confirmed.
func TestWithdrawThemeTemplates(t *testing.T) {
	t.Chdir(t.TempDir())
	original := "Title: Mine\nThemeTemplates: true\nThemeSettings:\n    zelda:\n        IgnoreLayout: []\n        Options:\n            frames: \"true\"\n        Templates: true\n" +
		"    night:\n        IgnoreLayout: []\n        Templates: true\n    other:\n        IgnoreLayout: []\n        Templates: true\n    plain:\n        IgnoreLayout: []\n        Templates: false\n"
	if err := os.WriteFile("config.yml", []byte(original), 0600); err != nil {
		t.Fatal(err)
	}
	withdrawn, err := WithdrawThemeTemplates([]string{"plain", "absent"})
	if err != nil || withdrawn != nil {
		t.Fatalf("%v %v", withdrawn, err)
	}
	if _, raw := readThemeSettings(t); raw != original {
		t.Fatalf("the file was written:\n%s", raw)
	}
	if withdrawn, err = WithdrawThemeTemplates(nil); err != nil || withdrawn != nil {
		t.Fatalf("%v %v", withdrawn, err)
	}

	withdrawn, err = WithdrawThemeTemplates([]string{"night", "plain", "absent", "zelda"})
	if err != nil || len(withdrawn) != 2 || withdrawn[0] != "night" || withdrawn[1] != "zelda" {
		t.Fatalf("%v %v", withdrawn, err)
	}
	options, _ := readThemeSettings(t)
	stored := options.ThemeSettings
	if stored["zelda"].Templates || stored["night"].Templates || !stored["other"].Templates || stored["plain"].Templates {
		t.Fatalf("stored: %+v", stored)
	}
	if _, known := stored["absent"]; known || len(stored) != 4 {
		t.Fatalf("stored: %+v", stored)
	}
	// What else is stored for a theme is kept, and so is the switch.
	if stored["zelda"].Options["frames"] != "true" || !options.ThemeTemplates {
		t.Fatalf("stored: %+v %v", stored, options.ThemeTemplates)
	}
}
