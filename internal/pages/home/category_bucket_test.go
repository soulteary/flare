package home

import (
	"strings"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/fn"
	"github.com/stretchr/testify/require"
)

func TestCategoryBucketsKeepDefaultPlacementAndStoredOrder(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Cleanup(func() { data.SaveNormalBookmarks(model.Bookmarks{}) })
	fixture := model.Bookmarks{
		Categories: []model.Category{{ID: "apps", Name: "First"}, {ID: "second", Name: "Second"}},
		Items: []model.Bookmark{
			{Name: "second-first", URL: "{origin}/second-first", Category: "second"},
			{Name: "implicit-first", URL: "{origin}/implicit-first"},
			{Name: "explicit-first", URL: "{origin}/explicit-first", Category: "apps"},
			{Name: "second-last", URL: "{origin}/second-last", Category: "second"},
		},
	}
	require.True(t, data.SaveNormalBookmarks(fixture))
	options := &model.Application{Locale: "en", ShowBookmarks: true, Homepage: &model.HomepageOptions{CategoryNavigation: "all"}}
	info := fn.DynamicURL{Origin: "https://nas.example"}
	output := string(GenerateBookmarkNavigationTemplate("", "", "/", options, &info))
	previous := -1
	for _, name := range []string{"implicit-first", "explicit-first", "second-first", "second-last"} {
		position := strings.Index(output, ">"+name+"<")
		require.Greater(t, position, previous, name)
		previous = position
		require.Contains(t, output, `href="https://nas.example/`+name+`"`)
	}
	require.NotContains(t, output, ">orphan<")
	options.Homepage.CategoryNavigation = "tabs"
	selected := string(GenerateBookmarkNavigationTemplate("", "second", "/", options, &info))
	require.NotContains(t, selected, ">implicit-first<")
	require.NotContains(t, selected, ">explicit-first<")
	require.Contains(t, selected, ">second-first<")
	filtered := string(GenerateBookmarkNavigationTemplate("last", "second", "/", options, &info))
	require.Contains(t, filtered, ">second-last<")
	require.NotContains(t, filtered, ">second-first<")
}
