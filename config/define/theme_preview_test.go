package define

import (
	"html/template"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

// A theme can be resolved for one page without being selected. Nothing about
// the selected theme changes by it.
func TestPageThemeForAThemeThatIsNotSelected(t *testing.T) {
	modeDirectory(t, "Theme: blackboard\n")
	writeUserTheme(t, "forest.yml", userThemeColors+"  radius: 10px\nname:\n  en: Forest at dusk\n  zh: 暮色森林\nlayout:\n  DesktopColumns: 6\n")
	writeUserTheme(t, "paired.yml", pairedThemeFile)
	writeUserTheme(t, "rejected.yml", userThemeColors+"  radius: 4em\n")
	UpdatePagePalettes()
	selected := GetPageTheme()
	name, primary, generation := ThemeRenderSnapshot()

	page, known := GetPageThemeFor("forest")
	require.True(t, known)
	require.Equal(t, PageTheme{BodyStyle: userThemeStyle + "--flare-radius:10px;"}, page)
	// A bundled theme and the custom colors can be resolved in the same way.
	page, known = GetPageThemeFor("tron")
	require.True(t, known)
	require.Equal(t, PageTheme{BodyStyle: "--color-background:#242B33;--color-primary:#EFFBFF;--color-accent:#6EE2FF;"}, page)
	_, known = GetPageThemeFor(model.ThemeCustomName)
	require.False(t, known, "no custom colors are stored")

	// The site setting for light and dark applies to it as to the selected theme.
	page, known = GetPageThemeFor("paired")
	require.True(t, known)
	require.Equal(t, PageTheme{Rules: template.CSS(pairedRules)}, page)

	// What is not a theme in use resolves to nothing.
	for _, other := range []string{"", "missing", "rejected", "../config", "forest.yml", "Forest"} {
		page, known = GetPageThemeFor(other)
		require.False(t, known, "%q", other)
		require.Equal(t, PageTheme{}, page, "%q", other)
	}

	// The layout of a theme that is not selected, weighed against the site
	// settings and the options turned off for that theme.
	options := model.Application{Homepage: &model.HomepageOptions{DesktopColumns: "4"}}
	require.Equal(t, "6", HomepageInEffectFor(options, "forest").DesktopColumns)
	require.Same(t, options.Homepage, HomepageInEffectFor(options, "tron"))
	require.Same(t, options.Homepage, HomepageInEffectFor(options, "missing"))
	options.ThemeSettings = map[string]model.ThemeSettings{"forest": {IgnoreLayout: []string{model.LayoutDesktopColumns}}}
	require.Same(t, options.Homepage, HomepageInEffectFor(options, "forest"))
	require.Same(t, options.Homepage, HomepageInEffect(options), "the selected theme specifies no layout")

	// The name a theme shows, by interface language.
	require.Equal(t, "Forest at dusk", ThemeTitle("forest", "en"))
	require.Equal(t, "暮色森林", ThemeTitle("forest", "zh"))
	require.Equal(t, "paired", ThemeTitle("paired", "en"))
	require.Equal(t, "tron", ThemeTitle("tron", "zh"))
	require.Equal(t, "missing", ThemeTitle("missing", "en"))

	// None of this selected anything or published a theme again.
	require.Equal(t, selected, GetPageTheme())
	afterName, afterPrimary, afterGeneration := ThemeRenderSnapshot()
	require.Equal(t, []any{name, primary, generation}, []any{afterName, afterPrimary, afterGeneration})
	raw, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, "Theme: blackboard\n", string(raw))

	// A site set to dark shows the dark values of such a theme as well.
	require.True(t, SetThemeMode(model.ThemeModeDark))
	page, _ = GetPageThemeFor("paired")
	require.Equal(t, PageTheme{BodyStyle: template.CSS(pairedDarkStyle)}, page)
}
