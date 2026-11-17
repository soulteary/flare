package server

import (
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/i18n"
)

const themeBackgroundText = "background:\n  image: assets/wallpaper.png\n  dim: 40\n"

// The rule a page has for a background image of a theme, with the name of
// the theme, the hash, the name of the image and the dim in it.
var themeBackgroundRule = regexp.MustCompile(`^body\{background-image:(?:linear-gradient\(color-mix\(in srgb,var\(--flare-background\) (\d+)%,transparent\),color-mix\(in srgb,var\(--flare-background\) (\d+)%,transparent\)\),)?url\(/theme/([a-z0-9-]+)/([0-9a-f]{16})/assets/([A-Za-z0-9._-]+)\);background-size:cover;background-position:center;background-repeat:no-repeat;background-attachment:fixed\}$`)

// pageBackgrounds returns the rules for a background image a page has in the
// style elements of its head, as the name of the theme, the hash, the name of
// the image and the dim of each.
func pageBackgrounds(t *testing.T, page string) [][4]string {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	var rules [][4]string
	walkNodes(document, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "style" || node.FirstChild == nil {
			return
		}
		if found := themeBackgroundRule.FindStringSubmatch(node.FirstChild.Data); found != nil {
			require.Equal(t, "head", node.Parent.Data)
			require.Equal(t, found[1], found[2], "the two ends of the layer over the image")
			rules = append(rules, [4]string{found[3], found[4], found[5], found[1]})
		}
	})
	return rules
}

// pageStyleTexts returns what the style elements of a page hold, in the
// order of the document.
func pageStyleTexts(t *testing.T, page string) []string {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	var texts []string
	walkNodes(document, func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "style" && node.FirstChild != nil {
			texts = append(texts, node.FirstChild.Data)
		}
	})
	return texts
}

