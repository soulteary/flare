package data

import (
	"os"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func TestDowngradeBlocksEverySettingsWriterWithoutSideEffects(t *testing.T) {
	for _, writer := range []struct {
		name string
		save func() bool
	}{
		{"theme", func() bool { return UpdateThemeName("blackboard") }},
		{"weather", func() bool { return UpdateWeatherAndLocation(true, "Beijing") }},
		{"location", func() bool { return UpdateLocation("Shanghai") }},
		{"search", func() bool { return UpdateSearch(false, true) }},
		{"search options", func() bool { return UpdateSearchOptions(false, true, nil) == nil }},
		{"appearance", func() bool { return UpdateAppearance(model.Application{Title: "changed"}) }},
		{"direct save", func() bool { return saveAppConfigToYamlFile("config", model.Application{Title: "changed"}) }},
	} {
		t.Run(writer.name, func(t *testing.T) {
			withApplicationVersion(t, "1.9.0")
			t.Chdir(t.TempDir())
			raw := []byte("# future config\nAppVersion: 2.0.0\nTitle: precious\nShowSearchComponent: true\nFuture: {secret: keep}\n")
			require.NoError(t, os.WriteFile("config.yml", raw, 0640))
			require.NoError(t, retainSearchBackup([]byte("older original"), true))
			for _, name := range []string{"apps.yml", "bookmarks.yml", "personal.bak"} {
				require.NoError(t, os.WriteFile(name, []byte("user-owned"), 0600))
			}
			before := snapshotConfigTree(t)
			options, err := GetAllSettingsOptions()
			require.NoError(t, err)
			require.Equal(t, "downgrade", options.ConfigVersionState)
			require.False(t, writer.save())
			require.Equal(t, before, snapshotConfigTree(t))
		})
	}
}

func TestVersionOrderingSemverEdgeCases(t *testing.T) {
	for _, tc := range []struct{ current, stored, want string }{
		{"1.0.0-alpha.1", "1.0.0-alpha", "upgrade"},
		{"1.0.0-alpha", "1.0.0-alpha.1", "downgrade"},
		{"1.0.0-beta", "1.0.0-alpha", "upgrade"},
		{"1.0.0-1", "1.0.0-alpha", "downgrade"},
		{"1.0.0-alpha", "1.0.0-1", "upgrade"},
		{"1.0.0-alpha.2", "1.0.0-alpha.1", "upgrade"},
		{"1.0.0-alpha.1", "1.0.0-alpha.2", "downgrade"},
		{"1.0.0+one", "1.0.0+two", "same"},
		{"1.0.0", "1.0.0-alpha", "upgrade"},
		{"1.0.0-alpha", "1.0.0", "downgrade"},
		{"1.0.0", "999999999999999999999999.0.0", "downgrade"},
		{"1.0.0", "1.0.0-alpha..1", "unknown"},
		{"1.0.0", "1.0.0+build..1", "unknown"},
		{"1.0.0", "1.0.0-00", "unknown"},
		{"  ", "1.0.0", "unknown"},
	} {
		t.Run(tc.current+"/"+tc.stored, func(t *testing.T) {
			withApplicationVersion(t, tc.current)
			require.Equal(t, tc.want, ConfigurationVersionState(tc.stored))
		})
	}
}

func TestUnknownVersionSavePreservesWriterAndUserExtensions(t *testing.T) {
	for _, tc := range []struct{ current, stored string }{
		{"dev", "3.0.0"}, {"2.0.0", "custom-user-build"}, {"", "3.0.0"},
	} {
		t.Run(tc.current+"/"+tc.stored, func(t *testing.T) {
			withApplicationVersion(t, tc.current)
			t.Chdir(t.TempDir())
			raw := []byte("# original\nAppVersion: " + tc.stored + "\nTitle: original\nCustom: secret\nSearch: {Mode: select, DefaultTarget: local}\n")
			require.NoError(t, os.WriteFile("config.yml", raw, 0600))
			require.True(t, UpdateThemeName("blackboard"))
			got, err := os.ReadFile("config.yml")
			require.NoError(t, err)
			require.Contains(t, string(got), "AppVersion: "+tc.stored)
			require.Contains(t, string(got), "# original")
			require.Contains(t, string(got), "Custom: secret")
			backedUp, err := os.ReadFile(searchBackupDir + "/latest.bak")
			require.NoError(t, err)
			require.Equal(t, raw, backedUp)
			before := snapshotConfigTree(t)
			require.True(t, UpdateThemeName("blackboard"))
			require.Equal(t, before, snapshotConfigTree(t))
		})
	}
}
