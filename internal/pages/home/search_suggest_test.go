package home

import (
	"strings"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/fn"
	"github.com/stretchr/testify/require"
)

func TestSuggestionsMatchExistingSearchAndVisibleModules(t *testing.T) {
	opts := model.Application{ShowApps: true, ShowBookmarks: true, OpenAppNewTab: true}
	apps := model.Bookmarks{Items: []model.Bookmark{{Name: "Console", URL: "https://console.example", Desc: "Needle"}}}
	bookmarks := model.Bookmarks{Categories: []model.Category{{ID: "known"}}, Items: []model.Bookmark{
		{Name: "Needle", URL: "https://book.example"},
		{Name: "Only description", URL: "https://desc.example", Desc: "Needle"},
		{Name: "Needle orphan", URL: "https://orphan.example", Category: "missing"},
		{Name: "Needle unsafe", URL: "javascript:alert(1)"},
	}}
	t.Chdir(t.TempDir())
	require.True(t, data.SaveFavoriteBookmarks(apps))
	require.True(t, data.SaveNormalBookmarks(bookmarks))
	result, err := collectSearchSuggestions(opts, "NEEDLE", nil)
	require.NoError(t, err)
	require.Len(t, result, 2)
	require.True(t, result[0].NewTab)
	require.False(t, result[1].NewTab)
	opts.ShowApps = false
	result, err = collectSearchSuggestions(opts, "needle", nil)
	require.NoError(t, err)
	require.Len(t, result, 1)
}

func TestSuggestionsBoundedAndPreserveLinkSemantics(t *testing.T) {
	opts := model.Application{ShowApps: true, EnableEncryptedLink: true}
	apps := model.Bookmarks{}
	for i := 0; i < 20; i++ {
		apps.Items = append(apps.Items, model.Bookmark{Name: "Dynamic", URL: "{origin}/panel"})
	}
	info := fn.DynamicURL{Origin: "https://home.example"}
	t.Chdir(t.TempDir())
	require.True(t, data.SaveFavoriteBookmarks(apps))
	result, err := collectSearchSuggestions(opts, "home.example", &info)
	require.NoError(t, err)
	require.Len(t, result, 8)
	require.True(t, strings.HasPrefix(result[0].URL, "/redir/url?go="))
}

func TestSuggestionURLs(t *testing.T) {
	for _, value := range []string{"https://example.com:8443/a?q=1#x", "http://[::1]:8080/", "/panel", "./panel", "panel", "chrome-extension://abc/page"} {
		require.True(t, safeSuggestionURL(value), value)
	}
	for _, value := range []string{"javascript:alert(1)", "data:text/html,x", "//external.example/", "https://u:p@example.com", "https://example.com/\n"} {
		require.False(t, safeSuggestionURL(value), value)
	}
}
