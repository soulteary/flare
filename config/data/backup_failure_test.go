package data

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/flare/config/model"
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

type backupWarning struct{ path, reason, action string }

// captureBackupWarnings records warnings instead of logging them and clears the
// once-per-process filter around the test.
func captureBackupWarnings(t *testing.T) *[]backupWarning {
	t.Helper()
	var seen []backupWarning
	previous := backupWarn
	backupWarn = func(path, reason, action string) {
		seen = append(seen, backupWarning{path, reason, action})
	}
	backupWarned.Clear()
	t.Cleanup(func() {
		backupWarn = previous
		backupWarned.Clear()
	})
	return &seen
}

func setAsideBackups(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob(searchBackupDir + ".quarantined-*")
	require.NoError(t, err)
	return matches
}

// subtree returns the snapshot entries at and below root, keyed relative to it.
func subtree(entries map[string]savedEntry, root string) map[string]savedEntry {
	out := map[string]savedEntry{}
	for path, entry := range entries {
		if path == root {
			out["."] = entry
		} else if rel, ok := strings.CutPrefix(path, root+string(filepath.Separator)); ok {
			out[rel] = entry
		}
	}
	return out
}

// A backup directory that cannot be trusted or continued is kept intact under
// another name. The save proceeds and a new directory holds its backup.
func TestRejectedBackupsAreSetAsideAndSaveContinues(t *testing.T) {
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
		{"managed symlink", func(t *testing.T, _ *searchBackupManifest) {
			p := filepath.Join(searchBackupDir, "latest.bak")
			require.NoError(t, os.Remove(p))
			require.NoError(t, os.Symlink("../user.bak", p))
		}, "not a regular file"},
		{"manifest missing", func(t *testing.T, _ *searchBackupManifest) {
			require.NoError(t, os.Remove(filepath.Join(searchBackupDir, "manifest.json")))
		}, "unrecognized backup directory"},
		{"permanent backup modified", func(t *testing.T, s *searchBackupManifest) {
			s.Upgrade, s.UpgradeFile = backupDigest([]byte("original upgrade")), "first-upgrade-to-1.0.0.bak"
			require.NoError(t, os.WriteFile(filepath.Join(searchBackupDir, s.UpgradeFile), []byte("user replacement"), 0600))
		}, "backup was externally modified"},
		{"unowned legacy permanent backup", func(t *testing.T, _ *searchBackupManifest) {
			require.NoError(t, os.WriteFile(filepath.Join(searchBackupDir, "first-upgrade.bak"), []byte("user original"), 0600))
		}, "unowned backup file"},
		{"unowned upgrade backup", func(t *testing.T, _ *searchBackupManifest) {
			require.NoError(t, os.WriteFile(filepath.Join(searchBackupDir, "first-upgrade-to-2.0.0.bak"), []byte("user original"), 0600))
		}, "unowned upgrade backup"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withApplicationVersion(t, "2.0.0")
			warnings := captureBackupWarnings(t)
			t.Chdir(t.TempDir())
			config := []byte("AppVersion: 1.0.0\nShowSearchComponent: true\n")
			require.NoError(t, os.WriteFile("config.yml", config, 0600))
			require.NoError(t, os.WriteFile("user.bak", []byte("precious"), 0640))
			require.NoError(t, retainSearchBackup([]byte("previous"), false))
			state := searchBackupManifest{Owner: "flare-search-backups-v1", Latest: backupDigest([]byte("previous"))}
			tc.prepare(t, &state)
			// Raw manifest cases intentionally supply their own bytes.
			if tc.name != "invalid JSON" && tc.name != "unknown fields" && tc.name != "manifest missing" {
				writeBackupManifest(t, state)
			}
			before := snapshotConfigTree(t)

			require.NoError(t, UpdateSearchOptions(false, false, nil))

			after := snapshotConfigTree(t)
			moved := setAsideBackups(t)
			require.Len(t, moved, 1)
			require.Equal(t, subtree(before, searchBackupDir), subtree(after, moved[0]), "the rejected directory must be kept unchanged")
			require.Equal(t, before["user.bak"], after["user.bak"])
			saved, err := os.ReadFile("config.yml")
			require.NoError(t, err)
			require.Contains(t, string(saved), "ShowSearchComponent: false")
			backup, err := os.ReadFile(filepath.Join(searchBackupDir, "first-upgrade-to-2.0.0.bak"))
			require.NoError(t, err)
			require.Equal(t, config, backup)
			require.Len(t, *warnings, 1)
			require.Contains(t, (*warnings)[0].reason, tc.want)
			require.Contains(t, (*warnings)[0].action, filepath.Base(moved[0]))
			require.True(t, filepath.IsAbs((*warnings)[0].path))
		})
	}
}

