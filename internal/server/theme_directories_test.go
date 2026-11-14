package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/i18n"
)

// themeCardLinks returns the addresses of the links on the card of each
// theme of the themes directory, by the name of the theme.
func themeCardLinks(t *testing.T, page string) map[string][]string {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	links := map[string][]string{}
	walkNodes(document, func(node *html.Node) {
		if !nodeHasClass(node, "theme-file") {
			return
		}
		name, found := "", []string{}
		walkNodes(node, func(part *html.Node) {
			switch {
			case nodeHasClass(part, "theme-choose"):
				name = nodeAttribute(part, "value")
			case part.Type == html.ElementNode && part.Data == "a":
				found = append(found, nodeAttribute(part, "href"))
			}
		})
		links[name] = found
	})
	return links
}

// A theme that is a directory is listed, selected, shown and previewed like
// a theme that is a file, and reported in the same way when it is not used.
func TestThemeDirectoriesHTTP(t *testing.T) {
	zelda := themeFileColors + "  radius: 10px\nname:\n  en: Zelda\n  zh: 塞尔达\nversion: \"2.0\"\n"
	zeldaStyle := themeFileStyle + "--flare-radius:10px;"
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\nShowApps: true\n", map[string]string{
		"zelda/theme.yml": zelda,
		// Of the directory of a theme, theme.yml is read and nothing else.
		"zelda/theme.css":  "body { color: red }",
		"zelda/other.yml":  themeFileColors + "  radius: 4em\n",
		"plain.yml":        themeFileColors,
		"broken/theme.yml": themeFileColors + "  radius: 4em\n",
		"Mixed/theme.yml":  themeFileColors,
		"yaml/theme.yaml":  themeFileColors,
		"notes/readme.txt": "A directory without a theme.yml is left alone.",
		// A name has one theme: the directory, and the file is reported.
		"shadowed.yml":       themeFileColors + "  radius: 2px\n",
		"shadowed/theme.yml": themeFileColors + "  radius: 12px\n",
	})

	raw := getPage(t, handler, "/settings/theme")
	page := readThemeFilePage(t, raw)
	swatches := []string{"background-color: #10241b;", "background-color: #e8f1e4;", "background-color: #f2b84b;"}
	require.Equal(t, []themeFilePageCard{
		{value: "plain", label: "plain", title: "plain", colors: swatches},
		{value: "shadowed", label: "shadowed", title: "shadowed", colors: swatches},
		{value: "zelda", label: "Zelda", title: "Zelda", about: "2.0", colors: swatches},
	}, page.cards)
	require.Equal(t, []string{
		"Mixed/theme.yml " + i18n.T("en", "theme_file_problem_name"),
		"broken/theme.yml " + i18n.T("en", "theme_file_problem_value") + " tokens.radius",
		"shadowed.yml " + i18n.T("en", "theme_file_problem_duplicate"),
		"yaml/theme.yaml " + i18n.T("en", "theme_file_problem_extension"),
	}, page.lineTexts)
	require.Empty(t, page.alerts)
	require.Contains(t, page.help, i18n.T("en", "theme_files_help"))

	// Every theme has a preview. A file is offered as a download; a
	// directory is not, because it is not one file.
	require.Equal(t, map[string][]string{
		"plain":    {"/?theme-preview=plain", "/settings/theme/export?name=plain"},
		"shadowed": {"/?theme-preview=shadowed"},
		"zelda":    {"/?theme-preview=zelda"},
	}, themeCardLinks(t, raw))
	for name, status := range map[string]int{
		"plain": http.StatusOK, "zelda": http.StatusNotFound, "shadowed": http.StatusNotFound,
		"zelda/theme": http.StatusNotFound, "zelda/theme.yml": http.StatusNotFound, "broken": http.StatusNotFound,
	} {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/theme/export?name="+url.QueryEscape(name), nil))
		require.Equal(t, status, rec.Code, name)
	}

	// The preview of the name with both forms shows the directory.
	require.Equal(t, themeFileStyle+"--flare-radius:12px;", bodyStyle(t, getPage(t, handler, "/?theme-preview=shadowed")))
	require.Equal(t, zeldaStyle, bodyStyle(t, getPage(t, handler, "/?theme-preview=zelda")))

	// Selecting the theme stores the name of the directory and applies the
	// theme to every themed page.
	selected := themeRequest(handler, url.Values{"theme": {"zelda"}})
	require.Equal(t, http.StatusOK, selected.Code)
	require.Equal(t, zeldaStyle, bodyStyle(t, selected.Body.String()))
	stored, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Contains(t, string(stored), "Theme: zelda")
	for _, path := range []string{"/", "/bookmarks", "/applications", "/help", "/settings/theme", "/settings/appearance"} {
		require.Equal(t, zeldaStyle, bodyStyle(t, getPage(t, handler, path)), path)
	}

	// What is not a theme cannot be selected: a directory that is not used,
	// one without a theme.yml, and anything inside a directory.
	for _, name := range []string{"broken", "Mixed", "yaml", "notes", "zelda/theme", "zelda/theme.yml", "zelda/other", "theme"} {
		require.Equal(t, http.StatusBadRequest, themeRequest(handler, url.Values{"theme": {name}}).Code, name)
		require.Equal(t, zeldaStyle, bodyStyle(t, getPage(t, handler, "/")), name)
	}

	// No file is imported under the name of a directory that is a theme,
	// with the box for replacing or without it.
	for _, replace := range []bool{false, true} {
		refused := importFile(t, handler, "zelda.yml", themeFileColors, replace)
		require.Equal(t, http.StatusConflict, refused.Code)
		require.Equal(t, []string{i18n.T("en", "theme_import_directory")}, readImportPage(t, refused.Body.String()).alerts)
		require.NoFileExists(t, filepath.Join(define.UserThemeDirectory, "zelda.yml"))
	}
	require.Equal(t, zeldaStyle, bodyStyle(t, getPage(t, handler, "/")))

	// An edit of theme.yml is applied by the next page, without a restart.
	writeThemeFile(t, "zelda/theme.yml", strings.Replace(zelda, "radius: 10px", "radius: 3px", 1))
	edited := themeFileStyle + "--flare-radius:3px;"
	for _, path := range []string{"/", "/help", "/settings/theme"} {
		require.Equal(t, edited, bodyStyle(t, getPage(t, handler, path)), path)
	}

	// A theme.yml that is rejected leaves the pages with the colors of the
	// stylesheet, and the theme page names the theme and the reason.
	writeThemeFile(t, "zelda/theme.yml", strings.Replace(zelda, "  primary: \"#e8f1e4\"\n", "", 1))
	require.Empty(t, bodyStyle(t, getPage(t, handler, "/")))
	page = readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Equal(t, []string{i18n.Format("en", "theme_unavailable", map[string]any{"name": "zelda"})}, page.alerts)
	require.Contains(t, page.lines, []string{"zelda/theme.yml", "tokens.primary"})

	// Removing the directory is reported in the same way, and putting it
	// back brings the theme back.
	require.NoError(t, os.RemoveAll(filepath.Join(define.UserThemeDirectory, "zelda")))
	require.Empty(t, bodyStyle(t, getPage(t, handler, "/")))
	require.Len(t, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).alerts, 1)
	writeThemeFile(t, "zelda/theme.yml", zelda)
	require.Equal(t, zeldaStyle, bodyStyle(t, getPage(t, handler, "/")))
	require.Empty(t, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).alerts)

	// The theme page names files and directories in the interface language.
	require.NoError(t, os.WriteFile("config.yml", []byte("Locale: zh\nTheme: zelda\nShowApps: true\n"), 0600))
	page = readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Equal(t, "塞尔达", page.cards[2].title)
	require.Contains(t, page.help, i18n.T("zh", "theme_files_help"))
	require.Contains(t, page.lineTexts, "shadowed.yml "+i18n.T("zh", "theme_file_problem_duplicate"))
}

// The theme of a directory in config.yml is applied when the instance starts.
func TestThemeDirectorySelectedAtStartup(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: zelda\n", map[string]string{"zelda/theme.yml": themeFileColors + "  radius: 10px\n"})
	for _, path := range []string{"/", "/help", "/settings/theme"} {
		require.Equal(t, themeFileStyle+"--flare-radius:10px;", bodyStyle(t, getPage(t, handler, path)), path)
	}
	require.Empty(t, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).alerts)
}
