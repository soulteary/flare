package themetemplate

import (
	"errors"
	"html/template"
	"strings"
	"testing"
)

func pageFuncs() PageFuncs {
	return PageFuncs{
		T: func(key string) string { return "text of " + key },
		Asset: func(name string) (string, error) {
			if name != "logo.png" {
				return "", errors.New("no asset " + name)
			}
			return assetPrefix + name, nil
		},
		AssetPrefix: assetPrefix,
	}
}

const everyPart = `{{template "flare/search"}}{{template "flare/clock"}}{{template "flare/greeting"}}{{template "flare/weather"}}` +
	`{{template "flare/subpage-header"}}{{template "flare/apps"}}{{template "flare/bookmarks"}}{{template "flare/toolbar"}}{{template "flare/footer"}}`

func TestPageTemplateBranchesOnWhatThePageIs(t *testing.T) {
	source := `<div class="shell" lang="{{.Site.Lang}}"><h1>{{.Site.Title}}</h1><img src="{{asset "logo.png"}}" alt="{{T "logo"}}">` +
		`{{if eq .Page.Kind "home"}}<p>home</p>{{end}}{{if .Page.Searching}}<p>results</p>{{end}}{{if .Modules.weather}}<p>weather on</p>{{end}}` +
		`{{if eq .Layout.BookmarkLayout "sections"}}<p>sections</p>{{end}}{{if eq .Theme.Options.density "compact"}}<p>compact</p>{{end}}` + everyPart + `</div>`
	values := PageValues{
		Lang: "en", Kind: PageHome, Searching: true, Modules: map[string]bool{"weather": true},
		Layout: map[string]string{"BookmarkLayout": "sections"}, Options: map[string]any{"density": "compact"},
	}
	fragment, err := CompilePage(source, PageHome, values, pageFuncs())
	if err != nil {
		t.Fatal(err)
	}
	got := render(fragment, entry{texts: map[string]string{"Title": "Mine & yours"}, slots: map[string]template.HTML{SlotApps: "<i>apps</i>"}})
	want := `<div class="shell" lang="en"><h1>Mine &amp; yours</h1><img src="` + assetPrefix + `logo.png" alt="text of logo">` +
		`<p>home</p><p>results</p><p>weather on</p><p>sections</p><p>compact</p><i>apps</i></div>`
	if got != want {
		t.Fatalf("rendered:\n%s", got)
	}
	// Other values take the other way, and what is missing counts as off.
	fragment, err = CompilePage(source, PageBookmarks, PageValues{Lang: "zh", Kind: PageBookmarks}, pageFuncs())
	if err != nil {
		t.Fatal(err)
	}
	if got = render(fragment, entry{}); strings.Contains(got, "<p>") || !strings.Contains(got, `lang="zh"`) {
		t.Fatalf("rendered:\n%s", got)
	}
	holes := fragment.Holes()
	if len(holes) != 1+len(PageSlotNames) || holes[0] != (Hole{HoleText, "Title"}) {
		t.Fatalf("holes: %v", holes)
	}
	for i, name := range PageSlotNames {
		if holes[i+1] != (Hole{HoleSlot, name}) {
			t.Fatalf("hole %d: %v", i+1, holes[i+1])
		}
	}
}

func TestPageTemplateHasToPlaceWhatItsPageShows(t *testing.T) {
	part := func(names ...string) string {
		var b strings.Builder
		b.WriteString("<div>")
		for _, name := range names {
			b.WriteString(`{{template "` + name + `"}}`)
		}
		return b.String() + "</div>"
	}
	cases := []struct {
		name    string
		kind    PageKind
		source  string
		missing string
	}{
		{"the home page with every part", PageHome, part(PageSlotNames...), ""},
		{"the home page without the weather", PageHome, part(SlotSearch, SlotClock, SlotGreeting, SlotSubpageHeader, SlotApps, SlotBookmarks, SlotToolbar, SlotFooter), SlotWeather},
		{"the application page with what it shows", PageApplications, part(SlotSubpageHeader, SlotApps, SlotToolbar, SlotFooter), ""},
		{"the application page with more", PageApplications, part(SlotSearch, SlotSubpageHeader, SlotApps, SlotBookmarks, SlotToolbar, SlotFooter), ""},
		{"the application page without its list", PageApplications, part(SlotSubpageHeader, SlotBookmarks, SlotToolbar, SlotFooter), SlotApps},
		{"the application page without its header", PageApplications, part(SlotApps, SlotToolbar, SlotFooter), SlotSubpageHeader},
		{"the bookmark page with what it shows", PageBookmarks, part(SlotSubpageHeader, SlotBookmarks, SlotToolbar, SlotFooter), ""},
		{"the bookmark page without its list", PageBookmarks, part(SlotSubpageHeader, SlotApps, SlotToolbar, SlotFooter), SlotBookmarks},
		{"the bookmark page without the toolbar", PageBookmarks, part(SlotSubpageHeader, SlotBookmarks, SlotFooter), SlotToolbar},
		{"the bookmark page without the footer", PageBookmarks, part(SlotSubpageHeader, SlotBookmarks, SlotToolbar), SlotFooter},
	}
	for _, tc := range cases {
		_, err := CompilePage(tc.source, tc.kind, PageValues{Kind: tc.kind}, pageFuncs())
		if tc.missing == "" {
			if err != nil {
				t.Errorf("%s: %v", tc.name, err)
			}
			continue
		}
		if err == nil {
			t.Errorf("%s: compiled", tc.name)
			continue
		}
		if reason, subject := reasonOf(t, err); reason != ReasonSlotMissing || subject != tc.missing {
			t.Errorf("%s: %s %s", tc.name, reason, subject)
		}
	}
	if PageHome.File() != "home.html" || PageApplications.File() != "applications.html" || PageBookmarks.File() != "bookmarks.html" || len(PageKinds) != 3 {
		t.Fatal("the files of the pages")
	}
}

// A page template is held to what every template is held to, with the
// functions and the limit of a page.
func TestPageTemplateIsCheckedLikeEveryTemplate(t *testing.T) {
	cases := []struct {
		name, inner string
		reason      Reason
	}{
		{"an asset that the theme does not have", `<img src="{{asset "other.png"}}">`, ReasonExecute},
		{"an image of another host", `<img src="https://collector.example/a.png">`, ReasonResource},
		{"a script", `<script>x()</script>`, ReasonElement},
		{"the title in a class", `<p class="{{.Site.Title}}">x</p>`, ReasonFieldPosition},
		{"a function that is not there", `{{env "HOME"}}`, ReasonParse},
		{"more than a page can write", `{{define "more"}}<b>0123456789</b>{{template "more"}}{{end}}{{template "more"}}`, ReasonTooLarge},
	}
	for _, tc := range cases {
		_, err := CompilePage(`<div>`+everyPart+tc.inner+`</div>`, PageHome, PageValues{Kind: PageHome}, pageFuncs())
		if err == nil {
			t.Errorf("%s: compiled", tc.name)
			continue
		}
		if reason, _ := reasonOf(t, err); reason != tc.reason {
			t.Errorf("%s: %s", tc.name, reason)
		}
	}
}
