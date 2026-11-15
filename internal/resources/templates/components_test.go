package templates

import (
	"bytes"
	"html/template"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/model"
)

// homeData is what a page with every module turned on hands to home.html,
// as far as the parts read it.
func homeData() map[string]any {
	return map[string]any{
		"Locale": "en", "OptionTitle": "Flare", "ClockMode": "server", "SubPage": true, "PageName": "Applications",
		"ShowSearchComponent": true, "SearchKeyword": "Search", "SearchQuery": "<query>", "SecurityCSRFToken": "token-value",
		"OptionShowDateTime": true, "HeroDate": "2026-11-15", "HeroDay": "Sunday", "HeroTime": "12:00:00",
		"OptionShowTitle": true, "Greetings": "Hello & welcome",
		"ShowWeatherModule": true, "WeatherData": model.Weather{Degree: 21, Humidity: 40}, "WeatherIcon": template.HTML(`<svg class="weather"></svg>`),
		"OptionShowApps": true, "ApplicationsURI": "/applications", "Applications": template.HTML(`<div class="app-item">app</div>`),
		"OptionShowBookmarks": true, "BookmarksURI": "/bookmarks", "Bookmarks": template.HTML(`<a class="bookmark">bookmark</a>`),
		"SettingsURI": "/settings", "OptionFooter": template.HTML(`<p class="mine">footer</p>`),
	}
}

func homeTemplates(t *testing.T) *template.Template {
	t.Helper()
	views, err := NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	return views.(*Renderer).templates
}

func execute(t *testing.T, templates *template.Template, name string, data any) string {
	t.Helper()
	var out bytes.Buffer
	if err := templates.ExecuteTemplate(&out, name, data); err != nil {
		t.Fatalf("%s: %v", name, err)
	}
	return out.String()
}

// The page is its parts in the listed order with the markup that home.html
// keeps for itself between them, and nothing of a part is rendered twice.
func TestHomePageIsPutTogetherFromItsComponents(t *testing.T) {
	templates := homeTemplates(t)
	data := homeData()
	page := execute(t, templates, "home.html", data)
	marks := map[string]string{
		"flare/search": `id=search-form`, "flare/clock": `id=clock `, "flare/greeting": `id=plugin-greetings`,
		"flare/weather": `id=plugin-weather`, "flare/subpage-header": `class=sub-page-header`, "flare/apps": `id=container-apps`,
		"flare/bookmarks": `id=container-bookmarks`, "flare/toolbar": `class=toolbar-container`, "flare/footer": `class=footer-container`,
	}
	if len(marks) != len(HomeComponents) {
		t.Fatalf("%d parts are listed and %d are checked", len(HomeComponents), len(marks))
	}
	rest := page
	for _, name := range HomeComponents {
		part := execute(t, templates, name, data)
		if !strings.Contains(part, marks[name]) {
			t.Fatalf("%s does not hold %s: %s", name, marks[name], part)
		}
		if strings.Count(page, part) != 1 {
			t.Fatalf("%s is in the page %d times", name, strings.Count(page, part))
		}
		at := strings.Index(rest, part)
		if at < 0 {
			t.Fatalf("%s is not after the part listed before it", name)
		}
		// What stands between two parts belongs to the page: no part of
		// another component is left there.
		for other, mark := range marks {
			if strings.Contains(rest[:at], mark) {
				t.Fatalf("%s stands outside its part, before %s", other, name)
			}
		}
		rest = rest[at+len(part):]
	}
	for other, mark := range marks {
		if strings.Contains(rest, mark) {
			t.Fatalf("%s stands outside its part, after the last one", other)
		}
	}
	// What the page hands to its parts is escaped in them as it was in the page.
	for _, want := range []string{`value="&lt;query&gt;"`, `Hello &amp; welcome`, `<p class="mine">footer</p>`, `value="token-value"`} {
		if !strings.Contains(page, want) {
			t.Fatalf("the page does not hold %s", want)
		}
	}
}

// A part carries the condition of its module, so a page template that places
// it need not, and cannot, decide whether the module is shown.
func TestHomeComponentsOfModulesTurnedOffRenderNothing(t *testing.T) {
	templates := homeTemplates(t)
	data := map[string]any{"Locale": "en", "ClockMode": "server", "OptionHideSettingsButton": true, "OptionHideHelpButton": true}
	for _, name := range HomeComponents {
		part := strings.Join(strings.Fields(execute(t, templates, name, data)), " ")
		switch name {
		case "flare/toolbar":
			if part != `<div class=toolbar-container> </div>` {
				t.Fatalf("%s: %s", name, part)
			}
		case "flare/footer":
			if part != `<div class=footer-container></div>` {
				t.Fatalf("%s: %s", name, part)
			}
		default:
			if part != "" {
				t.Fatalf("%s renders while its module is off: %s", name, part)
			}
		}
	}
}

// The file the pages are generated from names the same parts as the list,
// each once, and both forms of the file have them.
func TestHomeComponentsAreDefinedOnceInBothFormsOfTheTemplate(t *testing.T) {
	block := regexp.MustCompile(`\{\{ block "(flare/[a-z-]+)" \. \}\}`)
	for _, file := range []string{"../../../embed/templates/home.html", "html/home.html"} {
		raw, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		var found []string
		for _, match := range block.FindAllStringSubmatch(string(raw), -1) {
			found = append(found, match[1])
		}
		if strings.Join(found, " ") != strings.Join(HomeComponents, " ") {
			t.Fatalf("%s defines %v", file, found)
		}
	}
	templates := homeTemplates(t)
	for _, name := range HomeComponents {
		if templates.Lookup(name) == nil {
			t.Fatalf("%s is not a template", name)
		}
	}
}
