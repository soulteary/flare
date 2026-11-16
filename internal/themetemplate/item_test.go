package themetemplate

import (
	"bytes"
	"html/template"
	"io"
	"reflect"
	"strings"
	"testing"
)

const (
	appTemplate = `<div class="card{{if .NewTab}} opens-new{{end}}" data-lang="{{.Site.Lang}}" data-density="{{.Theme.Options.density}}">` +
		`<a href="{{.Href}}" title="{{.Name}}">{{if .Has.Icon}}<span class="icon">{{template "flare/icon"}}</span>{{else}}<span class="letter">{{.Initial}}</span>{{end}}` +
		`<strong>{{.Name}}</strong>{{if .Has.Description}}<small>{{.Description}}</small>{{else}}<small class="host">{{.Host}}</small>{{end}}</a></div>`
	bookmarkTemplate = `<li class="tile"><a href="{{.Href}}">{{if .Has.Icon}}{{template "flare/icon"}}{{end}}<span>{{.Name}}</span>{{if .NewTab}}<i>{{T "new"}}</i>{{end}}<em>{{.Host}}</em></a></li>`
	groupTemplate    = `{{if .Collapsible}}<details class="group"><summary>{{.Name}}</summary><ul>{{template "flare/items"}}</ul></details>` +
		`{{else}}<section class="group">{{if .Has.Name}}<h3>{{.Name}}</h3>{{end}}<ol>{{template "flare/items"}}</ol></section>{{end}}`
)

func itemEntry() entry {
	return entry{
		texts: map[string]string{FieldName: `Mine & "<yours>"`, FieldDescription: "what it <is>", FieldHost: "mine.example", FieldInitial: "M"},
		href:  "https://mine.example/?a=1&b=2", link: ` target="_blank" rel="noopener"`,
		slots: map[string]template.HTML{SlotIcon: `<svg class="icon"></svg>`, SlotItems: `<li>one</li><li>two</li>`},
	}
}

func compileItem(t *testing.T, source string, kind ItemKind, values ItemValues) string {
	t.Helper()
	fragment, err := CompileItem(source, kind, values, pageFuncs())
	if err != nil {
		t.Fatalf("%s %+v: %v", kind, values, err)
	}
	return render(fragment, itemEntry())
}

// An item template places the fields of an entry and branches on what the
// entry has. The fields are filled in when an entry is rendered, escaped,
// and the link to the entry gets its attributes from the application.
func TestItemTemplatesAreCompiledForWhatAnEntryHas(t *testing.T) {
	name, link := `Mine &amp; &#34;&lt;yours&gt;&#34;`, `href="https://mine.example/?a=1&amp;b=2" title="Mine &amp; &#34;&lt;yours&gt;&#34;" target="_blank" rel="noopener"`
	for _, tc := range []struct {
		values ItemValues
		want   string
	}{
		{ItemValues{Lang: "en", Icon: true, Description: true, Options: map[string]any{"density": "compact"}},
			`<div class="card" data-lang="en" data-density="compact"><a ` + link + `><span class="icon"><svg class="icon"></svg></span><strong>` + name + `</strong><small>what it &lt;is&gt;</small></a></div>`},
		{ItemValues{Lang: "zh", NewTab: true},
			`<div class="card opens-new" data-lang="zh" data-density=""><a ` + link + `><span class="letter">M</span><strong>` + name + `</strong><small class="host">mine.example</small></a></div>`},
	} {
		if got := compileItem(t, appTemplate, ItemApp, tc.values); got != tc.want {
			t.Errorf("application %+v:\n%s", tc.values, got)
		}
	}

	bookmarkLink := `href="https://mine.example/?a=1&amp;b=2" target="_blank" rel="noopener"`
	if got, want := compileItem(t, bookmarkTemplate, ItemBookmark, ItemValues{Icon: true, NewTab: true}),
		`<li class="tile"><a `+bookmarkLink+`><svg class="icon"></svg><span>`+name+`</span><i>text of new</i><em>mine.example</em></a></li>`; got != want {
		t.Errorf("bookmark:\n%s", got)
	}
	if got, want := compileItem(t, bookmarkTemplate, ItemBookmark, ItemValues{}), `<li class="tile"><a `+bookmarkLink+`><span>`+name+`</span><em>mine.example</em></a></li>`; got != want {
		t.Errorf("bookmark without an icon:\n%s", got)
	}

	items := `<li>one</li><li>two</li>`
	for _, tc := range []struct {
		values ItemValues
		want   string
	}{
		{ItemValues{Named: true}, `<section class="group"><h3>` + name + `</h3><ol>` + items + `</ol></section>`},
		{ItemValues{Named: true, Collapsible: true}, `<details class="group"><summary>` + name + `</summary><ul>` + items + `</ul></details>`},
		{ItemValues{}, `<section class="group"><ol>` + items + `</ol></section>`},
	} {
		if got := compileItem(t, groupTemplate, ItemGroup, tc.values); got != tc.want {
			t.Errorf("group %+v:\n%s", tc.values, got)
		}
	}
}

