package data

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/model"
)

type bookmarkHotPathCollection struct {
	name         string
	defaultItems int
	load         func() (model.Bookmarks, error)
	save         func(model.Bookmarks) bool
}

func bookmarkHotPathCollections() []bookmarkHotPathCollection {
	return []bookmarkHotPathCollection{
		{name: "apps", defaultItems: 8, load: LoadFavoriteBookmarks, save: SaveFavoriteBookmarks},
		{name: "bookmarks", defaultItems: 20, load: LoadNormalBookmarks, save: SaveNormalBookmarks},
	}
}

func prepareBookmarkHotPath(tb testing.TB) {
	tb.Helper()
	tb.Chdir(tb.TempDir())
	invalidateFileCache("apps")
	invalidateFileCache("bookmarks")
	tb.Cleanup(func() {
		invalidateFileCache("apps")
		invalidateFileCache("bookmarks")
	})
}

func assertBookmarkHotPathURL(t *testing.T, collection bookmarkHotPathCollection, expected string) {
	t.Helper()
	bookmarks, err := collection.load()
	if err != nil {
		t.Fatal(err)
	}
	if len(bookmarks.Items) != 1 || bookmarks.Items[0].URL != expected {
		t.Fatalf("%s bookmarks = %+v, want one URL %q", collection.name, bookmarks.Items, expected)
	}
}

func TestBookmarkHotPathReinitializesDeletedWarmFile(t *testing.T) {
	for _, collection := range bookmarkHotPathCollections() {
		t.Run(collection.name, func(t *testing.T) {
			prepareBookmarkHotPath(t)
			const previousURL = "https://before-delete.example"
			if !collection.save(model.Bookmarks{Items: []model.Bookmark{{Name: "before deletion", URL: previousURL}}}) {
				t.Fatal("save bookmarks failed")
			}
			assertBookmarkHotPathURL(t, collection, previousURL) // Warm the raw-byte cache.
			if err := os.Remove(collection.name + ".yml"); err != nil {
				t.Fatal(err)
			}
			bookmarks, err := collection.load()
			if err != nil {
				t.Fatal(err)
			}
			if len(bookmarks.Items) != collection.defaultItems {
				t.Fatalf("deleted %s file returned %d items, want %d defaults", collection.name, len(bookmarks.Items), collection.defaultItems)
			}
			contents, err := os.ReadFile(collection.name + ".yml")
			if err != nil {
				t.Fatalf("deleted file was not recreated: %v", err)
			}
			if strings.Contains(string(contents), previousURL) || !strings.Contains(string(contents), "https://link.example.com") {
				t.Fatalf("recreated %s file does not contain default bookmarks", collection.name)
			}
		})
	}
}

func TestBookmarkHotPathSaveInvalidatesWarmCache(t *testing.T) {
	for _, collection := range bookmarkHotPathCollections() {
		t.Run(collection.name, func(t *testing.T) {
			prepareBookmarkHotPath(t)
			for _, target := range []string{"https://before-save.example", "https://after-save.example"} {
				if !collection.save(model.Bookmarks{Items: []model.Bookmark{{Name: "saved bookmark", URL: target}}}) {
					t.Fatal("save bookmarks failed")
				}
				assertBookmarkHotPathURL(t, collection, target)
			}
		})
	}
}

func TestBookmarkHotPathUsesCurrentDirectoryWithWrongPWD(t *testing.T) {
	for _, collection := range bookmarkHotPathCollections() {
		t.Run(collection.name, func(t *testing.T) {
			prepareBookmarkHotPath(t)
			otherDirectory := t.TempDir()
			t.Setenv("PWD", otherDirectory)
			const expectedURL = "https://current-directory.example"
			current := []byte("links:\n- name: current\n  link: " + expectedURL + "\n")
			other := []byte("links:\n- name: wrong directory\n  link: https://wrong-directory.example\n")
			if err := os.WriteFile(collection.name+".yml", current, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(otherDirectory, collection.name+".yml"), other, 0600); err != nil {
				t.Fatal(err)
			}
			assertBookmarkHotPathURL(t, collection, expectedURL) // Cold cache resolves the actual directory.
			assertBookmarkHotPathURL(t, collection, expectedURL) // Warm cache checks the relative filename.
		})
	}
}

func TestBookmarkHotPathStillValidatesEntireYAML(t *testing.T) {
	invalidFiles := []struct{ name, contents string }{
		{name: "syntax", contents: "links: ["},
		{name: "unrelated field type", contents: "links:\n- name: example\n  link: https://valid-url.example\n  private: not-a-boolean\n"},
	}
	for _, collection := range bookmarkHotPathCollections() {
		t.Run(collection.name, func(t *testing.T) {
			for _, invalid := range invalidFiles {
				t.Run(invalid.name, func(t *testing.T) {
					prepareBookmarkHotPath(t)
					if err := os.WriteFile(collection.name+".yml", []byte(invalid.contents), 0600); err != nil {
						t.Fatal(err)
					}
					for range 2 {
						if _, err := collection.load(); err == nil || !strings.Contains(err.Error(), "解析配置文件 "+collection.name) {
							t.Fatalf("invalid YAML should fail on cold and warm reads, got %v", err)
						}
					}
				})
			}
		})
	}
}

func BenchmarkLoadFavoriteBookmarks(b *testing.B) {
	for _, pwdMode := range []string{"pwd-current", "pwd-stale"} {
		b.Run(pwdMode, func(b *testing.B) {
			prepareBookmarkHotPath(b)
			currentDirectory, err := os.Getwd()
			if err != nil {
				b.Fatal(err)
			}
			pwd := currentDirectory
			if pwdMode == "pwd-stale" {
				pwd = filepath.Dir(currentDirectory)
			}
			b.Setenv("PWD", pwd)
			// Create the default fixture and warm the raw-byte cache before timing.
			if _, err := initBookmarks(getConfigPath("apps"), true); err != nil {
				b.Fatal(err)
			}
			if _, err := LoadFavoriteBookmarks(); err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				bookmarks, err := LoadFavoriteBookmarks()
				if err != nil || len(bookmarks.Items) != 8 {
					b.Fatalf("load favorite bookmarks: %d items, %v", len(bookmarks.Items), err)
				}
			}
		})
	}
}