// A theme file can name an image of the assets as the background of the
// navigation pages.
func TestThemeBackgroundHTTP(t *testing.T) {
	paired := themeFileColors + "background:\n  image: assets/a.png\nmodes:\n  light:\n    background: \"#f6f3ea\"\n    primary: \"#2b2a27\"\n"
	handler := themeFilesRouter(t, "Locale: en\nTheme: walled\nShowApps: true\n", map[string]string{
		"walled/theme.yml":            themeFileColors + themeBackgroundText,
		"walled/assets/wallpaper.png": themeAssetPNG,
		"paired/theme.yml":            paired,
		"paired/theme.css":            themeStylesheetText,
		"paired/assets/a.png":         themeAssetPNG + "-PAIRED",
		"bare/theme.yml":              themeFileColors,
		"plain.yml":                   themeFileColors,
		"missing/theme.yml":           themeFileColors + "background:\n  image: assets/none.png\n",
		"missing/assets/a.png":        themeAssetPNG,
		"form/theme.yml":              themeFileColors + "background:\n  image: \"<b>wallpaper.png\"\n",
		"single.yml":                  themeFileColors + themeBackgroundText,
		"later/theme.yml":             themeFileColors + "background:\n  image: assets/a.png\n  size: contain\n",
		"later/assets/a.png":          themeAssetPNG,
	})
	navigation := []string{"/", "/applications", "/bookmarks", "/help"}
	settings := []string{"/settings/theme", "/settings/appearance", "/settings/search", "/settings/about", "/settings/weather"}

	// The navigation pages have the rule after the styles of the application,
	// with the address the image is served at. The settings take the colors
	// of the theme and no image.
	hash := ""
	for _, path := range navigation {
		body := getPage(t, handler, path)
		rules := pageBackgrounds(t, body)
		require.Len(t, rules, 1, path)
		require.Equal(t, [3]string{"walled", "wallpaper.png", "40"}, [3]string{rules[0][0], rules[0][2], rules[0][3]}, path)
		if hash == "" {
			hash = rules[0][1]
		}
		require.Equal(t, hash, rules[0][1], path)
		require.Equal(t, []string{"style", "style"}, pageStyles(t, body), path)
		require.Equal(t, themeFileStyle, bodyStyle(t, body), path)
	}
	for _, path := range settings {
		body := getPage(t, handler, path)
		require.Empty(t, pageBackgrounds(t, body), path)
		require.Equal(t, []string{"style"}, pageStyles(t, body), path)
		require.Equal(t, themeFileStyle, bodyStyle(t, body), path)
		require.NotContains(t, body, "/assets/wallpaper.png", path)
	}
	image := getStylesheet(handler, http.MethodGet, "/theme/walled/"+hash+"/assets/wallpaper.png")
	require.Equal(t, http.StatusOK, image.Code)
	require.Equal(t, "image/png", image.Header().Get("Content-Type"))
	require.Equal(t, themeAssetPNG, image.Body.String())

	// The theme page offers the themes whose image is there, and names the
	// others with the reason, as text. A part of a background for a later
	// release is listed as not used.
	raw := getPage(t, handler, "/settings/theme")
	page := readThemeFilePage(t, raw)
	offered := make([]string, 0, len(page.cards))
	for _, card := range page.cards {
		offered = append(offered, card.value)
	}
	require.Equal(t, []string{"bare", "later", "paired", "plain", "walled"}, offered)
	require.Equal(t, []string{
		"form/theme.yml " + i18n.T("en", "theme_file_problem_background") + " background.image",
		"later/theme.yml " + i18n.T("en", "theme_file_unused") + " background.size",
		"missing/theme.yml " + i18n.T("en", "theme_file_problem_background_asset") + " assets/none.png",
		"single.yml " + i18n.T("en", "theme_file_problem_background_asset") + " assets/wallpaper.png",
	}, page.lineTexts)
	require.NotContains(t, raw, "<b>wallpaper")

	// With a stylesheet and with values for light and dark, the rule comes
	// last: after the stylesheet of the theme and after its tokens. A
	// preview has the rule of the previewed theme, for that response alone.
	previewed := getPage(t, handler, "/?theme-preview=paired")
	styles := pageStyles(t, previewed)
	require.Len(t, styles, 4)
	require.Equal(t, "style", styles[0])
	require.Regexp(t, themeStylesheetAddress, styles[1])
	require.Equal(t, []string{"style", "style"}, styles[2:])
	rules := pageBackgrounds(t, previewed)
	require.Len(t, rules, 1)
	require.Equal(t, [3]string{"paired", "a.png", ""}, [3]string{rules[0][0], rules[0][2], rules[0][3]})
	require.Equal(t, strings.Split(styles[1], "/")[3], rules[0][1], "the stylesheet and the image have one hash")
	texts := pageStyleTexts(t, previewed)
	require.Len(t, texts, 3)
	require.True(t, strings.HasPrefix(texts[1], "@media "), "the tokens for light and dark")
	require.Regexp(t, themeBackgroundRule, texts[2], "the rule is the last of the styles")
	require.Equal(t, themeAssetPNG+"-PAIRED", getStylesheet(handler, http.MethodGet, "/theme/paired/"+rules[0][1]+"/assets/a.png").Body.String())
	require.Empty(t, pageBackgrounds(t, getPage(t, handler, "/?theme-preview=bare")))
	require.Empty(t, pageBackgrounds(t, getPage(t, handler, "/?theme-preview=blackboard")))
	require.Len(t, pageBackgrounds(t, getPage(t, handler, "/")), 1)

	// In safe mode a page has no image of a theme.
	safe := getPage(t, handler, "/?theme-safe-mode")
	require.Empty(t, pageBackgrounds(t, safe))
	require.Empty(t, bodyStyle(t, safe))

	// An edit of the image is taken by the next page under another address.
	writeThemeFile(t, "walled/assets/wallpaper.png", themeAssetPNG+"-EDITED")
	rules = pageBackgrounds(t, getPage(t, handler, "/"))
	require.Len(t, rules, 1)
	require.NotEqual(t, hash, rules[0][1])
	require.Equal(t, themeAssetPNG+"-EDITED", getStylesheet(handler, http.MethodGet, "/theme/walled/"+rules[0][1]+"/assets/wallpaper.png").Body.String())
	require.Equal(t, http.StatusNotFound, getStylesheet(handler, http.MethodGet, "/theme/walled/"+hash+"/assets/wallpaper.png").Code)

	// Without its image the theme is not in use: the pages have the colors
	// of the stylesheet of the application, and the theme page says why.
	require.NoError(t, os.Remove(filepath.Join(define.UserThemeDirectory, "walled", "assets", "wallpaper.png")))
	for _, path := range navigation {
		body := getPage(t, handler, path)
		require.Empty(t, pageBackgrounds(t, body), path)
		require.Equal(t, []string{"style"}, pageStyles(t, body), path)
		require.Empty(t, bodyStyle(t, body), path)
	}
	page = readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Equal(t, []string{i18n.Format("en", "theme_unavailable", map[string]any{"name": "walled"})}, page.alerts)
	require.Contains(t, page.lineTexts, "walled/theme.yml "+i18n.T("en", "theme_file_problem_background_asset")+" assets/wallpaper.png")
	writeThemeFile(t, "walled/assets/wallpaper.png", themeAssetPNG)
	require.Len(t, pageBackgrounds(t, getPage(t, handler, "/")), 1)

	// A theme without a background image is selected and has none.
	require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"bare"}}).Code)
	for _, path := range navigation {
		body := getPage(t, handler, path)
		require.Empty(t, pageBackgrounds(t, body), path)
		require.Equal(t, themeFileStyle, bodyStyle(t, body), path)
	}

	// The reasons are given in the interface language.
	require.NoError(t, os.WriteFile("config.yml", []byte("Locale: zh\nTheme: bare\nShowApps: true\n"), 0600))
	page = readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Contains(t, page.lineTexts, "form/theme.yml "+i18n.T("zh", "theme_file_problem_background")+" background.image")
	require.Contains(t, page.lineTexts, "missing/theme.yml "+i18n.T("zh", "theme_file_problem_background_asset")+" assets/none.png")
}

