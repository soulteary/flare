package data

import (
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/search"
	"os"
	"testing"
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
	if err := UpdateSearchOptions(true, true, &options); err != nil {
		t.Fatal(err)
	}
	got, err := GetAllSettingsOptions()
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "unchanged" || got.Locale != "en" || got.Location != "London" || got.Clock == nil || !got.Clock.Dynamic || got.Search == nil || got.Search.DefaultTarget != "bing" {
		t.Fatalf("settings lost: %+v", got)
	}
	if !UpdateSearch(false, false) {
		t.Fatal("legacy update failed")
	}
	got, err = GetAllSettingsOptions()
	if err != nil || got.Search == nil || got.Search.DefaultTarget != "bing" {
		t.Fatal("legacy form removed engines", err)
	}
	before, err := os.ReadFile("config.yml")
	if err != nil {
		t.Fatal(err)
	}
	options.DefaultTarget = "missing"
	if err := UpdateSearchOptions(true, true, &options); err == nil {
		t.Fatal("invalid change saved")
	}
	after, err := os.ReadFile("config.yml")
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Fatal("invalid change modified file")
	}
	// A malformed backing file must not be overwritten by an apparently successful save.
	if err := os.WriteFile("config.yml", []byte("Search: ["), 0600); err != nil {
		t.Fatal(err)
	}
	invalidateFileCache("config")
	if err := UpdateSearchOptions(true, true, nil); err == nil {
		t.Fatal("failed read was not reported")
	}
}
