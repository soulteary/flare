package data

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/flare/config/model"
	version "github.com/soulteary/version-kit/v4"
	"github.com/stretchr/testify/require"
)

func withApplicationVersion(t *testing.T, v string) {
	t.Helper()
	old := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = old })
}

func TestConfigurationVersionComparison(t *testing.T) {
	withApplicationVersion(t, "1.10.0")
	for _, tc := range []struct{ stored, want string }{
		{"", "legacy"}, {"1.2.9", "upgrade"}, {"v1.10.0", "same"}, {"1.10.0+build.1", "same"}, {"1.11.0", "downgrade"}, {"2.0.0", "downgrade"}, {"1.10.0-rc.2", "upgrade"}, {"dev", "unknown"}, {"invalid", "unknown"}, {"1.10", "unknown"}, {"1.10.0-01", "unknown"},
	} {
		require.Equal(t, tc.want, ConfigurationVersionState(tc.stored), tc.stored)
	}
	version.Version = "1.10.0-rc.2"
	require.Equal(t, "upgrade", ConfigurationVersionState("1.10.0-rc.1"))
	require.Equal(t, "downgrade", ConfigurationVersionState("1.10.0-rc.10"))
	require.Equal(t, "downgrade", ConfigurationVersionState("1.10.0"))
}

func TestVersionedUpgradeAndDowngradeProtection(t *testing.T) {
	withApplicationVersion(t, "1.10.0")
	t.Chdir(t.TempDir())
	invalidateFileCache("config")
	defer invalidateFileCache("config")
	raw := []byte("AppVersion: 1.2.0\nTitle: retained\nShowSearchComponent: true\n")
	require.NoError(t, os.WriteFile("config.yml", raw, 0600))
	opts, err := GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "upgrade", opts.ConfigVersionState)
	// Merely reading or saving identical settings leaves the version and file untouched.
	require.NoError(t, UpdateSearchOptions(true, false, nil))
	data, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, raw, data)
	require.NoError(t, UpdateSearchOptions(false, false, nil))
	backupPath := filepath.Join(searchBackupDir, "first-upgrade-to-1.10.0.bak")
	backup, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	require.Equal(t, raw, backup)
	manifest, err := os.ReadFile(filepath.Join(searchBackupDir, "manifest.json"))
	require.NoError(t, err)
	var state searchBackupManifest
	require.NoError(t, json.Unmarshal(manifest, &state))
	require.Equal(t, "1.10.0", state.UpgradeVersion)
	opts, err = GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "1.10.0", opts.AppVersion)
	require.Equal(t, "same", opts.ConfigVersionState)
	newer, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	version.Version = "1.9.0"
	opts, err = GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "downgrade", opts.ConfigVersionState)
	require.Error(t, UpdateSearchOptions(true, false, nil))
	require.False(t, UpdateThemeName("new-theme"))
	unchanged, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, newer, unchanged)
	// A development build cannot claim to be newer than a release.
	version.Version = "dev"
	require.NoError(t, UpdateSearchOptions(true, false, nil))
	opts, err = GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "1.10.0", opts.AppVersion)
	require.Equal(t, "unknown", opts.ConfigVersionState)
	first, err := os.ReadFile(backupPath)
	require.NoError(t, err)
	require.Equal(t, raw, first)
}

func TestNewConfigurationRecordsVersion(t *testing.T) {
	withApplicationVersion(t, "2.3.4")
	t.Chdir(t.TempDir())
	invalidateFileCache("config")
	defer invalidateFileCache("config")
	opts, err := GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "2.3.4", opts.AppVersion)
	require.Equal(t, "same", opts.ConfigVersionState)
	require.True(t, saveAppConfigToYamlFile("config", model.Application{Title: "saved"}))
	opts, err = GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "2.3.4", opts.AppVersion)
}

func TestPreviousPermanentBackupIsRetained(t *testing.T) {
	withApplicationVersion(t, "1.10.0")
	t.Chdir(t.TempDir())
	require.NoError(t, os.Mkdir(searchBackupDir, 0700))
	raw := []byte("old original")
	path := filepath.Join(searchBackupDir, "first-upgrade.bak")
	require.NoError(t, os.WriteFile(path, raw, 0600))
	state := searchBackupManifest{Owner: "flare-search-backups-v1", Upgrade: backupDigest(raw)}
	manifest, err := json.Marshal(state)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(searchBackupDir, "manifest.json"), manifest, 0600))
	require.NoError(t, retainSearchBackup([]byte("new change"), true))
	first, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, raw, first)
	_, err = os.Stat(filepath.Join(searchBackupDir, "first-upgrade-to-1.10.0.bak"))
	require.True(t, os.IsNotExist(err))
	latest, err := os.ReadFile(filepath.Join(searchBackupDir, "latest.bak"))
	require.NoError(t, err)
	require.Equal(t, "new change", string(latest))
}
