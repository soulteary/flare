package FlareData

import (
	"os"
	"testing"

	FlareModel "github.com/soulteary/flare/config/model"
)

func TestGetAndSetThemeName(t *testing.T) {
	const target = "test"
	UpdateThemeName(target)
	theme := GetThemeName()
	if theme != target {
		t.Fatal("GetThemeName Error")
	}

	filePath := getConfigPath("config")
	os.Remove(filePath)
}

func TestGetAndSetWeatherAndLocation(t *testing.T) {
	targetEnable := false
	targetLocation := "地球"

	ok := UpdateWeatherAndLocation(targetEnable, targetLocation)
	if !ok {
		t.Fatal("UpdateWeatherAndLocation Error")
	}

	location, enable := GetLocationAndWeatherShow()
	if enable != targetEnable && location != targetLocation {
		t.Fatal("GetLocationAndWeatherShow Error")
	}

	filePath := getConfigPath("config")
	os.Remove(filePath)

	targetLocation = "火星"
	ok = UpdateLocation(targetLocation)
	if !ok {
		t.Fatal("UpdateLocation Error")
	}

	location, _ = GetLocationAndWeatherShow()
	if location != targetLocation {
		t.Fatal("GetLocationAndWeatherShow Error")
	}

	os.Remove(filePath)
}

func TestUpdateSearchAndGetAllSettingsOptions(t *testing.T) {
	showSearchComponent := true
	disabledSearchAutoFocus := false

	ok := UpdateSearch(showSearchComponent, disabledSearchAutoFocus)
	if !ok {
		t.Fatal("UpdateSearch Error")
	}

	options := GetAllSettingsOptions()
	if options.ShowSearchComponent != showSearchComponent && options.DisabledSearchAutoFocus != disabledSearchAutoFocus {
		t.Fatal("GetAllSettingsOptions Error")
	}

	filePath := getConfigPath("config")
	os.Remove(filePath)
}

func TestUpdateAppearance(t *testing.T) {
	const Title = "Test"
	var update FlareModel.Application
	update.Title = Title

	ok := UpdateAppearance(update)
	if !ok {
		t.Fatal("UpdateAppearance Error")
	}

	options := GetAllSettingsOptions()
	if options.Title != Title {
		t.Fatal("GetAllSettingsOptions Error")
	}

	filePath := getConfigPath("config")
	os.Remove(filePath)
}
