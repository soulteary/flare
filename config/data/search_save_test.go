package data

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/soulteary/flare/internal/search"
	"github.com/stretchr/testify/require"
	yaml "go.yaml.in/yaml/v3"
)

func TestSearchUpgradeSaveProtectsOriginalData(t *testing.T) {
	t.Chdir(t.TempDir())
	invalidateFileCache("config")
	defer invalidateFileCache("config")
	raw := []byte(`# retain this comment
Title: My original title
UnknownExtension:
  Secret: keep-exact-value
ShowSearchComponent: true
Search:
  Mode: hybrid
  DefaultTarget: local
  UnknownSearch: retain
  Engines:
    - ID: custom
      Name: My engine
      URLTemplate: "https://custom.example/?q={query}"
      Keyword: c
      Enabled: true
      UnknownEngine: retain-engine
`)
	require.NoError(t, os.WriteFile("config.yml", raw, 0640))
	for _, name := range []string{"apps.yml", "bookmarks.yml"} {
		require.NoError(t, os.WriteFile(name, []byte("# untouched user data\n"), 0600))
	}
	settings, err := GetAllSettingsOptions()
	require.NoError(t, err)
	upgraded, changed := search.Upgrade(settings.Search)
	require.True(t, changed)
	require.Equal(t, "local-first", upgraded.Mode)
	untouched, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, raw, untouched)
	require.NoError(t, UpdateSearchOptions(true, true, &upgraded))
	backups, err := filepath.Glob(".flare-search-backups/first-upgrade-to-dev.bak")
	require.NoError(t, err)
	require.Len(t, backups, 1)
	backup, err := os.ReadFile(backups[0])
	require.NoError(t, err)
	require.Equal(t, raw, backup)
	info, err := os.Stat(backups[0])
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	saved, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.True(t, bytes.Contains(saved, []byte("retain this comment")))
	var doc map[string]any
	require.NoError(t, yaml.Unmarshal(saved, &doc))
	require.Equal(t, "My original title", doc["Title"])
	require.Equal(t, "keep-exact-value", doc["UnknownExtension"].(map[string]any)["Secret"])
	block := doc["Search"].(map[string]any)
	require.Equal(t, "retain", block["UnknownSearch"])
	engine := block["Engines"].([]any)[0].(map[string]any)
	require.Equal(t, "retain-engine", engine["UnknownEngine"])
	require.Equal(t, "https://custom.example/?q={query}", engine["URLTemplate"])
	for _, name := range []string{"apps.yml", "bookmarks.yml"} {
		got, readErr := os.ReadFile(name)
		require.NoError(t, readErr)
		require.Equal(t, "# untouched user data\n", string(got))
	}
	info, err = os.Stat("config.yml")
	require.NoError(t, err)
	require.Equal(t, os.FileMode(0640), info.Mode().Perm())
}

func TestSearchSaveRefusesAmbiguousDocument(t *testing.T) {
	t.Chdir(t.TempDir())
	invalidateFileCache("config")
	defer invalidateFileCache("config")
	raw := []byte("Title: first\nTitle: duplicate\n")
	require.NoError(t, os.WriteFile("config.yml", raw, 0600))
	require.Error(t, UpdateSearchOptions(true, false, nil))
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, raw, after)
}

