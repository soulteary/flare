package data

import (
	"github.com/soulteary/flare/config/model"
)

func GetThemeName() string {
	opts, err := GetAllSettingsOptions()
	if err != nil {
		return ""
	}
	return opts.Theme
}

func UpdateThemeName(theme string) bool {
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
	return options.Location, options.ShowWeather
}

func UpdateWeatherAndLocation(enable bool, location string) bool {
	options, err := GetAllSettingsOptions()
	if err != nil {
		return false
	}
	options.ShowWeather = enable
	options.Location = location
	return saveAppConfigToYamlFile("config", options)
}

func UpdateLocation(location string) bool {
	options, err := GetAllSettingsOptions()
	if err != nil {
		return false
	}
	options.Location = location
	return saveAppConfigToYamlFile("config", options)
}

func UpdateSearch(showSearchComponent bool, disabledSearchAutoFocus bool) bool {
	options, err := GetAllSettingsOptions()
	if err != nil {
		return false
	}
	options.ShowSearchComponent = showSearchComponent
	options.DisabledSearchAutoFocus = disabledSearchAutoFocus
	return saveAppConfigToYamlFile("config", options)
}

func GetAllSettingsOptions() (model.Application, error) {
	options, err := loadAppConfigFromYaml("config")
	if err != nil {
		return options, err
	}
	if options.Locale == "" {
		options.Locale = "zh"
	}
	return options, nil
}

func UpdateAppearance(update model.Application) bool {
	options, err := GetAllSettingsOptions()
	if err != nil {
		return false
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
	options.EnableEncryptedLink = update.EnableEncryptedLink
	options.IconMode = update.IconMode
	options.KeepLetterCase = update.KeepLetterCase
	options.Locale = update.Locale

	return saveAppConfigToYamlFile("config", options)
}
