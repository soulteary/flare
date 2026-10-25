package data

import (
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/soulteary/flare/config/model"
)

func TestParsedSnapshotsDoNotAliasReturnedModels(t *testing.T) {
	prepareBookmarkHotPath(t)
	invalidateFileCache("config")
	defer invalidateFileCache("config")
	options := model.Application{Title: "original", Clock: &model.ClockOptions{Timezone: "UTC", Seconds: new(true)}, Homepage: &model.HomepageOptions{DesktopColumns: "4"}, Search: &model.SearchOptions{Engines: []model.SearchEngine{{ID: "engine", Name: "Original"}}}}
	if !saveAppConfigToYamlFile("config", options) {
		t.Fatal("save config")
	}
	first, err := GetAllSettingsOptions()
	if err != nil {
		t.Fatal(err)
	}
	first.Clock.Timezone = "Europe/Paris"
	*first.Clock.Seconds = false
	first.Homepage.DesktopColumns = "8"
	first.Search.Engines[0].Name = "Mutated"
	first.Search.Engines = append(first.Search.Engines, model.SearchEngine{ID: "extra"})
	second, err := GetAllSettingsOptions()
	if err != nil {
		t.Fatal(err)
	}
	if second.Title != "original" || second.Clock.Timezone != "UTC" || second.Clock.Seconds == nil || !*second.Clock.Seconds || second.Homepage.DesktopColumns != "4" || len(second.Search.Engines) != 1 || second.Search.Engines[0].Name != "Original" {
		t.Fatalf("returned config aliased cache: %+v", second)
	}
	original := model.Bookmarks{Categories: []model.Category{{ID: "cat", Name: "Original"}}, Items: []model.Bookmark{{Name: "Original", URL: "https://original.example", Category: "cat"}}}
	if !SaveNormalBookmarks(original) {
		t.Fatal("save bookmarks")
	}
	loaded, err := LoadNormalBookmarks()
	if err != nil {
		t.Fatal(err)
	}
	loaded.Items[0].Name = "Mutated"
	loaded.Categories[0].Name = "Mutated"
	loaded.Items = append(loaded.Items, model.Bookmark{Name: "Extra"})
	again, err := LoadNormalBookmarks()
	if err != nil {
		t.Fatal(err)
	}
	if len(again.Items) != 1 || again.Items[0].Name != "Original" || again.Categories[0].Name != "Original" {
		t.Fatal("returned collection aliased cache")
	}
}

func TestParsedSnapshotsConcurrentReturnedMutations(t *testing.T) {
	prepareBookmarkHotPath(t)
	if !SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "Original", URL: "https://example.com"}}}) {
		t.Fatal("save")
	}
	var workers sync.WaitGroup
	for i := 0; i < 20; i++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for j := 0; j < 30; j++ {
				value, err := LoadFavoriteBookmarks()
				if err != nil {
					t.Error(err)
					return
				}
				if value.Items[0].Name != "Original" {
					t.Error("shared mutation")
				}
				value.Items[0].Name = "Local"
			}
		}()
	}
	workers.Wait()
}

func TestParsedSnapshotDeletionRemainsRegeneratedOnLaterReads(t *testing.T) {
	prepareBookmarkHotPath(t)
	if !SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "Old", URL: "https://old.example"}}}) {
		t.Fatal("save")
	}
	if _, err := LoadFavoriteBookmarks(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove("apps.yml"); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		value, err := LoadFavoriteBookmarks()
		if err != nil {
			t.Fatal(err)
		}
		if len(value.Items) != 8 || value.Items[0].Name == "Old" {
			t.Fatal("deleted cached file returned old entries")
		}
	}
}

func TestNavigationIndexesInvalidateAndKeepExpansionRequestLocal(t *testing.T) {
	prepareBookmarkHotPath(t)
	if !SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "Static", URL: "https://static.example"}, {Name: "Dynamic", URL: "{origin}/panel"}}}) || !SaveNormalBookmarks(model.Bookmarks{}) {
		t.Fatal("save")
	}
	if !MatchNavigationTarget("https://static.example", nil) {
		t.Fatal("static lookup failed")
	}
	dynamic := func(host string) func(string) bool {
		return func(template string) bool { return strings.ReplaceAll(template, "{origin}", host) == host+"/panel" }
	}
	if !MatchNavigationTarget("https://one.example/panel", dynamic("https://one.example")) {
		t.Fatal("request one dynamic lookup failed")
	}
	// The predicate controls expansion; the cache never retains a previously expanded host.
	if MatchNavigationTarget("https://one.example/panel", func(template string) bool {
		return strings.ReplaceAll(template, "{origin}", "https://two.example") == "https://one.example/panel"
	}) {
		t.Fatal("request host leaked into index")
	}
	if !SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "New", URL: "https://new.example"}}}) {
		t.Fatal("save replacement")
	}
	if MatchNavigationTarget("https://static.example", nil) || !MatchNavigationTarget("https://new.example", nil) {
		t.Fatal("save did not invalidate target index")
	}
}

