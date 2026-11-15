package themetemplate

import (
	"bytes"
	"errors"
	"html/template"
	"strings"
	"testing"
)

const assetPrefix = "/theme/zelda/0123abcd/assets/"

var testFuncs = template.FuncMap{
	"asset": func(name string) string { return assetPrefix + name },
	"T":     func(key string) string { return "text of " + key },
}

// itemSpec is a template for one entry: a link with a name.
func itemSpec() Spec {
	return Spec{
		Texts: []string{"Name", "Desc", "Host", "Initial"}, RequiredTexts: []string{"Name"}, Link: true,
		Slots: []Slot{{Name: "flare/icon", Inline: true}}, Funcs: testFuncs, AssetPrefix: assetPrefix, Limit: 64 << 10,
	}
}

// pageSpec is a template for a page: parts of the application in markup.
func pageSpec() Spec {
	return Spec{
		Texts: []string{"Title"}, Funcs: testFuncs, AssetPrefix: assetPrefix, Limit: 256 << 10,
		Slots: []Slot{{Name: "flare/search", Required: true}, {Name: "flare/apps", Required: true}, {Name: "flare/weather"}},
	}
}

func itemModel(hasDesc bool) func(Fields) any {
	return func(f Fields) any {
		return map[string]any{
			"Name": f.Text("Name"), "Desc": f.Text("Desc"), "Host": f.Text("Host"), "Initial": f.Text("Initial"),
			"Href": f.Address(), "HasDesc": hasDesc, "Unknown": f.Text("Unknown"),
		}
	}
}

func pageModel(f Fields) any {
	return map[string]any{"Title": f.Text("Title"), "Href": f.Address(), "Searching": false}
}

type entry struct {
	texts map[string]string
	href  string
	link  template.HTMLAttr
	slots map[string]template.HTML
}

func (e entry) Text(field string) string          { return e.texts[field] }
func (e entry) Address() string                   { return e.href }
func (e entry) LinkAttributes() template.HTMLAttr { return e.link }
func (e entry) Slot(name string) template.HTML    { return e.slots[name] }

func render(fragment *Fragment, content Content) string {
	var out bytes.Buffer
	fragment.Render(&out, content)
	return out.String()
}

func reasonOf(t *testing.T, err error) (Reason, string) {
	t.Helper()
	var refused *Error
	if !errors.As(err, &refused) {
		t.Fatalf("not a refusal: %v", err)
	}
	return refused.Reason, refused.Subject
}

const bookmark = `<li class="tile">
  <a href="{{.Href}}" title="{{.Name}}" target="_top" rel="opener">
    {{template "flare/icon"}}
    <span class="name">{{.Name}}</span>{{if .HasDesc}}<small>{{.Desc}}</small>{{end}}
  </a>
</li>`

func TestItemIsFilledInWithoutRunningTheTemplate(t *testing.T) {
	fragment, err := Compile(bookmark, itemSpec(), itemModel(true))
	if err != nil {
		t.Fatal(err)
	}
	want := []Hole{{HoleAddress, ""}, {HoleText, "Name"}, {HoleLink, ""}, {HoleSlot, "flare/icon"}, {HoleText, "Name"}, {HoleText, "Desc"}}
	holes := fragment.Holes()
	if len(holes) != len(want) {
		t.Fatalf("holes: %v", holes)
	}
	for i := range want {
		if holes[i] != want[i] {
			t.Fatalf("hole %d: %v", i, holes[i])
		}
	}
	got := render(fragment, entry{
		texts: map[string]string{"Name": `A "quoted" <b>name</b> & more`, "Desc": "<script>alert(1)</script>"},
		href:  `https://example.test/?a=1&b="2"`, link: ` rel="noopener" target="_blank"`,
		slots: map[string]template.HTML{"flare/icon": `<svg class="icon"></svg>`},
	})
	wantHTML := `<li class="tile">
  <a href="https://example.test/?a=1&amp;b=&#34;2&#34;" title="A &#34;quoted&#34; &lt;b&gt;name&lt;/b&gt; &amp; more" rel="noopener" target="_blank">
    <svg class="icon"></svg>
    <span class="name">A &#34;quoted&#34; &lt;b&gt;name&lt;/b&gt; &amp; more</span><small>&lt;script&gt;alert(1)&lt;/script&gt;</small>
  </a>
</li>`
	if got != wantHTML {
		t.Fatalf("rendered:\n%s", got)
	}
	// Without a description the branch of the template is not part of the
	// fragment, and the size is that of the text between the holes.
	plain, err := Compile(bookmark, itemSpec(), itemModel(false))
	if err != nil {
		t.Fatal(err)
	}
	if len(plain.Holes()) != 5 || strings.Contains(render(plain, entry{}), "<small>") {
		t.Fatalf("holes without a description: %v", plain.Holes())
	}
	if plain.Size() != len(render(plain, entry{})) {
		t.Fatalf("size %d, rendered empty %d", plain.Size(), len(render(plain, entry{})))
	}
}

