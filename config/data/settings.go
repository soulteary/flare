package data

import (
	"fmt"
	"sync"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/clock"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/search"
)

// settingsUpdateMu serializes read-modify-write operations within this process.
// Atomic file replacement alone prevents truncation, not lost updates between pages.
// This is not a cross-process lock or a merge strategy for concurrent edits of one section.
var settingsUpdateMu sync.Mutex

func GetThemeName() string {
	opts, err := GetAllSettingsOptions()
	if err != nil {
		return ""
	}
	return opts.Theme
}

func UpdateThemeName(theme string) bool {
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	options, err := GetAllSettingsOptions()
	if err != nil {
		return false
	}
	options.Theme = theme
	return saveAppConfigToYamlFile("config", options)
}

func GetLocationAndWeatherShow() (string, bool) {
	options, err := GetAllSettingsOptions()
	if err != nil {
		return "", false
	}
	if options.WeatherLocation != nil {
		return options.WeatherLocation.String(), options.ShowWeather
	}
	return options.Location, options.ShowWeather
}

func UpdateWeatherAndLocation(enable bool, location string) bool {
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	options, err := GetAllSettingsOptions()
	if err != nil {
		return false
	}
	options.ShowWeather = enable
	options.Location = location
	options.WeatherLocation = nil
	return saveAppConfigToYamlFile("config", options)
}

func UpdateLocation(location string) bool {
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	options, err := GetAllSettingsOptions()
	if err != nil {
		return false
	}
	options.Location = location
	options.WeatherLocation = nil
	return saveAppConfigToYamlFile("config", options)
}

func UpdateSearch(showSearchComponent bool, disabledSearchAutoFocus bool) bool {
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	options, err := GetAllSettingsOptions()
	if err != nil {
		return false
	}
	options.ShowSearchComponent = showSearchComponent
	options.DisabledSearchAutoFocus = disabledSearchAutoFocus
	return saveSearchConfigPreservingData(options) == nil
}

// UpdateSearchOptions validates and atomically saves search settings without changing other settings.
func UpdateSearchOptions(showSearchComponent, disabledSearchAutoFocus bool, update *model.SearchOptions, revision ...string) error {
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	var next *model.SearchOptions
	if update != nil {
		normalized := search.Normalize(update)
		if err := search.Validate(normalized); err != nil {
			return err
		}
		next = &normalized
	}
	options, err := GetAllSettingsOptions()
	if err != nil {
		return err
	}
	options.ShowSearchComponent = showSearchComponent
	options.DisabledSearchAutoFocus = disabledSearchAutoFocus
	options.Search = next
	if len(revision) > 0 && revision[0] != "" {
		options.ConfigRevision = revision[0]
	}
	if err := saveSearchConfigPreservingData(options); err != nil {
		return fmt.Errorf("could not save search settings: %w", err)
	}
	return nil
}

func GetAllSettingsOptions() (model.Application, error) {
	options, err := loadAppConfigFromYaml("config")
	if err != nil {
		return options, err
	}
	options.ConfigVersionState = ConfigurationVersionState(options.AppVersion)
	options.Locale = i18n.Normalize(options.Locale)
	return options, nil
}

// ReadExistingSettingsOptions is the read-only counterpart used by tentative
// buffered views. Missing settings return an error rather than initializing YAML.
func ReadExistingSettingsOptions() (model.Application, error) {
	options, err := loadExistingAppConfigFromYaml("config")
	if err != nil {
		return options, err
	}
	options.ConfigVersionState = ConfigurationVersionState(options.AppVersion)
	options.Locale = i18n.Normalize(options.Locale)
	return options, nil
}

func UpdateAppearance(update model.Application) bool {
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	if update.Homepage != nil && update.Homepage.Validate() != nil {
		return false
	}
	if update.Clock != nil && clock.Validate(*update.Clock) != nil {
		return false
	}
	options, err := GetAllSettingsOptions()
	if err != nil {
		return false
	}

	if update.Clock != nil {
		options.Clock = update.Clock
	}
	if update.Homepage != nil {
		options.Homepage = update.Homepage
	}
	options.Title = update.Title
	options.Footer = update.Footer
	options.OpenAppNewTab = update.OpenAppNewTab
	options.OpenBookmarkNewTab = update.OpenBookmarkNewTab
	options.ShowTitle = update.ShowTitle
	options.Greetings = update.Greetings
	options.ShowDateTime = update.ShowDateTime
	options.ShowApps = update.ShowApps
	options.ShowBookmarks = update.ShowBookmarks
	options.HideSettingsButton = update.HideSettingsButton
	options.HideHelpButton = update.HideHelpButton
	options.HideVersion = update.HideVersion
	options.EnableEncryptedLink = update.EnableEncryptedLink
	options.IconMode = update.IconMode
	options.KeepLetterCase = update.KeepLetterCase
	if update.Locale != "" {
		locale, ok := i18n.ParseLocale(update.Locale)
		if !ok {
			return false
		}
		options.Locale = locale
	}

	return saveAppConfigToYamlFile("config", options)
}

// UpdateSelectedWeather atomically saves the display address and provider fields.
func UpdateSelectedWeather(enable bool, location *model.WeatherLocation) bool {
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	options, err := GetAllSettingsOptions()
	if err != nil {
		return false
	}
	options.ShowWeather = enable
	options.WeatherLocation = location
	if location != nil {
		options.Location = location.String()
	}
	return saveAppConfigToYamlFile("config", options)
}

// MigrateWeatherLocation never overwrites a concurrent manual selection.
func MigrateWeatherLocation(previous string, selected *model.WeatherLocation) bool {
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	options, err := GetAllSettingsOptions()
	if err != nil || selected == nil || options.WeatherLocation != nil || options.Location != previous {
		return false
	}
	options.Location = selected.String()
	options.WeatherLocation = selected
	return saveAppConfigToYamlFile("config", options)
}
