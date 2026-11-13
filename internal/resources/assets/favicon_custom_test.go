package assets

import (
	"os"
	"testing"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

// Themed favicons are generated at build time for the bundled palettes only.
func TestCustomThemeUsesDefaultFavicon(t *testing.T) {
	previous, _ := define.ThemeSnapshot()
	t.Cleanup(func() { define.SetTheme(previous) })
	// Leave no stored palette behind for the other tests of this package. The
	// cleanup is registered after t.Chdir, so it runs in the test directory.
	t.Chdir(t.TempDir())
	t.Cleanup(func() {
		require.NoError(t, os.WriteFile("config.yml", []byte("Theme: blackboard\n"), 0600))
		define.UpdatePagePalettes()
	})

	require.NoError(t, define.SetCustomTheme(model.Palette{Background: "#10241b", Primary: "#e8f1e4", Accent: "#f2b84b"}))
	require.Equal(t, FaviconURL(), ThemeFaviconURL())

	define.SetTheme("cloud")
	require.Equal(t, themeFavicons["cloud"].url, ThemeFaviconURL())

	// An unknown name keeps its previous fallback, the first bundled theme.
	define.SetTheme("no-such-theme")
	require.Equal(t, themeFavicons[define.ThemePalettes[0].Name].url, ThemeFaviconURL())
}
