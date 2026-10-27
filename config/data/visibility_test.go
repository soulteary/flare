package data

import (
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/yamlutil"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestPrivateVisibilityInheritedAcrossReadIndexes(t *testing.T) {
	prepareBookmarkHotPath(t)
	value := model.Bookmarks{Categories: []model.Category{{ID: "public", Name: "Public"}, {ID: "secret", Name: "Hidden category", Private: true}}, Items: []model.Bookmark{{Name: "public sentinel", URL: "https://public.example", Category: "public"}, {Name: "private sentinel", URL: "https://private.example", Private: true}, {Name: "inherited sentinel", URL: "https://inherited.example", Category: "secret", Private: false}}}
	require.True(t, SaveNormalBookmarks(value))
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "app sentinel", URL: "https://app.example", Private: true}}}))
	visible := FilterNavigation(value, false)
	require.Len(t, visible.Categories, 1)
	require.Len(t, visible.Items, 1)
	require.Len(t, FilterNavigation(value, true).Items, 3)
	for _, link := range []string{"https://private.example", "https://inherited.example", "https://app.example"} {
		require.False(t, MatchNavigationTargetFor(link, false, nil))
		require.True(t, MatchNavigationTargetFor(link, true, nil))
	}
	matches, err := SearchNavigationMatchesFor("sentinel", true, true, 8, nil, nil, false)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Equal(t, "public sentinel", matches[0].Bookmark.Name)
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "public duplicate", URL: "https://private.example"}}}))
	require.True(t, MatchNavigationTargetFor("https://private.example", false, nil))
}
func TestMalformedNavigationFailsClosed(t *testing.T) {
	prepareBookmarkHotPath(t)
	require.NoError(t, os.WriteFile("bookmarks.yml", []byte("links:\n- name: Orphan\n  link: https://sentinel.example\n  category: missing\n"), 0600))
	invalidateFileCache("bookmarks")
	_, err := LoadNormalBookmarks()
	require.Error(t, err)
	require.False(t, MatchNavigationTargetFor("https://sentinel.example", true, nil))
}
func TestCheckedEditorRevocationPreventsDiskCommit(t *testing.T) {
	doc := editorFixture(t)
	before, err := os.ReadFile("apps.yml")
	require.NoError(t, err)
	doc.Apps[0].Name = "Changed"
	calls := 0
	_, err = CommitEditorDocumentChecked(doc, func() error {
		calls++
		if calls > 1 {
			return os.ErrPermission
		}
		return nil
	})
	require.Error(t, err)
	require.Equal(t, 2, calls)
	after, err := os.ReadFile("apps.yml")
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestOpaqueHandlesEnforceCurrentPermissionsAndExpire(t *testing.T) {
	prepareBookmarkHotPath(t)
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "Private", URL: "https://private.example/path?secret=value", Private: true}}}))
	_, err := LoadFavoriteBookmarks()
	require.NoError(t, err)
	id := OpaqueNavigationHandle("https://private.example/path?secret=value", nil)
	require.Len(t, id, 32)
	_, ok := MatchOpaqueNavigationTarget(id, false)
	require.False(t, ok)
	raw, ok := MatchOpaqueNavigationTarget(id, true)
	require.True(t, ok)
	require.Equal(t, "https://private.example/path?secret=value", raw)
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "Other", URL: "https://other.example"}}}))
	_, ok = MatchOpaqueNavigationTarget(id, true)
	require.False(t, ok)
}

func TestReadOnlyContentBrowseAllowedWritesRejected(t *testing.T) {
	doc := editorFixture(t)
	require.NoError(t, os.Chmod("apps.yml", 0444))
	_, err := LoadFavoriteBookmarks()
	require.NoError(t, err)
	doc.Apps[0].Name = "Change"
	_, err = CommitEditorDocument(doc)
	require.ErrorIs(t, err, ErrBookmarkReadOnly)
	value, version, err := BookmarkSnapshot("apps")
	require.NoError(t, err)
	value.Items[0].Name = "Import change"
	err = CommitBookmarkImport("apps", version, value)
	require.ErrorIs(t, err, ErrBookmarkReadOnly)
}

func TestAnchoredPrivateNavigationNeverPublishesSentinelsOrRewritesSource(t *testing.T) {
	prepareBookmarkHotPath(t)
	original := []byte("policy: &private {private: true}\ncategories:\n- &category\n  <<: *private\n  id: secret\n  title: Anchored secret title\nlinks:\n- <<: *private\n  name: Anchored secret entry\n  link: https://anchored.secret.example\n- name: Inherited false entry\n  link: https://inherited.secret.example\n  category: secret\n  private: false\n")
	require.NoError(t, os.WriteFile("bookmarks.yml", original, 0600))
	invalidateFileCache("bookmarks")
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{}))
	value, err := LoadNormalBookmarks()
	require.NoError(t, err)
	require.Empty(t, FilterNavigation(value, false).Categories)
	require.Empty(t, FilterNavigation(value, false).Items)
	require.False(t, MatchNavigationTargetFor("https://anchored.secret.example", false, nil))
	require.True(t, MatchNavigationTargetFor("https://anchored.secret.example", true, nil))
	_, version, err := BookmarkSnapshot("bookmarks")
	require.NoError(t, err)
	value.Items[0].Name = "Changed"
	require.ErrorIs(t, CommitBookmarkImport("bookmarks", version, value), yamlutil.ErrBookmarkAliasesReadOnly)
	after, err := os.ReadFile("bookmarks.yml")
	require.NoError(t, err)
	require.Equal(t, original, after)
}
