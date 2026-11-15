package define

import (
	"bytes"
	"html/template"
	"os"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/themetemplate"
)

// A template of the home page that shows what it was compiled for.
const valuesTemplateSource = `<div class="shell" data-kind="{{.Page.Kind}}" data-lang="{{.Site.Lang}}" data-columns="{{.Layout.DesktopColumns}}" data-frames="{{.Theme.Options.frames}}">` +
	`{{if .Page.Searching}}<em>searching</em>{{end}}{{if .Modules.weather}}<em>weather</em>{{end}}<h1>{{.Site.Title}}</h1><p>{{T "apps"}}</p>` +
	`<img src="{{asset "wallpaper.jpg"}}" alt="">` + everyPagePart + `</div>`

// shown is what a compiled template writes around the parts.
type shown struct{ title string }

func (s shown) Text(string) string                { return s.title }
func (s shown) Address() string                   { return "" }
func (s shown) LinkAttributes() template.HTMLAttr { return "" }
func (s shown) Slot(name string) template.HTML    { return template.HTML("[" + name + "]") }

func renderThemePage(t *testing.T, fragment *themetemplate.Fragment) string {
	t.Helper()
	require.NotNil(t, fragment)
	var out bytes.Buffer
	fragment.Render(&out, shown{title: "Mine & <yours>"})
	return out.String()
}

// allowed are settings under which the templates of zelda are in use.
func allowed() model.Application {
	return model.Application{ThemeTemplates: true, ThemeSettings: map[string]model.ThemeSettings{"zelda": {Templates: true}}}
}

func homePage() ThemePage {
	return ThemePage{Theme: "zelda", Kind: themetemplate.PageHome, Lang: "en", T: func(key string) string { return "<" + key + ">" }, Modules: map[string]bool{"apps": true}}
}

func resetCompiledThemePages(t *testing.T) {
	t.Helper()
	drop := func() {
		compiledThemePages.Lock()
		defer compiledThemePages.Unlock()
		clear(compiledThemePages.entries)
	}
	drop()
	t.Cleanup(drop)
}

func compiledThemePageCount() int {
	compiledThemePages.Lock()
	defer compiledThemePages.Unlock()
	return len(compiledThemePages.entries)
}

// A page takes the template of its theme once the site allows templates
// and those of the theme are confirmed. A preview takes it before the
// confirmation, and nothing takes it on an instance that uses no templates.
func TestThemePageTemplateIsUsedOnceAllowedAndConfirmed(t *testing.T) {
	userThemeDirectory(t, "zelda")
	resetCompiledThemePages(t)
	writeZeldaTheme(t)
	writeThemeTemplate(t, "zelda", "home.html", valuesTemplateSource)
	writeThemeDirectory(t, "bare", userThemeColors)
	ReloadUserThemes()

	require.NotNil(t, ThemePageTemplate(allowed(), homePage()))
	require.Equal(t, []string{"home.html"}, ThemeTemplateFiles("zelda"))
	require.Empty(t, ThemeTemplateFiles("bare"))
	require.Empty(t, ThemeTemplateFiles("absent"))

	off := allowed()
	off.ThemeTemplates = false
	require.Nil(t, ThemePageTemplate(off, homePage()), "the site does not allow templates")
	unconfirmed := model.Application{ThemeTemplates: true}
	require.Nil(t, ThemePageTemplate(unconfirmed, homePage()), "the templates of the theme are not confirmed")
	other := model.Application{ThemeTemplates: true, ThemeSettings: map[string]model.ThemeSettings{"bare": {Templates: true}}}
	require.Nil(t, ThemePageTemplate(other, homePage()), "another theme is confirmed")

	preview := homePage()
	preview.Preview = true
	require.NotNil(t, ThemePageTemplate(unconfirmed, preview), "a preview comes before the confirmation")
	require.Nil(t, ThemePageTemplate(model.Application{}, preview), "a preview needs the site to allow templates")

	for name, page := range map[string]ThemePage{
		"no theme":                    {Kind: themetemplate.PageHome},
		"a theme without templates":   {Theme: "bare", Kind: themetemplate.PageHome},
		"a bundled theme":             {Theme: "blackboard", Kind: themetemplate.PageHome},
		"a theme that is not in use":  {Theme: "absent", Kind: themetemplate.PageHome},
		"the custom colors":           {Theme: model.ThemeCustomName, Kind: themetemplate.PageHome},
		"a preview without templates": {Theme: "bare", Preview: true, Kind: themetemplate.PageHome},
	} {
		settings := allowed()
		settings.ThemeSettings[page.Theme] = model.ThemeSettings{Templates: true}
		require.Nil(t, ThemePageTemplate(settings, page), name)
	}

	// Without the policy that keeps a page from running script, and in
	// safe mode, no template is used, whatever the settings say.
	require.True(t, ThemeTemplatesAvailable())
	AppFlags.DisableCSP = true
	require.False(t, ThemeTemplatesAvailable())
	require.Nil(t, ThemePageTemplate(allowed(), homePage()))
	require.Nil(t, ThemePageTemplate(unconfirmed, preview))
	AppFlags.DisableCSP = false
	AppFlags.ThemeSafeMode = true
	t.Cleanup(func() { AppFlags.DisableCSP, AppFlags.ThemeSafeMode = false, false })
	require.False(t, ThemeTemplatesAvailable())
	require.Nil(t, ThemePageTemplate(allowed(), homePage()))
	AppFlags.ThemeSafeMode = false
	require.NotNil(t, ThemePageTemplate(allowed(), homePage()))
}

