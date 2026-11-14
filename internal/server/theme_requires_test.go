package server

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	version "github.com/soulteary/version-kit/v4"
	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/i18n"
)

// A theme can name the first release it works with. An earlier release does
// not use the theme, and the theme page gives the release it needs.
func TestThemeThatNeedsALaterReleaseHTTP(t *testing.T) {
	previous := version.Version
	version.Version = "0.7.0"
	t.Cleanup(func() { version.Version = previous })
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\nShowApps: true\n", map[string]string{
		"now.yml":   themeFileColors + "requires: 0.7.0\n",
		"later.yml": themeFileColors + "requires: 0.8.0\n",
		// A file for a later release can hold what this release does not
		// accept; it is listed for the release alone.
		"later-values.yml":    themeFileColors + "  radius: 4em\nrequires: 0.8.0\n",
		"later-dir/theme.yml": themeFileColors + "requires: 1.0.0\n",
		"later-dir/theme.css": "@import 'a.css';",
		"wrong.yml":           themeFileColors + "requires: \">=0.7.0\"\n",
	})

	page := readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Len(t, page.cards, 1)
	require.Equal(t, "now", page.cards[0].value)
	needs := i18n.T("en", "theme_file_problem_requires")
	require.Equal(t, []string{
		"later-dir/theme.yml " + needs + " 1.0.0",
		"later-values.yml " + needs + " 0.8.0",
		"later.yml " + needs + " 0.8.0",
		"wrong.yml " + i18n.T("en", "theme_file_problem_requires_value") + " >=0.7.0",
	}, page.lineTexts)

	// Such a theme cannot be selected or previewed.
	for _, name := range []string{"later", "later-values", "later-dir", "wrong"} {
		require.Equal(t, http.StatusBadRequest, themeRequest(handler, url.Values{"theme": {name}}).Code, name)
		require.Equal(t, "--color-background:#1a1a1a;--color-primary:#FFFDEA;--color-accent:#5c5c5c;", bodyStyle(t, getPage(t, handler, "/?theme-preview="+name)), name)
	}
	require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"now"}}).Code)
	require.Equal(t, themeFileStyle, bodyStyle(t, getPage(t, handler, "/")))

	// It is not imported either, and the refusal gives the release.
	refused := importFile(t, handler, "imported.yml", themeFileColors+"requires: 0.8.0\n", false)
	require.Equal(t, http.StatusBadRequest, refused.Code)
	rejected := i18n.Format("en", "theme_import_rejected", map[string]any{"reason": needs}) + " 0.8.0"
	require.Equal(t, []string{rejected}, readImportPage(t, refused.Body.String()).alerts)
	require.NoFileExists(t, filepath.Join(define.UserThemeDirectory, "imported.yml"))
	accepted := importFile(t, handler, "imported.yml", themeFileColors+"requires: 0.7.0\n", false)
	require.Equal(t, http.StatusOK, accepted.Code)

	// The reasons are given in the interface language.
	require.NoError(t, os.WriteFile("config.yml", []byte("Locale: zh\nTheme: now\nShowApps: true\n"), 0600))
	page = readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Contains(t, page.lineTexts, "later.yml "+i18n.T("zh", "theme_file_problem_requires")+" 0.8.0")
	require.Contains(t, page.lineTexts, "wrong.yml "+i18n.T("zh", "theme_file_problem_requires_value")+" >=0.7.0")
}
