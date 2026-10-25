package data

import (
	"errors"
	"github.com/soulteary/flare/config/model"
	"os"
	"testing"
)

func TestTransferAtomicBackupAndConflict(t *testing.T) {
	t.Chdir(t.TempDir())
	original := []byte("links:\n- name: old\n  link: https://old.example\n")
	if err := os.WriteFile("bookmarks.yml", original, 0600); err != nil {
		t.Fatal(err)
	}
	src, version, err := BookmarkSnapshot("bookmarks")
	if err != nil || len(src.Items) != 1 {
		t.Fatalf("snapshot: %v", err)
	}
	src.Items = append(src.Items, model.Bookmark{Name: "new", URL: "https://new.example"})
	if err = CommitBookmarkImport("bookmarks", version, src); err != nil {
		t.Fatal(err)
	}
	backup, _, err := LatestBookmarkBackup("bookmarks")
	if err != nil || string(backup) != string(original) {
		t.Fatalf("backup: %s %v", backup, err)
	}
	if err = CommitBookmarkImport("bookmarks", version, src); !errors.Is(err, ErrBookmarkConflict) {
		t.Fatalf("expected conflict: %v", err)
	}
	loaded, err := LoadNormalBookmarks()
	if err != nil || len(loaded.Items) != 2 {
		t.Fatalf("reload: %+v %v", loaded, err)
	}
	_, version, err = BookmarkSnapshot("bookmarks")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("bookmarks.yml", original, 0600); err != nil {
		t.Fatal(err)
	}
	if err = CommitBookmarkImport("bookmarks", version, src); !errors.Is(err, ErrBookmarkConflict) {
		t.Fatalf("manual edit ignored: %v", err)
	}
}

func TestTransferBackupFailureDoesNotWrite(t *testing.T) {
	t.Chdir(t.TempDir())
	src, version, err := BookmarkSnapshot("apps")
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("apps.yml")
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(bookmarkBackupDir, []byte("blocked"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = CommitBookmarkImport("apps", version, src); err == nil {
		t.Fatal("backup failure ignored")
	}
	after, err := os.ReadFile("apps.yml")
	if err != nil || string(after) != string(raw) {
		t.Fatal("source changed")
	}
	if _, _, err = BookmarkSnapshot("../config"); err == nil {
		t.Fatal("invalid target accepted")
	}
}
