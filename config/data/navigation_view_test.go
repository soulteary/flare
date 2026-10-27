package data

import (
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func TestNavigationViewsPinAuthorizedSnapshots(t *testing.T) {
	t.Chdir(t.TempDir())
	t.Cleanup(func() {
		require.True(t, SaveFavoriteBookmarks(model.Bookmarks{}))
		require.True(t, SaveNormalBookmarks(model.Bookmarks{}))
	})
	fixture := model.Bookmarks{Items: []model.Bookmark{{Name: "public", URL: "https://public.example"}, {Name: "secret", URL: "https://{host}", Private: true}}}
	require.True(t, SaveFavoriteBookmarks(fixture))
	require.True(t, SaveNormalBookmarks(fixture))
	adminApps, adminBookmarks, err := LoadNavigationViews(true, true, true, true)
	require.NoError(t, err)
	publicApps, publicBookmarks, err := LoadNavigationViews(true, true, false, true)
	require.NoError(t, err)
	require.NotEqual(t, adminApps.Version(), publicApps.Version())
	require.NotEqual(t, adminBookmarks.Version(), publicBookmarks.Version())
	require.False(t, adminApps.RequestIndependent())
	require.True(t, publicApps.RequestIndependent())
	copy := publicApps.Copy()
	copy.Items[0].Name = "mutated"
	require.Equal(t, "public", publicApps.Copy().Items[0].Name)
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "new", URL: "https://new.example"}}}))
	next, _, err := LoadNavigationViews(true, false, false, true)
	require.NoError(t, err)
	require.NotEqual(t, next.Version(), publicApps.Version())
	require.Equal(t, "public", publicApps.Copy().Items[0].Name)
	require.Equal(t, "new", next.Copy().Items[0].Name)
}
