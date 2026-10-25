package data

import (
	"github.com/soulteary/flare/config/model"
	"os"
	"testing"
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
	if err != nil {
		t.Fatal(err)
	}
	update.Title = "should not save"
	update.Clock.Timezone = "bad"
	if UpdateAppearance(update) {
		t.Fatal("invalid zone saved")
	}
	after, err := os.ReadFile("config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("validation failure modified file")
	}
	if !UpdateAppearance(model.Application{Title: "legacy form"}) {
		t.Fatal("legacy save failed")
	}
	got, err := GetAllSettingsOptions()
	if err != nil {
		t.Fatal(err)
	}
	if got.Clock == nil || got.Clock.ShowSeconds() || !got.Clock.Dynamic || got.Clock.Timezone != "America/New_York" || got.Location != "London" {
		t.Fatalf("settings lost: %+v", got)
	}
	if mkdirErr := os.Mkdir("blocked", 0700); mkdirErr != nil {
		t.Fatal(mkdirErr)
	}
	if saveConfigAtomically("blocked", []byte("x")) {
		t.Fatal("overwrote directory")
	}
	files, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Name() != "config.yml" && f.Name() != "blocked" && f.Name() != searchBackupDir {
			t.Fatalf("temporary file leaked: %s", f.Name())
		}
	}
}