func TestPagePlacesThePartsOfTheApplication(t *testing.T) {
	source := `<div class="shell"><!-- a note -->
  <header><h1 title="{{.Title}}">{{.Title}}</h1><img src="{{asset "logo.png"}}" srcset="{{asset "logo.png"}} 1x, {{asset "logo-2x.png"}} 2x" alt="{{T "logo"}}"></header>
  <main>{{template "flare/search"}}{{if not .Searching}}{{template "flare/weather"}}{{end}}{{template "flare/apps"}}</main>
  <footer><a href="https://example.test/about" target="_blank" rel="opener">About</a> <a href="/help" target="_self">Help</a> <a href="//example.test/x">Other</a> <a class="plain" class="second">none</a><br/></footer>
</div>`
	fragment, err := Compile(source, pageSpec(), pageModel)
	if err != nil {
		t.Fatal(err)
	}
	got := render(fragment, entry{
		texts: map[string]string{"Title": "Mine & yours"},
		slots: map[string]template.HTML{"flare/search": `<form id="search-form"></form>`, "flare/apps": `<div id="container-apps"></div>`},
	})
	want := `<div class="shell">
  <header><h1 title="Mine &amp; yours">Mine &amp; yours</h1><img src="/theme/zelda/0123abcd/assets/logo.png" srcset="/theme/zelda/0123abcd/assets/logo.png 1x, /theme/zelda/0123abcd/assets/logo-2x.png 2x" alt="text of logo"></header>
  <main><form id="search-form"></form><div id="container-apps"></div></main>
  <footer><a href="https://example.test/about" rel="noopener noreferrer" target="_blank">About</a> <a href="/help" rel="noopener noreferrer">Help</a> <a href="//example.test/x" rel="noopener noreferrer">Other</a> <a class="plain">none</a><br></footer>
</div>`
	if got != want {
		t.Fatalf("rendered:\n%s", got)
	}
}