// Every set of values an entry can have compiles the templates above, and
// the sets are all the combinations.
func TestItemValueSets(t *testing.T) {
	counts := map[ItemKind]int{ItemApp: 8, ItemBookmark: 4, ItemGroup: 3}
	sources := map[ItemKind]string{ItemApp: appTemplate, ItemBookmark: bookmarkTemplate, ItemGroup: groupTemplate}
	for _, kind := range ItemKinds {
		sets := ItemValueSets(kind)
		if len(sets) != counts[kind] {
			t.Errorf("%s: %d sets", kind, len(sets))
		}
		for i, values := range sets {
			for _, other := range sets[:i] {
				if reflect.DeepEqual(values, other) {
					t.Errorf("%s: %+v twice", kind, values)
				}
			}
			if kind != ItemApp && values.Description || kind == ItemGroup && (values.Icon || values.NewTab) || kind != ItemGroup && (values.Named || values.Collapsible) {
				t.Errorf("%s: %+v has a value of another kind", kind, values)
			}
			if values.Collapsible && !values.Named {
				t.Errorf("%s: a group without a name is not opened and closed", kind)
			}
			if _, err := CompileItem(sources[kind], kind, values, pageFuncs()); err != nil {
				t.Errorf("%s %+v: %v", kind, values, err)
			}
		}
	}
	if got := []string{ItemApp.File(), ItemBookmark.File(), ItemGroup.File()}; !reflect.DeepEqual(got, []string{"app.html", "bookmark.html", "group.html"}) {
		t.Errorf("files: %v", got)
	}
}

