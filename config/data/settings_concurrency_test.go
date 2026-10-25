package data

import (
	"fmt"
	"sync"
	"testing"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/search"
)

// Independent settings pages must not overwrite each other's successful saves.
func TestConcurrentSettingsUpdatesPreserveIndependentFields(t *testing.T) {
	t.Chdir(t.TempDir())
	invalidateFileCache("config")
	defer invalidateFileCache("config")
	for round := 0; round < 40; round++ {
		seed := model.Application{Title: "original", Theme: "original", Location: "original", Footer: "original"}
		if !saveAppConfigToYamlFile("config", seed) {
			t.Fatal("seed settings")
		}
		want := fmt.Sprintf("updated-%d", round)
		searchOptions := model.SearchOptions{DefaultTarget: "bing", Engines: search.Presets()}
		start := make(chan struct{})
		failures := make(chan string, 5)
		var workers sync.WaitGroup
		updates := []struct {
			name string
			run  func() bool
		}{
			{"appearance", func() bool {
				return UpdateAppearance(model.Application{Title: want, Footer: "retained footer", Locale: "en"})
			}},
			{"theme", func() bool { return UpdateThemeName(want) }},
			{"search engines", func() bool { return UpdateSearchOptions(true, true, &searchOptions) == nil }},
			{"legacy search", func() bool { return UpdateSearch(true, true) }},
			{"weather", func() bool {
				if round%2 == 0 {
					return UpdateWeatherAndLocation(true, want)
				}
				return UpdateLocation(want)
			}},
		}
		for _, update := range updates {
			workers.Add(1)
			go func(name string, run func() bool) {
				defer workers.Done()
				<-start
				if !run() {
					failures <- name
				}
			}(update.name, update.run)
		}
		close(start)
		workers.Wait()
		close(failures)
		for name := range failures {
			t.Errorf("round %d: %s save failed", round, name)
		}
		got, err := GetAllSettingsOptions()
		if err != nil {
			t.Fatal(err)
		}
		if got.Title != want || got.Theme != want || got.Location != want || got.Footer != "retained footer" || got.Locale != "en" || !got.ShowSearchComponent || !got.DisabledSearchAutoFocus || got.Search == nil || got.Search.DefaultTarget != "bing" {
			t.Fatalf("round %d lost an independently saved field: %+v", round, got)
		}
		if round%2 == 0 && !got.ShowWeather {
			t.Fatalf("round %d lost weather toggle", round)
		}
	}
}
