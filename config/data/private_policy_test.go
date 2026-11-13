package data

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

// requireLoginForPrivateContent enables the policy for one test only.
func requireLoginForPrivateContent(t *testing.T) {
	t.Helper()
	RequireLoginForPrivateContent(true)
	t.Cleanup(func() { RequireLoginForPrivateContent(false) })
}

func privateBookmarkFixture() model.Bookmarks {
	return model.Bookmarks{
		Categories: []model.Category{{ID: "open", Name: "Open"}},
		Items: []model.Bookmark{
			{Name: "public sentinel", URL: "https://public.example", Category: "open"},
			{Name: "private sentinel", URL: "https://private.example", Category: "open", Private: true},
		},
	}
}

func TestContainsPrivateNavigation(t *testing.T) {
	require.False(t, containsPrivateNavigation(model.Bookmarks{}))
	require.False(t, containsPrivateNavigation(model.Bookmarks{Categories: []model.Category{{ID: "a"}}, Items: []model.Bookmark{{Name: "a"}}}))
	require.True(t, containsPrivateNavigation(model.Bookmarks{Categories: []model.Category{{ID: "a", Private: true}}}))
	require.True(t, containsPrivateNavigation(model.Bookmarks{Items: []model.Bookmark{{Name: "a", Private: true}}}))
	require.NoError(t, privateContentError(privateBookmarkFixture()), "the policy is off by default")
}

func TestPrivateCollectionIsWithheldFromDisplayReadersWithoutLogin(t *testing.T) {
	prepareBookmarkHotPath(t)
	require.True(t, SaveNormalBookmarks(privateBookmarkFixture()))
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "app sentinel", URL: "https://app.example"}}}))
	// Warm every snapshot and index before the policy applies.
	_, err := LoadNormalBookmarks()
	require.NoError(t, err)
	_, err = LoadFavoriteBookmarks()
	require.NoError(t, err)
	privateHandle := OpaqueNavigationHandle("https://private.example", nil)
	require.Len(t, privateHandle, 32)
	appHandle := OpaqueNavigationHandle("https://app.example", nil)
	require.Len(t, appHandle, 32)

	requireLoginForPrivateContent(t)

	for _, administrator := range []bool{false, true} {
		_, err = LoadVisibleNormalBookmarks(administrator)
		require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)
		_, err = ReadExistingVisibleNormalBookmarks(administrator)
		require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)
		_, _, err = LoadVisibleNavigationPair(true, true, administrator)
		require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)
		_, _, err = ReadExistingVisibleNavigationPair(true, true, administrator)
		require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)
		_, _, err = LoadNavigationViews(true, true, administrator, false)
		require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)
		readable, categoryErr := ReadableBookmarkCategory("open", administrator)
		require.ErrorIs(t, categoryErr, ErrPrivateContentRequiresLogin)
		require.False(t, readable)
		readable, categoryErr = ReadableExistingBookmarkCategory("open", administrator)
		require.ErrorIs(t, categoryErr, ErrPrivateContentRequiresLogin)
		require.False(t, readable)
		_, err = SearchNavigationMatchesFor("sentinel", true, true, 8, nil, nil, administrator)
		require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)

		// Public entries of the withheld collection are not served either.
		for _, link := range []string{"https://private.example", "https://public.example"} {
			require.False(t, MatchNavigationTargetFor(link, administrator, nil), link)
		}
		_, ok := MatchOpaqueNavigationTarget(privateHandle, administrator)
		require.False(t, ok)

		// The other collection has no private entry and stays available.
		apps, _, appsErr := LoadNavigationViews(true, false, administrator, false)
		require.NoError(t, appsErr)
		require.Equal(t, "app sentinel", apps.Copy().Items[0].Name)
		matches, searchErr := SearchNavigationMatchesFor("sentinel", true, false, 8, nil, nil, administrator)
		require.NoError(t, searchErr)
		require.Len(t, matches, 1)
		require.True(t, MatchNavigationTargetFor("https://app.example", administrator, nil))
		target, ok := MatchOpaqueNavigationTarget(appHandle, administrator)
		require.True(t, ok)
		require.Equal(t, "https://app.example", target)
	}
	_, err = LoadNormalBookmarks()
	require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)
	require.Empty(t, OpaqueNavigationHandle("https://private.example", nil), "no new handle for a withheld snapshot")
	require.Empty(t, OpaqueNavigationHandle("https://public.example", nil))
	_, err = HasPrivateNavigation()
	require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)

	// The decision follows the switch; nothing about it is cached on the snapshot.
	RequireLoginForPrivateContent(false)
	value, err := LoadNormalBookmarks()
	require.NoError(t, err)
	require.Len(t, value.Items, 2)
}

func TestWithheldApplicationsDoNotBlockBookmarkRedirects(t *testing.T) {
	prepareBookmarkHotPath(t)
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "private app", URL: "https://private-app.example", Private: true}}}))
	require.True(t, SaveNormalBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "bookmark", URL: "https://bookmark.example"}}}))
	_, err := LoadNormalBookmarks()
	require.NoError(t, err)
	handle := OpaqueNavigationHandle("https://bookmark.example", nil)
	require.Len(t, handle, 32)

	requireLoginForPrivateContent(t)

	_, err = LoadFavoriteBookmarks()
	require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)
	require.True(t, MatchNavigationTargetFor("https://bookmark.example", false, nil))
	require.False(t, MatchNavigationTargetFor("https://private-app.example", true, nil))
	target, ok := MatchOpaqueNavigationTarget(handle, false)
	require.True(t, ok)
	require.Equal(t, "https://bookmark.example", target)
}

