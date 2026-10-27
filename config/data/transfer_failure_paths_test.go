package data

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func TestImportFailureLeavesOriginalCollectionIntact(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T)
		want  error
	}{
		{"backup path file", func(t *testing.T) { require.NoError(t, os.WriteFile(bookmarkBackupDir, []byte("blocked"), 0600)) }, nil},
		{"backup path symlink", func(t *testing.T) { require.NoError(t, os.Symlink(t.TempDir(), bookmarkBackupDir)) }, nil},
		{"read only destination", func(t *testing.T) {
			require.NoError(t, os.Chmod("bookmarks.yml", 0400))
			t.Cleanup(func() { _ = os.Chmod("bookmarks.yml", 0600) })
		}, ErrBookmarkReadOnly},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			original := []byte("links:\n- name: Original\n  link: https://original.example\n")
			require.NoError(t, os.WriteFile("bookmarks.yml", original, 0600))
			tc.setup(t)
			err := CommitBookmarkImport("bookmarks", bookmarkVersion(original), model.Bookmarks{Items: []model.Bookmark{{Name: "New", URL: "https://new.example"}}})
			require.Error(t, err)
			if tc.want != nil {
				require.ErrorIs(t, err, tc.want)
			}
			raw, err := os.ReadFile("bookmarks.yml")
			require.NoError(t, err)
			require.Equal(t, original, raw)
		})
	}
	t.Run("revocation before rename", func(t *testing.T) {
		t.Chdir(t.TempDir())
		current, version, err := BookmarkSnapshot("bookmarks")
		require.NoError(t, err)
		original, err := os.ReadFile("bookmarks.yml")
		require.NoError(t, err)
		denied := errors.New("revoked")
		require.ErrorIs(t, CommitBookmarkImportChecked("bookmarks", version, current, func() error { return denied }), denied)
		raw, err := os.ReadFile("bookmarks.yml")
		require.NoError(t, err)
		require.Equal(t, original, raw)
	})
}

func TestSnapshotsAndAtomicImportRejectInvalidSources(t *testing.T) {
	t.Chdir(t.TempDir())
	_, err := RawBookmarkSnapshot("invalid")
	require.Error(t, err)
	_, err = RawBookmarkSnapshot("apps")
	require.ErrorIs(t, err, os.ErrNotExist)
	require.Error(t, CommitBookmarkImport("invalid", "", model.Bookmarks{}))
	require.ErrorIs(t, CommitBookmarkImport("apps", "", model.Bookmarks{}), os.ErrNotExist)
	require.NoError(t, os.WriteFile("apps.yml", []byte("links: [invalid"), 0600))
	_, err = RawBookmarkSnapshot("apps")
	require.Error(t, err)
	_, _, err = BookmarkSnapshot("apps")
	require.Error(t, err)
	require.NoError(t, os.Remove("apps.yml"))
	require.NoError(t, os.Mkdir("apps.yml", 0700))
	_, _, err = BookmarkSnapshot("apps")
	require.Error(t, err)
	require.Error(t, atomicBookmarkWrite(filepath.Join("missing", "file"), []byte("data"), 0600))
	require.Error(t, atomicBookmarkWrite("apps.yml", []byte("data"), 0600))
	entries, err := filepath.Glob(".flare-transfer-*")
	require.NoError(t, err)
	require.Empty(t, entries)
	require.NoError(t, os.Mkdir(bookmarkBackupDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(bookmarkBackupDir, "irrelevant.txt"), []byte("ignored"), 0600))
	_, _, err = LatestBookmarkBackup("apps")
	require.ErrorIs(t, err, os.ErrNotExist)
}
