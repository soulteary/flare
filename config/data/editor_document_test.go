package data

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func editorFixture(t *testing.T) EditorDocument {
	t.Helper()
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile("apps.yml", []byte("links:\n- name: App\n  link: http://nas:8080/a?q=1#f\n  private: true\n"), 0600))
	require.NoError(t, os.WriteFile("bookmarks.yml", []byte("categories:\n- id: stable-category\n  title: Original\nlinks:\n- name: Bookmark\n  link: custom://server/a?x=1&y=2#f\n  category: stable-category\n"), 0600))
	doc, err := EditorSnapshot()
	require.NoError(t, err)
	return doc
}
func TestEditorDocumentStableIDsAndConflict(t *testing.T) {
	doc := editorFixture(t)
	doc.Categories[0].Name = "Renamed"
	doc.Bookmarks[0].Desc = "保存描述"
	saved, err := CommitEditorDocument(doc)
	require.NoError(t, err)
	require.NotEqual(t, doc.Version, saved.Version)
	read, err := EditorSnapshot()
	require.NoError(t, err)
	require.Equal(t, "stable-category", read.Categories[0].ID)
	require.Equal(t, "stable-category", read.Bookmarks[0].Category)
	require.True(t, read.Apps[0].Private)
	_, err = CommitEditorDocument(doc)
	require.ErrorIs(t, err, ErrBookmarkConflict)
	file, openErr := os.OpenFile("apps.yml", os.O_APPEND|os.O_WRONLY, 0600)
	require.NoError(t, openErr)
	_, writeErr := file.WriteString("\n# external edit\n")
	require.NoError(t, writeErr)
	require.NoError(t, file.Close())
	_, err = CommitEditorDocument(saved)
	require.ErrorIs(t, err, ErrBookmarkConflict)
	info, err := os.Stat("bookmarks.yml")
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
}
func TestEditorDocumentValidation(t *testing.T) {
	doc := editorFixture(t)
	doc.Bookmarks[0].Category = "missing"
	err := ValidateEditorDocument(doc)
	var invalid *EditorValidationError
	require.ErrorAs(t, err, &invalid)
	require.Equal(t, "bookmarks.0.Category", invalid.Field)
	doc.Bookmarks[0].Category = doc.Categories[0].ID
	doc.Categories = append(doc.Categories, model.Category{ID: "another", Name: " Original "})
	require.Error(t, ValidateEditorDocument(doc))
}
func TestEditorDocumentRollbackAndRecovery(t *testing.T) {
	doc := editorFixture(t)
	beforeApps, err := os.ReadFile("apps.yml")
	require.NoError(t, err)
	beforeBookmarks, err := os.ReadFile("bookmarks.yml")
	require.NoError(t, err)
	doc.Apps[0].Name = "Changed"
	doc.Categories[0].Name = "Changed"
	_, err = commitEditorDocument(doc, func(src, dest string) error {
		if filepath.Base(dest) == "bookmarks.yml" {
			return errors.New("simulated rename failure")
		}
		return os.Rename(src, dest)
	})
	require.ErrorContains(t, err, "已恢复")
	var recovery *EditorSaveError
	require.ErrorAs(t, err, &recovery)
	require.False(t, recovery.RecoveryRequired)
	apps, err := os.ReadFile("apps.yml")
	require.NoError(t, err)
	require.Equal(t, beforeApps, apps)
	bookmarks, err := os.ReadFile("bookmarks.yml")
	require.NoError(t, err)
	require.Equal(t, beforeBookmarks, bookmarks)
	backups, err := filepath.Glob(".flare-editor-recovery/save-*/apps.yml")
	require.NoError(t, err)
	require.Len(t, backups, 1)
	backup, err := os.ReadFile(backups[0])
	require.NoError(t, err)
	require.Equal(t, beforeApps, backup)
	staged, err := filepath.Glob(".flare-editor-stage-*")
	require.NoError(t, err)
	require.Empty(t, staged)
}
func TestEditorDocumentEmptyAndPreflightFailure(t *testing.T) {
	doc := editorFixture(t)
	doc.Categories = nil
	doc.Apps = nil
	doc.Bookmarks = nil
	_, err := CommitEditorDocument(doc)
	require.NoError(t, err)
	doc, err = EditorSnapshot()
	require.NoError(t, err)
	require.Empty(t, doc.Bookmarks)
	raw, err := os.ReadFile("apps.yml")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(".flare-editor-recovery", 0700))
	require.NoError(t, os.WriteFile(".flare-editor-recovery/sentinel", []byte("keep"), 0600))
	require.NoError(t, os.RemoveAll(".flare-editor-recovery"))
	require.NoError(t, os.WriteFile(".flare-editor-recovery", []byte("blocked"), 0600))
	doc.Apps = []model.Bookmark{{Name: "Changed", URL: "http://nas"}}
	_, err = CommitEditorDocument(doc)
	require.Error(t, err)
	after, err := os.ReadFile("apps.yml")
	require.NoError(t, err)
	require.Equal(t, raw, after)
}

