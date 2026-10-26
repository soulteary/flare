package data

import (
	"os"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func TestClockSettingsPersistenceAndValidation(t *testing.T) {
	t.Chdir(t.TempDir())
	invalidateFileCache("config")
	defer invalidateFileCache("config")
	no := false
	update := model.Application{Title: "original", Locale: "en", Clock: &model.ClockOptions{Dynamic: true, Seconds: &no, TimezoneMode: "custom", Timezone: "America/New_York"}}
	if !UpdateAppearance(update) {
		t.Fatal("save failed")
	}
	if !UpdateWeatherAndLocation(true, "London") {
		t.Fatal("weather save failed")
	}
	before, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	update.Title = "should not save"
	update.Clock.Timezone = "bad"
	if UpdateAppearance(update) {
		t.Fatal("invalid zone saved")
	}
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	if string(before) != string(after) {
		t.Fatal("validation failure modified file")
	}
	if !UpdateAppearance(model.Application{Title: "legacy form"}) {
		t.Fatal("legacy save failed")
	}
	got, err := GetAllSettingsOptions()
	require.NoError(t, err)
	require.NotNil(t, got.Clock)
	require.False(t, got.Clock.ShowSeconds())
	require.True(t, got.Clock.Dynamic)
	require.Equal(t, "America/New_York", got.Clock.Timezone)
	require.Equal(t, "London", got.Location)
	require.NoError(t, os.Mkdir("blocked", 0700))
	if saveConfigAtomically("blocked", []byte("x")) {
		t.Fatal("overwrote directory")
	}
	files, err := os.ReadDir(".")
	require.NoError(t, err)
	for _, f := range files {
		if f.Name() != "config.yml" && f.Name() != "blocked" && f.Name() != searchBackupDir {
			t.Fatalf("temporary file leaked: %s", f.Name())
		}
	}
}
