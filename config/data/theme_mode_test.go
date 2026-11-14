package data

import (
	"os"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/model"
)

func TestSaveThemeModeStoresTheSetting(t *testing.T) {
	t.Chdir(t.TempDir())
	raw := "# kept comment\nTitle: Mine\nTheme: cloud\nFutureOption: value\n"
	if err := os.WriteFile("config.yml", []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if mode := GetThemeMode(); mode != "" {
		t.Fatalf("before saving: %q", mode)
	}
	// Saving another setting does not add the key.
	if err := SaveThemeName("tron"); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile("config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(saved), "ThemeMode") {
		t.Fatalf("config.yml gained the key without a setting:\n%s", saved)
	}

	for _, mode := range []string{model.ThemeModeDark, model.ThemeModeLight, model.ThemeModeAuto} {
		if err = SaveThemeMode(mode); err != nil {
			t.Fatalf("%s: %v", mode, err)
		}
		if stored := GetThemeMode(); stored != mode {
			t.Fatalf("stored %q, want %q", stored, mode)
		}
		saved, err = os.ReadFile("config.yml")
		if err != nil {
			t.Fatal(err)
		}
		for _, line := range []string{"# kept comment", "Title: Mine", "FutureOption: value", "Theme: tron", "ThemeMode: " + mode} {
			if !strings.Contains(string(saved), line) {
				t.Errorf("%s: config.yml lacks %q:\n%s", mode, line, saved)
			}
		}
	}

	// The theme keeps the setting, and the setting keeps the theme.
	if err = SaveThemeMode(model.ThemeModeDark); err != nil {
		t.Fatal(err)
	}
	if err = SaveThemeName("cloud"); err != nil {
		t.Fatal(err)
	}
	if theme, _ := GetThemeSelection(); theme != "cloud" || GetThemeMode() != model.ThemeModeDark {
		t.Fatalf("after choosing a theme: %q %q", theme, GetThemeMode())
	}
}

func TestSaveThemeModeRejectsOtherValues(t *testing.T) {
	t.Chdir(t.TempDir())
	raw := "Theme: cloud\nThemeMode: light\n"
	if err := os.WriteFile("config.yml", []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"", "Dark", "night", "system", "dark\n", "auto;", "light dark"} {
		err := SaveThemeMode(mode)
		if SaveFailureReasonOf(err) != SaveFailureInvalidInput {
			t.Errorf("%q: got %v", mode, err)
		}
		if saved, readErr := os.ReadFile("config.yml"); readErr != nil || string(saved) != raw {
			t.Errorf("%q: config.yml changed: %q %v", mode, saved, readErr)
		}
	}
}

// config.yml is edited by hand. A value that is not a mode is returned as
// written, for the caller to decide, and survives the saving of other
// settings.
func TestThemeModeIsReadAsWritten(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("config.yml", []byte("Theme: cloud\nThemeMode: night\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if mode := GetThemeMode(); mode != "night" {
		t.Fatalf("stored %q", mode)
	}
	if err := SaveThemeName("tron"); err != nil {
		t.Fatal(err)
	}
	if mode := GetThemeMode(); mode != "night" {
		t.Fatalf("after another save: %q", mode)
	}

	// A configuration that cannot be read has no setting.
	if err := os.Remove("config.yml"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("config.yml", 0700); err != nil {
		t.Fatal(err)
	}
	if mode := GetThemeMode(); mode != "" {
		t.Fatalf("unreadable configuration: %q", mode)
	}
}
