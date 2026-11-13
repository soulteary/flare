package data

import (
	"os"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/model"
)

func TestSaveThemeCustomStoresAndSelectsThePalette(t *testing.T) {
	t.Chdir(t.TempDir())
	raw := "# kept comment\nTitle: Mine\nTheme: cloud\nFutureOption: value\n"
	if err := os.WriteFile("config.yml", []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	if theme, stored := GetThemeSelection(); theme != "cloud" || stored != nil {
		t.Fatalf("before saving: %q %+v", theme, stored)
	}
	palette := model.Palette{Background: "#10241B", Primary: " #e8f1e4", Accent: "#f2b84b"}
	want := palette.Normalized()
	if err := SaveThemeCustom(palette); err != nil {
		t.Fatal(err)
	}
	theme, stored := GetThemeSelection()
	if theme != model.ThemeCustomName || stored == nil || *stored != want {
		t.Fatalf("after saving: %q %+v", theme, stored)
	}
	saved, err := os.ReadFile("config.yml")
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range []string{"# kept comment", "Title: Mine", "FutureOption: value", "Theme: custom", "ThemeCustom:", "Background: '#10241b'", "Primary: '#e8f1e4'", "Accent: '#f2b84b'"} {
		if !strings.Contains(string(saved), line) {
			t.Errorf("config.yml lacks %q:\n%s", line, saved)
		}
	}

	// Callers receive a copy of the cached configuration.
	stored.Background = "#000000"
	if _, again := GetThemeSelection(); again == nil || *again != want {
		t.Fatalf("a caller changed the cached palette: %+v", again)
	}

	// A built-in theme keeps the palette for a later return to it.
	if err = SaveThemeName("cloud"); err != nil {
		t.Fatal(err)
	}
	if theme, stored = GetThemeSelection(); theme != "cloud" || stored == nil || *stored != want {
		t.Fatalf("after choosing a built-in theme: %q %+v", theme, stored)
	}
}

func TestSaveThemeCustomRejectsInvalidPalette(t *testing.T) {
	t.Chdir(t.TempDir())
	raw := "Theme: cloud\n"
	if err := os.WriteFile("config.yml", []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	for name, palette := range map[string]model.Palette{
		"format":   {Background: "#000;background:url(x)", Primary: "#ffffff", Accent: "#ffffff"},
		"contrast": {Background: "#202020", Primary: "#2a2a2a", Accent: "#ffffff"},
	} {
		if err := SaveThemeCustom(palette); SaveFailureReasonOf(err) != SaveFailureInvalidInput {
			t.Errorf("%s: got %v, want an invalid-input failure", name, err)
		}
	}
	if after, err := os.ReadFile("config.yml"); err != nil || string(after) != raw {
		t.Fatalf("a rejected palette changed config.yml: %q %v", after, err)
	}
}

func TestThemeSelectionWithUnreadableConfiguration(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir("config.yml", 0700); err != nil {
		t.Fatal(err)
	}
	if theme, stored := GetThemeSelection(); theme != "" || stored != nil {
		t.Fatalf("got %q %+v", theme, stored)
	}
	err := SaveThemeCustom(model.Palette{Background: "#10241b", Primary: "#e8f1e4", Accent: "#f2b84b"})
	if SaveFailureReasonOf(err) != SaveFailureConfigUnreadable {
		t.Fatalf("got %v, want a configuration-unreadable failure", err)
	}
}

// config.yml is edited by hand. A ThemeCustom block of the wrong shape must not
// make the other settings unreadable, and a block that holds no color is left
// in the file as it was written when other settings are saved.
func TestMalformedThemeCustomKeepsConfigurationReadable(t *testing.T) {
	for name, tc := range map[string]struct {
		block string
		kept  string
	}{
		"scalar":            {"ThemeCustom: red\n", "ThemeCustom: red"},
		"list":              {"ThemeCustom:\n  - '#10241b'\n", "- '#10241b'"},
		"empty block":       {"ThemeCustom: {}\n", "ThemeCustom: {}"},
		"unquoted colors":   {"ThemeCustom:\n  Background: #10241b\n  Primary: #e8f1e4\n", "Background:"},
		"nested value":      {"ThemeCustom:\n  Background:\n    - red\n  Primary: '#e8f1e4'\n", "- red"},
		"number as a color": {"ThemeCustom:\n  Background: 10241\n  Primary: '#e8f1e4'\n  Accent: '#f2b84b'\n", ""},
	} {
		t.Run(name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("config.yml", []byte("Title: Mine\nTheme: cloud\n"+tc.block), 0600); err != nil {
				t.Fatal(err)
			}
			options, err := GetAllSettingsOptions()
			if err != nil || options.Title != "Mine" || options.Theme != "cloud" {
				t.Fatalf("configuration is not readable: %+v %v", options, err)
			}
			if stored := options.ThemeCustom; stored != nil && stored.Validate() == nil {
				t.Fatalf("malformed block produced a usable palette: %+v", stored)
			}
			if tc.kept == "" {
				return
			}
			if options.ThemeCustom != nil {
				t.Fatalf("a block without colors is reported as a palette: %+v", options.ThemeCustom)
			}
			if err = SaveThemeName("tron"); err != nil {
				t.Fatal(err)
			}
			saved, err := os.ReadFile("config.yml")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(saved), "Theme: tron") || !strings.Contains(string(saved), tc.kept) {
				t.Fatalf("saving another setting rewrote the block:\n%s", saved)
			}
		})
	}
}