func TestTemplatesThatAreRefused(t *testing.T) {
	item := func(inner string) string { return `<li><a href="{{.Href}}">{{.Name}}</a>` + inner + `</li>` }
	page := func(inner string) string {
		return `<div>{{template "flare/search"}}{{template "flare/apps"}}` + inner + `</div>`
	}
	cases := []struct {
		name, source string
		page         bool
		reason       Reason
		subject      string
	}{
		{"not a template", `<li>{{.Name</li>`, false, ReasonParse, ""},
		{"a reserved name defined", item(``) + `{{define "flare/icon"}}<b>x</b>{{end}}`, false, ReasonReservedName, "flare/icon"},
		{"another reserved name defined", page(``) + `{{define "flare/footer"}}{{end}}`, true, ReasonReservedName, "flare/footer"},
		{"a template that is not there", item(`{{template "flare/search"}}`), false, ReasonExecute, ""},
		{"a part in the value of an attribute", page(`<p title="{{template "flare/weather"}}">x</p>`), true, ReasonExecute, ""},
		{"more than may be written", item(`{{define "more"}}<b>0123456789</b>{{template "more"}}{{end}}{{template "more"}}`), false, ReasonTooLarge, "65536"},
		{"a doctype", `<!DOCTYPE html>` + item(``), false, ReasonMarkup, "doctype"},
		{"a link inside a link", `<li><a href="{{.Href}}">{{.Name}}<a href="/x">x</a></a></li>`, false, ReasonMarkup, "a link inside a link"},
		{"an element closed in its tag", item(`<span/>`), false, ReasonMarkup, "<span/>"},
		{"an end for an element without one", item(`</br>`), false, ReasonMarkup, "</br>"},
		{"a script", item(`<script>fetch("/")</script>`), false, ReasonElement, "script"},
		{"a stylesheet", item(`<style>li{}</style>`), false, ReasonElement, "style"},
		{"a frame", item(`<iframe src="/"></iframe>`), false, ReasonElement, "iframe"},
		{"a form around a part", `<div><form action="https://collector.example/">{{template "flare/search"}}</form>{{template "flare/apps"}}</div>`, true, ReasonElement, "form"},
		{"a control of a form", item(`<button formaction="https://collector.example/">x</button>`), false, ReasonElement, "button"},
		{"a drawing", item(`<svg><a href="/x"></a></svg>`), false, ReasonElement, "svg"},
		{"text that is not read as markup", item(`<textarea>{{.Name}}</textarea>`), false, ReasonElement, "textarea"},
		{"an element of its own", item(`<x-card>x</x-card>`), false, ReasonElement, "x-card"},
		{"a link to a stylesheet", item(`<link rel="stylesheet" href="https://collector.example/a.css">`), false, ReasonElement, "link"},
		{"the mark of a part written by hand", page(`<flare-slot-0000000000000000 name="flare/weather"></flare-slot-0000000000000000>`), true, ReasonElement, "flare-slot-0000000000000000"},
		{"an event", item(`<span onclick="x()">x</span>`), false, ReasonAttribute, "onclick"},
		{"a style", item(`<span style="background:url(https://collector.example/)">x</span>`), false, ReasonAttribute, "style"},
		{"an id", item(`<span id="search">x</span>`), false, ReasonAttribute, "id"},
		{"a ping", `<li><a href="{{.Href}}" ping="https://collector.example/">{{.Name}}</a></li>`, false, ReasonAttribute, "ping"},
		{"an attribute of another element", item(`<span src="/x">x</span>`), false, ReasonAttribute, "src"},
		{"a target of a frame", item(`<a href="/x" target="other">x</a>`), false, ReasonAttribute, "target"},
		{"a name that is no attribute", item(`<span class="a"">x</span>`), false, ReasonExecute, ""},
		{"a data attribute with a name that is not plain", item(`<span data-a_b="1">x</span>`), false, ReasonAttribute, "data-a_b"},
		{"a data attribute without a name", item(`<span data-="1">x</span>`), false, ReasonAttribute, "data-"},
		{"an element not closed", `<li><a href="{{.Href}}">{{.Name}}</a>`, false, ReasonUnbalanced, "li"},
		{"an element closed that is not open", item(``) + `</div>`, false, ReasonUnbalanced, "div"},
		{"elements closed in another order", `<li><a href="{{.Href}}"><b>{{.Name}}</a></b></li>`, false, ReasonUnbalanced, "a"},
		{"a link left open around what follows", page(`<a href="https://collector.example/">`), true, ReasonUnbalanced, "div"},
		{"a name in a class", item(`<span class="{{.Name}}">x</span>`), false, ReasonFieldPosition, "class"},
		{"a name in the address of an image", item(`<img src="https://collector.example/?n={{.Name}}">`), false, ReasonFieldPosition, "src"},
		{"a name in the addresses of an image", item(`<img srcset="{{.Name}} 2x">`), false, ReasonFieldPosition, "srcset"},
		{"a name as an attribute", item(`<span {{.Name}}="1">x</span>`), false, ReasonFieldPosition, "attribute"},
		{"the address as text", item(`<span>{{.Href}}</span>`), false, ReasonAddressPosition, ""},
		{"the address in a title", item(`<span title="{{.Href}}">x</span>`), false, ReasonAddressPosition, ""},
		{"the address behind another address", `<li><a href="https://collector.example/?to={{.Href}}">{{.Name}}</a></li>`, false, ReasonAddressPosition, ""},
		{"a name behind the address", `<li><a href="{{.Href}}?n={{.Name}}">{{.Name}}</a></li>`, false, ReasonAddressPosition, ""},
		{"a name as the address of a link", item(`<a href="{{.Name}}">x</a>`), false, ReasonAddressPosition, ""},
		{"the address in a page", page(`<a href="{{.Href}}">x</a>`), true, ReasonAddressPosition, ""},
		{"a name cut short", item(`<span>{{slice .Name 0 21}}</span>`), false, ReasonComputed, ""},
		{"a field that was not handed out", item(`<span>{{slice .Name 0 20}}9z</span>`), false, ReasonComputed, ""},
		{"a field with a number that is none", item(`<span>{{slice .Name 0 20}}99999999999999999999z</span>`), false, ReasonComputed, ""},
		{"a link that runs a script", item(`<a href="javascript:alert(1)">x</a>`), false, ReasonLink, "javascript:alert(1)"},
		{"a link with data", item(`<a href="data:text/html,x">x</a>`), false, ReasonLink, "data:text/html,x"},
		{"a link with a tab in its scheme", item("<a href=\"java\tscript:alert(1)\">x</a>"), false, ReasonLink, "java\tscript:alert(1)"},
		{"an image of another site", item(`<img src="https://collector.example/a.png">`), false, ReasonResource, "https://collector.example/a.png"},
		{"an image of the instance", item(`<img src="/assets/mdi/home.svg">`), false, ReasonResource, "/assets/mdi/home.svg"},
		{"an asset of another directory", item(`<img src="{{asset "../theme.css"}}">`), false, ReasonResource, assetPrefix + "../theme.css"},
		{"an asset with a query", item(`<img src="{{asset "a.png"}}?x=1">`), false, ReasonResource, assetPrefix + "a.png?x=1"},
		{"one address of several from another site", item(`<img srcset="{{asset "a.png"}} 1x, https://collector.example/b.png 2x">`), false, ReasonResource, assetPrefix + "a.png 1x, https://collector.example/b.png 2x"},
		{"a part inside a link", `<div><a href="/x">{{template "flare/search"}}</a>{{template "flare/apps"}}</div>`, true, ReasonSlotPosition, "flare/search"},
		{"a part that has to be there", `<div>{{template "flare/search"}}</div>`, true, ReasonSlotMissing, "flare/apps"},
		{"a part twice", page(`{{template "flare/apps"}}`), true, ReasonSlotRepeated, "flare/apps"},
		{"a part that can be left out, twice", page(`{{template "flare/weather"}}{{template "flare/weather"}}`), true, ReasonSlotRepeated, "flare/weather"},
		{"no link to the address", `<li><span>{{.Name}}</span></li>`, false, ReasonLinkCount, "0"},
		{"two links to the address", item(`<a href="{{.Href}}">again</a>`), false, ReasonLinkCount, "2"},
		{"the name only in a title", `<li><a href="{{.Href}}" title="{{.Name}}">open</a></li>`, false, ReasonTextMissing, "Name"},
		{"the name only in a comment", `<li><a href="{{.Href}}">open</a><!-- {{.Name}} --></li>`, false, ReasonTextMissing, "Name"},
	}
	for _, tc := range cases {
		spec, model := itemSpec(), itemModel(true)
		if tc.page {
			spec, model = pageSpec(), pageModel
		}
		fragment, err := Compile(tc.source, spec, model)
		if err == nil {
			t.Errorf("%s: compiled to %q", tc.name, render(fragment, entry{}))
			continue
		}
		reason, subject := reasonOf(t, err)
		if reason != tc.reason {
			t.Errorf("%s: reason %s (%s), want %s", tc.name, reason, subject, tc.reason)
			continue
		}
		// What the template engine says is not compared.
		if tc.reason != ReasonParse && tc.reason != ReasonExecute && subject != tc.subject {
			t.Errorf("%s: subject %q, want %q", tc.name, subject, tc.subject)
		}
		if !strings.Contains(err.Error(), string(tc.reason)) {
			t.Errorf("%s: %v", tc.name, err)
		}
	}
}

