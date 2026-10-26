package home

import (
	"strings"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/fn"
	"github.com/soulteary/flare/internal/resources/mdi"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"
)

// Inspect parsed DOM rather than source substrings: quotes in escaped text must
// survive as data without creating attributes, elements, or executable URLs.
func requireSafeNavigationDOM(t *testing.T, markup string, payload string) {
	t.Helper()
	document, err := html.Parse(strings.NewReader(markup))
	require.NoError(t, err)
	foundPayload := false
	var visit func(*html.Node)
	visit = func(node *html.Node) {
		if node.Type == html.ElementNode {
			require.NotContains(t, []string{"script", "iframe", "object", "embed"}, node.Data)
			for _, attribute := range node.Attr {
				require.False(t, strings.HasPrefix(strings.ToLower(attribute.Key), "on"), "event attribute injected: %s", attribute.Key)
				if attribute.Key == "href" {
					require.True(t, attribute.Val == "#" || fn.IsSafeNavigationURL(attribute.Val), "unsafe href: %s", attribute.Val)
				}
				if attribute.Key == "src" {
					require.True(t, fn.IsSafeImageURL(attribute.Val) || strings.HasPrefix(attribute.Val, "/assets/mdi/"), "unsafe image URL: %s", attribute.Val)
				}
			}
		}
		if node.Type == html.TextNode && strings.Contains(node.Data, payload) {
			foundPayload = true
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(document)
	require.True(t, foundPayload, "expected user text did not survive HTML escaping")
}

func TestNavigationRenderersEscapeAdversarialSavedFields(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, mdi.Init())
	payload := `"><script>alert('name')</script><img src=x onerror="alert(1)"> & text`
	fixture := model.Bookmarks{Items: []model.Bookmark{
		{Name: payload, Desc: payload, URL: `https://example.com/path?quote="&value=<script>`, Icon: `https://example.com/icon.svg?quote=" onerror="alert(1)`},
		{Name: payload, Desc: payload, URL: `javascript:alert(1)`, Icon: `"><img src=x onerror="alert(1)">`},
		{Name: payload, Desc: payload, URL: `https://example.com/`, Icon: "home"},
	}}
	require.True(t, data.SaveFavoriteBookmarks(fixture))
	require.True(t, data.SaveNormalBookmarks(fixture))
	for _, newTab := range []bool{false, true} {
		for _, encoded := range []bool{false, true} {
			options := &model.Application{Locale: "en", OpenAppNewTab: newTab, OpenBookmarkNewTab: newTab, EnableEncryptedLink: encoded, IconMode: "FILLING"}
			requireSafeNavigationDOM(t, string(GenerateApplicationsTemplate("", options)), payload)
			requireSafeNavigationDOM(t, string(GenerateBookmarkTemplate("", options)), payload)
			for _, mode := range []string{"all", "tabs", "collapse"} {
				categorized := fixture
				categorized.Categories = []model.Category{{ID: `category" onclick="alert(1)`, Name: payload}}
				require.True(t, data.SaveNormalBookmarks(categorized))
				options.Homepage = &model.HomepageOptions{CategoryNavigation: mode}
				requireSafeNavigationDOM(t, string(GenerateBookmarkNavigationTemplate("", "", "/", options)), payload)
			}
			require.True(t, data.SaveNormalBookmarks(fixture))
		}
	}
}
