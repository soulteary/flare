package home

import (
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/fn"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestReadIndexMatchesLegacyRendering(t *testing.T) {
	t.Chdir(t.TempDir())
	fixture := model.Bookmarks{Categories: []model.Category{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}, {ID: "secret", Private: true}}, Items: []model.Bookmark{
		{Name: "Alpha <tag>", URL: "https://example.com/A", Category: "a"},
		{Name: "Beta", URL: "https://example.com/b", Category: "b", Desc: "Description"},
		{Name: "Default", URL: "https://example.com/default"},
		{Name: "Hidden", URL: "https://example.com/secret", Category: "secret"},
		{Name: "Private", URL: "https://example.com/private", Category: "a", Private: true},
		{Name: "Dynamic", URL: "https://{host}/dynamic", Category: "b"},
	}}
	require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "Alpha <tag>", URL: "https://example.com/A"}, {Name: "Beta", URL: "https://example.com/b", Desc: "Description"}, {Name: "Private", URL: "https://example.com/private", Private: true}}}))
	require.True(t, data.SaveNormalBookmarks(fixture))
	t.Cleanup(func() {
		require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{}))
		require.True(t, data.SaveNormalBookmarks(model.Bookmarks{}))
	})
	for _, admin := range []bool{false, true} {
		apps, bookmarks, err := data.LoadNavigationViews(true, true, admin, true)
		require.NoError(t, err)
		for _, mode := range []string{"all", "tabs", "collapse"} {
			opts := &model.Application{Locale: "en", Homepage: &model.HomepageOptions{CategoryNavigation: mode}}
			for _, query := range []string{"", "ALPHA", "description", "not-found", "example", "client.test"} {
				for _, selected := range []string{"", "a", "b", "secret", "missing"} {
					info := &fn.DynamicURL{Host: "client.test", Hostname: "client.test", Origin: "https://client.test"}
					expected, _ := generateBookmarkSnapshot(query, selected, "/bookmarks", opts, bookmarks.Copy(), info)
					require.Equal(t, expected, navigationFragment(bookmarks, opts, false, query, selected, "/bookmarks", info), "admin=%v mode=%s query=%s category=%s", admin, mode, query, selected)
				}
				expected, _ := generateApplicationsSnapshot(query, opts, apps.Copy())
				require.Equal(t, expected, navigationFragment(apps, opts, true, query, "", "/applications", nil))
			}
		}
		categories := bookmarks.Categories()
		categories[0].Name = "mutated"
		require.Equal(t, "A", bookmarks.Categories()[0].Name)
		item := bookmarks.Item(0)
		item.Name = "mutated"
		require.Equal(t, "mutated", item.Name)
		require.Equal(t, "Alpha <tag>", bookmarks.Item(0).Name)
	}
}
