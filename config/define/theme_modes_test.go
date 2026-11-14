package define

import (
	"html/template"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

const (
	// A theme that is dark as written and gives other values for light.
	pairedThemeFile = userThemeColors + "  radius: 8px\n" +
		"modes:\n  light:\n    background: \"#f6f3ea\"\n    primary: \"#2b2a27\"\n    muted: \"#6b6860\"\n"
	pairedLightStyle = "--color-background:#f6f3ea;--color-primary:#2b2a27;--color-accent:#f2b84b;--flare-muted:#6b6860;--flare-radius:8px;"
	pairedDarkStyle  = "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#f2b84b;--flare-radius:8px;"
	pairedRules      = "@media not all and (prefers-color-scheme:dark){body{" + pairedLightStyle + "}}@media (prefers-color-scheme:dark){body{" + pairedDarkStyle + "}}"
)

// modeDirectory is userThemeDirectory with the site setting for light and
// dark put back to its default afterwards.
func modeDirectory(t *testing.T, config string) {
	t.Helper()
	userThemeDirectory(t, "blackboard")
	t.Cleanup(func() {
		themeMu.Lock()
		defer themeMu.Unlock()
		themeMode = model.ThemeModeAuto
	})
	require.NoError(t, os.WriteFile("config.yml", []byte(config), 0600))
}

// A theme with different values for light and for dark is written as rules
// while the device chooses, and into the style attribute once the site is
// set to one of the two.
func TestThemeModesReachThePage(t *testing.T) {
	modeDirectory(t, "Theme: paired\n")
	writeUserTheme(t, "paired.yml", pairedThemeFile)
	UpdatePagePalettes()

	mode, choice := ThemeModeState()
	require.Equal(t, model.ThemeModeAuto, mode)
	require.True(t, choice)
	require.Equal(t, PageTheme{Rules: template.CSS(pairedRules)}, GetPageTheme())
	require.Empty(t, GetAppBodyStyle())
	// What needs one set of colors takes the light one.
	_, primary := ThemeSnapshot()
	require.Equal(t, "#2b2a27", primary)
	require.Equal(t, model.Palette{Background: "#f6f3ea", Primary: "#2b2a27", Accent: "#f2b84b"}, CurrentThemePalette())

	_, _, generation := ThemeRenderSnapshot()
	require.True(t, SetThemeMode(model.ThemeModeDark))
	require.Equal(t, PageTheme{BodyStyle: template.CSS(pairedDarkStyle)}, GetPageTheme())
	_, primary = ThemeSnapshot()
	require.Equal(t, "#e8f1e4", primary)
	require.Equal(t, model.Palette{Background: "#10241b", Primary: "#e8f1e4", Accent: "#f2b84b"}, CurrentThemePalette())
	_, _, changed := ThemeRenderSnapshot()
	require.NotEqual(t, generation, changed)
	mode, choice = ThemeModeState()
	require.Equal(t, model.ThemeModeDark, mode)
	require.True(t, choice, "the theme still has two sets of values")

	require.True(t, SetThemeMode(model.ThemeModeLight))
	require.Equal(t, PageTheme{BodyStyle: template.CSS(pairedLightStyle)}, GetPageTheme())
	_, primary = ThemeSnapshot()
	require.Equal(t, "#2b2a27", primary)

	// Setting the mode it already has publishes nothing.
	_, _, generation = ThemeRenderSnapshot()
	require.True(t, SetThemeMode(model.ThemeModeLight))
	_, _, unchanged := ThemeRenderSnapshot()
	require.Equal(t, generation, unchanged)

	// A value that is not a mode changes nothing.
	for _, value := range []string{"", "Dark", "night", "auto;", "light dark"} {
		require.False(t, SetThemeMode(value), "%q", value)
		require.Equal(t, PageTheme{BodyStyle: template.CSS(pairedLightStyle)}, GetPageTheme(), "%q", value)
	}

	require.True(t, SetThemeMode(model.ThemeModeAuto))
	require.Equal(t, PageTheme{Rules: template.CSS(pairedRules)}, GetPageTheme())
}

// Changing the mode does not make the next page read the themes directory:
// the theme and its file are the same as before.
func TestThemeModeChangeReadsNoFile(t *testing.T) {
	modeDirectory(t, "Theme: paired\n")
	writeUserTheme(t, "paired.yml", pairedThemeFile)
	UpdatePagePalettes()
	require.Equal(t, template.CSS(pairedRules), GetPageTheme().Rules)

	writeUserTheme(t, "added.yml", userThemeColors)
	require.True(t, SetThemeMode(model.ThemeModeDark))
	require.Equal(t, template.CSS(pairedDarkStyle), GetAppBodyStyle())
	require.False(t, IsUserTheme("added"))

	// An edit of the file is still followed, in every mode.
	writeUserTheme(t, "paired.yml", pairedThemeFile+"  dark:\n    radius: 2px\n")
	require.Equal(t, template.CSS("--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#f2b84b;--flare-radius:2px;"), GetAppBodyStyle())
	require.True(t, IsUserTheme("added"))
}

// A theme that is the same in light and in dark is written as it always
// was, whatever the site is set to, and the setting has no effect on it.
func TestThemeWithoutModesIgnoresTheSetting(t *testing.T) {
	modeDirectory(t, "Theme: blackboard\n")
	writeUserTheme(t, "plain.yml", userThemeColors+"  radius: 8px\n")
	// The values for light repeat those of the theme, and dark has none.
	writeUserTheme(t, "repeated.yml", userThemeColors+"modes:\n  light:\n    background: \"#10241b\"\n")
	UpdatePagePalettes()

	custom := model.Palette{Background: "#ffffff", Primary: "#222222", Accent: "#888888"}
	styles := map[string]template.CSS{
		"blackboard":          blackboardStyle,
		"plain":               userThemeStyle + "--flare-radius:8px;",
		"repeated":            userThemeStyle,
		model.ThemeCustomName: "--color-background:#ffffff;--color-primary:#222222;--color-accent:#888888;",
	}
	for _, mode := range []string{model.ThemeModeAuto, model.ThemeModeLight, model.ThemeModeDark} {
		require.True(t, SetThemeMode(mode))
		for name, style := range styles {
			if name == model.ThemeCustomName {
				require.NoError(t, SetCustomTheme(custom))
			} else {
				SetTheme(name)
			}
			require.Equal(t, PageTheme{BodyStyle: style}, GetPageTheme(), "%s in %s", name, mode)
			stored, choice := ThemeModeState()
			require.Equal(t, mode, stored)
			require.False(t, choice, "%s in %s", name, mode)
		}
	}
	// A selection without a theme has nothing to choose between either.
	SetTheme("missing")
	_, choice := ThemeModeState()
	require.False(t, choice)
	require.Equal(t, PageTheme{}, GetPageTheme())
}

// A mode can replace any token, and each mode keeps what it does not replace.
func TestThemeModesWithBothSets(t *testing.T) {
	modeDirectory(t, "Theme: both\n")
	writeUserTheme(t, "both.yml", userThemeColors+"  hover: \"#ffffff1a\"\n"+
		"modes:\n  light:\n    background: \"#ffffff\"\n    primary: \"#222222\"\n    hover: \"#0000001a\"\n    font-family: Georgia, serif\n"+
		"  dark:\n    accent: \"#ffcc66\"\n    radius: 0\n")
	UpdatePagePalettes()
	light := "--color-background:#ffffff;--color-primary:#222222;--color-accent:#f2b84b;--flare-hover:#0000001a;--flare-font-family:Georgia, serif;"
	dark := "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#ffcc66;--flare-hover:#ffffff1a;--flare-radius:0;"
	require.Equal(t, PageTheme{Rules: template.CSS("@media not all and (prefers-color-scheme:dark){body{" + light + "}}@media (prefers-color-scheme:dark){body{" + dark + "}}")}, GetPageTheme())
}

// The setting is read from config.yml at startup. The file can be edited by
// hand, so anything that is not a mode leaves the choice to the device.
func TestThemeModeFromConfiguration(t *testing.T) {
	modeDirectory(t, "Theme: paired\n")
	writeUserTheme(t, "paired.yml", pairedThemeFile)
	for stored, want := range map[string]PageTheme{
		"":                          {Rules: template.CSS(pairedRules)},
		"ThemeMode: auto\n":         {Rules: template.CSS(pairedRules)},
		"ThemeMode: light\n":        {BodyStyle: template.CSS(pairedLightStyle)},
		"ThemeMode: dark\n":         {BodyStyle: template.CSS(pairedDarkStyle)},
		"ThemeMode: Dark\n":         {Rules: template.CSS(pairedRules)},
		"ThemeMode: night\n":        {Rules: template.CSS(pairedRules)},
		"ThemeMode: \"dark;x:y\"\n": {Rules: template.CSS(pairedRules)},
	} {
		require.NoError(t, os.WriteFile("config.yml", []byte("Theme: paired\n"+stored), 0600))
		// A previous setting does not survive a configuration without one.
		themeMu.Lock()
		themeMode = model.ThemeModeDark
		themeMu.Unlock()
		UpdatePagePalettes()
		require.Equal(t, want, GetPageTheme(), "%q", stored)
		// The setting in use is always one of the three, which is what the
		// settings page shows as selected.
		mode, _ := ThemeModeState()
		switch stored {
		case "ThemeMode: light\n":
			require.Equal(t, model.ThemeModeLight, mode)
		case "ThemeMode: dark\n":
			require.Equal(t, model.ThemeModeDark, mode)
		default:
			require.Equal(t, model.ThemeModeAuto, mode, "%q", stored)
		}
	}
}
