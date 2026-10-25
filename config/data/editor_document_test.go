package data

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
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
