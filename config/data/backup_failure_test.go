package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type savedEntry struct {
	Mode os.FileMode
	Data string
}

// Include names, permissions and symlink targets, not just the live config.
// A refused save must not rotate backups or remove an unrelated user file.
func snapshotConfigTree(t *testing.T) map[string]savedEntry {
	t.Helper()
	entries := map[string]savedEntry{}
	require.NoError(t, filepath.WalkDir(".", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		var contents []byte
		if info.Mode().IsRegular() {
			contents, err = os.ReadFile(path)
		} else if info.Mode()&os.ModeSymlink != 0 {
			var target string
			target, err = os.Readlink(path)
			contents = []byte(target)
		}
		entries[path] = savedEntry{Mode: info.Mode(), Data: string(contents)}
		return err
	}))
	return entries
}

func writeBackupManifest(t *testing.T, state searchBackupManifest) {
	t.Helper()
	raw, err := json.Marshal(state)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(searchBackupDir, "manifest.json"), raw, 0600))
}

func TestBackupRefusalPreservesEntireUserTree(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, *searchBackupManifest)
		want    string
	}{
		{"invalid JSON", func(t *testing.T, _ *searchBackupManifest) {
			require.NoError(t, os.WriteFile(filepath.Join(searchBackupDir, "manifest.json"), []byte("{"), 0600))
		}, "unexpected EOF"},
		{"unknown fields", func(t *testing.T, _ *searchBackupManifest) {
			require.NoError(t, os.WriteFile(filepath.Join(searchBackupDir, "manifest.json"), []byte(`{"owner":"flare-search-backups-v1","future":true}`), 0600))
		}, "unknown field"},
		{"foreign owner", func(_ *testing.T, s *searchBackupManifest) { s.Owner = "user" }, "unrecognized backup owner"},
		{"incomplete upgrade", func(_ *testing.T, s *searchBackupManifest) { s.UpgradeFile = "first-upgrade-to-2.0.0.bak" }, "incomplete upgrade"},
		{"upgrade traversal", func(_ *testing.T, s *searchBackupManifest) {
			s.Upgrade, s.UpgradeFile = "hash", "../user.bak"
		}, "invalid managed upgrade filename"},
		{"empty pending digest", func(_ *testing.T, s *searchBackupManifest) {
			s.Pending = &searchPendingBackup{Name: "latest.bak"}
		}, "invalid pending backup"},
		{"pending traversal", func(_ *testing.T, s *searchBackupManifest) {
			s.Pending = &searchPendingBackup{Name: "../user.bak", Hash: "hash"}
		}, "invalid pending backup"},
		{"second pending upgrade", func(_ *testing.T, s *searchBackupManifest) {
			s.Upgrade = "hash"
			s.Pending = &searchPendingBackup{Name: "first-upgrade-to-2.0.0.bak", Hash: "hash"}
		}, "invalid pending backup"},
		{"pending modified externally", func(_ *testing.T, s *searchBackupManifest) {
			s.Latest = backupDigest([]byte("different previous backup"))
			s.Pending = &searchPendingBackup{Name: "latest.bak", Hash: backupDigest([]byte("pending"))}
		}, "pending backup was externally modified"},
		{"pending missing", func(t *testing.T, s *searchBackupManifest) {
			s.Pending = &searchPendingBackup{Name: "latest.bak", Hash: "hash"}
			require.NoError(t, os.Remove(filepath.Join(searchBackupDir, "latest.bak")))
		}, "pending backup missing"},
		{"pending public", func(t *testing.T, s *searchBackupManifest) {
			s.Pending = &searchPendingBackup{Name: "latest.bak", Hash: "hash"}
			require.NoError(t, os.Chmod(filepath.Join(searchBackupDir, "latest.bak"), 0644))
		}, "unsafe pending backup"},
		{"pending symlink", func(t *testing.T, s *searchBackupManifest) {
			s.Pending = &searchPendingBackup{Name: "latest.bak", Hash: "hash"}
			p := filepath.Join(searchBackupDir, "latest.bak")
			require.NoError(t, os.Remove(p))
			require.NoError(t, os.Symlink("../user.bak", p))
		}, "unsafe pending backup"},
		{"managed missing", func(t *testing.T, _ *searchBackupManifest) {
			require.NoError(t, os.Remove(filepath.Join(searchBackupDir, "latest.bak")))
		}, "no such file"},
		{"managed directory", func(t *testing.T, _ *searchBackupManifest) {
			p := filepath.Join(searchBackupDir, "latest.bak")
			require.NoError(t, os.Remove(p))
			require.NoError(t, os.Mkdir(p, 0700))
		}, "not a regular file"},
		{"manifest public", func(t *testing.T, _ *searchBackupManifest) {
			require.NoError(t, os.Chmod(filepath.Join(searchBackupDir, "manifest.json"), 0644))
		}, "unrecognized backup directory"},
		{"directory public", func(t *testing.T, _ *searchBackupManifest) {
			require.NoError(t, os.Chmod(searchBackupDir, 0755))
		}, "unsafe backup directory"},
		{"directory symlink", func(t *testing.T, _ *searchBackupManifest) {
			require.NoError(t, os.Rename(searchBackupDir, "user-directory"))
			require.NoError(t, os.Symlink("user-directory", searchBackupDir))
		}, "unsafe backup directory"},
		{"permanent backup modified", func(t *testing.T, s *searchBackupManifest) {
			s.Upgrade, s.UpgradeFile = backupDigest([]byte("original upgrade")), "first-upgrade-to-1.0.0.bak"
			require.NoError(t, os.WriteFile(filepath.Join(searchBackupDir, s.UpgradeFile), []byte("user replacement"), 0600))
		}, "backup was externally modified"},
		{"unowned legacy permanent backup", func(t *testing.T, _ *searchBackupManifest) {
			require.NoError(t, os.WriteFile(filepath.Join(searchBackupDir, "first-upgrade.bak"), []byte("user original"), 0600))
		}, "unowned backup file"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withApplicationVersion(t, "2.0.0")
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile("config.yml", []byte("AppVersion: 1.0.0\nShowSearchComponent: true\n"), 0600))
			require.NoError(t, os.WriteFile("user.bak", []byte("precious"), 0640))
			require.NoError(t, retainSearchBackup([]byte("previous"), false))
			state := searchBackupManifest{Owner: "flare-search-backups-v1", Latest: backupDigest([]byte("previous"))}
			tc.prepare(t, &state)
			// Raw manifest cases intentionally supply their own bytes.
			if tc.name != "invalid JSON" && tc.name != "unknown fields" {
				writeBackupManifest(t, state)
			}
			before := snapshotConfigTree(t)
			err := UpdateSearchOptions(false, false, nil)
			require.ErrorContains(t, err, tc.want)
			require.Equal(t, before, snapshotConfigTree(t))
		})
	}
}

