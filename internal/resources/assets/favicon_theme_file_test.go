package assets

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/flare/config/define"
	"github.com/stretchr/testify/require"
)

// A theme file has no generated favicon, so it uses the default icon. While
// its file cannot be used, the selection is an unknown name like any other.
func TestThemeFileUsesDefaultFavicon(t *testing.T) {
	t.Chdir(t.TempDir())
	// Registered after t.Chdir, so it runs in the test directory and leaves
	// no theme file behind for the other tests of this package.
	t.Cleanup(func() {
		require.NoError(t, os.RemoveAll(define.UserThemeDirectory))
		require.NoError(t, os.WriteFile("config.yml", []byte("Theme: blackboard\n"), 0600))
		define.UpdatePagePalettes()
	})
	require.NoError(t, os.Mkdir(define.UserThemeDirectory, 0700))
	file := filepath.Join(define.UserThemeDirectory, "forest.yml")
	require.NoError(t, os.WriteFile(file, []byte("contract: 1\ntokens:\n  background: \"#10241b\"\n  primary: \"#e8f1e4\"\n  accent: \"#f2b84b\"\n"), 0600))
	require.NoError(t, os.WriteFile("config.yml", []byte("Theme: forest\n"), 0600))
	define.UpdatePagePalettes()
	require.Equal(t, FaviconURL(), ThemeFaviconURL())

	define.SetTheme("cloud")
	require.Equal(t, themeFavicons["cloud"].url, ThemeFaviconURL())

	require.NoError(t, os.Remove(file))
	define.SetTheme("forest")
	define.ReloadUserThemes()
	require.Equal(t, themeFavicons[define.ThemePalettes[0].Name].url, ThemeFaviconURL())
}
