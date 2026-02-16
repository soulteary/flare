package data

import (
	"os"
	"testing"
)

func TestFavoriteBookmarks(t *testing.T) {

	filePath := getConfigPath("apps")
	os.Remove(filePath)

	data, err := LoadFavoriteBookmarks()
	if err != nil {
		t.Fatalf("LoadFavoriteBookmarks: %v", err)
	}
	if len(data.Categories) != 0 || len(data.Items) == 0 {
		t.Fatal("Load Favorite Bookmarks Failed")
	}
	ok := SaveFavoriteBookmarks(data)
	if !ok {
		t.Fatal("Save Favorite Bookmarks Failed")
	}

	os.Remove(filePath)

}

func TestNormalBookmarks(t *testing.T) {

	filePath := getConfigPath("bookmarks")
	os.Remove(filePath)

	data, err := LoadNormalBookmarks()
	if err != nil {
		t.Fatalf("LoadNormalBookmarks: %v", err)
	}
	if len(data.Categories) == 0 || len(data.Items) == 0 {
		t.Fatal("Load Normal Bookmarks Failed")
	}
	ok := SaveNormalBookmarks(data)
	if !ok {
		t.Fatal("Save Normal Bookmarks Failed")
	}

	os.Remove(filePath)

}
