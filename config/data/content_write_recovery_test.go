package data

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

// These narrow failure tests check the editor's actual staging gate and its
// injected rename boundary. They do not simulate kernel power-loss semantics.
func contentWriteRecoveryFixture(t *testing.T) (EditorDocument, map[string][]byte) {
	t.Helper()
	doc := editorFixture(t)
	originals := map[string][]byte{}
	for _, name := range []string{"apps.yml", "bookmarks.yml"} {
		raw, e := os.ReadFile(name)
		require.NoError(t, e)
		raw = append([]byte("# exact original formatting\nx-user-extension: preserve-me\n"), raw...)
		require.NoError(t, os.WriteFile(name, raw, 0600))
		originals[name] = raw
	}
	doc, e := EditorSnapshot()
	require.NoError(t, e)
	// Warm both read caches before the failed write; rollback must not publish a
	// proposed draft through a successful-looking revision or cached collection.
	_, e = LoadFavoriteBookmarks()
	require.NoError(t, e)
	_, e = LoadNormalBookmarks()
	require.NoError(t, e)
	doc.Apps[0].Name = "FAILED-DRAFT-APP"
	doc.Bookmarks[0].Name = "FAILED-DRAFT-BOOKMARK"
	return doc, originals
}
func assertContentWriteOriginals(t *testing.T, original map[string][]byte, revision string) {
	t.Helper()
	for name, want := range original {
		raw, e := os.ReadFile(name)
		require.NoError(t, e)
		require.Equal(t, want, raw, name)
	}
	stages, e := filepath.Glob(".flare-editor-stage-*")
	require.NoError(t, e)
	require.Empty(t, stages)
	current, e := EditorSnapshot()
	require.NoError(t, e)
	require.Equal(t, revision, current.Version)
	apps, e := LoadFavoriteBookmarks()
	require.NoError(t, e)
	require.Equal(t, "App", apps.Items[0].Name)
	require.True(t, apps.Items[0].Private)
	bookmarks, e := LoadNormalBookmarks()
	require.NoError(t, e)
	require.Equal(t, "Bookmark", bookmarks.Items[0].Name)
}
func TestContentWriteRecoveryStagingFailuresPreserveExactPair(t *testing.T) {
	for _, point := range []string{"directory-create-denied", "second-file-readonly"} {
		t.Run(point, func(t *testing.T) {
			doc, original := contentWriteRecoveryFixture(t)
			if point == "directory-create-denied" {
				if os.Geteuid() == 0 {
					t.Skip("root bypasses directory DAC; container read-only tests cover mount enforcement")
				}
				require.NoError(t, os.Chmod(".", 0500))
				t.Cleanup(func() { require.NoError(t, os.Chmod(".", 0700)) })
			} else {
				require.NoError(t, os.Chmod("bookmarks.yml", 0400))
			}
			saved, e := CommitEditorDocument(doc)
			require.Error(t, e)
			require.Empty(t, saved.Version, "failed staging must not report a committed revision")
			if point == "directory-create-denied" {
				require.NoError(t, os.Chmod(".", 0700))
			}
			assertContentWriteOriginals(t, original, doc.Version)
			copies, e := filepath.Glob(".flare-editor-recovery/save-*")
			require.NoError(t, e)
			require.Empty(t, copies, "staging failure precedes recovery snapshots")
		})
	}
}
func TestContentWriteRecoveryRenameFailuresRetainExactPairAndCopies(t *testing.T) {
	for _, target := range []string{"apps.yml", "bookmarks.yml"} {
		t.Run(target, func(t *testing.T) {
			doc, original := contentWriteRecoveryFixture(t)
			saved, e := commitEditorDocument(doc, func(src, dst string) error {
				if filepath.Base(dst) == target {
					return &os.LinkError{Op: "rename", Old: src, New: dst, Err: syscall.EIO}
				}
				return os.Rename(src, dst)
			})
			require.Error(t, e)
			require.Empty(t, saved.Version, "failed replacement must not report success")
			var failure *EditorSaveError
			require.ErrorAs(t, e, &failure)
			require.False(t, failure.RecoveryRequired)
			require.ErrorIs(t, e, syscall.EIO)
			assertContentWriteOriginals(t, original, doc.Version)
			for name, want := range original {
				raw, e := os.ReadFile(filepath.Join(failure.RecoveryPath, name))
				require.NoError(t, e)
				require.Equal(t, want, raw, "exact pre-save recovery copy")
			}
		})
	}
}