func TestMalformedCollectionDoesNotBlockRedirectsToTheOther(t *testing.T) {
	prepareBookmarkHotPath(t)
	require.True(t, SaveNormalBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "bookmark", URL: "https://bookmark.example"}}}))
	_, err := LoadNormalBookmarks()
	require.NoError(t, err)
	handle := OpaqueNavigationHandle("https://bookmark.example", nil)
	require.Len(t, handle, 32)
	require.NoError(t, os.WriteFile("apps.yml", []byte("links: [broken"), 0600))
	invalidateFileCache("apps")

	require.True(t, MatchNavigationTarget("https://bookmark.example", nil))
	target, ok := MatchOpaqueNavigationTarget(handle, false)
	require.True(t, ok)
	require.Equal(t, "https://bookmark.example", target)
}

func TestPrivateContentIsWithheldFromEditorAndTransferWithoutLogin(t *testing.T) {
	prepareBookmarkHotPath(t)
	require.True(t, SaveNormalBookmarks(privateBookmarkFixture()))
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "app sentinel", URL: "https://app.example"}}}))
	before, err := os.ReadFile("bookmarks.yml")
	require.NoError(t, err)

	requireLoginForPrivateContent(t)

	document, err := EditorSnapshot()
	require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)
	require.Empty(t, document.Apps)
	require.Empty(t, document.Bookmarks)
	require.Empty(t, document.Categories)
	_, err = CommitEditorDocument(EditorDocument{Version: "stale", Bookmarks: []model.Bookmark{{Name: "replacement", URL: "https://replacement.example"}}})
	require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)

	value, version, err := BookmarkSnapshot("bookmarks")
	require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)
	require.Empty(t, value.Items)
	require.Empty(t, version)
	raw, err := RawBookmarkSnapshot("bookmarks")
	require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)
	require.Nil(t, raw)

	// A collection without private entries can still be read and exported.
	apps, appsVersion, err := BookmarkSnapshot("apps")
	require.NoError(t, err)
	require.Len(t, apps.Items, 1)
	raw, err = RawBookmarkSnapshot("apps")
	require.NoError(t, err)
	require.Contains(t, string(raw), "app sentinel")

	// An import that would introduce private entries is refused before any write.
	appsBefore, err := os.ReadFile("apps.yml")
	require.NoError(t, err)
	apps.Items = append(apps.Items, model.Bookmark{Name: "imported private", URL: "https://imported.example", Private: true})
	require.ErrorIs(t, CommitBookmarkImport("apps", appsVersion, apps), ErrPrivateContentRequiresLogin)
	appsAfter, err := os.ReadFile("apps.yml")
	require.NoError(t, err)
	require.Equal(t, appsBefore, appsAfter)
	_, statErr := os.Stat(bookmarkBackupDir)
	require.True(t, os.IsNotExist(statErr), "a refused import must not leave a backup")

	after, err := os.ReadFile("bookmarks.yml")
	require.NoError(t, err)
	require.Equal(t, before, after, "withholding never rewrites the source file")
}

func TestEditorCommitCannotIntroducePrivateContentWithoutLogin(t *testing.T) {
	prepareBookmarkHotPath(t)
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "app", URL: "https://app.example"}}}))
	require.True(t, SaveNormalBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "bookmark", URL: "https://bookmark.example"}}}))
	requireLoginForPrivateContent(t)

	document, err := EditorSnapshot()
	require.NoError(t, err)
	before, err := os.ReadFile("apps.yml")
	require.NoError(t, err)
	document.Apps[0].Private = true
	_, err = CommitEditorDocument(document)
	require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)
	after, err := os.ReadFile("apps.yml")
	require.NoError(t, err)
	require.Equal(t, before, after)

	document.Apps[0].Private = false
	document.Apps[0].Name = "renamed"
	saved, err := CommitEditorDocument(document)
	require.NoError(t, err)
	require.Equal(t, "renamed", saved.Apps[0].Name)
}

func TestImportBackupWithPrivateEntriesIsNotReleasedWithoutLogin(t *testing.T) {
	prepareBookmarkHotPath(t)
	require.True(t, SaveNormalBookmarks(privateBookmarkFixture()))
	_, version, err := BookmarkSnapshot("bookmarks")
	require.NoError(t, err)
	// The import replaces the collection with public entries; its backup keeps
	// the earlier state, including the private entry.
	require.NoError(t, CommitBookmarkImport("bookmarks", version, model.Bookmarks{Items: []model.Bookmark{{Name: "public only", URL: "https://public-only.example"}}}))
	raw, _, err := LatestBookmarkBackup("bookmarks")
	require.NoError(t, err)
	require.Contains(t, string(raw), "private sentinel")

	requireLoginForPrivateContent(t)

	_, err = LoadNormalBookmarks()
	require.NoError(t, err, "the live collection has no private entry")
	raw, _, err = LatestBookmarkBackup("bookmarks")
	require.ErrorIs(t, err, ErrPrivateContentRequiresLogin)
	require.Nil(t, raw)

	// A newer backup that cannot be decoded cannot be shown to be free of private entries.
	require.NoError(t, os.WriteFile(filepath.Join(bookmarkBackupDir, "bookmarks-99991231T000000.000000000-undecodable.yml"), []byte("links: [broken"), 0600))
	raw, _, err = LatestBookmarkBackup("bookmarks")
	require.Error(t, err)
	require.NotErrorIs(t, err, ErrPrivateContentRequiresLogin)
	require.Nil(t, raw)

	// A newer backup without private entries is released.
	require.NoError(t, os.WriteFile(filepath.Join(bookmarkBackupDir, "bookmarks-99991231T000001.000000000-public.yml"), []byte("links:\n- name: backup public\n  link: https://backup.example\n"), 0600))
	raw, _, err = LatestBookmarkBackup("bookmarks")
	require.NoError(t, err)
	require.Contains(t, string(raw), "backup public")
}
