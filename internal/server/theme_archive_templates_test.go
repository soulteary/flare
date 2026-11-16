package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/internal/i18n"
)

// An archive brings the templates of a theme and a download holds them. An
// import withdraws the confirmation of the templates of the theme and of
// the themes that have it below, because what was confirmed are other
// templates; the confirmations of other themes stay.
func TestImportedTemplatesAreConfirmedAgainHTTP(t *testing.T) {
	zeldaFile := themeFileColors + "name: Zelda\n"
	files := map[string]string{
		"zelda/theme.yml":           zeldaFile,
		"zelda/templates/home.html": themeShellTemplate,
		"night/theme.yml":           "contract: 1\nextends: zelda\nname: Night\n",
		"other/theme.yml":           themeFileColors,
		"other/templates/home.html": themeShellTemplate,
		"single.yml":                themeFileColors,
	}
	confirmed := "ThemeTemplates: true\nThemeSettings:\n"
	for _, theme := range []string{"zelda", "night", "other", "single"} {
		confirmed += "    " + theme + ":\n        IgnoreLayout: []\n        Templates: true\n"
	}
	handler := themeFilesRouter(t, "Locale: en\nTheme: night\nShowApps: true\nShowBookmarks: true\n"+confirmed, files)
	stored := func() map[string]bool {
		t.Helper()
		options, err := data.GetAllSettingsOptions()
		require.NoError(t, err)
		themes := map[string]bool{}
		for theme, settings := range options.ThemeSettings {
			themes[theme] = settings.Templates
		}
		return themes
	}
	withdrawn := func(page string, themes string) bool {
		text := strings.ReplaceAll(i18n.T("en", "theme_import_withdrawn"), "{themes}", themes)
		return strings.Contains(page, `<p class="theme-notice theme-import-withdrawn" role=status>`+text+`</p>`)
	}
	require.Contains(t, getPage(t, handler, "/"), `<div class="theme-shell" data-kind="home">`)

	// The download holds the template.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/theme/export?name=zelda", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	names, contents := unpackThemeArchive(t, rec.Body.Bytes())
	require.Equal(t, []string{"zelda/theme.yml", "zelda/templates/home.html"}, names)
	require.Equal(t, themeShellTemplate, contents["zelda/templates/home.html"])

	// An import that is refused withdraws nothing.
	imported := strings.Replace(themeShellTemplate, "theme-shell", "theme-shell imported", 1)
	archive := packThemeArchive(t, "theme.yml", zeldaFile, "templates/home.html", imported, "templates/notes.txt", "notes")
	rec = importFile(t, handler, "zelda.zip", archive, false)
	require.Equal(t, http.StatusConflict, rec.Code)
	require.NotContains(t, rec.Body.String(), "theme-import-withdrawn")
	require.Equal(t, map[string]bool{"zelda": true, "night": true, "other": true, "single": true}, stored())
	require.Contains(t, getPage(t, handler, "/"), `<div class="theme-shell" data-kind="home">`)

	// The theme is replaced. The page says whose templates are to be
	// confirmed again, and the pages have the markup of the application.
	rec = importFile(t, handler, "zelda.zip", archive, true)
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, withdrawn(rec.Body.String(), "Night, Zelda"), rec.Body.String())
	require.Contains(t, rec.Body.String(), "templates/notes.txt")
	require.Equal(t, map[string]bool{"zelda": false, "night": false, "other": true, "single": true}, stored())
	home := getPage(t, handler, "/")
	require.NotContains(t, home, "theme-shell")
	require.Contains(t, home, "id=hero-container")

	// Confirmed again, the theme above is shown with the template that was
	// imported.
	fields, shown := templateForm(t, getPage(t, handler, "/settings/theme"), "theme-templates-confirm-form")
	require.True(t, shown)
	require.Equal(t, "confirm-templates", fields["action"])
	require.Equal(t, "zelda/home.html", fields["theme-templates-files"])
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"action": {"confirm-templates"}, "theme-templates-theme": {"night"}, "theme-templates-files": {"zelda/home.html"}}).Code)
	require.Contains(t, getPage(t, handler, "/"), `<div class="theme-shell imported" data-kind="home">`)

	// A theme nothing was confirmed for is imported without that remark,
	// and so is the theme whose confirmation is withdrawn already.
	rec = importFile(t, handler, "fresh.zip", archive, false)
	require.Equal(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "theme-import-withdrawn")
	rec = importFile(t, handler, "zelda.zip", archive, true)
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, withdrawn(rec.Body.String(), "Night"), "the theme above was confirmed in between")
	require.Equal(t, map[string]bool{"zelda": false, "night": false, "other": true, "single": true}, stored())

	// A theme file that is imported is no different.
	rec = importFile(t, handler, "single.yml", themeFileColors+"name: Single\n", true)
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, withdrawn(rec.Body.String(), "Single"))
	require.Equal(t, map[string]bool{"zelda": false, "night": false, "other": true, "single": false}, stored())
}
