package data

import (
	"fmt"
	"os"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/bookmarks"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// Exercise the release-sized fixture through cold/warm permission indexes and
// an external YAML policy edit. A warmed public cache must never admit a private
// last entry, and old opaque handles must honor the current policy.
func TestScaleAcceptanceTwentyThousandPermissionIndexes(t *testing.T) {
	prepareBookmarkHotPath(t)
	value := model.Bookmarks{}
	for i := 0; i < 100; i++ {
		value.Categories = append(value.Categories, model.Category{ID: fmt.Sprintf("c%03d", i), Name: fmt.Sprintf("Category %03d", i)})
	}
	for i := 0; i < 20000; i++ {
		value.Items = append(value.Items, model.Bookmark{Name: fmt.Sprintf("Entry %05d", i), URL: fmt.Sprintf("https://service.example/%05d", i), Category: fmt.Sprintf("c%03d", i%100), Private: i%2 == 1})
	}
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{}))
	require.True(t, SaveNormalBookmarks(value))
	for iteration := 0; iteration < 2; iteration++ {
		loaded, err := LoadNormalBookmarks()
		require.NoError(t, err)
		require.Len(t, loaded.Items, 20000)
		require.Len(t, FilterNavigation(loaded, false).Items, 10000)
		for _, admin := range []bool{false, true} {
			matches, err := SearchNavigationMatchesFor("Entry 19999", false, true, 8, nil, nil, admin)
			require.NoError(t, err)
			if admin {
				require.Len(t, matches, 1)
			} else {
				require.Empty(t, matches)
			}
			require.Equal(t, admin, MatchNavigationTargetFor(value.Items[19999].URL, admin, nil))
		}
	}
	target := value.Items[19998].URL
	handle := OpaqueNavigationHandle(target, nil)
	_, ok := MatchOpaqueNavigationTarget(handle, false)
	require.True(t, ok)
	// A parent restriction overrides explicit false for every member of c098.
	value.Categories[98].Private = true
	require.True(t, SaveNormalBookmarks(value))
	_, ok = MatchOpaqueNavigationTarget(handle, false)
	require.False(t, ok)
	require.True(t, MatchNavigationTargetFor(target, true, nil))
	handle = OpaqueNavigationHandle(target, nil)
	_, ok = MatchOpaqueNavigationTarget(handle, true)
	require.True(t, ok)
	matches, err := SearchNavigationMatchesFor("Entry 19998", false, true, 8, nil, nil, false)
	require.NoError(t, err)
	require.Empty(t, matches)
	loaded, err := LoadNormalBookmarks()
	require.NoError(t, err)
	require.Len(t, FilterNavigation(loaded, false).Items, 9800)
}

// The maximum supported transfer must preserve per-entry privacy, category
// placement and the exact previous-file backup across a real durable commit.
func TestScaleAcceptanceTwentyThousandImportRoundTrip(t *testing.T) {
	prepareBookmarkHotPath(t)
	source := model.Bookmarks{}
	for i := 0; i < 100; i++ {
		source.Categories = append(source.Categories, model.Category{ID: fmt.Sprintf("c%03d", i), Name: fmt.Sprintf("Category %03d", i)})
	}
	for i := 0; i < 20000; i++ {
		source.Items = append(source.Items, model.Bookmark{Name: fmt.Sprintf("Entry %05d", i), URL: fmt.Sprintf("https://service.example/%05d", i), Category: fmt.Sprintf("c%03d", i%100), Private: i%2 == 1})
	}
	raw, err := yaml.Marshal(source)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile("bookmarks.yml", []byte("# preserved original\nlinks: []\n"), 0600))
	before, err := os.ReadFile("bookmarks.yml")
	require.NoError(t, err)
	current, version, err := BookmarkSnapshot("bookmarks")
	require.NoError(t, err)
	entries, err := bookmarks.Parse(raw, "yaml")
	require.NoError(t, err)
	plan, err := bookmarks.Build(current, entries, bookmarks.Options{Strategy: "skip", Public: true})
	require.NoError(t, err)
	require.Equal(t, 20000, plan.Added)
	require.Equal(t, 10000, plan.Public)
	require.NoError(t, CommitBookmarkImport("bookmarks", version, plan.Result))
	backup, _, err := LatestBookmarkBackup("bookmarks")
	require.NoError(t, err)
	require.Equal(t, before, backup)
	loaded, err := LoadNormalBookmarks()
	require.NoError(t, err)
	require.Len(t, loaded.Categories, 100)
	require.Len(t, loaded.Items, 20000)
	require.Len(t, FilterNavigation(loaded, false).Items, 10000)
	require.Equal(t, source.Items[19999].URL, loaded.Items[19999].URL)
	require.True(t, loaded.Items[19999].Private)
	// A repeated import skips existing records and cannot erase private markers.
	entries[19999].Bookmark.Private = false
	next, err := bookmarks.Build(loaded, entries, bookmarks.Options{Strategy: "skip", Public: true})
	require.NoError(t, err)
	require.Equal(t, 20000, next.Skipped)
	require.True(t, next.Result.Items[19999].Private)
	require.ErrorIs(t, CommitBookmarkImport("bookmarks", version, next.Result), ErrBookmarkConflict)
}
