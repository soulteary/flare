package mdi

import (
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
)

// The custom theme changes its colors without changing its name. Icon files
// carry no color, so saving other colors needs neither a new file nor a new URL.
func TestCustomThemeKeepsIconFiles(t *testing.T) {
	app := newIconTestRouter(t, false)
	// Leave no stored palette behind for the other tests of this package. The
	// cleanup is registered after t.Chdir, so it runs in the test directory.
	t.Chdir(t.TempDir())
	t.Cleanup(func() {
		require.NoError(t, os.WriteFile("config.yml", []byte("Theme: blackboard\n"), 0600))
		define.UpdatePagePalettes()
	})
	require.NoError(t, define.SetCustomTheme(model.Palette{Background: "#10241b", Primary: "#e8f1e4", Accent: "#f2b84b"}))
	first := generatedIconURL(t)
	require.True(t, strings.HasPrefix(first, "/assets/mdi/servernetwork.svg?v="), first)
	body := requestIcon(app, http.MethodGet, first, "", "").Body.String()
	require.NotContains(t, body, "fill")
	require.NotContains(t, body, "#e8f1e4")

	require.NoError(t, define.SetCustomTheme(model.Palette{Background: "#10241b", Primary: "#ffd166", Accent: "#f2b84b"}))
	require.Equal(t, first, generatedIconURL(t))
	require.Equal(t, body, requestIcon(app, http.MethodGet, first, "", "").Body.String())
}
