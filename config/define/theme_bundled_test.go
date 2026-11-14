package define

import (
	"html/template"
	"io/fs"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
)

// The bundled themes as they were defined in Go code before they became theme
// files. Their names, order and colors, including letter case, are what stored
// configurations, favicons and rendered pages rely on.
var bundledThemesBeforeFiles = []model.Theme{
	{Name: "blackboard", Colors: model.Palette{Background: "#1a1a1a", Primary: "#FFFDEA", Accent: "#5c5c5c"}},
	{Name: "gazette", Colors: model.Palette{Background: "#F2F7FF", Primary: "#000000", Accent: "#5c5c5c"}},
	{Name: "espresso", Colors: model.Palette{Background: "#21211F", Primary: "#D1B59A", Accent: "#4E4E4E"}},
	{Name: "cab", Colors: model.Palette{Background: "#F6D305", Primary: "#1F1F1F", Accent: "#424242"}},
	{Name: "cloud", Colors: model.Palette{Background: "#f1f2f0", Primary: "#35342f", Accent: "#37bbe4"}},
	{Name: "lime", Colors: model.Palette{Background: "#263238", Primary: "#AABBC3", Accent: "#aeea00"}},
	{Name: "white", Colors: model.Palette{Background: "#ffffff", Primary: "#222222", Accent: "#dddddd"}},
	{Name: "tron", Colors: model.Palette{Background: "#242B33", Primary: "#EFFBFF", Accent: "#6EE2FF"}},
	{Name: "blues", Colors: model.Palette{Background: "#2B2C56", Primary: "#EFF1FC", Accent: "#6677EB"}},
	{Name: "passion", Colors: model.Palette{Background: "#f5f5f5", Primary: "#12005e", Accent: "#8e24aa"}},
	{Name: "chalk", Colors: model.Palette{Background: "#263238", Primary: "#AABBC3", Accent: "#FF869A"}},
	{Name: "paper", Colors: model.Palette{Background: "#F8F6F1", Primary: "#4C432E", Accent: "#AA9A73"}},
	{Name: "neon", Colors: model.Palette{Background: "#091833", Primary: "#EFFBFF", Accent: "#ea00d9"}},
	{Name: "pumpkin", Colors: model.Palette{Background: "#2d3436", Primary: "#EFFBFF", Accent: "#ffa500"}},
	{Name: "onedark", Colors: model.Palette{Background: "#282c34", Primary: "#dfd9d6", Accent: "#98c379"}},
}

func TestBundledThemeFilesKeepTheBundledThemes(t *testing.T) {
	themes, err := loadBundledThemes()
	require.NoError(t, err)
	require.Equal(t, bundledThemesBeforeFiles, themes)
	require.Equal(t, bundledThemesBeforeFiles, ThemePalettes)
}

func TestEveryBundledThemeFileIsListed(t *testing.T) {
	entries, err := fs.ReadDir(bundledThemeFiles, "bundled")
	require.NoError(t, err)
	var files []string
	for _, entry := range entries {
		name, isTheme := strings.CutSuffix(entry.Name(), ".yml")
		require.True(t, isTheme && !entry.IsDir(), "unexpected entry %q", entry.Name())
		files = append(files, name)
	}
	listed := slices.Clone(bundledThemeOrder)
	slices.Sort(listed)
	require.Equal(t, files, listed, "bundledThemeOrder and the embedded files differ")
	require.Equal(t, len(listed), len(slices.Compact(listed)), "a theme is listed twice")
	for _, name := range bundledThemeOrder {
		require.NotEqual(t, model.ThemeCustomName, name)
	}
}

func TestThemeStyleWritesTokensAfterTheColors(t *testing.T) {
	theme := model.Theme{Name: "mine", Colors: model.Palette{Background: "#101010", Primary: "#f0f0f0", Accent: "#808080"}}
	require.Equal(t, "--color-background:#101010;--color-primary:#f0f0f0;--color-accent:#808080;", themeStyle(theme))

	// Tokens appear in the order of their documentation, whatever the map order.
	theme.Tokens = map[string]string{
		model.TokenFontFamily: "Helvetica Neue, sans-serif",
		model.TokenRadius:     "8px",
		model.TokenHover:      "#00000033",
		model.TokenIcon:       "#ffcc66",
	}
	require.Equal(t,
		"--color-background:#101010;--color-primary:#f0f0f0;--color-accent:#808080;"+
			"--flare-icon:#ffcc66;--flare-hover:#00000033;--flare-radius:8px;--flare-font-family:Helvetica Neue, sans-serif;",
		themeStyle(theme))

	// The font scale follows the font family, as in the documentation.
	theme.Tokens = map[string]string{model.TokenFontScale: "1.25", model.TokenFontFamily: "Georgia, serif", model.TokenRadius: "0"}
	require.Equal(t,
		"--color-background:#101010;--color-primary:#f0f0f0;--color-accent:#808080;"+
			"--flare-radius:0;--flare-font-family:Georgia, serif;--flare-font-scale:1.25;",
		themeStyle(theme))

	// A token outside the documented list is never written.
	theme.Tokens = map[string]string{"shadow": "0 0 4px red"}
	require.Equal(t, "--color-background:#101010;--color-primary:#f0f0f0;--color-accent:#808080;", themeStyle(theme))
}

func TestSelectedThemePublishesItsTokens(t *testing.T) {
	originalThemes, originalTheme, originalColor, originalStyle := ThemePalettes, ThemeCurrent, ThemePrimaryColor, _CACHE_PAGE_BODY_THEME_NAME
	t.Cleanup(func() {
		themeMu.Lock()
		defer themeMu.Unlock()
		ThemePalettes, ThemeCurrent, ThemePrimaryColor, _CACHE_PAGE_BODY_THEME_NAME = originalThemes, originalTheme, originalColor, originalStyle
	})
	file, err := model.ParseThemeFile([]byte("contract: 1\ntokens:\n  background: \"#101010\"\n  primary: \"#f0f0f0\"\n  accent: \"#808080\"\n  radius: 12px\n  muted: \"#b0b0b0\"\n"))
	require.NoError(t, err)
	themeMu.Lock()
	ThemePalettes = append(slices.Clone(ThemePalettes), file.Theme("rounded"))
	themeMu.Unlock()

	SetTheme("rounded")
	require.Equal(t, template.CSS("--color-background:#101010;--color-primary:#f0f0f0;--color-accent:#808080;--flare-muted:#b0b0b0;--flare-radius:12px;"), GetAppBodyStyle())
	name, primary := ThemeSnapshot()
	require.Equal(t, "rounded", name)
	require.Equal(t, "#f0f0f0", primary)

	// A bundled theme sets no other token, so its style is the three colors.
	SetTheme("blackboard")
	require.Equal(t, template.CSS("--color-background:#1a1a1a;--color-primary:#FFFDEA;--color-accent:#5c5c5c;"), GetAppBodyStyle())
}