func TestBackupPermissionsAreRepairedInPlace(t *testing.T) {
	manifest, latest := filepath.Join(searchBackupDir, "manifest.json"), filepath.Join(searchBackupDir, "latest.bak")
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T, *searchBackupManifest)
	}{
		{"directory", func(t *testing.T, _ *searchBackupManifest) { require.NoError(t, os.Chmod(searchBackupDir, 0755)) }},
		{"manifest", func(t *testing.T, _ *searchBackupManifest) { require.NoError(t, os.Chmod(manifest, 0644)) }},
		{"backup", func(t *testing.T, _ *searchBackupManifest) { require.NoError(t, os.Chmod(latest, 0644)) }},
		{"pending backup", func(t *testing.T, s *searchBackupManifest) {
			s.Pending = &searchPendingBackup{Name: "latest.bak", Hash: "hash"}
			require.NoError(t, os.Chmod(latest, 0644))
		}},
		{"copied without modes", func(t *testing.T, _ *searchBackupManifest) {
			require.NoError(t, os.Chmod(searchBackupDir, 0777))
			require.NoError(t, os.Chmod(manifest, 0666))
			require.NoError(t, os.Chmod(latest, 0666))
		}},
		{"owner cannot read", func(t *testing.T, _ *searchBackupManifest) {
			require.NoError(t, os.Chmod(manifest, 0000))
			require.NoError(t, os.Chmod(latest, 0000))
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withApplicationVersion(t, "2.0.0")
			warnings := captureBackupWarnings(t)
			t.Chdir(t.TempDir())
			config := []byte("AppVersion: 1.0.0\nShowSearchComponent: true\n")
			require.NoError(t, os.WriteFile("config.yml", config, 0600))
			require.NoError(t, retainSearchBackup([]byte("previous"), false))
			state := searchBackupManifest{Owner: "flare-search-backups-v1", Latest: backupDigest([]byte("previous"))}
			writeBackupManifest(t, state)
			tc.prepare(t, &state)
			if state.Pending != nil {
				raw, err := json.Marshal(state)
				require.NoError(t, err)
				require.NoError(t, os.WriteFile(manifest, raw, 0600))
			}

			require.NoError(t, UpdateSearchOptions(false, false, nil))

			require.Empty(t, setAsideBackups(t))
			require.Empty(t, *warnings)
			info, err := os.Stat(searchBackupDir)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0700), info.Mode().Perm())
			for _, name := range []string{"manifest.json", "latest.bak", "first-upgrade-to-2.0.0.bak"} {
				info, err = os.Stat(filepath.Join(searchBackupDir, name))
				require.NoError(t, err)
				require.Equal(t, os.FileMode(0600), info.Mode().Perm(), name)
			}
			previous, err := os.ReadFile(latest)
			require.NoError(t, err)
			require.Equal(t, "previous", string(previous))
			backup, err := os.ReadFile(filepath.Join(searchBackupDir, "first-upgrade-to-2.0.0.bak"))
			require.NoError(t, err)
			require.Equal(t, config, backup)
		})
	}
}

