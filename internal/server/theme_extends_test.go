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

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/i18n"
)

// A theme can be based on another theme. Its pages load the stylesheets of
// the themes below it before its own, an image it does not replace is served
// by the theme that brings it, and the theme page says what it is based on.
func TestThemeExtendsHTTP(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: night\nShowApps: true\n", map[string]string{
		"zelda/theme.yml": themeFileColors + "  radius: 10px\nname: Zelda\nbackground:\n  image: assets/paper.png\n  dim: 40\n" +
			"options:\n  - id: frames\n    type: boolean\n",
		"zelda/theme.css":        themeStylesheetText,
		"zelda/assets/paper.png": themeAssetPNG,
		"compact.yml":            "contract: 1\nextends: zelda\nname: Zelda Compact\ntokens:\n  radius: 2px\n",
		"night/theme.yml":        "contract: 1\nextends: compact\ntokens:\n  accent: \"#00ff88\"\n",
		"night/theme.css":        ".app-title { letter-spacing: 1px }\n",
		"soft.yml":               "contract: 1\nextends: tron\ntokens:\n  accent: \"#ff8800\"\n",
		"lost.yml":               "contract: 1\nextends: nowhere\n",
		"loop.yml":               "contract: 1\nextends: loop\n",
	})
	navigation := []string{"/", "/applications", "/bookmarks", "/help"}
	settings := []string{"/settings/theme", "/settings/appearance", "/settings/search"}
	nightStyle := "--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#00ff88;--flare-radius:2px;"

	// The navigation pages load the stylesheet of the theme two below and
	// then the one of the theme, between the stylesheet of the application
	// and the rule for the image. The image is among the assets of the theme
	// that brings it.
	styles := pageStyles(t, getPage(t, handler, "/"))
	require.Len(t, styles, 4)
	zeldaSheet, nightSheet := styles[1], styles[2]
	require.Regexp(t, themeStylesheetAddress, zeldaSheet)
	require.Regexp(t, themeStylesheetAddress, nightSheet)
	require.True(t, strings.HasPrefix(zeldaSheet, "/theme/zelda/"), zeldaSheet)
	require.True(t, strings.HasPrefix(nightSheet, "/theme/night/"), nightSheet)
	zeldaHash := strings.Split(zeldaSheet, "/")[3]
	for _, path := range navigation {
		page := getPage(t, handler, path)
		require.Equal(t, []string{"style", zeldaSheet, nightSheet, "style"}, pageStyles(t, page), path)
		require.Equal(t, [][4]string{{"zelda", zeldaHash, "paper.png", "40"}}, pageBackgrounds(t, page), path)
		require.Equal(t, nightStyle, bodyStyle(t, page), path)
		attributes, _ := bodyAttributes(t, page)
		require.Equal(t, []string{"data-theme-frames=false"}, attributes, path)
	}
	// The settings take the tokens and load no stylesheet of a theme.
	for _, path := range settings {
		page := getPage(t, handler, path)
		require.Equal(t, nightStyle, bodyStyle(t, page), path)
		for _, style := range pageStyles(t, page) {
			require.NotContains(t, style, "/theme/", path)
		}
		require.Empty(t, pageBackgrounds(t, page), path)
	}

	// Each theme serves what it brings, under its own name.
	served := getStylesheet(handler, http.MethodGet, zeldaSheet)
	require.Equal(t, http.StatusOK, served.Code)
	require.Equal(t, themeStylesheetText, served.Body.String())
	served = getStylesheet(handler, http.MethodGet, nightSheet)
	require.Equal(t, http.StatusOK, served.Code)
	require.Equal(t, ".app-title { letter-spacing: 1px }\n", served.Body.String())
	served = getStylesheet(handler, http.MethodGet, "/theme/zelda/"+zeldaHash+"/assets/paper.png")
	require.Equal(t, http.StatusOK, served.Code)
	require.Equal(t, themeAssetPNG, served.Body.String())
	for _, address := range []string{
		"/theme/night/" + zeldaHash + "/assets/paper.png", "/theme/compact/" + zeldaHash + "/assets/paper.png",
		"/theme/compact/" + zeldaHash + "/theme.css", "/theme/night/" + zeldaHash + "/theme.css",
	} {
		require.Equal(t, http.StatusNotFound, getStylesheet(handler, http.MethodGet, address).Code, address)
	}

	// A preview has the themes below the previewed theme, and a page in safe
	// mode has no theme of the directory at all.
	previewed := getPage(t, handler, "/?theme-preview=compact")
	require.Equal(t, []string{"style", zeldaSheet, "style"}, pageStyles(t, previewed))
	require.Equal(t, themeFileStyle+"--flare-radius:2px;", bodyStyle(t, previewed))
	require.Equal(t, [][4]string{{"zelda", zeldaHash, "paper.png", "40"}}, pageBackgrounds(t, previewed))
	previewed = getPage(t, handler, "/?theme-preview=soft")
	require.Equal(t, []string{"style"}, pageStyles(t, previewed))
	require.Equal(t, "--color-background:#242B33;--color-primary:#EFFBFF;--color-accent:#ff8800;", bodyStyle(t, previewed))
	safe := getPage(t, handler, "/?theme-safe-mode")
	require.Equal(t, []string{"style"}, pageStyles(t, safe))
	require.Empty(t, pageBackgrounds(t, safe))

	// The theme page says what a theme is based on, by the name the card of
	// that theme has, and names the theme a file is based on that is not in
	// use for it.
	raw := getPage(t, handler, "/settings/theme")
	sheet, options := i18n.T("en", "theme_stylesheet_note"), i18n.T("en", "theme_options_note")
	based := func(name string) string { return i18n.Format("en", "theme_extends_note", map[string]any{"name": name}) }
	require.Equal(t, map[string][]string{
		"compact": {based("Zelda"), options},
		"night":   {based("Zelda Compact"), sheet, options},
		"soft":    {based("tron")},
		"zelda":   {sheet, options},
	}, readThemeOptionsPage(t, raw).notes)
	lines := readThemeFilePage(t, raw).lineTexts
	require.Contains(t, lines, "lost.yml "+i18n.T("en", "theme_file_problem_extends")+" nowhere")
	require.Contains(t, lines, "loop.yml "+i18n.T("en", "theme_file_problem_extends_cycle")+" loop")

	// The options of the theme below are options of the theme, with values
	// that are stored for the theme.
	page := readThemeOptionsPage(t, raw)
	require.Equal(t, "Options of night", page.title)
	require.Equal(t, [][4]string{{"option-frames", "checkbox", "off", "frames"}}, page.fields)
	saved := themeRequest(handler, url.Values{"action": {"save-options"}, "theme-options-theme": {"night"}, "option-frames": {"true"}})
	require.Equal(t, http.StatusOK, saved.Code)
	require.Equal(t, map[string]string{"frames": "true"}, storedThemeOptions(t, "night"))
	require.Empty(t, storedThemeOptions(t, "zelda"))
	attributes, _ := bodyAttributes(t, getPage(t, handler, "/"))
	require.Equal(t, []string{"data-theme-frames=true"}, attributes)
	attributes, _ = bodyAttributes(t, getPage(t, handler, "/?theme-preview=zelda"))
	require.Equal(t, []string{"data-theme-frames=false"}, attributes)

	// An edit of the stylesheet of a theme below shows on the next page.
	writeThemeFile(t, "zelda/theme.css", themeStylesheetText+".app-title { font-weight: 600 }\n")
	edited := pageStyles(t, getPage(t, handler, "/"))
	require.Len(t, edited, 4)
	require.NotEqual(t, zeldaSheet, edited[1])
	require.Equal(t, nightSheet, edited[2])
	require.Equal(t, http.StatusNotFound, getStylesheet(handler, http.MethodGet, zeldaSheet).Code)
	require.Equal(t, http.StatusOK, getStylesheet(handler, http.MethodGet, edited[1]).Code)

	// A theme below that goes away leaves the selection without a theme,
	// and the page names the reason.
	require.NoError(t, os.Rename(filepath.Join(define.UserThemeDirectory, "compact.yml"), filepath.Join(define.UserThemeDirectory, ".compact.yml")))
	without := getPage(t, handler, "/")
	require.Equal(t, []string{"style"}, pageStyles(t, without))
	require.Empty(t, bodyStyle(t, without))
	require.Contains(t, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).lineTexts, "night/theme.yml "+i18n.T("en", "theme_file_problem_extends")+" compact")
	require.NoError(t, os.Rename(filepath.Join(define.UserThemeDirectory, ".compact.yml"), filepath.Join(define.UserThemeDirectory, "compact.yml")))
	require.Equal(t, nightStyle, bodyStyle(t, getPage(t, handler, "/")))
}