// A template is compiled for the values of the page: the page it is for,
// the language, whether a search is shown, the parts that are shown, the
// layout in effect and the values of the options of the theme. The title of
// the site is filled in when the page is rendered, and escaped.
func TestThemePageTemplateIsCompiledForTheValuesOfThePage(t *testing.T) {
	userThemeDirectory(t, "zelda")
	resetCompiledThemePages(t)
	writeZeldaTheme(t)
	writeThemeTemplate(t, "zelda", "home.html", valuesTemplateSource)
	ReloadUserThemes()
	hash := userTheme(t, "zelda").Hash

	page := renderThemePage(t, ThemePageTemplate(allowed(), homePage()))
	require.Equal(t, `<div class="shell" data-kind="home" data-lang="en" data-columns="legacy" data-frames="false">`+
		`<h1>Mine &amp; &lt;yours&gt;</h1><p>&lt;apps&gt;</p><img src="/theme/zelda/`+hash+`/assets/wallpaper.jpg" alt="">`+
		`[flare/search][flare/clock][flare/greeting][flare/weather][flare/subpage-header][flare/apps][flare/bookmarks][flare/toolbar][flare/footer]</div>`, page)

	settings := allowed()
	settings.Homepage = &model.HomepageOptions{DesktopColumns: "6"}
	settings.ThemeSettings["zelda"] = model.ThemeSettings{Templates: true, Options: map[string]string{"frames": "true"}}
	request := homePage()
	request.Lang, request.Searching, request.Modules = "zh", true, map[string]bool{"weather": true}
	page = renderThemePage(t, ThemePageTemplate(settings, request))
	require.Contains(t, page, `data-lang="zh" data-columns="6" data-frames="true"><em>searching</em><em>weather</em>`)

	// A value the option cannot have is its default, as everywhere.
	settings.ThemeSettings["zelda"] = model.ThemeSettings{Templates: true, Options: map[string]string{"frames": "sometimes", "absent": "x"}}
	require.Contains(t, renderThemePage(t, ThemePageTemplate(settings, homePage())), `data-frames="false"`)

	// The application page and the bookmark page take the template of the
	// home page from a theme that has none for them, compiled for them.
	for _, kind := range []themetemplate.PageKind{themetemplate.PageApplications, themetemplate.PageBookmarks} {
		request = homePage()
		request.Kind = kind
		require.Contains(t, renderThemePage(t, ThemePageTemplate(allowed(), request)), `data-kind="`+string(kind)+`"`)
	}

	// A template of the page itself comes first. It places what that page
	// shows; the parts it leaves out are not on the page.
	writeThemeTemplate(t, "zelda", "bookmarks.html", `<section data-kind="{{.Page.Kind}}">{{template "flare/subpage-header"}}{{template "flare/bookmarks"}}{{template "flare/toolbar"}}{{template "flare/footer"}}</section>`)
	ReloadUserThemes()
	require.Equal(t, []string{"home.html", "bookmarks.html"}, ThemeTemplateFiles("zelda"))
	request = homePage()
	request.Kind = themetemplate.PageBookmarks
	require.Equal(t, `<section data-kind="bookmarks">[flare/subpage-header][flare/bookmarks][flare/toolbar][flare/footer]</section>`, renderThemePage(t, ThemePageTemplate(allowed(), request)))
	request.Kind = themetemplate.PageApplications
	require.Contains(t, renderThemePage(t, ThemePageTemplate(allowed(), request)), `class="shell" data-kind="applications"`)

	// Where the template of the home page stands in for another page, it
	// has to place what that page shows, and can leave out the rest there.
	writeThemeTemplate(t, "zelda", "home.html", `<div>{{if eq .Page.Kind "home"}}`+everyPagePart+`{{else}}{{template "flare/subpage-header"}}{{template "flare/apps"}}{{template "flare/toolbar"}}{{template "flare/footer"}}{{end}}</div>`)
	ReloadUserThemes()
	require.Equal(t, `<div>[flare/subpage-header][flare/apps][flare/toolbar][flare/footer]</div>`, renderThemePage(t, ThemePageTemplate(allowed(), request)))
	require.Contains(t, renderThemePage(t, ThemePageTemplate(allowed(), homePage())), `[flare/search][flare/clock]`)
	// The bookmark page has a template of its own; without it, the template
	// of the home page would lack the bookmarks there.
	require.NoError(t, os.Remove(filepath.Join(UserThemeDirectory, "zelda", "templates", "bookmarks.html")))
	ReloadUserThemes()
	request.Kind = themetemplate.PageBookmarks
	require.Nil(t, ThemePageTemplate(allowed(), request))
	require.Equal(t, []ThemeProblem{{File: "zelda/templates/home.html", Reason: "template_slot_missing", Subject: "flare/bookmarks"}}, ThemePageTemplateProblems("zelda"))
}

