package define

import (
	"errors"
	"html/template"
	"os"
	"testing"

	"github.com/soulteary/flare/config/model"
)

// restoreThemeState puts back the theme and the custom palette that a test replaces.
func restoreThemeState(t *testing.T) {
	t.Helper()
	themeMu.RLock()
	name, custom, stored := ThemeCurrent, themeCustom, themeCustomSet
	themeMu.RUnlock()
	t.Cleanup(func() {
		themeMu.Lock()
		defer themeMu.Unlock()
		themeCustom, themeCustomSet = custom, stored
		applyTheme(name)
	})
}

func storedCustomTheme() (model.Palette, bool) {
	themeMu.RLock()
	defer themeMu.RUnlock()
	return themeCustom, themeCustomSet
}

func clearCustomTheme(theme string) {
	themeMu.Lock()
	defer themeMu.Unlock()
	themeCustom, themeCustomSet = model.Palette{}, false
	applyTheme(theme)
}

func TestCustomThemeUsesStoredPalette(t *testing.T) {
	restoreThemeState(t)
	clearCustomTheme("blackboard")
	if IsKnownTheme(model.ThemeCustomName) {
		t.Fatal("the custom theme is selectable before a palette is stored")
	}
	if _, stored := storedCustomTheme(); stored {
		t.Fatal("a custom palette is held before one is stored")
	}

	// A rejected palette changes nothing, including the selectable themes.
	before := GetAppBodyStyle()
	err := SetCustomTheme(model.Palette{Background: "#000;background:url(x)", Primary: "#ffffff", Accent: "#ffffff"})
	if !errors.Is(err, model.ErrPaletteColor) {
		t.Fatalf("got %v, want ErrPaletteColor", err)
	}
	if name, _ := ThemeSnapshot(); name != "blackboard" || GetAppBodyStyle() != before || IsKnownTheme(model.ThemeCustomName) {
		t.Fatalf("rejected palette changed the theme state: %q %q", name, GetAppBodyStyle())
	}

	palette := model.Palette{Background: "#10241B", Primary: " #E8F1E4", Accent: "#f2b84b"}
	want := palette.Normalized()
	_, _, generation := ThemeRenderSnapshot()
	if err = SetCustomTheme(palette); err != nil {
		t.Fatal(err)
	}
	name, color, next := ThemeRenderSnapshot()
	if name != model.ThemeCustomName || color != "#e8f1e4" || next == generation {
		t.Fatalf("snapshot %q %q, generation %d -> %d", name, color, generation, next)
	}
	style := template.CSS("--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#f2b84b;")
	if GetAppBodyStyle() != style {
		t.Fatalf("body style %q", GetAppBodyStyle())
	}
	if CurrentThemePalette() != want || GetThemePrimaryColor(model.ThemeCustomName) != want.Primary {
		t.Fatalf("current palette %+v", CurrentThemePalette())
	}

	// Choosing a built-in theme keeps the palette for a later return to it.
	SetTheme("cloud")
	if stored, ok := storedCustomTheme(); !ok || stored != want {
		t.Fatalf("stored palette after a built-in theme: %+v %v", stored, ok)
	}
	if CurrentThemePalette() == want {
		t.Fatal("the built-in theme reports the custom palette")
	}
	SetTheme(model.ThemeCustomName)
	if GetAppBodyStyle() != style {
		t.Fatalf("body style after returning to the custom theme: %q", GetAppBodyStyle())
	}
}

func TestUnknownThemeHasNoPalette(t *testing.T) {
	restoreThemeState(t)
	SetTheme("no-such-theme")
	if GetAppBodyStyle() != emptyPageBodyStyle {
		t.Fatalf("body style %q", GetAppBodyStyle())
	}
	if CurrentThemePalette() != ThemePalettes[0].Colors || GetThemePrimaryColor("no-such-theme") != ThemePalettes[0].Colors.Primary {
		t.Fatalf("fallback palette %+v", CurrentThemePalette())
	}
}

// The custom color form starts from the colors in use, so every bundled
// palette has to be acceptable to the validation it is submitted to.
func TestBundledPalettesPassCustomValidation(t *testing.T) {
	for _, theme := range ThemePalettes {
		if err := theme.Colors.Normalized().Validate(); err != nil {
			t.Errorf("%s: %v", theme.Name, err)
		}
	}
}

func TestUpdatePagePalettesReadsCustomPalette(t *testing.T) {
	restoreThemeState(t)
	custom := "ThemeCustom:\n  Background: '#10241B'\n  Primary: '#e8f1e4'\n  Accent: '#f2b84b'\n"
	customStyle := template.CSS("--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#f2b84b;")
	first := ThemePalettes[0]
	firstStyle := template.CSS("--color-background:" + first.Colors.Background + ";--color-primary:" + first.Colors.Primary + ";--color-accent:" + first.Colors.Accent + ";")
	for _, tc := range []struct {
		name, config, theme string
		stored              bool
		style               template.CSS
	}{
		{"custom theme", "Theme: custom\n" + custom, model.ThemeCustomName, true, customStyle},
		{"built-in theme keeps the palette", "Theme: " + first.Name + "\n" + custom, first.Name, true, firstStyle},
		{"custom theme without a palette", "Theme: custom\n", first.Name, false, firstStyle},
		{"color that is not hexadecimal", "Theme: custom\nThemeCustom:\n  Background: red\n  Primary: '#e8f1e4'\n  Accent: '#f2b84b'\n", first.Name, false, firstStyle},
		{"color carrying a declaration", "Theme: custom\nThemeCustom:\n  Background: '#000;background:url(//example.test/)'\n  Primary: '#e8f1e4'\n  Accent: '#f2b84b'\n", first.Name, false, firstStyle},
		{"unreadable primary color", "Theme: custom\nThemeCustom:\n  Background: '#202020'\n  Primary: '#2a2a2a'\n  Accent: '#f2b84b'\n", first.Name, false, firstStyle},
		{"incomplete palette", "Theme: custom\nThemeCustom:\n  Background: '#10241b'\n", first.Name, false, firstStyle},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			if err := os.WriteFile("config.yml", []byte(tc.config), 0600); err != nil {
				t.Fatal(err)
			}
			UpdatePagePalettes()
			name, _ := ThemeSnapshot()
			_, stored := storedCustomTheme()
			if name != tc.theme || stored != tc.stored || GetAppBodyStyle() != tc.style {
				t.Fatalf("theme %q stored %v style %q", name, stored, GetAppBodyStyle())
			}
			if IsKnownTheme(model.ThemeCustomName) != tc.stored {
				t.Fatalf("custom theme selectable: %v", !tc.stored)
			}
		})
	}
}