func TestSearchBackupRetentionAndNoOp(t *testing.T) {
	t.Chdir(t.TempDir())
	invalidateFileCache("config")
	defer invalidateFileCache("config")
	raw := []byte("# original formatting\nTitle: retained\nShowSearchComponent: true\nDisabledSearchAutoFocus: false\n")
	require.NoError(t, os.WriteFile("config.yml", raw, 0600))
	// Even a legacy document must not be rewritten by an unchanged save.
	require.NoError(t, UpdateSearchOptions(true, false, nil))
	untouched, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, raw, untouched)
	_, err = os.Stat(searchBackupDir)
	require.True(t, os.IsNotExist(err))
	userFiles := []string{"config.yml.pre-search-user.bak", "config.yml.pre-search-123.bak", "my-config.bak"}
	for _, name := range userFiles {
		require.NoError(t, os.WriteFile(name, []byte("user-owned"), 0600))
	}
	opts := search.Normalize(nil)
	require.NoError(t, UpdateSearchOptions(false, false, &opts))
	first, err := os.ReadFile(filepath.Join(searchBackupDir, "first-upgrade-to-dev.bak"))
	require.NoError(t, err)
	require.Equal(t, raw, first)
	for _, show := range []bool{true, false, true} {
		before, readErr := os.ReadFile("config.yml")
		require.NoError(t, readErr)
		require.NoError(t, UpdateSearchOptions(show, false, &opts))
		latest, readErr := os.ReadFile(filepath.Join(searchBackupDir, "latest.bak"))
		require.NoError(t, readErr)
		require.Equal(t, before, latest)
		permanent, readErr := os.ReadFile(filepath.Join(searchBackupDir, "first-upgrade-to-dev.bak"))
		require.NoError(t, readErr)
		require.Equal(t, raw, permanent)
	}
	before, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	configInfo, err := os.Stat("config.yml")
	require.NoError(t, err)
	latestPath := filepath.Join(searchBackupDir, "latest.bak")
	latestBefore, err := os.ReadFile(latestPath)
	require.NoError(t, err)
	info, err := os.Stat(latestPath)
	require.NoError(t, err)
	require.NoError(t, UpdateSearchOptions(true, false, &opts))
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, before, after)
	configAfter, err := os.Stat("config.yml")
	require.NoError(t, err)
	require.True(t, os.SameFile(configInfo, configAfter))
	latestInfo, err := os.Stat(latestPath)
	require.NoError(t, err)
	require.True(t, os.SameFile(info, latestInfo))
	files, err := filepath.Glob(filepath.Join(searchBackupDir, "*.bak"))
	require.NoError(t, err)
	require.Len(t, files, 2)
	for _, name := range userFiles {
		data, readErr := os.ReadFile(name)
		require.NoError(t, readErr)
		require.Equal(t, "user-owned", string(data))
	}
	// A file changed by the user ceases to be replaceable by rotation.
	require.NoError(t, os.WriteFile(latestPath, []byte("user replacement"), 0600))
	require.Error(t, UpdateSearchOptions(false, false, &opts))
	after, err = os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, before, after)
	userCopy, err := os.ReadFile(latestPath)
	require.NoError(t, err)
	require.Equal(t, "user replacement", string(userCopy))
	require.NotEqual(t, string(latestBefore), string(userCopy))
}

func TestSearchBackupRefusesUnownedDirectory(t *testing.T) {
	t.Chdir(t.TempDir())
	invalidateFileCache("config")
	defer invalidateFileCache("config")
	raw := []byte("ShowSearchComponent: true\n")
	require.NoError(t, os.WriteFile("config.yml", raw, 0600))
	require.NoError(t, os.Mkdir(searchBackupDir, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(searchBackupDir, "latest.bak"), []byte("my backup"), 0600))
	require.Error(t, UpdateSearchOptions(false, false, nil))
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, raw, after)
	mine, err := os.ReadFile(filepath.Join(searchBackupDir, "latest.bak"))
	require.NoError(t, err)
	require.Equal(t, "my backup", string(mine))
}

func TestSearchBackupRefusesSymlinkReplacement(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, retainSearchBackup([]byte("initial"), false))
	require.NoError(t, os.WriteFile("user.bak", []byte("precious"), 0600))
	require.NoError(t, os.Remove(filepath.Join(searchBackupDir, "latest.bak")))
	require.NoError(t, os.Symlink("../user.bak", filepath.Join(searchBackupDir, "latest.bak")))
	require.Error(t, retainSearchBackup([]byte("replacement"), false))
	got, err := os.ReadFile("user.bak")
	require.NoError(t, err)
	require.Equal(t, "precious", string(got))
}
