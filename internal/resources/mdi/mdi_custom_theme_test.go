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

func TestCustomThemeIconsUseItsPrimaryColor(t *testing.T) {
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
	require.True(t, strings.HasPrefix(first, "/assets/mdi/custom-servernetwork.svg?v="), first)
	require.Contains(t, requestIcon(app, http.MethodGet, first, "", "").Body.String(), "fill:#e8f1e4;")

	// Saving other colors keeps the theme name, so the icon is told apart by
	// its content version.
	require.NoError(t, define.SetCustomTheme(model.Palette{Background: "#10241b", Primary: "#ffd166", Accent: "#f2b84b"}))
	second := generatedIconURL(t)
	require.NotEqual(t, first, second)
	require.True(t, strings.HasPrefix(second, "/assets/mdi/custom-servernetwork.svg?v="), second)
	require.Contains(t, requestIcon(app, http.MethodGet, second, "", "").Body.String(), "fill:#ffd166;")
}
