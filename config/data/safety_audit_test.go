package data

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func TestAllSettingsUpgradePreservesExtensions(t *testing.T) {
	withApplicationVersion(t, "2.0.0")
	t.Chdir(t.TempDir())
	raw := []byte("# user comment\nAppVersion: 1.0.0\nTitle: personal\nCustom: {private: keep}\nSearch:\n  Mode: select\n  DefaultTarget: local\n  CustomSearch: keep\n  Engines:\n    - ID: my-engine\n      Name: Personal\n      URLTemplate: 'https://example.com/?q={query}'\n      Enabled: true\n      CustomEngine: keep\n")
	require.NoError(t, os.WriteFile("config.yml", raw, 0600))
	require.True(t, UpdateThemeName("blackboard"))
	saved, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	for _, text := range []string{"user comment", "private: keep", "CustomSearch: keep", "CustomEngine: keep", "AppVersion: 2.0.0"} {
		require.Contains(t, string(saved), text)
	}
	first, err := os.ReadFile(filepath.Join(searchBackupDir, "first-upgrade-to-2.0.0.bak"))
	require.NoError(t, err)
	require.Equal(t, raw, first)
	// A manual upgrade must become visible even after a previous read warmed caches.
	require.NoError(t, os.WriteFile("config.yml", []byte("AppVersion: 3.0.0\nTitle: externally-upgraded\n"), 0600))
	opts, err := GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "downgrade", opts.ConfigVersionState)
	require.False(t, UpdateWeatherAndLocation(true, "must-not-write"))
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, "AppVersion: 3.0.0\nTitle: externally-upgraded\n", string(after))
}

func TestAmbiguousConfigurationNeverOverwritten(t *testing.T) {
	for _, raw := range []string{
		"Title: first\n---\nTitle: second\n", "Title: one\nTitle: two\n",
		"Title: first\n---\n[invalid\n",
	} {
		t.Run(raw, func(t *testing.T) {
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile("config.yml", []byte(raw), 0600))
			_, err := GetAllSettingsOptions()
			require.Error(t, err)
			require.False(t, UpdateThemeName("changed"))
			require.Error(t, UpdateSearchOptions(true, false, nil))
			after, err := os.ReadFile("config.yml")
			require.NoError(t, err)
			require.Equal(t, raw, string(after))
			_, err = os.Stat(searchBackupDir)
			require.True(t, os.IsNotExist(err))
		})
	}
}

func TestAliasesAndSymlinksFailClosedOnChangedSave(t *testing.T) {
	t.Run("shared alias", func(t *testing.T) {
		t.Chdir(t.TempDir())
		raw := []byte("ShowSearchComponent: true\nSearch: &shared\n  Mode: select\n  DefaultTarget: local\nOtherData: *shared\n")
		require.NoError(t, os.WriteFile("config.yml", raw, 0600))
		require.Error(t, UpdateSearchOptions(false, false, nil))
		require.False(t, UpdateThemeName("changed"))
		after, err := os.ReadFile("config.yml")
		require.NoError(t, err)
		require.Equal(t, raw, after)
	})
	for _, dangling := range []bool{true, false} {
		t.Run(fmt.Sprint("symlink-", dangling), func(t *testing.T) {
			t.Chdir(t.TempDir())
			raw := []byte("Title: target\nShowSearchComponent: true\n")
			if !dangling {
				require.NoError(t, os.WriteFile("target.yml", raw, 0600))
			}
			require.NoError(t, os.Symlink("target.yml", "config.yml"))
			require.False(t, UpdateThemeName("changed"))
			require.Error(t, UpdateSearchOptions(false, false, nil))
			info, err := os.Lstat("config.yml")
			require.NoError(t, err)
			require.NotZero(t, info.Mode()&os.ModeSymlink)
			after, err := os.ReadFile("target.yml")
			if dangling {
				require.True(t, os.IsNotExist(err))
			} else {
				require.NoError(t, err)
				require.Equal(t, raw, after)
			}
		})
	}
}

func TestBackupInterruptionRecovery(t *testing.T) {
	for _, upgrading := range []bool{false, true} {
		for phase := 1; phase <= 4; phase++ {
			t.Run(fmt.Sprintf("upgrade=%v/phase=%d", upgrading, phase), func(t *testing.T) {
				t.Chdir(t.TempDir())
				if !upgrading {
					require.NoError(t, retainSearchBackup([]byte("older"), false))
				}
				calls := 0
				interrupted := func(path string, raw []byte) bool {
					calls++
					if calls == phase {
						return false
					}
					return saveConfigAtomically(path, raw)
				}
				err := retainSearchBackupWithWriter([]byte("before-save"), upgrading, interrupted)
				// Initial upgrade takes 4 writes, existing rotation takes 3.
				if phase <= 3 || upgrading {
					require.Error(t, err)
				}
				require.NoError(t, retainSearchBackup([]byte("before-save"), upgrading))
				name := "latest.bak"
				if upgrading {
					name = "first-upgrade-to-" + applicationVersion() + ".bak"
				}
				got, err := os.ReadFile(filepath.Join(searchBackupDir, name))
				require.NoError(t, err)
				require.Equal(t, "before-save", string(got))
				manifest, err := os.ReadFile(filepath.Join(searchBackupDir, "manifest.json"))
				require.NoError(t, err)
				var state searchBackupManifest
				require.NoError(t, json.Unmarshal(manifest, &state))
				require.Nil(t, state.Pending)
				files, err := filepath.Glob(filepath.Join(searchBackupDir, "*.bak"))
				require.NoError(t, err)
				if upgrading {
					require.LessOrEqual(t, len(files), 2)
				} else {
					require.Len(t, files, 1)
				}
			})
		}
	}
}