func TestEditorDocumentReportsIncompleteRecovery(t *testing.T) {
	doc := editorFixture(t)
	doc.Apps[0].Name = "Changed"
	_, err := commitEditorDocument(doc, func(src, dest string) error {
		if filepath.Base(dest) == "bookmarks.yml" {
			if removeErr := os.Remove("apps.yml"); removeErr != nil {
				return removeErr
			}
			if mkdirErr := os.Mkdir("apps.yml", 0700); mkdirErr != nil {
				return mkdirErr
			}
			return errors.New("simulated failure with blocked recovery")
		}
		return os.Rename(src, dest)
	})
	require.ErrorContains(t, err, "恢复未完成")
	require.ErrorContains(t, err, "可能存在部分写入")
	require.ErrorContains(t, err, "simulated failure with blocked recovery")
	var recovery *EditorSaveError
	require.ErrorAs(t, err, &recovery)
	require.True(t, recovery.RecoveryRequired)
	require.NotEmpty(t, recovery.RecoveryPath)
	copies, globErr := filepath.Glob(".flare-editor-recovery/save-*/apps.yml")
	require.NoError(t, globErr)
	require.Len(t, copies, 1)
	raw, readErr := os.ReadFile(copies[0])
	require.NoError(t, readErr)
	require.Contains(t, string(raw), "name: App")
}

func TestEditorDocumentPreservesImplicitDefaultCategory(t *testing.T) {
	for _, withCategory := range []bool{false, true} {
		t.Run(fmt.Sprint(withCategory), func(t *testing.T) {
			_ = editorFixture(t)
			raw := "links:\n- name: Implicit\n  link: http://nas:8080/default?q=1#f\n  private: true\n"
			if withCategory {
				raw = "categories:\n- id: first\n  title: First\n" + raw
			}
			require.NoError(t, os.WriteFile("bookmarks.yml", []byte(raw), 0600))
			doc, err := EditorSnapshot()
			require.NoError(t, err)
			doc.Apps[0].Name = "Changed app only"
			saved, err := CommitEditorDocument(doc)
			require.NoError(t, err)
			require.Len(t, saved.Bookmarks, 1)
			require.Empty(t, saved.Bookmarks[0].Category)
			require.True(t, saved.Bookmarks[0].Private)
			saved.Bookmarks[0].Name = "Edited without reclassification"
			_, err = CommitEditorDocument(saved)
			require.NoError(t, err)
			read, err := EditorSnapshot()
			require.NoError(t, err)
			require.Len(t, read.Apps, 1)
			require.Len(t, read.Bookmarks, 1)
			require.Empty(t, read.Bookmarks[0].Category)
			written, err := os.ReadFile("bookmarks.yml")
			require.NoError(t, err)
			require.NotContains(t, string(written), "category:")
		})
	}
}

