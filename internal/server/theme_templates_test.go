package server

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/internal/i18n"
)

// The theme page shows, on the card of a theme, the page templates it brings
// and that they are not applied, and lists a template that is not accepted.
// The pages of a theme with templates are those of the application.
func TestThemePageTemplatesHTTP(t *testing.T) {
	parts := `{{template "flare/search"}}{{template "flare/clock"}}{{template "flare/greeting"}}{{template "flare/weather"}}` +
		`{{template "flare/subpage-header"}}{{template "flare/apps"}}{{template "flare/bookmarks"}}{{template "flare/toolbar"}}{{template "flare/footer"}}`
	files := map[string]string{
		"zelda/theme.yml":                themeFileColors,
		"zelda/templates/home.html":      `<div class="theme-shell">` + parts + `</div>`,
		"zelda/templates/bookmarks.html": `<div onclick="x()">` + parts + `</div>`,
		"plain/theme.yml":                themeFileColors,
	}
	handler := themeFilesRouter(t, "Locale: en\nTheme: zelda\nShowApps: true\nShowBookmarks: true\n", files)

	raw := getPage(t, handler, "/settings/theme")
	note := `<p class="theme-about theme-templates-note">` + strings.ReplaceAll(i18n.T("en", "theme_templates_note"), "{files}", "home.html") + `</p>`
	require.Equal(t, 1, strings.Count(raw, note), "the card of the theme with templates has the note, and no other card")
	page := readThemeFilePage(t, raw)
	require.Equal(t, []string{"zelda/templates/bookmarks.html " + i18n.T("en", "theme_file_problem_template_attribute") + " onclick"}, page.lineTexts)

	// Templates of themes are not applied: the pages have the markup of
	// the application and nothing of the template.
	withTemplates := getPage(t, handler, "/")
	require.NotContains(t, withTemplates, "theme-shell")
	require.Contains(t, withTemplates, "id=hero-container")
	for _, path := range []string{"/applications", "/bookmarks"} {
		require.NotContains(t, getPage(t, handler, path), "theme-shell", path)
	}
}