func TestConcurrentDiskEditStopsCommit(t *testing.T) {
	t.Chdir(t.TempDir())
	original := []byte("Title: original\n")
	competing := []byte("Title: external-change\n")
	require.NoError(t, os.WriteFile("config.yml", original, 0600))
	err := commitConfigDocumentWithBackup("config.yml", original, []byte("Title: proposed\n"), true, func(raw []byte, upgrading bool) error {
		require.Equal(t, original, raw)
		require.NoError(t, retainSearchBackup(raw, upgrading))
		return os.WriteFile("config.yml", competing, 0600)
	})
	require.Error(t, err)
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, competing, after)
}

func TestBackupCollisionAndUnknownManifestAreNotOverwritten(t *testing.T) {
	captureBackupWarnings(t)
	t.Chdir(t.TempDir())
	require.NoError(t, retainSearchBackup([]byte("initial"), false))
	upgradeName := "first-upgrade-to-" + applicationVersion() + ".bak"
	require.NoError(t, os.WriteFile(filepath.Join(searchBackupDir, upgradeName), []byte("user-backup"), 0600))
	// A user file under the managed name is never overwritten: its directory is
	// set aside and the upgrade backup goes into a new one.
	require.NoError(t, retainSearchBackup([]byte("upgrade"), true))
	moved := setAsideBackups(t)
	require.Len(t, moved, 1)
	mine, err := os.ReadFile(filepath.Join(moved[0], upgradeName))
	require.NoError(t, err)
	require.Equal(t, "user-backup", string(mine))
	latest, err := os.ReadFile(filepath.Join(moved[0], "latest.bak"))
	require.NoError(t, err)
	require.Equal(t, "initial", string(latest))
	managed, err := os.ReadFile(filepath.Join(searchBackupDir, upgradeName))
	require.NoError(t, err)
	require.Equal(t, "upgrade", string(managed))

	manifest := filepath.Join(searchBackupDir, "manifest.json")
	raw, err := os.ReadFile(manifest)
	require.NoError(t, err)
	tampered := append(raw, []byte(" {}")...)
	require.NoError(t, os.WriteFile(manifest, tampered, 0600))
	require.NoError(t, retainSearchBackup([]byte("next"), false))
	all := setAsideBackups(t)
	require.Len(t, all, 2)
	for _, dir := range all {
		if dir == moved[0] {
			continue
		}
		kept, readErr := os.ReadFile(filepath.Join(dir, "manifest.json"))
		require.NoError(t, readErr)
		require.Equal(t, tampered, kept)
		kept, readErr = os.ReadFile(filepath.Join(dir, upgradeName))
		require.NoError(t, readErr)
		require.Equal(t, "upgrade", string(kept))
	}
	latest, err = os.ReadFile(filepath.Join(searchBackupDir, "latest.bak"))
	require.NoError(t, err)
	require.Equal(t, "next", string(latest))
}

func TestEngineReorderRetainsExtensionsAndAmbiguousRenameAborts(t *testing.T) {
	t.Chdir(t.TempDir())
	raw := []byte("ShowSearchComponent: true\nSearch:\n  Mode: select\n  DefaultTarget: local\n  Engines:\n    - ID: a\n      Name: A\n      URLTemplate: 'https://a.example/?q={query}'\n      Enabled: true\n      Extension: secret-a\n    - ID: b\n      Name: B\n      URLTemplate: 'https://b.example/?q={query}'\n      Enabled: true\n      Extension: secret-b\n")
	require.NoError(t, os.WriteFile("config.yml", raw, 0600))
	opts, err := GetAllSettingsOptions()
	require.NoError(t, err)
	opts.Search.Engines[0], opts.Search.Engines[1] = opts.Search.Engines[1], opts.Search.Engines[0]
	require.NoError(t, UpdateSearchOptions(true, false, opts.Search))
	saved, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	doc, _, err := readConfigDocument(saved)
	require.NoError(t, err)
	engines := mappingValue(mappingValue(doc.Content[0], "Search"), "Engines")
	require.Equal(t, "b", mappingValue(engines.Content[0], "ID").Value)
	require.Equal(t, "secret-b", mappingValue(engines.Content[0], "Extension").Value)
	opts.Search.Engines[0].ID = "renamed"
	require.Error(t, UpdateSearchOptions(true, false, opts.Search))
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, saved, after)
}