// A theme file that is based on another theme is imported if it is a theme
// in use once it is there, and refused with the reason otherwise.
func TestThemeExtendsImportHTTP(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\n", map[string]string{
		"zelda/theme.yml": themeFileColors + "name: Zelda\n",
		"zelda/theme.css": themeStylesheetText,
		"plain.yml":       themeFileColors,
	})
	rejected := func(reason string) string {
		return i18n.Format("en", "theme_import_rejected", map[string]any{"reason": i18n.T("en", reason)})
	}
	entries := func() []string {
		t.Helper()
		listed, err := os.ReadDir(define.UserThemeDirectory)
		require.NoError(t, err)
		names := make([]string, 0, len(listed))
		for _, entry := range listed {
			names = append(names, entry.Name())
		}
		return names
	}
	before := entries()
	for name, tc := range map[string]struct {
		filename, content string
		replace           bool
		text              string
	}{
		"a theme that is not there":  {"orphan.yml", "contract: 1\nextends: nowhere\n", false, rejected("theme_file_problem_extends") + " nowhere"},
		"itself":                     {"orphan.yml", "contract: 1\nextends: orphan\n", false, rejected("theme_file_problem_extends_cycle") + " orphan"},
		"a name that is none":        {"orphan.yml", "contract: 1\nextends: ../zelda\n", false, rejected("theme_file_problem_extends_value") + " ../zelda"},
		"colors that do not go":      {"orphan.yml", "contract: 1\nextends: zelda\ntokens:\n  primary: \"#1b2a22\"\n", false, rejected("theme_file_problem_contrast")},
		"an image above a directory": {"orphan.yml", "contract: 1\nextends: zelda\nbackground:\n  image: assets/a.png\n", false, rejected("theme_file_problem_background_asset") + " assets/a.png"},
	} {
		refused := importFile(t, handler, tc.filename, tc.content, tc.replace)
		require.Equal(t, http.StatusBadRequest, refused.Code, name)
		page := readImportPage(t, refused.Body.String())
		require.Equal(t, []string{tc.text}, page.alerts, name)
		require.Contains(t, page.alerts[0], "Nothing was imported.", name)
		require.Equal(t, before, entries(), name)
	}

	// Based on a directory and on a bundled theme. The card says so, and a
	// preview has the stylesheet of the directory.
	for filename, content := range map[string]string{
		"child.yml": "contract: 1\nextends: zelda\nname: Child\ntokens:\n  radius: 2px\n",
		"soft.yml":  "contract: 1\nextends: tron\ntokens:\n  accent: \"#ff8800\"\n",
	} {
		imported := importFile(t, handler, filename, content, false)
		require.Equal(t, http.StatusOK, imported.Code, filename)
		require.Empty(t, readImportPage(t, imported.Body.String()).alerts, filename)
	}
	notes := readThemeOptionsPage(t, getPage(t, handler, "/settings/theme")).notes
	require.Equal(t, []string{"Based on Zelda."}, notes["child"])
	require.Equal(t, []string{"Based on tron."}, notes["soft"])
	previewed := getPage(t, handler, "/?theme-preview=child")
	styles := pageStyles(t, previewed)
	require.Len(t, styles, 2)
	require.True(t, strings.HasPrefix(styles[1], "/theme/zelda/"), styles[1])
	require.Equal(t, themeFileStyle+"--flare-radius:2px;", bodyStyle(t, previewed))

	// The file is handed out as it was stored, with the theme it names.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/theme/export?name=child", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, "contract: 1\nextends: zelda\nname: Child\ntokens:\n  radius: 2px\n", rec.Body.String())

	// Replacing a file with one that is based on a theme based on it is
	// refused: the two would be based on each other.
	require.NoError(t, os.WriteFile(filepath.Join(define.UserThemeDirectory, "plain-child.yml"), []byte("contract: 1\nextends: plain\n"), 0600))
	refused := importFile(t, handler, "plain.yml", "contract: 1\nextends: plain-child\n", true)
	require.Equal(t, http.StatusBadRequest, refused.Code)
	require.Equal(t, []string{rejected("theme_file_problem_extends_cycle") + " plain-child → plain"}, readImportPage(t, refused.Body.String()).alerts)
	stored, err := os.ReadFile(filepath.Join(define.UserThemeDirectory, "plain.yml"))
	require.NoError(t, err)
	require.Equal(t, themeFileColors, string(stored))
}