// What a template does with a field decides nothing about what is shown: it
// compares a placeholder, so every entry goes the same way, and the value of
// an entry reaches the page only through a hole.
func TestTemplateCannotBranchOnTheValueOfAField(t *testing.T) {
	source := `<li><a href="{{.Href}}">{{if eq .Name "secret"}}<b>found</b>{{end}}{{.Name}}{{len .Name}}</a>{{.Unknown}}<i>{{slice .Name 0 8}}</i></li>`
	fragment, err := Compile(source, itemSpec(), itemModel(false))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"secret", "other"} {
		got := render(fragment, entry{texts: map[string]string{"Name": name}, href: "/x"})
		if strings.Contains(got, "found") || strings.Count(got, name) != 1 {
			t.Fatalf("%s: %s", name, got)
		}
	}
	// Two compilations hand out other placeholders, so a template cannot
	// write one down.
	var first, second string
	keep := func(to *string) func(Fields) any {
		return func(f Fields) any { *to = f.Text("Name") + f.Address(); return itemModel(false)(f) }
	}
	if _, err = Compile(bookmark, itemSpec(), keep(&first)); err != nil {
		t.Fatal(err)
	}
	if _, err = Compile(bookmark, itemSpec(), keep(&second)); err != nil {
		t.Fatal(err)
	}
	if first == second || first == "" {
		t.Fatalf("placeholders: %q and %q", first, second)
	}
}

