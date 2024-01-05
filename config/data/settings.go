package FlareData

import (
	FlareModel "github.com/soulteary/flare/config/model"
)

func GetThemeName() string {
	return GetAllSettingsOptions().Theme
}

func UpdateThemeName(theme string) bool {
	options := GetAllSettingsOptions()
	options.Theme = theme
	return saveAppConfigToYamlFile("config", options)
}

func GetLocationAndWeatherShow() (string, bool) {
	options := GetAllSettingsOptions()
	return options.Location, options.ShowWeather
}

func UpdateWeatherAndLocation(enable bool, location string) bool {
	options := GetAllSettingsOptions()
	options.ShowWeather = enable
	options.Location = location
	return saveAppConfigToYamlFile("config", options)
}

func UpdateLocation(location string) bool {
	options := GetAllSettingsOptions()
	options.Location = location
	return saveAppConfigToYamlFile("config", options)
}

func UpdateSearch(showSearchComponent bool, disabledSearchAutoFocus bool) bool {
	options := GetAllSettingsOptions()
	options.ShowSearchComponent = showSearchComponent
	options.DisabledSearchAutoFocus = disabledSearchAutoFocus
	return saveAppConfigToYamlFile("config", options)
}

func GetAllSettingsOptions() (options FlareModel.Application) {
	return loadAppConfigFromYaml("config")
}

func UpdateAppearance(update FlareModel.Application) bool {

	options := GetAllSettingsOptions()

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

	return saveAppConfigToYamlFile("config", options)
}
