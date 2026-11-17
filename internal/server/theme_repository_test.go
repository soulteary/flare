package server

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/i18n"
)

// The theme page links to the theme repository, and that is all the
// application has to do with it: the link opens in a new window, and the
// page is one that sends no referrer to another site. It is there on an
// instance in offline mode as well, which this router is.
func TestThemePageLinksToTheRepositoryHTTP(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\n", nil)
	require.True(t, define.AppFlags.EnableOfflineMode)
	page := getPage(t, handler, "/settings/theme")
	link := `<a href="https://github.com/soulteary/flare-themes" target=_blank rel="noopener noreferrer">https://github.com/soulteary/flare-themes</a>`
	require.Contains(t, page, `<p class="theme-help theme-repository">`+i18n.T("en", "theme_repository")+` `+link)
	require.Equal(t, 1, strings.Count(page, "flare-themes\""))
	require.Contains(t, page, `<meta name=referrer content="same-origin">`)
	// No other page has it, and nothing loads from it.
	for _, path := range []string{"/", "/settings/layout", "/help"} {
		require.NotContains(t, getPage(t, handler, path), "flare-themes", path)
	}
	require.Equal(t, 2, strings.Count(page, "github.com/soulteary/flare-themes"))
}
