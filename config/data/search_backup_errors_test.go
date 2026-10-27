package data

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestBackupManifestRejectsUnknownFieldsAndTrailingData(t *testing.T) {
	for _, raw := range []string{`{"owner":"flare-search-backups-v1","unknown":true}`, `{"owner":"flare-search-backups-v1"} {}`, `{"owner":"other"}`} {
		var state searchBackupManifest
		require.Error(t, decodeSearchBackupManifest([]byte(raw), &state))
	}
}

func TestPendingBackupRecoveryValidatesOwnershipAndContents(t *testing.T) {
	for _, tc := range []struct {
		name      string
		pending   searchPendingBackup
		oldHash   string
		content   []byte
		mode      os.FileMode
		wantError bool
	}{
		{name: "invalid pending name", pending: searchPendingBackup{Name: "../outside", Hash: "hash"}, wantError: true},
		{name: "missing pending hash", pending: searchPendingBackup{Name: "latest.bak"}, wantError: true},
		{name: "missing old backup", pending: searchPendingBackup{Name: "latest.bak", Hash: "new"}, oldHash: "old", wantError: true},
		{name: "modified pending backup", pending: searchPendingBackup{Name: "latest.bak", Hash: "new"}, content: []byte("modified"), mode: 0600, wantError: true},
		{name: "public pending backup", pending: searchPendingBackup{Name: "latest.bak", Hash: "new"}, content: []byte("new"), mode: 0644, wantError: true},
		{name: "completed upgrade", pending: searchPendingBackup{Name: "first-upgrade-to-1.0.bak", Hash: backupDigest([]byte("new")), Version: "1.0"}, content: []byte("new"), mode: 0600},
		{name: "old backup retained", pending: searchPendingBackup{Name: "latest.bak", Hash: "new"}, oldHash: backupDigest([]byte("old")), content: []byte("old"), mode: 0600},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			state := searchBackupManifest{Latest: tc.oldHash, Pending: &tc.pending}
			if tc.content != nil {
				require.NoError(t, os.WriteFile(filepath.Join(dir, tc.pending.Name), tc.content, tc.mode))
			}
			err := recoverSearchBackup(dir, &state)
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Nil(t, state.Pending)
				if tc.pending.Name != "latest.bak" {
					require.Equal(t, tc.pending.Name, state.UpgradeFile)
					require.Equal(t, "1.0", state.UpgradeVersion)
					require.Equal(t, tc.pending.Hash, state.Upgrade)
				}
			}
		})
	}
}

func TestManagedBackupValidationRejectsMissingOrUnsafeFiles(t *testing.T) {
	dir := t.TempDir()
	require.Error(t, validateSearchBackups(dir, searchBackupManifest{UpgradeFile: "../outside"}))
	require.Error(t, validateSearchBackups(dir, searchBackupManifest{Latest: "missing"}))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "latest.bak"), []byte("private"), 0644))
	require.Error(t, regularBackupFile(filepath.Join(dir, "latest.bak")))
	require.NoError(t, os.Chmod(filepath.Join(dir, "latest.bak"), 0600))
	require.Error(t, validateSearchBackups(dir, searchBackupManifest{Latest: "incorrect hash"}))
}
