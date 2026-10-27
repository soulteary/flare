package data

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func visibleNavigationFixture(t *testing.T) {
	t.Helper()
	prepareBookmarkHotPath(t)
	require.True(t, SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "version-0", URL: "https://public-app.example"}, {Name: "private-app", URL: "https://private-app.example", Private: true}}}))
	require.True(t, SaveNormalBookmarks(model.Bookmarks{
		Categories: []model.Category{{ID: "public", Name: "Public"}, {ID: "secret", Name: "Secret", Private: true}},
		Items:      []model.Bookmark{{Name: "version-0", URL: "https://public.example", Category: "public"}, {Name: "parent-private-sentinel", URL: "https://parent-private.example", Category: "secret", Private: false}, {Name: "direct-private-sentinel", URL: "https://direct-private.example", Category: "public", Private: true}},
	}))
}
func TestVisibleNavigationViewsCannotMutateCachedAuthorization(t *testing.T) {
	visibleNavigationFixture(t)
	for _, administrator := range []bool{false, true} {
		apps, bookmarks, e := LoadVisibleNavigationPair(true, true, administrator)
		require.NoError(t, e)
		apps.Items[0].Name = "poison-app"
		apps.Items[0].URL = "https://poison.example"
		bookmarks.Items[0].Name = "poison-bookmark"
		bookmarks.Categories[0].Name = "poison-category"
		if administrator {
			apps.Items[1].Private = false
			bookmarks.Categories[1].Private = false
			bookmarks.Items[1].Category = "public"
		}
		nextApps, nextBookmarks, e := LoadVisibleNavigationPair(true, true, administrator)
		require.NoError(t, e)
		require.Equal(t, "version-0", nextApps.Items[0].Name)
		require.Equal(t, "version-0", nextBookmarks.Items[0].Name)
		require.Equal(t, "Public", nextBookmarks.Categories[0].Name)
	}
	apps, bookmarks, e := LoadVisibleNavigationPair(true, true, false)
	require.NoError(t, e)
	require.Len(t, apps.Items, 1)
	require.Len(t, bookmarks.Items, 1)
	require.Len(t, bookmarks.Categories, 1)
	adminApps, adminBookmarks, e := LoadNavigationPair(true, true)
	require.NoError(t, e)
	require.Len(t, adminApps.Items, 2)
	require.Len(t, adminBookmarks.Items, 3)
	require.True(t, adminBookmarks.Categories[1].Private)
	require.False(t, adminBookmarks.Items[1].Private)
	// The single-view APIs must also copy and preserve the cached parent ACL.
	singleApps, e := LoadVisibleFavoriteBookmarks(false)
	require.NoError(t, e)
	singleApps.Items[0].Name = "single-poison"
	singleBookmarks, e := LoadVisibleNormalBookmarks(true)
	require.NoError(t, e)
	singleBookmarks.Categories[1].Private = false
	apps, bookmarks, e = LoadVisibleNavigationPair(true, true, false)
	require.NoError(t, e)
	require.Equal(t, "version-0", apps.Items[0].Name)
	require.Len(t, bookmarks.Items, 1)
	for _, target := range []string{"https://private-app.example", "https://parent-private.example", "https://direct-private.example"} {
		require.False(t, MatchNavigationTargetFor(target, false, nil))
		require.True(t, MatchNavigationTargetFor(target, true, nil))
		id := OpaqueNavigationHandle(target, nil)
		_, ok := MatchOpaqueNavigationTarget(id, false)
		require.False(t, ok)
	}
	matches, e := SearchNavigationMatchesFor("private-sentinel", true, true, 8, nil, nil, false)
	require.NoError(t, e)
	require.Empty(t, matches)
}
func TestVisibleNavigationPairRemainsCoherentDuringEditorPublish(t *testing.T) {
	visibleNavigationFixture(t)
	start := make(chan struct{})
	failures := make(chan error, 3)
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		<-start
		for i := 1; i <= 12; i++ {
			doc, e := EditorSnapshot()
			if e != nil {
				failures <- e
				return
			}
			name := fmt.Sprintf("version-%d", i)
			doc.Apps[0].Name = name
			doc.Bookmarks[0].Name = name
			if _, e = CommitEditorDocument(doc); e != nil {
				failures <- e
				return
			}
		}
	}()
	for _, administrator := range []bool{false, true} {
		workers.Add(1)
		go func(administrator bool) {
			defer workers.Done()
			<-start
			for i := 0; i < 100; i++ {
				apps, bookmarks, e := LoadVisibleNavigationPair(true, true, administrator)
				if e != nil {
					failures <- e
					return
				}
				if apps.Items[0].Name != bookmarks.Items[0].Name {
					failures <- fmt.Errorf("mixed pair: %s / %s", apps.Items[0].Name, bookmarks.Items[0].Name)
					return
				}
				if !administrator && (len(apps.Items) != 1 || len(bookmarks.Items) != 1 || len(bookmarks.Categories) != 1) {
					failures <- fmt.Errorf("private content published into anonymous view")
					return
				}
				if administrator && (!bookmarks.Categories[1].Private || bookmarks.Items[1].Private) {
					failures <- fmt.Errorf("parent permission semantics changed")
					return
				}
			}
		}(administrator)
	}
	close(start)
	workers.Wait()
	close(failures)
	for e := range failures {
		require.NoError(t, e)
	}
}
func TestVisibleNavigationLoadsOnlyRequestedCollections(t *testing.T) {
	visibleNavigationFixture(t)
	require.NoError(t, os.WriteFile("apps.yml", []byte("links: [invalid"), 0600))
	invalidateFileCache("apps")
	apps, bookmarks, e := LoadVisibleNavigationPair(false, true, false)
	require.NoError(t, e)
	require.Empty(t, apps.Items)
	require.Len(t, bookmarks.Items, 1)
	_, _, e = LoadVisibleNavigationPair(true, true, false)
	require.ErrorContains(t, e, "apps")
	_, e = LoadVisibleFavoriteBookmarks(false)
	require.ErrorContains(t, e, "apps")
	apps, bookmarks, e = LoadVisibleNavigationPair(false, false, false)
	require.NoError(t, e)
	require.Empty(t, apps.Items)
	require.Empty(t, bookmarks.Items)
}

func TestReadableBookmarkCategoryUsesCurrentParentACL(t *testing.T) {
	visibleNavigationFixture(t)
	for _, test := range []struct {
		id          string
		admin, want bool
	}{{"public", false, true}, {"secret", false, false}, {"secret", true, true}, {"missing", true, false}} {
		visible, e := ReadableBookmarkCategory(test.id, test.admin)
		require.NoError(t, e)
		require.Equal(t, test.want, visible)
	}
	doc, e := EditorSnapshot()
	require.NoError(t, e)
	doc.Categories[0].Private = true
	_, e = CommitEditorDocument(doc)
	require.NoError(t, e)
	visible, e := ReadableBookmarkCategory("public", false)
	require.NoError(t, e)
	require.False(t, visible)
	visible, e = ReadableBookmarkCategory("public", true)
	require.NoError(t, e)
	require.True(t, visible)
	require.NoError(t, os.WriteFile("bookmarks.yml", []byte("categories: [invalid"), 0600))
	invalidateFileCache("bookmarks")
	visible, e = ReadableBookmarkCategory("public", true)
	require.ErrorContains(t, e, "bookmarks")
	require.False(t, visible, "malformed content must not authorize a category")
}
