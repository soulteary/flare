package server

import (
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
)

// bodyStyle returns the style attribute of the page body as a browser reads it.
func bodyStyle(t *testing.T, page string) string {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	var find func(*html.Node) (string, bool)
	find = func(node *html.Node) (string, bool) {
		if node.Type == html.ElementNode && node.Data == "body" {
			for _, attribute := range node.Attr {
				if attribute.Key == "style" {
					return attribute.Val, true
				}
			}
			return "", true
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if style, found := find(child); found {
				return style, true
			}
		}
		return "", false
	}
	style, found := find(document)
	require.True(t, found, "page has no body")
	return style
}

// A theme that sets tokens besides its three colors reaches the themed pages
// through the style attribute of the body, unchanged by HTML escaping.
func TestThemeTokensReachThemedPages(t *testing.T) {
	handler := newMDITestRouter(t)
	original := define.ThemePalettes
	t.Cleanup(func() {
		define.ThemePalettes = original
		define.UpdatePagePalettes()
	})
	file, err := model.ParseThemeFile([]byte("contract: 1\ntokens:\n" +
		"  background: \"#101010\"\n  primary: \"#f0f0f0\"\n  accent: \"#808080\"\n" +
		"  hover: \"#ffffff22\"\n  radius: 12px\n  font-family: Helvetica Neue, 思源黑体, sans-serif\n"))
	require.NoError(t, err)
	define.ThemePalettes = append(slices.Clone(original), file.Theme("rounded"))
	define.SetTheme("rounded")

	want := "--color-background:#101010;--color-primary:#f0f0f0;--color-accent:#808080;" +
		"--flare-hover:#ffffff22;--flare-radius:12px;--flare-font-family:Helvetica Neue, 思源黑体, sans-serif;"
	for _, path := range []string{"/", "/bookmarks", "/applications", "/help", define.SettingPages.Theme.Path} {
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, response.Code, path)
		require.Equal(t, want, bodyStyle(t, response.Body.String()), path)
	}

	// A bundled theme sets its three colors only.
	define.SetTheme("blackboard")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, "--color-background:#1a1a1a;--color-primary:#FFFDEA;--color-accent:#5c5c5c;", bodyStyle(t, response.Body.String()))
}
