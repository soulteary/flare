package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/internal/i18n"
)

// A theme that is based on another is shown with the templates of that
// theme for what it brings none for. The theme page lists them by the theme
// that brings them, and the confirmation of the theme is about all of them.
func TestThemeIsShownWithTheTemplatesOfTheThemeBelowHTTP(t *testing.T) {
	files := map[string]string{
		"zelda/theme.yml":                   itemThemeFile,
		"zelda/templates/home.html":         themeShellTemplate,
		"zelda/templates/applications.html": strings.Replace(themeShellTemplate, "theme-shell", "theme-shell replaced", 1),
		"zelda/templates/app.html":          itemAppTemplate,
		"night/theme.yml":                   "contract: 1\nextends: zelda\nname: Night\n",
		"night/templates/applications.html": strings.Replace(themeShellTemplate, "theme-shell", "theme-shell of-night", 1),
		"night/templates/bookmark.html":     itemBookmarkTemplate,
	}
	handler := themeFilesRouter(t, "Locale: en\nTheme: night\nShowApps: true\nShowBookmarks: true\nThemeTemplates: true\n", files)
	itemEntries(t, true)
	const listed = "applications.html, bookmark.html, zelda/home.html, zelda/app.html"

	page := getPage(t, handler, "/settings/theme")
	require.Contains(t, page, `<p class="theme-about theme-templates-note">`+strings.ReplaceAll(i18n.T("en", "theme_templates_note_unconfirmed"), "{files}", listed)+`</p>`)
	require.Contains(t, page, strings.ReplaceAll(i18n.T("en", "theme_templates_note_unconfirmed"), "{files}", "home.html, applications.html, app.html"), "the card of the theme below lists its own")
	fields, shown := templateForm(t, page, "theme-templates-confirm-form")
	require.True(t, shown)
	require.Equal(t, map[string]string{
		"action": "confirm-templates", "theme-templates-theme": "night",
		"theme-templates-files": strings.ReplaceAll(listed, ", ", ","), "submit": i18n.T("en", "theme_templates_confirm_submit"),
	}, fields)
	require.Contains(t, page, strings.ReplaceAll(i18n.T("en", "theme_templates_confirm_files"), "{files}", listed))
	require.NotContains(t, getPage(t, handler, "/"), "theme-shell")

	// A confirmation that leaves the templates of the theme below out is
	// not about the templates the theme is shown with.
	stale := url.Values{"action": {"confirm-templates"}, "theme-templates-theme": {"night"}, "theme-templates-files": {"applications.html,bookmark.html"}}
	require.Equal(t, http.StatusConflict, postThemeForm(t, handler, stale).Code)
	require.NotContains(t, getPage(t, handler, "/"), "theme-shell")

	confirm := url.Values{"action": {"confirm-templates"}, "theme-templates-theme": {"night"}, "theme-templates-files": {fields["theme-templates-files"]}}
	rec := postThemeForm(t, handler, confirm)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), strings.ReplaceAll(i18n.T("en", "theme_templates_note_used"), "{files}", listed))

	home := getPage(t, handler, "/")
	require.Contains(t, home, `<div class="theme-shell" data-kind="home">`)
	require.Contains(t, home, `<div class="t-app"`, "an application has the template of the theme below")
	require.Contains(t, home, `<li class="t-mark"`, "a bookmark has the template of the theme")
	require.Contains(t, getPage(t, handler, "/applications"), `<div class="theme-shell of-night" data-kind="applications">`)
	// The theme below has a template of the home page, which stands in for
	// the bookmarks of the theme above.
	require.Contains(t, getPage(t, handler, "/bookmarks"), `<div class="theme-shell" data-kind="bookmarks">`)

	// The theme below is not confirmed by that: selected, it has the
	// markup of the application.
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"theme": {"zelda"}}).Code)
	require.NotContains(t, getPage(t, handler, "/"), "theme-shell")
	require.Contains(t, getPage(t, handler, "/?theme-preview=night"), `<div class="theme-shell" data-kind="home">`)
}
