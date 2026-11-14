package data

import (
	"fmt"
	"slices"
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
	return reportSave(SaveThemeName(theme))
}

// SaveThemeName is UpdateThemeName with the classified failure.
func SaveThemeName(theme string) error {
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	options, err := GetAllSettingsOptions()
	if err != nil {
		return saveError(SaveFailureConfigUnreadable, err)
	}
	options.Theme = theme
	return saveAppConfig("config", options)
}

// GetThemeSelection returns the stored theme name and the stored custom
// palette. The palette may be nil, and it is not validated here: config.yml
// can be edited by hand.
func GetThemeSelection() (string, *model.Palette) {
	opts, err := GetAllSettingsOptions()
	if err != nil {
		return "", nil
	}
	return opts.Theme, opts.ThemeCustom
}

// GetThemeMode returns the stored setting for light and dark as written. It
// is empty when the configuration has none, and it is not validated here:
// config.yml can be edited by hand.
func GetThemeMode() string {
	opts, err := GetAllSettingsOptions()
	if err != nil {
		return ""
	}
	return opts.ThemeMode
}

// SaveThemeMode stores the setting for light and dark: auto, light or dark.
func SaveThemeMode(mode string) error {
	if !model.IsThemeMode(mode) {
		return saveError(SaveFailureInvalidInput, fmt.Errorf("unsupported theme mode %q", mode))
	}
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	options, err := GetAllSettingsOptions()
	if err != nil {
		return saveError(SaveFailureConfigUnreadable, err)
	}
	options.ThemeMode = mode
	return saveAppConfig("config", options)
}

// SaveThemeCustom stores a custom palette and selects it in a single write.
// Choosing a built-in theme afterwards keeps the stored palette.
func SaveThemeCustom(palette model.Palette) error {
	palette = palette.Normalized()
	if err := palette.Validate(); err != nil {
		return saveError(SaveFailureInvalidInput, err)
	}
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	options, err := GetAllSettingsOptions()
	if err != nil {
		return saveError(SaveFailureConfigUnreadable, err)
	}
	options.Theme = model.ThemeCustomName
	options.ThemeCustom = &palette
	return saveAppConfig("config", options)
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
	return reportSave(SaveSearch(showSearchComponent, disabledSearchAutoFocus))
}

// SaveSearch is UpdateSearch with the classified failure.
func SaveSearch(showSearchComponent bool, disabledSearchAutoFocus bool) error {
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	options, err := GetAllSettingsOptions()
	if err != nil {
		return saveError(SaveFailureConfigUnreadable, err)
	}
	options.ShowSearchComponent = showSearchComponent
	options.DisabledSearchAutoFocus = disabledSearchAutoFocus
	return saveSearchConfigPreservingData(options)
}

// UpdateSearchOptions validates and atomically saves search settings without changing other settings.
func UpdateSearchOptions(showSearchComponent, disabledSearchAutoFocus bool, update *model.SearchOptions, revision ...string) error {
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	var next *model.SearchOptions
	if update != nil {
		normalized := search.Normalize(update)
		if err := search.Validate(normalized); err != nil {
			return saveError(SaveFailureInvalidInput, err)
		}
		next = &normalized
	}
	options, err := GetAllSettingsOptions()
	if err != nil {
		return saveError(SaveFailureConfigUnreadable, err)
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
	return reportSave(SaveAppearance(update))
}

// SaveAppearance is UpdateAppearance with the classified failure.
func SaveAppearance(update model.Application) error {
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	if update.Homepage != nil {
		if err := update.Homepage.Validate(); err != nil {
			return saveError(SaveFailureInvalidInput, err)
		}
	}
	if update.Clock != nil {
		if err := clock.Validate(*update.Clock); err != nil {
			return saveError(SaveFailureInvalidInput, err)
		}
	}
	options, err := GetAllSettingsOptions()
	if err != nil {
		return saveError(SaveFailureConfigUnreadable, err)
	}

	if update.Clock != nil {
		options.Clock = update.Clock
	}
	if update.Homepage != nil {
		options.Homepage = update.Homepage
	}
	for name, settings := range update.ThemeSettings {
		ignored, err := normalizeIgnoredLayout(name, settings.IgnoreLayout)
		if err != nil {
			return saveError(SaveFailureInvalidInput, err)
		}
		// An empty list is stored only to clear an earlier one: a theme
		// nothing was ever turned off for gets no entry.
		if _, stored := options.ThemeSettings[name]; !stored && len(ignored) == 0 {
			continue
		}
		if options.ThemeSettings == nil {
			options.ThemeSettings = make(map[string]model.ThemeSettings)
		}
		current := options.ThemeSettings[name]
		current.IgnoreLayout = ignored
		options.ThemeSettings[name] = current
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
			return saveError(SaveFailureInvalidInput, fmt.Errorf("unsupported locale %q", update.Locale))
		}
		options.Locale = locale
	}

	return saveAppConfig("config", options)
}

// normalizeIgnoredLayout checks the layout options turned off for a theme and
// returns them without repetition, in the order of the appearance page.
func normalizeIgnoredLayout(theme string, options []string) ([]string, error) {
	if !model.IsThemeName(theme) {
		return nil, fmt.Errorf("invalid theme name %q", theme)
	}
	for _, option := range options {
		if !model.IsThemeLayoutOption(option) {
			return nil, fmt.Errorf("invalid layout option %q", option)
		}
	}
	ignored := make([]string, 0, len(options))
	for _, option := range model.ThemeLayoutOptions {
		if slices.Contains(options, option) {
			ignored = append(ignored, option)
		}
	}
	return ignored, nil
}

// UpdateSelectedWeather atomically saves the display address and provider fields.
func UpdateSelectedWeather(enable bool, location *model.WeatherLocation) bool {
	return reportSave(SaveSelectedWeather(enable, location))
}

// SaveSelectedWeather is UpdateSelectedWeather with the classified failure.
func SaveSelectedWeather(enable bool, location *model.WeatherLocation) error {
	settingsUpdateMu.Lock()
	defer settingsUpdateMu.Unlock()
	options, err := GetAllSettingsOptions()
	if err != nil {
		return saveError(SaveFailureConfigUnreadable, err)
	}
	options.ShowWeather = enable
	options.WeatherLocation = location
	if location != nil {
		options.Location = location.String()
	}
	return saveAppConfig("config", options)
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