func TestUnreadableBackupMetadataRefusesSaveWithoutRotation(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file read permissions")
	}
	for _, which := range []string{"manifest", "latest", "pending"} {
		t.Run(which, func(t *testing.T) {
			t.Chdir(t.TempDir())
			require.NoError(t, retainSearchBackup([]byte("previous"), false))
			if which == "pending" {
				writeBackupManifest(t, searchBackupManifest{
					Owner: "flare-search-backups-v1", Latest: backupDigest([]byte("previous")),
					Pending: &searchPendingBackup{Name: "latest.bak", Hash: backupDigest([]byte("new"))},
				})
			}
			before := snapshotConfigTree(t)
			name := "latest.bak"
			if which == "manifest" {
				name = "manifest.json"
			}
			path := filepath.Join(searchBackupDir, name)
			require.NoError(t, os.Chmod(path, 0000))
			err := retainSearchBackup([]byte("next"), false)
			require.NoError(t, os.Chmod(path, 0600))
			require.ErrorIs(t, err, os.ErrPermission)
			require.Equal(t, before, snapshotConfigTree(t))
		})
	}
}

func TestUpgradeRefusesInvalidDocumentWithoutTouchingUserFiles(t *testing.T) {
	for _, raw := range []string{"", "[one, two]\n", "scalar\n", "Title: [invalid\n", "ShowSearchComponent: definitely-not-a-boolean\n"} {
		t.Run(raw, func(t *testing.T) {
			withApplicationVersion(t, "2.0.0")
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile("config.yml", []byte(raw), 0600))
			require.NoError(t, os.WriteFile("personal.bak", []byte("precious"), 0600))
			before := snapshotConfigTree(t)
			_, err := GetAllSettingsOptions()
			require.Error(t, err)
			require.Error(t, UpdateSearchOptions(false, false, nil))
			require.False(t, UpdateThemeName("blackboard"))
			require.Equal(t, before, snapshotConfigTree(t))
		})
	}
}

func TestFailedAtomicCommitRetainsOriginalAndBackup(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory write permissions")
	}
	t.Chdir(t.TempDir())
	withApplicationVersion(t, "2.0.0")
	original := []byte("Title: original\nCustom: secret\n")
	require.NoError(t, os.WriteFile("config.yml", original, 0600))
	err := commitConfigDocumentWithBackup("config.yml", original, []byte("proposed"), true, func(raw []byte, upgrading bool) error {
		require.NoError(t, retainSearchBackup(raw, upgrading))
		// The original remains readable but temporary-file creation must fail.
		return os.Chmod(".", 0500)
	})
	require.NoError(t, os.Chmod(".", 0700))
	require.ErrorContains(t, err, "atomic replacement failed; original backup retained")
	got, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, original, got)
	got, err = os.ReadFile(filepath.Join(searchBackupDir, "first-upgrade-to-2.0.0.bak"))
	require.NoError(t, err)
	require.Equal(t, original, got)
	require.NoError(t, commitConfigDocument("config.yml", original, []byte("proposed"), true))
}

