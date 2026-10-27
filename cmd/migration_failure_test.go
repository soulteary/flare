package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPrivateJSONWriteFailuresPreserveDestination(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "installation.json")
	require.NoError(t, os.WriteFile(dest, []byte("original"), 0600))
	require.Error(t, writePrivateJSON(dest, make(chan int)))
	raw, err := os.ReadFile(dest)
	require.NoError(t, err)
	require.Equal(t, "original", string(raw))
	require.Error(t, writePrivateJSON(filepath.Join(dir, "missing", "state.json"), map[string]int{"version": 1}))
	require.Error(t, writePrivateJSON(dir, map[string]int{"version": 1}))
	matches, err := filepath.Glob(filepath.Join(dir, ".security-write-*"))
	require.NoError(t, err)
	require.Empty(t, matches)
}

func TestMigrationRejectsDamagedBackupArtifacts(t *testing.T) {
	for _, tc := range []struct {
		name  string
		setup func(*testing.T, string)
	}{
		{"corrupt manifest", func(t *testing.T, path string) {
			require.NoError(t, os.WriteFile(filepath.Join(path, "manifest.json"), []byte("{"), 0600))
		}},
		{"manifest directory", func(t *testing.T, path string) {
			require.NoError(t, os.Mkdir(filepath.Join(path, "manifest.json"), 0700))
		}},
		{"backup directory instead of file", func(t *testing.T, path string) { require.NoError(t, os.Mkdir(filepath.Join(path, "apps.yml"), 0700)) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile("apps.yml", []byte("links: []\n"), 0600))
			dir := t.TempDir()
			path := filepath.Join(dir, "migration-backups", "initial")
			require.NoError(t, os.MkdirAll(path, 0700))
			tc.setup(t, path)
			require.Error(t, migrationSnapshot(dir, map[string][]byte{"apps.yml": []byte("links: []\n")}))
			raw, err := os.ReadFile("apps.yml")
			require.NoError(t, err)
			require.Equal(t, "links: []\n", string(raw))
		})
	}
	t.Run("blocked backup parent", func(t *testing.T) {
		t.Chdir(t.TempDir())
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "migration-backups"), []byte("blocked"), 0600))
		require.Error(t, migrationSnapshot(dir, nil))
	})
	t.Run("unreadable legacy secret", func(t *testing.T) {
		t.Chdir(t.TempDir())
		require.NoError(t, os.Mkdir(".flare-cookie-secret", 0700))
		require.Error(t, migrationSnapshot(t.TempDir(), nil))
	})
}