// Unrelated edits must not destroy comments or rewrite the other collection.
func TestEditorDocumentPreservesUnchangedYAML(t *testing.T) {
	_ = editorFixture(t)
	require.NoError(t, os.WriteFile("bookmarks.yml", []byte("# Preserve this comment\ncategories:\n- id: stable-category\n  title: Original\nlinks:\n- name: Bookmark\n  link: custom://server/a?x=1&y=2#f\n  category: stable-category\n"), 0600))
	doc, err := EditorSnapshot()
	require.NoError(t, err)
	raw, err := os.ReadFile("bookmarks.yml")
	require.NoError(t, err)
	doc.Apps[0].Desc = "Application edit only"
	saved, err := CommitEditorDocument(doc)
	require.NoError(t, err)
	after, err := os.ReadFile("bookmarks.yml")
	require.NoError(t, err)
	require.Equal(t, raw, after)
	require.NotEqual(t, doc.Version, saved.Version)
}

// A semantic no-op must neither replace files nor manufacture a new revision.
func TestEditorDocumentNoopPreservesExactBytesAndRevision(t *testing.T) {
	doc := editorFixture(t)
	beforeApps, err := os.ReadFile("apps.yml")
	require.NoError(t, err)
	beforeBookmarks, err := os.ReadFile("bookmarks.yml")
	require.NoError(t, err)
	saved, err := commitEditorDocument(doc, func(string, string) error {
		t.Fatal("unchanged documents must not replace files")
		return nil
	})
	require.NoError(t, err)
	require.Equal(t, doc.Version, saved.Version)
	apps, err := os.ReadFile("apps.yml")
	require.NoError(t, err)
	bookmarks, err := os.ReadFile("bookmarks.yml")
	require.NoError(t, err)
	require.Equal(t, beforeApps, apps)
	require.Equal(t, beforeBookmarks, bookmarks)
	// Even no-op submissions must not bypass stale-version detection.
	doc.Version = "old"
	_, err = CommitEditorDocument(doc)
	require.ErrorIs(t, err, ErrBookmarkConflict)
}

func TestEditorRecoveryDetectionDoesNotRestoreOrDeleteCopies(t *testing.T) {
	t.Chdir(t.TempDir())
	before, err := EditorSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(".flare-editor-recovery", "save-interrupted")
	if err = os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"apps.yml", "bookmarks.yml"} {
		b, readErr := os.ReadFile(name)
		if readErr != nil {
			t.Fatal(readErr)
		}
		if err = os.WriteFile(filepath.Join(root, name), b, 0600); err != nil {
			t.Fatal(err)
		}
	}
	doc, err := EditorSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if doc.Version != before.Version || len(doc.RecoveryPaths) != 1 || !strings.HasSuffix(doc.RecoveryPaths[0], "save-interrupted") {
		t.Fatalf("unexpected recovery snapshot: %+v", doc)
	}
	if _, err = os.Stat(filepath.Join(root, "apps.yml")); err != nil {
		t.Fatal("recovery copies were removed", err)
	}
	// A symlink must not be followed as a recovery directory.
	if err = os.Symlink(t.TempDir(), filepath.Join(".flare-editor-recovery", "save-link")); err != nil {
		t.Fatal(err)
	}
	paths, err := EditorRecoveryDirectories()
	if err != nil || len(paths) != 1 {
		t.Fatalf("unexpected recovery scan %v %v", paths, err)
	}
}

func TestRecoveryCopiesPreventInitializationOfMissingLiveFiles(t *testing.T) {
	t.Chdir(t.TempDir())
	root := filepath.Join(".flare-editor-recovery", "save-crash")
	require.NoError(t, os.MkdirAll(root, 0700))
	_, err := EditorSnapshot()
	require.Error(t, err)
	require.Contains(t, err.Error(), "recovery copies exist")
	_, err = os.Stat("apps.yml")
	require.True(t, os.IsNotExist(err), "must not silently initialize replacement data")
}

func TestOrdinaryBookmarkLoadsCannotInitializeOverRecoveryCopies(t *testing.T) {
	for _, collection := range bookmarkHotPathCollections() {
		t.Run(collection.name, func(t *testing.T) {
			prepareBookmarkHotPath(t)
			require.NoError(t, os.MkdirAll(filepath.Join(".flare-editor-recovery", "save-interrupted"), 0700))
			_, err := collection.load()
			require.Error(t, err)
			require.Contains(t, err.Error(), "recovery copies exist")
			_, err = os.Stat(collection.name + ".yml")
			require.True(t, os.IsNotExist(err), "navigation must not create example data over recovery state")
		})
	}
}