// A data directory copied or restored without its modes used to block every
// settings page. Each page's save must succeed and leave the backups private.
func TestEverySettingsSaveSurvivesWidenedBackupPermissions(t *testing.T) {
	// The first search save of a document without a Search block is an upgrade
	// and keeps the previous document in the permanent backup.
	for _, tc := range []struct {
		name   string
		save   func() error
		backup string
	}{
		{"theme", func() error { return SaveThemeName("cloud") }, "latest.bak"},
		{"appearance", func() error { return SaveAppearance(model.Application{Title: "changed"}) }, "latest.bak"},
		{"weather", func() error {
			return SaveSelectedWeather(true, &model.WeatherLocation{Province: "北京", City: "北京"})
		}, "latest.bak"},
		{"search visibility", func() error { return SaveSearch(false, false) }, "first-upgrade-to-2.0.0.bak"},
		{"search options", func() error { return UpdateSearchOptions(false, true, nil) }, "first-upgrade-to-2.0.0.bak"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withApplicationVersion(t, "2.0.0")
			warnings := captureBackupWarnings(t)
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile("config.yml", []byte("AppVersion: 2.0.0\nTitle: current\nShowSearchComponent: true\n"), 0600))
			require.NoError(t, SaveThemeName("blackboard"))
			previous, err := os.ReadFile("config.yml")
			require.NoError(t, err)
			require.NoError(t, os.Chmod(searchBackupDir, 0755))
			entries, err := os.ReadDir(searchBackupDir)
			require.NoError(t, err)
			require.NotEmpty(t, entries)
			for _, entry := range entries {
				require.NoError(t, os.Chmod(filepath.Join(searchBackupDir, entry.Name()), 0644))
			}

			require.NoError(t, tc.save())

			require.Empty(t, *warnings)
			require.Empty(t, setAsideBackups(t))
			saved, err := os.ReadFile("config.yml")
			require.NoError(t, err)
			require.NotEqual(t, previous, saved)
			backup, err := os.ReadFile(filepath.Join(searchBackupDir, tc.backup))
			require.NoError(t, err)
			require.Equal(t, previous, backup)
			info, err := os.Stat(searchBackupDir)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0700), info.Mode().Perm())
			for _, entry := range entries {
				info, err = os.Stat(filepath.Join(searchBackupDir, entry.Name()))
				require.NoError(t, err)
				require.Equal(t, os.FileMode(0600), info.Mode().Perm(), entry.Name())
			}
		})
	}
}

// Some filesystems accept chmod without applying it, or reject it. Neither may
// stop a save; the condition is reported once.
func TestBackupContinuesWhenPermissionsCannotBeChanged(t *testing.T) {
	for _, tc := range []struct {
		name  string
		chmod func(string, os.FileMode) error
		want  string
	}{
		{"ignored", func(string, os.FileMode) error { return nil }, "still allows group or other access"},
		{"refused", func(string, os.FileMode) error { return os.ErrPermission }, "permission denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			warnings := captureBackupWarnings(t)
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile("config.yml", []byte("ShowSearchComponent: true\nSearch:\n  Mode: select\n  DefaultTarget: local\n"), 0600))
			require.NoError(t, retainSearchBackup([]byte("previous"), false))
			latest := filepath.Join(searchBackupDir, "latest.bak")
			require.NoError(t, os.Chmod(searchBackupDir, 0755))
			require.NoError(t, os.Chmod(latest, 0644))
			previous := backupChmod
			backupChmod = tc.chmod
			t.Cleanup(func() { backupChmod = previous })

			require.NoError(t, UpdateSearchOptions(false, false, nil))
			require.NoError(t, UpdateSearchOptions(true, false, nil))

			require.Empty(t, setAsideBackups(t))
			saved, err := os.ReadFile("config.yml")
			require.NoError(t, err)
			require.Contains(t, string(saved), "ShowSearchComponent: true")
			rotated, err := os.ReadFile(latest)
			require.NoError(t, err)
			require.Contains(t, string(rotated), "ShowSearchComponent: false", "the second save backs up the first")
			// One warning per path across both saves: the directory and the backup.
			require.Len(t, *warnings, 2)
			for _, warning := range *warnings {
				require.Contains(t, warning.reason, tc.want)
				require.Contains(t, warning.action, "kept using it")
			}
			require.Equal(t, absoluteBackupPath(searchBackupDir), (*warnings)[0].path)
			require.Equal(t, absoluteBackupPath(latest), (*warnings)[1].path)
		})
	}
}

// A symbolic link or a file at the backup path belongs to the operator. Nothing
// is written through it and it is not renamed; the save continues unprotected.
func TestOccupiedBackupPathIsLeftAlone(t *testing.T) {
	for _, tc := range []struct {
		name    string
		prepare func(*testing.T)
	}{
		{"symlink", func(t *testing.T) {
			require.NoError(t, retainSearchBackup([]byte("previous"), false))
			require.NoError(t, os.Rename(searchBackupDir, "user-directory"))
			require.NoError(t, os.Symlink("user-directory", searchBackupDir))
		}},
		{"dangling symlink", func(t *testing.T) { require.NoError(t, os.Symlink("missing-directory", searchBackupDir)) }},
		{"regular file", func(t *testing.T) { require.NoError(t, os.WriteFile(searchBackupDir, []byte("mine"), 0600)) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			warnings := captureBackupWarnings(t)
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile("config.yml", []byte("ShowSearchComponent: true\nSearch:\n  Mode: select\n  DefaultTarget: local\n"), 0600))
			tc.prepare(t)
			before := snapshotConfigTree(t)

			require.NoError(t, UpdateSearchOptions(false, false, nil))
			require.NoError(t, UpdateSearchOptions(true, false, nil))
			require.NoError(t, UpdateSearchOptions(false, false, nil))

			after := snapshotConfigTree(t)
			saved := after["config.yml"]
			require.Contains(t, saved.Data, "ShowSearchComponent: false")
			delete(before, "config.yml")
			delete(after, "config.yml")
			require.Equal(t, before, after)
			require.Len(t, *warnings, 1, "a persistent condition is reported once")
			require.Contains(t, (*warnings)[0].reason, "symbolic link or not a directory")
			require.Contains(t, (*warnings)[0].action, "without a backup")
			require.Equal(t, absoluteBackupPath(searchBackupDir), (*warnings)[0].path)
		})
	}
}

