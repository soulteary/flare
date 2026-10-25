package data

import (
	"github.com/soulteary/flare/config/model"
	"os"
	"testing"
)

func TestHomepageAppearancePreservesOlderForms(t *testing.T) {
	t.Chdir(t.TempDir())
	update := model.Application{Title: "Layout", Homepage: &model.HomepageOptions{MobileBookmarkLayout: "compact"}}
	if !UpdateAppearance(update) {
		t.Fatal("unable to save compact layout")
	}
	before, err := os.ReadFile("config.yml")
	if err != nil {
		t.Fatal(err)
	}
	update.Homepage.MobileBookmarkLayout = "invalid"
	if UpdateAppearance(update) {
		t.Fatal("invalid layout persisted")
	}
	after, _ := os.ReadFile("config.yml")
	if string(before) != string(after) {
		t.Fatal("invalid input changed configuration")
	}
	update.Homepage = nil
	if !UpdateAppearance(update) {
		t.Fatal("older form failed")
	}
	options, err := GetAllSettingsOptions()
	if err != nil {
		t.Fatal(err)
	}
	if options.Homepage == nil || options.Homepage.MobileBookmarkLayout != "compact" {
		t.Fatal("older form erased layout")
	}
}