func TestIndexedSearchScopeOrderAndInvalidation(t *testing.T) {
	prepareBookmarkHotPath(t)
	apps := model.Bookmarks{Items: []model.Bookmark{{Name: "Console", Desc: "Needle", URL: "https://app.example"}, {Name: "Dynamic", URL: "{origin}/panel"}}}
	bookmarks := model.Bookmarks{Categories: []model.Category{{ID: "known"}}, Items: []model.Bookmark{{Name: "Needle", URL: "https://book.example"}, {Name: "Description only", Desc: "Needle", URL: "https://desc.example"}, {Name: "Needle orphan", URL: "https://orphan.example", Category: "missing"}, {Name: "Needle unsafe", URL: "javascript:alert(1)"}}}
	if !SaveFavoriteBookmarks(apps) || !SaveNormalBookmarks(bookmarks) {
		t.Fatal("save fixtures")
	}
	accept := func(link string) bool { return !strings.HasPrefix(link, "javascript:") }
	matches, err := SearchNavigationMatches("NEEDLE", true, true, 8, nil, accept)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 2 || !matches[0].IsApp || matches[1].IsApp || matches[1].Bookmark.URL != "https://book.example" {
		t.Fatalf("scope/order mismatch: %+v", matches)
	}
	resolve := func(link string) string { return strings.ReplaceAll(link, "{origin}", "https://request.example") }
	matches, err = SearchNavigationMatches("request.example", true, false, 8, resolve, accept)
	if err != nil || len(matches) != 1 || matches[0].URL != "https://request.example/panel" {
		t.Fatal("dynamic search mismatch", err, matches)
	}
	if !SaveNormalBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "New needle", URL: "https://new.example"}}}) {
		t.Fatal("update")
	}
	matches, err = SearchNavigationMatches("needle", false, true, 8, nil, accept)
	if err != nil || len(matches) != 1 || matches[0].URL != "https://new.example" {
		t.Fatal("search index stale", err, matches)
	}
	if err = os.WriteFile("apps.yml", []byte("links: ["), 0600); err != nil {
		t.Fatal(err)
	}
	invalidateFileCache("apps")
	if _, err = SearchNavigationMatches("needle", false, true, 8, nil, accept); err != nil {
		t.Fatal("hidden malformed apps broke search", err)
	}
	if _, err = SearchNavigationMatches("needle", true, true, 8, nil, accept); err == nil {
		t.Fatal("visible malformed apps silently ignored")
	}
}

func TestNavigationIndexesFollowEditorImportAndDeletion(t *testing.T) {
	prepareBookmarkHotPath(t)
	if !SaveFavoriteBookmarks(model.Bookmarks{}) || !SaveNormalBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "Old", URL: "https://old.example"}}}) {
		t.Fatal("save")
	}
	assertTarget := func(want, absent string) {
		t.Helper()
		if !MatchNavigationTarget(want, nil) || MatchNavigationTarget(absent, nil) {
			t.Fatalf("target index mismatch: want %s, absent %s", want, absent)
		}
		matches, err := SearchNavigationMatches("", false, true, 8, nil, nil)
		if err != nil || len(matches) != 1 || matches[0].URL != want {
			t.Fatal("search index mismatch", err, matches)
		}
	}
	assertTarget("https://old.example", "https://editor.example")
	document, err := EditorSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	document.Bookmarks = []model.Bookmark{{Name: "Editor", URL: "https://editor.example"}}
	if _, err = CommitEditorDocument(document); err != nil {
		t.Fatal(err)
	}
	assertTarget("https://editor.example", "https://old.example")
	_, version, err := BookmarkSnapshot("bookmarks")
	if err != nil {
		t.Fatal(err)
	}
	if err = CommitBookmarkImport("bookmarks", version, model.Bookmarks{Items: []model.Bookmark{{Name: "Import", URL: "https://import.example"}}}); err != nil {
		t.Fatal(err)
	}
	assertTarget("https://import.example", "https://editor.example")
	if err = os.Remove("bookmarks.yml"); err != nil {
		t.Fatal(err)
	}
	if MatchNavigationTarget("https://import.example", nil) || !MatchNavigationTarget("https://link.example.com", nil) {
		t.Fatal("deleted file kept stale target index")
	}
}
