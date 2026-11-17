package server

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/internal/i18n"
)

// layoutRemarks reads the remarks of the appearance page about the layout,
// by the name of the control each stands behind.
func layoutRemarks(t *testing.T, page string) map[string]string {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	remarks := map[string]string{}
	walkNodes(document, func(node *html.Node) {
		if node.Type != html.ElementNode || !nodeHasClass(node, "theme-layout-note") {
			return
		}
		text := ""
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if child.Type == html.TextNode {
				text += child.Data
			}
		}
		for before := node.PrevSibling; before != nil; before = before.PrevSibling {
			if before.Type == html.ElementNode && before.Data == "select" {
				remarks[nodeAttribute(before, "name")] = strings.TrimSpace(text)
				return
			}
		}
		t.Fatalf("a remark behind no control: %s", text)
	})
	return remarks
}

// A theme lists the layout options its templates support. While its
// templates are in use, the appearance page says of every other option that
// the theme does not support it; the control stays, and the option the theme
// sets itself has the remark about that.
func TestLayoutOptionsTheTemplatesOfTheThemeDoNotSupportHTTP(t *testing.T) {
	files := map[string]string{
		"zelda/theme.yml":           themeFileColors + "supports:\n  - DesktopColumns\n  - CategoryNavigation\nlayout:\n  TitleSize: compact\n",
		"zelda/templates/home.html": themeShellTemplate,
		"quiet/theme.yml":           themeFileColors,
		"quiet/templates/home.html": themeShellTemplate,
		"plain/theme.yml":           themeFileColors + "supports: []\n",
		"night/theme.yml":           "contract: 1\nextends: zelda\n",
	}
	handler := themeFilesRouter(t, "Locale: en\nTheme: zelda\nShowApps: true\nShowBookmarks: true\n", files)
	unsupported := i18n.T("en", "theme_layout_unsupported")
	sets := map[string]string{"title-size": note("theme_layout_note", "Compact")}
	appearance := func() map[string]string { return layoutRemarks(t, getPage(t, handler, "/settings/layout")) }
	confirm := func(theme string) {
		t.Helper()
		require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"theme": {theme}}).Code)
		fields, shown := templateForm(t, getPage(t, handler, "/settings/theme-access"), "theme-templates-confirm-form")
		require.True(t, shown, theme)
		rec := postThemeForm(t, handler, url.Values{"action": {"confirm-templates"}, "theme-templates-theme": {theme}, "theme-templates-files": {fields["theme-templates-files"]}})
		require.Equal(t, http.StatusOK, rec.Code, theme)
	}

	// The card of a theme with templates names the options before the
	// theme is used: once for zelda and once for night, which has the
	// templates and the list of zelda. A theme without templates has no
	// such line, whatever it lists.
	card := i18n.Format("en", "theme_layout_unsupported_note", map[string]any{"options": "Phone and tablet layout, Application descriptions, Bookmark layout"})
	page := getPage(t, handler, "/settings/theme")
	require.Equal(t, 2, strings.Count(page, `<p class=theme-about>`+card+`</p>`))
	require.Equal(t, 2, strings.Count(page, strings.SplitN(i18n.T("en", "theme_layout_unsupported_note"), "{", 2)[0]))

	// Templates are turned off, and then not confirmed: the pages have the
	// markup of the application, and every option works.
	require.Equal(t, sets, appearance())
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"action": {"save-templates"}, "theme-templates-allowed": {"1"}}).Code)
	require.Equal(t, sets, appearance())

	confirm("zelda")
	require.Contains(t, getPage(t, handler, "/"), `<div class="theme-shell"`)
	raw := getPage(t, handler, "/settings/layout")
	require.Equal(t, map[string]string{
		"title-size": sets["title-size"], "mobile-bookmark-layout": unsupported, "hide-app-descriptions": unsupported, "bookmark-layout": unsupported,
	}, layoutRemarks(t, raw))
	// The remark has no box, and the controls are there as before.
	read := readLayoutPage(t, raw)
	require.Equal(t, "", read.notes[unsupported])
	require.Equal(t, "theme-layout-ignore-TitleSize", read.notes[sets["title-size"]])
	for _, control := range []string{"mobile-bookmark-layout", "hide-app-descriptions", "bookmark-layout", "desktop-columns", "category-navigation"} {
		require.Contains(t, read.selected, control)
	}
	// The setting is stored all the same.
	require.Equal(t, http.StatusOK, appearanceRequest(handler, url.Values{"bookmark-layout": {"sections"}}).Code)
	require.Equal(t, "sections", readLayoutPage(t, getPage(t, handler, "/settings/layout")).selected["bookmark-layout"])

	// A theme above has the list of the theme below with its templates.
	confirm("night")
	require.Equal(t, unsupported, appearance()["bookmark-layout"])
	require.Len(t, appearance(), 4)

	// A theme that does not say what it supports has no such remark, and
	// neither has a theme without templates that lists nothing.
	confirm("quiet")
	require.Empty(t, appearance())
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"theme": {"plain"}}).Code)
	require.Empty(t, appearance())

	// The remarks go with the confirmation.
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"theme": {"zelda"}}).Code)
	require.Len(t, appearance(), 4)
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"action": {"withdraw-templates"}, "theme-templates-theme": {"zelda"}}).Code)
	require.Equal(t, sets, appearance())
}