func TestUnreadableBackupFilesAreSetAside(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses file read permissions")
	}
	for _, which := range []string{"manifest", "latest", "pending"} {
		t.Run(which, func(t *testing.T) {
			warnings := captureBackupWarnings(t)
			t.Chdir(t.TempDir())
			require.NoError(t, retainSearchBackup([]byte("previous"), false))
			if which == "pending" {
				writeBackupManifest(t, searchBackupManifest{
					Owner: "flare-search-backups-v1", Latest: backupDigest([]byte("previous")),
					Pending: &searchPendingBackup{Name: "latest.bak", Hash: backupDigest([]byte("new"))},
				})
			}
			name := "latest.bak"
			if which == "manifest" {
				name = "manifest.json"
			}
			before := snapshotConfigTree(t)
			require.NoError(t, os.Chmod(filepath.Join(searchBackupDir, name), 0000))
			// The file belongs to another user: its mode cannot be changed either.
			previous := backupChmod
			backupChmod = func(string, os.FileMode) error { return os.ErrPermission }
			t.Cleanup(func() { backupChmod = previous })

			require.NoError(t, retainSearchBackup([]byte("next"), false))

			moved := setAsideBackups(t)
			require.Len(t, moved, 1)
			info, err := os.Stat(filepath.Join(moved[0], name))
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0000), info.Mode().Perm())
			require.NoError(t, os.Chmod(filepath.Join(moved[0], name), 0600))
			require.Equal(t, subtree(before, searchBackupDir), subtree(snapshotConfigTree(t), moved[0]))
			latest, err := os.ReadFile(filepath.Join(searchBackupDir, "latest.bak"))
			require.NoError(t, err)
			require.Equal(t, "next", string(latest))
			require.Len(t, *warnings, 1)
			require.Contains(t, (*warnings)[0].reason, "permission denied")
		})
	}
}

func TestSetAsideNeverReplacesAnExistingPath(t *testing.T) {
	warnings := captureBackupWarnings(t)
	t.Chdir(t.TempDir())
	at := time.Date(2026, 11, 14, 3, 15, 2, 0, time.FixedZone("local", 8*3600))
	previous := backupNow
	backupNow = func() time.Time { return at }
	t.Cleanup(func() { backupNow = previous })
	taken := searchBackupDir + ".quarantined-20261113T191502Z"
	require.NoError(t, os.Mkdir(taken, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(taken, "earlier"), []byte("keep"), 0600))
	require.NoError(t, os.Mkdir(searchBackupDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(searchBackupDir, "latest.bak"), []byte("mine"), 0600))

	require.NoError(t, retainSearchBackup([]byte("next"), false))

	moved := setAsideBackups(t)
	require.Len(t, moved, 2)
	earlier, err := os.ReadFile(filepath.Join(taken, "earlier"))
	require.NoError(t, err)
	require.Equal(t, "keep", string(earlier))
	var second string
	for _, name := range moved {
		if name != taken {
			second = name
		}
	}
	require.True(t, strings.HasPrefix(second, taken+"-"), second)
	mine, err := os.ReadFile(filepath.Join(second, "latest.bak"))
	require.NoError(t, err)
	require.Equal(t, "mine", string(mine))
	require.Len(t, *warnings, 1)
}

