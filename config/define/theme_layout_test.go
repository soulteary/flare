package define

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
)

const arrangedThemeFile = userThemeColors + "layout:\n  DesktopColumns: 6\n  TitleSize: compact\n  HideAppDescriptions: true\n"

func layoutSettings(t *testing.T) model.Application {
	t.Helper()
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	return options
}

// The bundled themes specify no layout, so an instance that uses only them
// is rendered with its own settings after an upgrade as before.
func TestBundledThemesSpecifyNoLayout(t *testing.T) {
	for _, theme := range ThemePalettes {
		require.Empty(t, theme.Layout, theme.Name)
		require.Empty(t, theme.Modes, theme.Name)
	}
}

func TestHomepageInEffect(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeUserTheme(t, "arranged.yml", arrangedThemeFile)
	writeUserTheme(t, "plain.yml", userThemeColors)
	config := "Theme: arranged\nHomepage:\n  DesktopColumns: \"4\"\n  TitleSize: prominent\n  NavigationTextSize: large\n  CategoryNavigation: tabs\n"
	require.NoError(t, os.WriteFile("config.yml", []byte(config), 0600))
	UpdatePagePalettes()

	name, layout := SelectedThemeLayout()
	require.Equal(t, "arranged", name)
	require.Equal(t, map[string]string{"DesktopColumns": "6", "TitleSize": "compact", "HideAppDescriptions": "true"}, layout)
	// The caller owns what it gets.
	layout["DesktopColumns"] = "8"
	_, again := SelectedThemeLayout()
	require.Equal(t, "6", again["DesktopColumns"])

	// The theme's values replace those of the site; the rest of the site
	// settings stays, and the settings themselves are not changed.
	options := layoutSettings(t)
	site := *options.Homepage
	effective := HomepageInEffect(options)
	require.Equal(t, model.HomepageOptions{DesktopColumns: "6", TitleSize: "compact", HideAppDescriptions: true, NavigationTextSize: "large", CategoryNavigation: "tabs"}, *effective)
	require.Equal(t, site, *options.Homepage)

	// Options turned off for this theme keep the site setting. A list for
	// another theme has no effect.
	options.ThemeSettings = map[string]model.ThemeSettings{"plain": {IgnoreLayout: model.ThemeLayoutOptions}}
	require.Equal(t, "6", HomepageInEffect(options).DesktopColumns)
	options.ThemeSettings["arranged"] = model.ThemeSettings{IgnoreLayout: []string{model.LayoutDesktopColumns}}
	effective = HomepageInEffect(options)
	require.Equal(t, "4", effective.DesktopColumns)
	require.Equal(t, "compact", effective.TitleSize)
	require.True(t, effective.HideAppDescriptions)

	// Turned off for every theme, the site settings are used as they are.
	options.ThemeSettings = nil
	options.Homepage.IgnoreThemeLayout = true
	require.Same(t, options.Homepage, HomepageInEffect(options))
	options.Homepage.IgnoreThemeLayout = false

	// A site without a Homepage block gets the theme's values on the defaults.
	effective = HomepageInEffect(model.Application{})
	require.Equal(t, model.HomepageOptions{DesktopColumns: "6", TitleSize: "compact", HideAppDescriptions: true}, *effective)

	// A theme without a layout, a bundled theme, custom colors and a
	// selection without a theme leave the site settings as they are.
	for _, selected := range []string{"plain", "blackboard", "missing"} {
		SetTheme(selected)
		require.Same(t, options.Homepage, HomepageInEffect(options), selected)
		_, layout = SelectedThemeLayout()
		require.Empty(t, layout, selected)
	}
	require.NoError(t, SetCustomTheme(model.Palette{Background: "#ffffff", Primary: "#222222", Accent: "#888888"}))
	require.Same(t, options.Homepage, HomepageInEffect(options))
	require.Nil(t, HomepageInEffect(model.Application{}))
}

// The layout follows the file of the selected theme like its tokens do, and
// one page takes both from the same reading of the file.
func TestHomepageInEffectFollowsTheThemeFile(t *testing.T) {
	userThemeDirectory(t, "arranged")
	writeUserTheme(t, "arranged.yml", arrangedThemeFile)
	UpdatePagePalettes()
	options := model.Application{Homepage: &model.HomepageOptions{DesktopColumns: "4"}}
	require.Equal(t, "6", HomepageInEffect(options).DesktopColumns)

	// The layout is asked for before the tokens when a page is rendered.
	writeUserTheme(t, "arranged.yml", userThemeColors+"  radius: 10px\nlayout:\n  DesktopColumns: 8\n")
	require.Equal(t, "8", HomepageInEffect(options).DesktopColumns)
	require.Equal(t, userThemeStyle+"--flare-radius:10px;", GetPageTheme().BodyStyle)

	// A value this release does not have falls back to the site setting,
	// and the rest of the layout stays.
	writeUserTheme(t, "arranged.yml", userThemeColors+"layout:\n  DesktopColumns: 12\n  TitleSize: compact\n")
	effective := HomepageInEffect(options)
	require.Equal(t, "4", effective.DesktopColumns)
	require.Equal(t, "compact", effective.TitleSize)
	_, _, notices := UserThemes()
	require.Equal(t, []ThemeNotice{{File: "arranged.yml", Unused: model.ThemeFileUnused{Keys: []string{"layout.DesktopColumns: 12"}}}}, notices)

	// Without a usable file there is no layout to apply.
	writeUserTheme(t, "arranged.yml", "contract: 1\nlayout:\n  DesktopColumns: 8\n")
	require.Same(t, options.Homepage, HomepageInEffect(options))
	require.NoError(t, os.Remove("themes/arranged.yml"))
	require.Same(t, options.Homepage, HomepageInEffect(options))
}
