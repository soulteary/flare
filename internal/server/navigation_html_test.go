package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
	htmlparse "golang.org/x/net/html"
)

func TestNavigationFieldsAreTextAndEscapedAttributes(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	settings, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	settings.ShowApps = true
	settings.ShowBookmarks = true
	const name = `<b id="injected">Name & "quoted"</b>`
	const desc = `<img src=x onerror="audit()">Description & text`
	const address = `https://nas.example/path?q=" onmouseover="audit()&x=1#part`
	const icon = `https://icons.example/path?q=" onerror="audit()&x=1`
	for _, categorized := range []bool{false, true} {
		bookmarks := model.Bookmarks{Items: []model.Bookmark{{Name: name, URL: address, Icon: icon}, {Name: "Unsafe bookmark", URL: "javascript:alert(1)", Icon: "data:image/svg+xml,<svg/>"}}}
		if categorized {
			bookmarks.Categories = []model.Category{{ID: "first", Name: `<b>Group & text</b>`}}
		}
		require.True(t, data.SaveNormalBookmarks(bookmarks))
		require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: name, Desc: desc, URL: address, Icon: icon}, {Name: "Unsafe app", URL: "data:text/html,<b>test</b>", Icon: `"><img src=x onerror="audit()">`}}}))
		for _, encoded := range []bool{false, true} {
			for _, newTab := range []bool{false, true} {
				settings.EnableEncryptedLink = encoded
				settings.OpenAppNewTab = newTab
				settings.OpenBookmarkNewTab = newTab
				require.True(t, data.UpdateAppearance(settings))
				for _, path := range []string{"/", "/applications", "/bookmarks"} {
					recorder := httptest.NewRecorder()
					handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
					require.Equal(t, 200, recorder.Code)
					doc, err := htmlparse.Parse(strings.NewReader(recorder.Body.String()))
					require.NoError(t, err)
					count := assertNavigationDOM(t, doc, encoded, name, desc, address, icon)
					require.GreaterOrEqual(t, count, 2)
				}
			}
		}
	}
}

func textContent(node *htmlparse.Node) string {
	var out strings.Builder
	var visit func(*htmlparse.Node)
	visit = func(n *htmlparse.Node) {
		if n.Type == htmlparse.TextNode {
			out.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(node)
	return out.String()
}

func assertNavigationDOM(t *testing.T, doc *htmlparse.Node, encoded bool, name, desc, address, icon string) int {
	t.Helper()
	count := 0
	var visit func(*htmlparse.Node)
	visit = func(node *htmlparse.Node) {
		if node.Type == htmlparse.ElementNode {
			attrs := map[string]string{}
			for _, a := range node.Attr {
				attrs[a.Key] = a.Val
				require.NotEqual(t, "injected", a.Val, "name markup became a DOM element")
				require.False(t, strings.HasPrefix(strings.ToLower(a.Key), "on"), "injected event attribute")
			}
			require.NotEqual(t, "b", node.Data, "name/description markup became an element")
			if node.Data == "a" && (attrs["class"] == "app-item" || attrs["class"] == "bookmark") {
				count++
				if strings.Contains(textContent(node), "Unsafe") {
					require.Equal(t, "#", attrs["href"])
				} else {
					require.Contains(t, textContent(node), name)
					if encoded {
						require.True(t, strings.HasPrefix(attrs["href"], "/redir/url?id="))
					} else {
						require.Equal(t, address, attrs["href"])
					}
					if attrs["class"] == "app-item" {
						require.Equal(t, name, attrs["title"])
						require.Contains(t, textContent(node), desc)
					}
				}
			}
			if node.Data == "img" {
				require.Equal(t, icon, attrs["src"], "untrusted icon generated extra markup")
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			visit(child)
		}
	}
	visit(doc)
	return count
}
