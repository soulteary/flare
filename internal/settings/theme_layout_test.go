package settings

import (
	"strings"
	"testing"

	"github.com/soulteary/flare/config/model"
)

// Every option a theme can specify, and every value it can give, has a name
// in both interface languages, the one the appearance form uses.
func TestLayoutLabels(t *testing.T) {
	values := map[string][]string{
		model.LayoutMobileBookmarkLayout: {"classic", "single-column", "compact"},
		model.LayoutTitleSize:            {"standard", "compact", "prominent"},
		model.LayoutHideAppDescriptions:  {"false", "true"},
		model.LayoutCategoryNavigation:   {"all", "tabs", "collapse"},
		model.LayoutBookmarkLayout:       {"groups", "sections"},
		model.LayoutDesktopColumns:       {"legacy", "auto"},
	}
	for _, locale := range []string{"en", "zh"} {
		for _, option := range model.ThemeLayoutOptions {
			label := LayoutOptionLabel(locale, option)
			if label == "" || label == option || strings.Contains(label, "_") {
				t.Errorf("%s: option %s has no name: %q", locale, option, label)
			}
			for _, value := range values[option] {
				if !model.IsThemeLayoutValue(option, value) {
					t.Fatalf("%s is not a value of %s", value, option)
				}
				name := LayoutValueLabel(locale, option, value)
				if name == "" || name == value || strings.Contains(name, "_") {
					t.Errorf("%s: %s of %s has no name: %q", locale, value, option, name)
				}
			}
		}
		// A number of columns is its own name.
		for _, columns := range []string{"4", "5", "6", "8"} {
			if got := LayoutValueLabel(locale, model.LayoutDesktopColumns, columns); got != columns {
				t.Errorf("%s: %s columns are named %q", locale, columns, got)
			}
		}
	}
	// What has no name is shown as written.
	if got := LayoutOptionLabel("en", "CardStyle"); got != "CardStyle" {
		t.Fatalf("unknown option: %q", got)
	}
	if got := LayoutValueLabel("en", "CardStyle", "icon"); got != "icon" {
		t.Fatalf("unknown value: %q", got)
	}

	// A list follows the order of the appearance page, whatever order it
	// is given in, and leaves out what is not an option.
	given := []string{model.LayoutDesktopColumns, "CardStyle", model.LayoutTitleSize}
	if got := LayoutOptionList("en", given); got != "Homepage title size, Desktop columns" {
		t.Fatalf("list: %q", got)
	}
	if got := LayoutOptionList("zh", given); !strings.Contains(got, "、") || strings.Contains(got, ", ") {
		t.Fatalf("list in Chinese: %q", got)
	}
	if got := LayoutOptionList("en", nil); got != "" {
		t.Fatalf("empty list: %q", got)
	}
}