func TestItemTemplatesThatAreRefused(t *testing.T) {
	named := ItemValues{Named: true}
	for _, tc := range []struct {
		name    string
		kind    ItemKind
		values  ItemValues
		source  string
		reason  Reason
		subject string
	}{
		{"an application without a link to it", ItemApp, ItemValues{}, `<div><strong>{{.Name}}</strong></div>`, ReasonLinkCount, "0"},
		{"an application with two links to it", ItemApp, ItemValues{}, `<div><a href="{{.Href}}">{{.Name}}</a><a href="{{.Href}}">again</a></div>`, ReasonLinkCount, "2"},
		{"an application whose name is not shown", ItemApp, ItemValues{}, `<a href="{{.Href}}" title="{{.Name}}">open</a>`, ReasonTextMissing, FieldName},
		{"the name of an application in a class", ItemApp, ItemValues{}, `<a class="{{.Name}}" href="{{.Href}}">{{.Name}}</a>`, ReasonFieldPosition, "class"},
		{"the address of an application as text", ItemApp, ItemValues{}, `<a href="{{.Href}}">{{.Name}} {{.Href}}</a>`, ReasonAddressPosition, ""},
		{"the list of a group in an application", ItemApp, ItemValues{}, `<a href="{{.Href}}">{{.Name}}</a>{{template "flare/items"}}`, ReasonExecute, ""},
		{"a branch for one set of values only", ItemApp, ItemValues{Description: true}, `<a href="{{.Href}}">{{.Name}}</a>{{if .Has.Description}}<script></script>{{end}}`, ReasonElement, "script"},
		{"a bookmark that is no item of a list", ItemBookmark, ItemValues{}, `<div><a href="{{.Href}}">{{.Name}}</a></div>`, ReasonRoot, "li"},
		{"a bookmark of two items", ItemBookmark, ItemValues{}, `<li><a href="{{.Href}}">{{.Name}}</a></li><li>more</li>`, ReasonRoot, "li"},
		{"text beside the item of a bookmark", ItemBookmark, ItemValues{}, `<li><a href="{{.Href}}">{{.Name}}</a></li> more`, ReasonRoot, "li"},
		{"the icon beside the item of a bookmark", ItemBookmark, ItemValues{}, `{{template "flare/icon"}}<li><a href="{{.Href}}">{{.Name}}</a></li>`, ReasonRoot, "li"},
		{"an item of a list inside something else", ItemBookmark, ItemValues{}, `<ul><li><a href="{{.Href}}">{{.Name}}</a></li></ul>`, ReasonRoot, "li"},
		{"a bookmark that writes nothing", ItemBookmark, ItemValues{}, ``, ReasonLinkCount, "0"},
		{"a group without its bookmarks", ItemGroup, named, `<section><h3>{{.Name}}</h3></section>`, ReasonSlotMissing, SlotItems},
		{"a group with its bookmarks twice", ItemGroup, named, `<h3>{{.Name}}</h3><ul>{{template "flare/items"}}</ul><ul>{{template "flare/items"}}</ul>`, ReasonSlotRepeated, SlotItems},
		{"the bookmarks of a group outside a list", ItemGroup, named, `<section><h3>{{.Name}}</h3>{{template "flare/items"}}</section>`, ReasonSlotPosition, SlotItems},
		{"the bookmarks of a group beside everything", ItemGroup, named, `<h3>{{.Name}}</h3>{{template "flare/items"}}`, ReasonSlotPosition, SlotItems},
		{"the bookmarks of a group deeper in a list", ItemGroup, named, `<h3>{{.Name}}</h3><ul><li>{{template "flare/items"}}</li></ul>`, ReasonSlotPosition, SlotItems},
		{"a group with a name that is not shown", ItemGroup, named, `<ul>{{template "flare/items"}}</ul>`, ReasonTextMissing, FieldName},
		{"a group that cannot be opened and closed", ItemGroup, ItemValues{Named: true, Collapsible: true}, `<section><h3>{{.Name}}</h3><ul>{{template "flare/items"}}</ul></section>`, ReasonElementMissing, "details"},
		{"a group that is opened without a summary", ItemGroup, ItemValues{Named: true, Collapsible: true}, `<details><h3>{{.Name}}</h3><ul>{{template "flare/items"}}</ul></details>`, ReasonElementMissing, "summary"},
		{"the icon of an entry in a group", ItemGroup, named, `<h3>{{.Name}}</h3><ul>{{template "flare/items"}}</ul>{{template "flare/icon"}}`, ReasonExecute, ""},
	} {
		_, err := CompileItem(tc.source, tc.kind, tc.values, pageFuncs())
		if err == nil {
			t.Errorf("%s: accepted", tc.name)
			continue
		}
		reason, subject := reasonOf(t, err)
		if reason != tc.reason || (tc.reason != ReasonExecute && subject != tc.subject) {
			t.Errorf("%s: %s %q", tc.name, reason, subject)
		}
	}

	// A group without a name does not have to show one, and one that is
	// not opened and closed needs neither details nor summary.
	if _, err := CompileItem(`<ul>{{template "flare/items"}}</ul>`, ItemGroup, ItemValues{}, pageFuncs()); err != nil {
		t.Errorf("a group without a name: %v", err)
	}
	// An application can be written as several elements, and with text
	// beside them.
	if _, err := CompileItem(`<a href="{{.Href}}">{{.Name}}</a> · <span>{{.Host}}</span>`, ItemApp, ItemValues{}, pageFuncs()); err != nil {
		t.Errorf("an application of several elements: %v", err)
	}
	// An item writes at most what it may.
	_, err := CompileItem(`<a href="{{.Href}}">{{.Name}}</a>`+strings.Repeat(" ", MaximumItemBytes), ItemApp, ItemValues{}, pageFuncs())
	if reason, subject := reasonOf(t, err); reason != ReasonTooLarge || subject != "65536" {
		t.Errorf("an item that is too large: %s %q", reason, subject)
	}
	// What a kind does not have is empty for it: a bookmark has no
	// description to branch on, and a group has no address.
	if got := compileItem(t, `<li><a href="{{.Href}}">{{.Name}}</a>{{if .Has.Description}}described{{end}}{{.Description}}</li>`, ItemBookmark, ItemValues{Description: true}); strings.Contains(got, "described") || strings.Contains(got, "what it") {
		t.Errorf("a bookmark with a description: %s", got)
	}
	if got := compileItem(t, `<h3>{{.Name}}</h3><p>{{.Href}}</p><ul>{{template "flare/items"}}</ul>`, ItemGroup, ItemValues{Named: true}); !strings.Contains(got, "<p></p>") {
		t.Errorf("a group with an address: %s", got)
	}
}

// A fragment is rendered into a buffer and into the builder of a string
// alike, and a content that writes a part itself is handed what the
// fragment is rendered into.
func TestFragmentRendersIntoABufferAndABuilder(t *testing.T) {
	fragment, err := CompileItem(groupTemplate, ItemGroup, ItemValues{Named: true}, pageFuncs())
	if err != nil {
		t.Fatal(err)
	}
	want := render(fragment, itemEntry())
	var built strings.Builder
	fragment.RenderString(&built, itemEntry())
	if built.String() != want || !strings.Contains(want, "<li>one</li>") {
		t.Errorf("into a builder:\n%s\ninto a buffer:\n%s", built.String(), want)
	}
	var buffer bytes.Buffer
	fragment.Render(&buffer, directItems{itemEntry()})
	built.Reset()
	fragment.RenderString(&built, directItems{itemEntry()})
	direct := strings.Replace(want, "<li>one</li><li>two</li>", "<li>written</li>", 1)
	if buffer.String() != direct || built.String() != direct {
		t.Errorf("written by the content:\n%s\n%s", buffer.String(), built.String())
	}
}

// directItems writes the bookmarks of a group itself.
type directItems struct{ entry }

func (directItems) WriteSlot(out io.Writer, name string) {
	if name == SlotItems {
		_, _ = io.WriteString(out, "<li>written</li>")
	}
}
