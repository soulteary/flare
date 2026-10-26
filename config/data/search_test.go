package data

import (
	"os"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/search"
	"github.com/stretchr/testify/require"
)

func TestSearchSettingsPreserveOtherSettingsAndRejectInvalidChanges(t *testing.T) {
	t.Chdir(t.TempDir())
	invalidateFileCache("config")
	defer invalidateFileCache("config")
	original := model.Application{Title: "unchanged", Locale: "en", Clock: &model.ClockOptions{Dynamic: true, TimezoneMode: "server"}, Location: "London"}
	if !saveAppConfigToYamlFile("config", original) {
		t.Fatal("seed failed")
	}
	options := model.SearchOptions{DefaultTarget: "bing", Engines: search.Presets()}
	require.NoError(t, UpdateSearchOptions(true, true, &options))
	got, err := GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "unchanged", got.Title)
	require.Equal(t, "en", got.Locale)
	require.Equal(t, "London", got.Location)
	require.NotNil(t, got.Clock)
	require.True(t, got.Clock.Dynamic)
	require.NotNil(t, got.Search)
	require.Equal(t, "bing", got.Search.DefaultTarget)
	if !UpdateSearch(false, false) {
		t.Fatal("legacy update failed")
	}
	got, err = GetAllSettingsOptions()
	if err != nil || got.Search == nil || got.Search.DefaultTarget != "bing" {
		t.Fatal("legacy form removed engines", err)
	}
	before, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	options.DefaultTarget = "missing"
	if updateErr := UpdateSearchOptions(true, true, &options); updateErr == nil {
		t.Fatal("invalid change saved")
	}
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	if string(before) != string(after) {
		t.Fatal("invalid change modified file")
	}
	// A malformed backing file must not be overwritten by an apparently successful save.
	require.NoError(t, os.WriteFile("config.yml", []byte("Search: ["), 0600))
	invalidateFileCache("config")
	if updateErr := UpdateSearchOptions(true, true, nil); updateErr == nil {
		t.Fatal("failed read was not reported")
	}
}
