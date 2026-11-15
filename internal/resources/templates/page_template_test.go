package templates

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/internal/themetemplate"
)

func compileThemePage(t *testing.T, kind themetemplate.PageKind, source string) *themetemplate.Fragment {
	t.Helper()
	fragment, err := themetemplate.CompilePage(source, kind, themetemplate.PageValues{Kind: kind}, themetemplate.PageFuncs{})
	require.NoError(t, err)
	return fragment
}

// A page that hands over the template of its theme is rendered with it in
// place of the skeleton of home.html: the parts are those the application
// renders, in the places the template gives them, and the page around them
// is the same.
func TestHomePageWithTheTemplateOfATheme(t *testing.T) {
	renderer := homeRenderer(t)
	own := func() string {
		var out bytes.Buffer
		require.NoError(t, renderer.Render(&out, homeTemplate, homeData()))
		return out.String()
	}()
	skeleton := execute(t, renderer.templates, homeSkeleton, homeData())
	at := strings.Index(own, skeleton)
	require.Greater(t, at, 0)
	before, after := own[:at], own[at+len(skeleton):]

	source := `<div class="theme-shell"><h1 title="{{.Site.Title}}">{{.Site.Title}}</h1><main>{{template "flare/bookmarks"}}{{template "flare/apps"}}</main>` +
		`<aside>{{template "flare/search"}}{{template "flare/clock"}}{{template "flare/greeting"}}{{template "flare/weather"}}{{template "flare/subpage-header"}}</aside>` +
		`{{template "flare/toolbar"}}{{template "flare/footer"}}</div>`
	data := homeData()
	data["OptionTitle"] = `Mine & "<yours>"`
	data[themetemplate.PageData] = compileThemePage(t, themetemplate.PageHome, source)
	part := func(name string) string { return execute(t, renderer.templates, name, data) }
	want := before + `<div class="theme-shell"><h1 title="Mine &amp; &#34;&lt;yours&gt;&#34;">Mine &amp; &#34;&lt;yours&gt;&#34;</h1><main>` + part("flare/bookmarks") + part("flare/apps") + `</main>` +
		`<aside>` + part("flare/search") + part("flare/clock") + part("flare/greeting") + part("flare/weather") + part("flare/subpage-header") + `</aside>` +
		part("flare/toolbar") + part("flare/footer") + `</div>` + after
	// The title in the head of the page is the one of the data, too.
	want = strings.Replace(want, "<title>"+titleOf(t, own)+"</title>", "<title>Mine &amp; &#34;&lt;yours&gt;&#34;</title>", 1)
	var out bytes.Buffer
	require.NoError(t, renderer.Render(&out, homeTemplate, data))
	require.Equal(t, want, out.String())
	require.NotContains(t, out.String(), "hero-container", "what stands between the parts is the template's")

	// A template for a page places what that page shows. A part it leaves
	// out is not on the page.
	data = homeData()
	data[themetemplate.PageData] = compileThemePage(t, themetemplate.PageBookmarks, `<section>{{template "flare/subpage-header"}}{{template "flare/bookmarks"}}{{template "flare/toolbar"}}{{template "flare/footer"}}</section>`)
	out.Reset()
	require.NoError(t, renderer.Render(&out, homeTemplate, data))
	require.Equal(t, before+`<section>`+part("flare/subpage-header")+part("flare/bookmarks")+part("flare/toolbar")+part("flare/footer")+`</section>`+after, out.String())

	// Anything else under the key leaves the page as the application has it.
	for _, value := range []any{nil, (*themetemplate.Fragment)(nil), "text", 1} {
		data = homeData()
		data[themetemplate.PageData] = value
		out.Reset()
		require.NoError(t, renderer.Render(&out, homeTemplate, data))
		require.Equal(t, own, out.String(), "%v", value)
	}
}

func titleOf(t *testing.T, page string) string {
	t.Helper()
	_, rest, found := strings.Cut(page, "<title>")
	require.True(t, found)
	title, _, found := strings.Cut(rest, "</title>")
	require.True(t, found)
	return title
}