func TestBackupThatCannotBeSetAsideIsSkipped(t *testing.T) {
	warnings := captureBackupWarnings(t)
	t.Chdir(t.TempDir())
	raw := []byte("Title: original\n")
	require.NoError(t, os.WriteFile("config.yml", raw, 0600))
	require.NoError(t, os.Mkdir(searchBackupDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(searchBackupDir, "latest.bak"), []byte("mine"), 0600))
	previous := backupRename
	backupRename = func(string, string) error { return os.ErrPermission }
	t.Cleanup(func() { backupRename = previous })
	before := snapshotConfigTree(t)

	err := retainSearchBackup(raw, false)
	require.ErrorContains(t, err, "unrecognized backup directory")
	require.ErrorContains(t, err, "could not be set aside")
	require.ErrorIs(t, err, os.ErrPermission)
	require.Equal(t, before, snapshotConfigTree(t))
	require.Empty(t, *warnings)

	// The save itself is not blocked by the backup it could not take.
	require.NoError(t, commitConfigDocument("config.yml", raw, []byte("Title: proposed\n"), false))
	saved, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, "Title: proposed\n", string(saved))
	mine, err := os.ReadFile(filepath.Join(searchBackupDir, "latest.bak"))
	require.NoError(t, err)
	require.Equal(t, "mine", string(mine))
	require.Len(t, *warnings, 1)
	require.Contains(t, (*warnings)[0].reason, "could not be set aside")
	require.Contains(t, (*warnings)[0].action, "without a backup")
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
	require.ErrorContains(t, err, "atomic replacement failed; original configuration retained")
	require.Equal(t, SaveFailureWrite, SaveFailureReasonOf(err))
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

// A backup that fails at any of its writes no longer stops the save. The
// interrupted backup state stays recoverable for the following save.
func TestCommitContinuesWhenBackupWritesFail(t *testing.T) {
	withApplicationVersion(t, "2.0.0")
	for _, upgrading := range []bool{false, true} {
		for phase := 1; phase <= 4; phase++ {
			t.Run(fmt.Sprintf("upgrade=%v/write=%d", upgrading, phase), func(t *testing.T) {
				warnings := captureBackupWarnings(t)
				t.Chdir(t.TempDir())
				original := []byte("# user comment\nTitle: original\nCustom: secret\n")
				proposed := []byte("Title: updated\nCustom: secret\n")
				next := []byte("Title: updated again\nCustom: secret\n")
				require.NoError(t, os.WriteFile("config.yml", original, 0640))
				require.NoError(t, os.WriteFile("personal.bak", []byte("user data"), 0600))
				writes := 0
				backup := func(raw []byte, upgrade bool) error {
					return retainSearchBackupWithWriter(raw, upgrade, func(path string, contents []byte) bool {
						writes++
						return writes != phase && saveConfigAtomically(path, contents)
					})
				}
				require.NoError(t, commitConfigDocumentWithBackup("config.yml", original, proposed, upgrading, backup))
				got, err := os.ReadFile("config.yml")
				require.NoError(t, err)
				require.Equal(t, proposed, got)
				require.Len(t, *warnings, 1)
				require.Contains(t, (*warnings)[0].reason, "cannot")
				require.Contains(t, (*warnings)[0].action, "without a backup")
				require.Empty(t, setAsideBackups(t), "an interrupted backup is continued, not set aside")

				require.NoError(t, commitConfigDocument("config.yml", proposed, next, false))
				got, err = os.ReadFile("config.yml")
				require.NoError(t, err)
				require.Equal(t, next, got)
				require.Len(t, *warnings, 1)
				require.Empty(t, setAsideBackups(t))
				got, err = os.ReadFile(filepath.Join(searchBackupDir, "latest.bak"))
				require.NoError(t, err)
				require.Equal(t, proposed, got)
				if upgrading && phase == 4 {
					// The permanent backup was written before its manifest update failed.
					got, err = os.ReadFile(filepath.Join(searchBackupDir, "first-upgrade-to-2.0.0.bak"))
					require.NoError(t, err)
					require.Equal(t, original, got)
				}
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
	t.Run("backup error does not block the save", func(t *testing.T) {
		warnings := captureBackupWarnings(t)
		t.Chdir(t.TempDir())
		raw := []byte("Title: original\n")
		require.NoError(t, os.WriteFile("config.yml", raw, 0600))
		failure := errors.New("disk full")
		backup := func(_ []byte, _ bool) error { return failure }
		require.NoError(t, commitConfigDocumentWithBackup("config.yml", raw, []byte("proposed"), true, backup))
		saved, err := os.ReadFile("config.yml")
		require.NoError(t, err)
		require.Equal(t, "proposed", string(saved))
		require.NoError(t, commitConfigDocumentWithBackup("config.yml", saved, []byte("proposed again"), false, backup))
		require.Equal(t, []backupWarning{{absoluteBackupPath(searchBackupDir), "disk full", "continuing the save without a backup"}}, *warnings)
	})
}