// A file that is imported becomes a theme of one file, which has no assets:
// one that names a background image is refused, and nothing is stored.
func TestThemeImportWithABackgroundHTTP(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\n", map[string]string{"forest.yml": themeFileColors})
	rejected := func(reason string) string {
		return i18n.Format("en", "theme_import_rejected", map[string]any{"reason": i18n.T("en", reason)})
	}
	for name, tc := range map[string]struct {
		content string
		replace bool
		want    string
	}{
		"an image":           {themeFileColors + themeBackgroundText, false, rejected("theme_file_problem_background_asset") + " assets/wallpaper.png"},
		"in place of a file": {themeFileColors + themeBackgroundText, true, rejected("theme_file_problem_background_asset") + " assets/wallpaper.png"},
		"another form":       {themeFileColors + "background:\n  image: wallpaper.png\n", false, rejected("theme_file_problem_background") + " background.image"},
		"a fraction":         {themeFileColors + "background:\n  image: assets/a.png\n  dim: 0.4\n", false, rejected("theme_file_problem_background") + " background.dim"},
	} {
		filename := "walled.yml"
		if tc.replace {
			filename = "forest.yml"
		}
		rec := importFile(t, handler, filename, tc.content, tc.replace)
		require.Equal(t, http.StatusBadRequest, rec.Code, name)
		require.Equal(t, []string{tc.want}, readImportPage(t, rec.Body.String()).alerts, name)
		_, err := os.Stat(filepath.Join(define.UserThemeDirectory, "walled.yml"))
		require.ErrorIs(t, err, os.ErrNotExist, name)
		kept, err := os.ReadFile(filepath.Join(define.UserThemeDirectory, "forest.yml"))
		require.NoError(t, err, name)
		require.Equal(t, themeFileColors, string(kept), name)
	}
	// The same file without the background is imported.
	rec := importFile(t, handler, "walled.yml", themeFileColors, false)
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, define.IsUserTheme("walled"))
}
