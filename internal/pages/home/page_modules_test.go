package home

import (
	"html/template"
	"maps"
	"testing"
)

// A page template is told which parts the page shows. Each part is shown
// under the condition its markup in home.html has.
func TestPageModules(t *testing.T) {
	none := map[string]bool{
		"search": false, "clock": false, "greeting": false, "weather": false, "subpage-header": false,
		"apps": false, "bookmarks": false, "toolbar": true, "footer": false,
	}
	if got := pageModules(map[string]any{}); !maps.Equal(got, none) {
		t.Fatalf("an empty page: %v", got)
	}
	with := func(changes map[string]bool) map[string]bool {
		want := maps.Clone(none)
		maps.Copy(want, changes)
		return want
	}
	for _, tc := range []struct {
		name string
		page map[string]any
		want map[string]bool
	}{
		{"search", map[string]any{"ShowSearchComponent": true}, with(map[string]bool{"search": true})},
		{"clock", map[string]any{"OptionShowDateTime": true}, with(map[string]bool{"clock": true})},
		{"greeting", map[string]any{"OptionShowTitle": true}, with(map[string]bool{"greeting": true})},
		{"weather without the title", map[string]any{"ShowWeatherModule": true}, none},
		{"weather", map[string]any{"OptionShowTitle": true, "ShowWeatherModule": true}, with(map[string]bool{"greeting": true, "weather": true})},
		{"subpage header", map[string]any{"SubPage": true}, with(map[string]bool{"subpage-header": true})},
		{"applications", map[string]any{"OptionShowApps": true}, with(map[string]bool{"apps": true})},
		{"bookmarks", map[string]any{"OptionShowBookmarks": true}, with(map[string]bool{"bookmarks": true})},
		{"one button hidden", map[string]any{"OptionHideSettingsButton": true}, none},
		{"the other button hidden", map[string]any{"OptionHideHelpButton": true}, none},
		{"both buttons hidden", map[string]any{"OptionHideSettingsButton": true, "OptionHideHelpButton": true}, with(map[string]bool{"toolbar": false})},
		{"footer", map[string]any{"OptionFooter": template.HTML("<p>mine</p>")}, with(map[string]bool{"footer": true})},
		{"empty footer", map[string]any{"OptionFooter": template.HTML("")}, none},
		{"values that are not on", map[string]any{"ShowSearchComponent": false, "OptionShowApps": "true", "SubPage": 1}, none},
	} {
		if got := pageModules(tc.page); !maps.Equal(got, tc.want) {
			t.Errorf("%s: %v", tc.name, got)
		}
	}
}