// The address of an asset holds the state of the theme. A template that
// names one is compiled anew when the assets change, so that a page never
// names an address that is no longer served.
func TestCompiledThemePageFollowsTheAssetsOfTheTheme(t *testing.T) {
	userThemeDirectory(t, "zelda")
	resetCompiledThemePages(t)
	writeZeldaTheme(t)
	writeThemeTemplate(t, "zelda", "home.html", valuesTemplateSource)
	ReloadUserThemes()
	before := userTheme(t, "zelda").Hash
	require.Contains(t, renderThemePage(t, ThemePageTemplate(allowed(), homePage())), `src="/theme/zelda/`+before+`/assets/wallpaper.jpg"`)

	writeThemeAsset(t, "zelda", "wallpaper.jpg", assetJPEG+"more")
	ReloadUserThemes()
	after := userTheme(t, "zelda").Hash
	require.NotEqual(t, before, after)
	require.Contains(t, renderThemePage(t, ThemePageTemplate(allowed(), homePage())), `src="/theme/zelda/`+after+`/assets/wallpaper.jpg"`)
}

// A template is compiled once for a set of values and kept. An edit of the
// template is another template, and what is kept has a limit.
func TestCompiledThemePagesAreKept(t *testing.T) {
	userThemeDirectory(t, "zelda")
	resetCompiledThemePages(t)
	writeZeldaTheme(t)
	writeThemeTemplate(t, "zelda", "home.html", valuesTemplateSource)
	ReloadUserThemes()

	first := ThemePageTemplate(allowed(), homePage())
	require.Same(t, first, ThemePageTemplate(allowed(), homePage()))
	require.Equal(t, 1, compiledThemePageCount())

	// Each value a template can branch on is part of what it is kept under.
	changes := []func(*model.Application, *ThemePage){
		func(_ *model.Application, page *ThemePage) { page.Kind = themetemplate.PageApplications },
		func(_ *model.Application, page *ThemePage) { page.Lang = "zh" },
		func(_ *model.Application, page *ThemePage) { page.Searching = true },
		func(_ *model.Application, page *ThemePage) {
			page.Modules = map[string]bool{"apps": true, "footer": true}
		},
		func(settings *model.Application, _ *ThemePage) {
			settings.Homepage = &model.HomepageOptions{BookmarkLayout: "sections"}
		},
		func(settings *model.Application, _ *ThemePage) {
			settings.ThemeSettings["zelda"] = model.ThemeSettings{Templates: true, Options: map[string]string{"frames": "true"}}
		},
	}
	for i, change := range changes {
		settings, page := allowed(), homePage()
		change(&settings, &page)
		fragment := ThemePageTemplate(settings, page)
		require.NotSame(t, first, fragment, "change %d", i)
		require.Same(t, fragment, ThemePageTemplate(settings, page), "change %d", i)
	}
	require.Equal(t, 1+len(changes), compiledThemePageCount())
	// What a template cannot branch on is not: a setting of the site that
	// is no layout a theme can specify, and a part that has no name.
	settings, page := allowed(), homePage()
	settings.Homepage = &model.HomepageOptions{NavigationTextSize: "large"}
	settings.Title = "Another"
	page.Modules = map[string]bool{"apps": true, "other": true}
	require.Same(t, first, ThemePageTemplate(settings, page))

	// An edited template is compiled anew.
	writeThemeTemplate(t, "zelda", "home.html", `<div class="edited">`+everyPagePart+`</div>`)
	ReloadUserThemes()
	edited := ThemePageTemplate(allowed(), homePage())
	require.Contains(t, renderThemePage(t, edited), `class="edited"`)
	require.NotSame(t, first, edited)

	// Everything kept is dropped when the limit is reached, so that what
	// is kept cannot grow without end.
	compiledThemePages.Lock()
	clear(compiledThemePages.entries)
	for i := range maximumCompiledThemePages - 1 {
		compiledThemePages.entries[strconv.Itoa(i)] = compiledThemePage{}
	}
	compiledThemePages.Unlock()
	require.NotNil(t, ThemePageTemplate(allowed(), homePage()))
	require.Equal(t, maximumCompiledThemePages, compiledThemePageCount())
	page = homePage()
	page.Lang = "zh"
	require.NotNil(t, ThemePageTemplate(allowed(), page))
	require.Equal(t, 1, compiledThemePageCount())
}