func TestBackupRepairsPublicPermissions(t *testing.T) {
	warnings := captureBackupWarnings(t)
	t.Chdir(t.TempDir())
	require.NoError(t, retainSearchBackup([]byte("initial"), false))
	p := filepath.Join(searchBackupDir, "latest.bak")
	require.NoError(t, os.Chmod(p, 0644))
	require.NoError(t, os.Chmod(searchBackupDir, 0755))
	require.NoError(t, retainSearchBackup([]byte("next"), false))
	got, err := os.ReadFile(p)
	require.NoError(t, err)
	require.Equal(t, "next", string(got))
	info, err := os.Stat(p)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	info, err = os.Stat(searchBackupDir)
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0700), info.Mode().Perm())
	require.Empty(t, *warnings)
	require.Empty(t, setAsideBackups(t))
}

func TestStaleSettingsCannotReplaceExternalEdits(t *testing.T) {
	t.Chdir(t.TempDir())
	raw := []byte("Title: original\nShowSearchComponent: true\n")
	require.NoError(t, os.WriteFile("config.yml", raw, 0600))
	loaded, err := GetAllSettingsOptions()
	require.NoError(t, err)
	require.NotEmpty(t, loaded.ConfigRevision)
	external := []byte("Title: updated externally\nShowSearchComponent: true\n")
	require.NoError(t, os.WriteFile("config.yml", external, 0600))
	loaded.Theme = "changed"
	require.False(t, saveAppConfigToYamlFile("config", loaded))
	// A stale browser form supplies the revision it actually displayed.
	require.Error(t, UpdateSearchOptions(false, false, nil, loaded.ConfigRevision))
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, external, after)
	_, err = os.Stat(searchBackupDir)
	require.True(t, os.IsNotExist(err))
}

func TestAppearanceResetsPersistWithoutDroppingExtensions(t *testing.T) {
	t.Chdir(t.TempDir())
	raw := []byte("Title: original\nHomepage:\n  DesktopColumns: '8'\n  CategoryNavigation: tabs\n  MobileBookmarkLayout: compact\n  NavigationTextSize: large\n  TitleSize: prominent\n  HideAppDescriptions: true\n  CustomAppearance: retain\nClock:\n  Dynamic: true\n  Seconds: false\n  TimezoneMode: server\n  CustomClock: retain\n")
	require.NoError(t, os.WriteFile("config.yml", raw, 0600))
	options, err := GetAllSettingsOptions()
	require.NoError(t, err)
	options.Homepage = &model.HomepageOptions{}
	options.Clock.Seconds = nil
	require.True(t, UpdateAppearance(options))
	saved, err := GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, &model.HomepageOptions{}, saved.Homepage)
	require.True(t, saved.Clock.ShowSeconds())
	contents, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	doc, _, err := readConfigDocument(contents)
	require.NoError(t, err)
	require.Equal(t, "retain", mappingValue(mappingValue(doc.Content[0], "Homepage"), "CustomAppearance").Value)
	require.Equal(t, "retain", mappingValue(mappingValue(doc.Content[0], "Clock"), "CustomClock").Value)
	// The reset itself must be stable: saving it again is a no-op.
	info, err := os.Stat("config.yml")
	require.NoError(t, err)
	require.True(t, UpdateAppearance(saved))
	after, err := os.Stat("config.yml")
	require.NoError(t, err)
	require.True(t, os.SameFile(info, after))
}

func TestRetiredEngineKeywordDoesNotBlockReplacement(t *testing.T) {
	for _, replacement := range []string{"rename", "replace", "extension"} {
		t.Run(replacement, func(t *testing.T) {
			t.Chdir(t.TempDir())
			raw := "ShowSearchComponent: true\nSearch:\n  Mode: select\n  DefaultTarget: local\n  EnableKeywords: true\n  Engines:\n    - ID: old\n      Name: Old\n      URLTemplate: 'https://old.example/?q={query}'\n      Keyword: old\n      Enabled: true\n"
			if replacement == "extension" {
				raw += "      Extension: keep-me\n"
			}
			require.NoError(t, os.WriteFile("config.yml", []byte(raw), 0600))
			opts, err := GetAllSettingsOptions()
			require.NoError(t, err)
			// A normal save retains unknown YAML fields, including the retired Keyword.
			opts.Search.OpenNewTab = true
			require.NoError(t, UpdateSearchOptions(true, false, opts.Search))
			before, err := os.ReadFile("config.yml")
			require.NoError(t, err)
			require.Contains(t, string(before), "Keyword: old")
			if replacement == "replace" {
				opts.Search.Engines = []model.SearchEngine{{ID: "new", Name: "New", URLTemplate: "https://new.example/?q={query}", Enabled: true}}
			} else {
				opts.Search.Engines[0].ID = "new"
			}
			err = UpdateSearchOptions(true, false, opts.Search)
			if replacement == "extension" {
				require.ErrorContains(t, err, "extension fields")
				after, readErr := os.ReadFile("config.yml")
				require.NoError(t, readErr)
				require.Equal(t, before, after)
				return
			}
			require.NoError(t, err)
			saved, err := GetAllSettingsOptions()
			require.NoError(t, err)
			require.Len(t, saved.Search.Engines, 1)
			require.Equal(t, "new", saved.Search.Engines[0].ID)
		})
	}
}