func TestBackupInitializationFailureNeverDeletesUserFiles(t *testing.T) {
	for _, userFile := range []bool{false, true} {
		t.Run(fmt.Sprint(userFile), func(t *testing.T) {
			t.Chdir(t.TempDir())
			err := retainSearchBackupWithWriter([]byte("original"), true, func(_ string, _ []byte) bool {
				if userFile {
					require.NoError(t, os.WriteFile(filepath.Join(searchBackupDir, "user.bak"), []byte("keep"), 0600))
				}
				return false
			})
			require.ErrorContains(t, err, "cannot initialize backup manifest")
			if userFile {
				got, readErr := os.ReadFile(filepath.Join(searchBackupDir, "user.bak"))
				require.NoError(t, readErr)
				require.Equal(t, "keep", string(got))
			} else {
				_, statErr := os.Stat(searchBackupDir)
				require.ErrorIs(t, statErr, os.ErrNotExist)
			}
		})
	}
}

func TestCommitBackupWriteFailuresCanBeRetriedWithoutLosingOriginal(t *testing.T) {
	withApplicationVersion(t, "2.0.0")
	for _, upgrading := range []bool{false, true} {
		for phase := 1; phase <= 4; phase++ {
			t.Run(fmt.Sprintf("upgrade=%v/write=%d", upgrading, phase), func(t *testing.T) {
				t.Chdir(t.TempDir())
				original := []byte("# user comment\nTitle: original\nCustom: secret\n")
				proposed := []byte("Title: updated\nCustom: secret\n")
				require.NoError(t, os.WriteFile("config.yml", original, 0640))
				require.NoError(t, os.WriteFile("personal.bak", []byte("user data"), 0600))
				writes := 0
				backup := func(raw []byte, upgrade bool) error {
					return retainSearchBackupWithWriter(raw, upgrade, func(path string, contents []byte) bool {
						writes++
						return writes != phase && saveConfigAtomically(path, contents)
					})
				}
				require.Error(t, commitConfigDocumentWithBackup("config.yml", original, proposed, upgrading, backup))
				got, err := os.ReadFile("config.yml")
				require.NoError(t, err)
				require.Equal(t, original, got)
				require.NoError(t, commitConfigDocument("config.yml", original, proposed, upgrading))
				got, err = os.ReadFile("config.yml")
				require.NoError(t, err)
				require.Equal(t, proposed, got)
				name := "latest.bak"
				if upgrading {
					name = "first-upgrade-to-2.0.0.bak"
				}
				got, err = os.ReadFile(filepath.Join(searchBackupDir, name))
				require.NoError(t, err)
				require.Equal(t, original, got)
				got, err = os.ReadFile("personal.bak")
				require.NoError(t, err)
				require.Equal(t, "user data", string(got))
				info, err := os.Stat("config.yml")
				require.NoError(t, err)
				require.Equal(t, os.FileMode(0640), info.Mode().Perm())
			})
		}
	}
}

func TestCommitRejectsChangedOrReplacedConfiguration(t *testing.T) {
	for _, tc := range []struct {
		name   string
		before bool
		change func(*testing.T)
	}{
		{"changed before backup", true, func(t *testing.T) { require.NoError(t, os.WriteFile("config.yml", []byte("external edit"), 0600)) }},
		{"missing before backup", true, func(t *testing.T) { require.NoError(t, os.Remove("config.yml")) }},
		{"missing after backup", false, func(t *testing.T) { require.NoError(t, os.Remove("config.yml")) }},
		{"symlink after backup", false, func(t *testing.T) {
			require.NoError(t, os.Remove("config.yml"))
			require.NoError(t, os.Symlink("user.bak", "config.yml"))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			raw := []byte("Title: original\n")
			require.NoError(t, os.WriteFile("config.yml", raw, 0600))
			require.NoError(t, os.WriteFile("user.bak", []byte("precious"), 0600))
			if tc.before {
				tc.change(t)
			}
			var expected map[string]savedEntry
			if tc.before {
				expected = snapshotConfigTree(t)
			}
			called := false
			err := commitConfigDocumentWithBackup("config.yml", raw, []byte("proposed"), true, func(_ []byte, _ bool) error {
				called = true
				tc.change(t)
				expected = snapshotConfigTree(t)
				return nil
			})
			require.Error(t, err)
			require.Equal(t, !tc.before, called)
			require.Equal(t, expected, snapshotConfigTree(t))
		})
	}
	t.Run("backup error is preserved", func(t *testing.T) {
		t.Chdir(t.TempDir())
		raw := []byte("Title: original\n")
		require.NoError(t, os.WriteFile("config.yml", raw, 0600))
		before := snapshotConfigTree(t)
		failure := errors.New("disk full")
		err := commitConfigDocumentWithBackup("config.yml", raw, []byte("proposed"), true, func(_ []byte, _ bool) error { return failure })
		require.ErrorIs(t, err, failure)
		require.Equal(t, before, snapshotConfigTree(t))
	})
}