// Text that a template writes itself is sent in one spelling, whatever
// stands beside a hole, and a link of a template can lead to a page or to
// an address for mail.
func TestTextOfTheTemplateIsWrittenOutEscaped(t *testing.T) {
	source := `<li><a href="{{.Href}}" title="Tom &amp; &quot;Jerry&quot;: {{.Name}} &lt;3">1 &lt; 2 &amp; {{.Name}} &gt; 0</a><a href="http://example.test/">a</a><a href="mailto:someone@example.test">b</a></li>`
	fragment, err := Compile(source, itemSpec(), itemModel(false))
	if err != nil {
		t.Fatal(err)
	}
	got := render(fragment, entry{texts: map[string]string{"Name": "N"}, href: "/x"})
	want := `<li><a href="/x" title="Tom &amp; &#34;Jerry&#34;: N &lt;3">1 &lt; 2 &amp; N &gt; 0</a><a href="http://example.test/" rel="noopener noreferrer">a</a><a href="mailto:someone@example.test" rel="noopener noreferrer">b</a></li>`
	if got != want {
		t.Fatalf("rendered:\n%s", got)
	}
}

// Without a prefix for assets no address that loads something is accepted,
// and a template without a link cannot use the address at all.
func TestSpecLimitsWhatATemplateCanName(t *testing.T) {
	spec := itemSpec()
	spec.AssetPrefix = ""
	_, err := Compile(`<li><a href="{{.Href}}">{{.Name}}</a><img src="{{asset "a.png"}}"></li>`, spec, itemModel(false))
	if reason, _ := reasonOf(t, err); reason != ReasonResource {
		t.Fatalf("reason %s", reason)
	}
	_, err = Compile(`<li><a href="{{.Href}}">{{.Name}}</a><img src="a.png"></li>`, spec, itemModel(false))
	if reason, subject := reasonOf(t, err); reason != ReasonResource || subject != "a.png" {
		t.Fatalf("reason %s %s", reason, subject)
	}
	spec = itemSpec()
	spec.Limit = 10
	_, err = Compile(bookmark, spec, itemModel(false))
	if reason, subject := reasonOf(t, err); reason != ReasonTooLarge || subject != "10" {
		t.Fatalf("reason %s %s", reason, subject)
	}
	if (&Error{Reason: ReasonMarkup}).Error() != "theme template: markup" {
		t.Fatal("an error without a subject")
	}
}