// A template is checked with every part shown and the options of the theme
// at their defaults. One that does not compile for other values leaves the
// page with the markup of the application, is compiled once for them, and
// is listed for the theme.
func TestThemePageTemplateThatDoesNotCompileForSomeValues(t *testing.T) {
	userThemeDirectory(t, "zelda")
	resetCompiledThemePages(t)
	writeZeldaTheme(t)
	writeThemeTemplate(t, "zelda", "home.html", `<div>{{if eq .Theme.Options.frames "true"}}<script>x()</script>{{end}}{{if eq .Site.Lang "zh"}}{{template "flare/apps"}}{{end}}`+everyPagePart+`</div>`)
	writeThemeTemplate(t, "zelda", "bookmarks.html", applicationsTemplateSourceFor("bookmarks"))
	ReloadUserThemes()
	require.Equal(t, []string{"home.html", "bookmarks.html"}, ThemeTemplateFiles("zelda"))
	require.NotNil(t, ThemePageTemplate(allowed(), homePage()))
	require.Empty(t, ThemePageTemplateProblems("zelda"))

	framed := allowed()
	framed.ThemeSettings["zelda"] = model.ThemeSettings{Templates: true, Options: map[string]string{"frames": "true"}}
	require.Nil(t, ThemePageTemplate(framed, homePage()))
	kept := compiledThemePageCount()
	require.Nil(t, ThemePageTemplate(framed, homePage()))
	require.Equal(t, kept, compiledThemePageCount(), "a template that does not compile is not compiled for each page")
	require.Equal(t, []ThemeProblem{{File: "zelda/templates/home.html", Reason: "template_element", Subject: "script"}}, ThemePageTemplateProblems("zelda"))

	// The same reason for other values is listed once, another reason and
	// another template by themselves.
	searching := homePage()
	searching.Searching = true
	require.Nil(t, ThemePageTemplate(framed, searching))
	chinese := homePage()
	chinese.Lang = "zh"
	require.Nil(t, ThemePageTemplate(allowed(), chinese))
	bookmarks := homePage()
	bookmarks.Kind = themetemplate.PageBookmarks
	require.Nil(t, ThemePageTemplate(framed, bookmarks), "the template of the page is not replaced by the one of the home page")
	require.Equal(t, []ThemeProblem{
		{File: "zelda/templates/bookmarks.html", Reason: "template_slot_missing", Subject: "flare/subpage-header"},
		{File: "zelda/templates/home.html", Reason: "template_element", Subject: "script"},
		{File: "zelda/templates/home.html", Reason: "template_slot_repeated", Subject: "flare/apps"},
	}, ThemePageTemplateProblems("zelda"))
	require.Empty(t, ThemePageTemplateProblems("blackboard"))
	require.Empty(t, ThemePageTemplateProblems("absent"))

	// A corrected template has nothing listed.
	writeThemeTemplate(t, "zelda", "home.html", `<div class="corrected">`+everyPagePart+`</div>`)
	writeThemeTemplate(t, "zelda", "bookmarks.html", `<div>{{template "flare/subpage-header"}}{{template "flare/bookmarks"}}{{template "flare/toolbar"}}{{template "flare/footer"}}</div>`)
	ReloadUserThemes()
	require.NotNil(t, ThemePageTemplate(framed, homePage()))
	require.Empty(t, ThemePageTemplateProblems("zelda"))
}

// applicationsTemplateSourceFor is a template that places the parts of a
// page only where the option of the theme is off, as it is by default.
func applicationsTemplateSourceFor(list string) string {
	return `<div>{{if eq .Theme.Options.frames "false"}}{{template "flare/subpage-header"}}{{template "flare/` + list + `"}}{{template "flare/toolbar"}}{{template "flare/footer"}}{{end}}</div>`
}
