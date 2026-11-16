package define

import (
	"bytes"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/themetemplate"
)

const (
	appItemSource = `<div class="tile" data-lang="{{.Site.Lang}}" data-frames="{{.Theme.Options.frames}}">{{if .NewTab}}<i>new</i>{{end}}` +
		`<a href="{{.Href}}">{{if .Has.Icon}}{{template "flare/icon"}}{{else}}<b>{{.Initial}}</b>{{end}}<strong>{{.Name}}</strong>` +
		`{{if .Has.Description}}<small>{{.Description}}</small>{{else}}<small>{{.Host}}</small>{{end}}</a><img src="{{asset "wallpaper.jpg"}}" alt="{{T "apps"}}"></div>`
	bookmarkItemSource = `<li class="mark"><a href="{{.Href}}">{{if .Has.Icon}}{{template "flare/icon"}}{{end}}<span>{{.Name}}</span></a></li>`
	groupItemSource    = `{{if .Collapsible}}<details><summary>{{.Name}}</summary><ul>{{template "flare/items"}}</ul></details>` +
		`{{else}}<section>{{if .Has.Name}}<h3>{{.Name}}</h3>{{end}}<ul>{{template "flare/items"}}</ul></section>{{end}}`
)

func itemTemplates(t *testing.T, settings model.Application) *ThemeItems {
	t.Helper()
	return ThemeItemTemplates(settings, "zelda", false, "en", func(key string) string { return "<" + key + ">" })
}

func renderThemeItem(t *testing.T, fragment *themetemplate.Fragment) string {
	t.Helper()
	require.NotNil(t, fragment)
	var out bytes.Buffer
	fragment.Render(&out, shown{title: "Mine"})
	return out.String()
}

// A theme brings templates for an application, a bookmark and a group
// beside those for the pages. An item template is checked for every
// combination of what an entry has, and one that does not compile for one
// of them is remarked on and not used for any.
func TestThemeBringsItemTemplates(t *testing.T) {
	userThemeDirectory(t, "blackboard")
	writeZeldaTheme(t)
	writeThemeTemplate(t, "zelda", "home.html", homeTemplateSource)
	writeThemeTemplate(t, "zelda", "app.html", appItemSource)
	writeThemeTemplate(t, "zelda", "bookmark.html", bookmarkItemSource)
	writeThemeTemplate(t, "zelda", "group.html", groupItemSource)
	ReloadUserThemes()
	require.Equal(t, []string{"home.html", "app.html", "bookmark.html", "group.html"}, ThemeTemplateFiles("zelda"))
	require.Empty(t, templateRemarks(t))

	for _, tc := range []struct {
		name, file, source, reason, subject string
	}{
		{"an application without an icon", "app.html", `<a href="{{.Href}}">{{.Name}}</a>{{if not .Has.Icon}}<script></script>{{end}}`, "template_element", "script"},
		{"an application that opens in a new window", "app.html", `<a href="{{.Href}}">{{.Name}}</a>{{if .NewTab}}<iframe></iframe>{{end}}`, "template_element", "iframe"},
		{"an application without a description", "app.html", `<a href="{{.Href}}">{{if .Has.Description}}{{.Name}}{{end}}</a>`, "template_text_missing", "Name"},
		{"a bookmark with an icon", "bookmark.html", `<li><a href="{{.Href}}">{{.Name}}</a></li>{{if .Has.Icon}}<li></li>{{end}}`, "template_root", "li"},
		{"a bookmark that is no item of a list", "bookmark.html", `<div><a href="{{.Href}}">{{.Name}}</a></div>`, "template_root", "li"},
		{"a group that is opened and closed", "group.html", `<section><h3>{{.Name}}</h3><ul>{{template "flare/items"}}</ul></section>`, "template_element_missing", "details"},
		{"a group without a name", "group.html", `{{if .Collapsible}}<details><summary>{{.Name}}</summary><ul>{{template "flare/items"}}</ul></details>{{else if .Has.Name}}<h3>{{.Name}}</h3><ul>{{template "flare/items"}}</ul>{{else}}<p>none</p>{{end}}`, "template_slot_missing", "flare/items"},
		{"a group whose bookmarks are in no list", "group.html", `<h3>{{.Name}}</h3><div>{{template "flare/items"}}</div>`, "template_slot_position", "flare/items"},
	} {
		writeThemeTemplate(t, "zelda", "app.html", appItemSource)
		writeThemeTemplate(t, "zelda", "bookmark.html", bookmarkItemSource)
		writeThemeTemplate(t, "zelda", "group.html", groupItemSource)
		writeThemeTemplate(t, "zelda", tc.file, tc.source)
		ReloadUserThemes()
		require.Equal(t, []ThemeProblem{{File: "zelda/templates/" + tc.file, Reason: tc.reason, Subject: tc.subject}}, templateRemarks(t), tc.name)
		require.NotContains(t, ThemeTemplateFiles("zelda"), tc.file, tc.name)
		require.Len(t, ThemeTemplateFiles("zelda"), 3, tc.name)
		// The theme is in use without the template.
		require.Equal(t, "zelda", userTheme(t, "zelda").Name, tc.name)
	}
}

// The item templates of the selected theme are followed like its other
// files.
func TestItemTemplatesOfTheSelectedThemeAreFollowed(t *testing.T) {
	userThemeDirectory(t, "zelda")
	resetCompiledThemePages(t)
	writeZeldaTheme(t)
	ReloadUserThemes()
	require.Nil(t, itemTemplates(t, allowed()), "a theme without templates")

	for _, file := range []string{"app.html", "bookmark.html", "group.html"} {
		writeThemeTemplate(t, "zelda", file, map[string]string{"app.html": appItemSource, "bookmark.html": bookmarkItemSource, "group.html": groupItemSource}[file])
		GetPageTheme()
		require.Contains(t, ThemeTemplateFiles("zelda"), file, "a template that is added is read by the next page")
	}
	before := itemTemplates(t, allowed()).State()
	writeThemeTemplate(t, "zelda", "bookmark.html", `<li class="edited"><a href="{{.Href}}">{{.Name}}</a></li>`)
	GetPageTheme()
	items := itemTemplates(t, allowed())
	require.NotEqual(t, before, items.State())
	require.Contains(t, renderThemeItem(t, items.Fragment(themetemplate.ItemBookmark, themetemplate.ItemValues{})), `class="edited"`)
}

// The entries of a response take the item templates of its theme under the
// conditions the page takes its template: the site allows templates and
// those of the theme are confirmed, or the response is a preview.
func TestThemeItemTemplatesAreUsedOnceAllowedAndConfirmed(t *testing.T) {
	userThemeDirectory(t, "zelda")
	resetCompiledThemePages(t)
	writeZeldaTheme(t)
	writeThemeTemplate(t, "zelda", "app.html", appItemSource)
	writeThemeDirectory(t, "pages", userThemeColors)
	writeThemeTemplate(t, "pages", "home.html", `<div>`+everyPagePart+`</div>`)
	ReloadUserThemes()
	translate := func(key string) string { return key }

	require.NotNil(t, itemTemplates(t, allowed()))
	off := allowed()
	off.ThemeTemplates = false
	require.Nil(t, itemTemplates(t, off))
	unconfirmed := model.Application{ThemeTemplates: true}
	require.Nil(t, itemTemplates(t, unconfirmed))
	require.NotNil(t, ThemeItemTemplates(unconfirmed, "zelda", true, "en", translate), "a preview comes before the confirmation")
	require.Nil(t, ThemeItemTemplates(model.Application{}, "zelda", true, "en", translate))
	for _, theme := range []string{"", "blackboard", "absent", model.ThemeCustomName} {
		settings := allowed()
		settings.ThemeSettings[theme] = model.ThemeSettings{Templates: true}
		require.Nil(t, ThemeItemTemplates(settings, theme, false, "en", translate), theme)
	}
	// A theme that brings templates for pages only has none for entries.
	settings := model.Application{ThemeTemplates: true, ThemeSettings: map[string]model.ThemeSettings{"pages": {Templates: true}}}
	require.Nil(t, ThemeItemTemplates(settings, "pages", false, "en", translate))
	require.NotNil(t, ThemePageTemplate(settings, ThemePage{Theme: "pages", Kind: themetemplate.PageHome, Lang: "en", T: translate}))

	AppFlags.DisableCSP = true
	t.Cleanup(func() { AppFlags.DisableCSP, AppFlags.ThemeSafeMode = false, false })
	require.Nil(t, itemTemplates(t, allowed()))
	AppFlags.DisableCSP, AppFlags.ThemeSafeMode = false, true
	require.Nil(t, itemTemplates(t, allowed()))
	AppFlags.ThemeSafeMode = false

	// What stands for no templates is empty and has no template for
	// anything, so that it can be asked like any other.
	var none *ThemeItems
	require.Empty(t, none.State())
	require.False(t, none.HideDescriptions())
	require.Nil(t, none.Fragment(themetemplate.ItemApp, themetemplate.ItemValues{}))
}

// A template of an item is compiled for what the entry has, with the
// language and the options of the theme as the page has them, and kept. A
// kind the theme has no template for has none.
func TestThemeItemTemplateIsCompiledForWhatTheEntryHas(t *testing.T) {
	userThemeDirectory(t, "zelda")
	resetCompiledThemePages(t)
	writeZeldaTheme(t)
	writeThemeTemplate(t, "zelda", "app.html", appItemSource)
	writeThemeTemplate(t, "zelda", "group.html", groupItemSource)
	ReloadUserThemes()
	hash := userTheme(t, "zelda").Hash
	image := `<img src="/theme/zelda/` + hash + `/assets/wallpaper.jpg" alt="&lt;apps&gt;">`

	items := itemTemplates(t, allowed())
	require.Equal(t, `<div class="tile" data-lang="en" data-frames="false"><a href=""><b>Mine</b><strong>Mine</strong><small>Mine</small></a>`+image+`</div>`,
		renderThemeItem(t, items.Fragment(themetemplate.ItemApp, themetemplate.ItemValues{})))
	require.Equal(t, `<div class="tile" data-lang="en" data-frames="false"><i>new</i><a href="">[flare/icon]<strong>Mine</strong><small>Mine</small></a>`+image+`</div>`,
		renderThemeItem(t, items.Fragment(themetemplate.ItemApp, themetemplate.ItemValues{Icon: true, Description: true, NewTab: true})))
	require.Equal(t, `<section><h3>Mine</h3><ul>[flare/items]</ul></section>`, renderThemeItem(t, items.Fragment(themetemplate.ItemGroup, themetemplate.ItemValues{Named: true})))
	require.Equal(t, `<details><summary>Mine</summary><ul>[flare/items]</ul></details>`, renderThemeItem(t, items.Fragment(themetemplate.ItemGroup, themetemplate.ItemValues{Named: true, Collapsible: true})))
	require.Equal(t, `<section><ul>[flare/items]</ul></section>`, renderThemeItem(t, items.Fragment(themetemplate.ItemGroup, themetemplate.ItemValues{})))
	require.Nil(t, items.Fragment(themetemplate.ItemBookmark, themetemplate.ItemValues{}), "the theme has no template for a bookmark")

	// What was compiled is kept: for the response, and for the next one.
	first := items.Fragment(themetemplate.ItemApp, themetemplate.ItemValues{Icon: true})
	require.Same(t, first, items.Fragment(themetemplate.ItemApp, themetemplate.ItemValues{Icon: true}))
	kept := compiledThemePageCount()
	require.Same(t, first, itemTemplates(t, allowed()).Fragment(themetemplate.ItemApp, themetemplate.ItemValues{Icon: true}))
	require.Equal(t, kept, compiledThemePageCount())
	require.NotSame(t, first, items.Fragment(themetemplate.ItemApp, themetemplate.ItemValues{Icon: true, Description: true}))
	// A response asks once for a set of values, however many entries have
	// it: what it was given stays with it.
	compiledThemePages.Lock()
	clear(compiledThemePages.entries)
	compiledThemePages.Unlock()
	require.Same(t, first, items.Fragment(themetemplate.ItemApp, themetemplate.ItemValues{Icon: true}))
	require.Zero(t, compiledThemePageCount())
	// What is kept for the next response has the limit the pages have.
	compiledThemePages.Lock()
	for i := range maximumCompiledThemePages {
		compiledThemePages.entries[strconv.Itoa(i)] = compiledThemePage{}
	}
	compiledThemePages.Unlock()
	require.NotNil(t, itemTemplates(t, allowed()).Fragment(themetemplate.ItemApp, themetemplate.ItemValues{}))
	require.Equal(t, 1, compiledThemePageCount())

	// The language, the values of the options and whether the site hides
	// descriptions are part of what the entries of a response stand for.
	state := items.State()
	chinese := ThemeItemTemplates(allowed(), "zelda", false, "zh", func(key string) string { return key })
	require.NotEqual(t, state, chinese.State())
	require.Contains(t, renderThemeItem(t, chinese.Fragment(themetemplate.ItemApp, themetemplate.ItemValues{})), `data-lang="zh"`)
	framed := allowed()
	framed.ThemeSettings["zelda"] = model.ThemeSettings{Templates: true, Options: map[string]string{"frames": "true"}}
	require.NotEqual(t, state, itemTemplates(t, framed).State())
	require.Contains(t, renderThemeItem(t, itemTemplates(t, framed).Fragment(themetemplate.ItemApp, themetemplate.ItemValues{})), `data-frames="true"`)
	hidden := allowed()
	hidden.Homepage = &model.HomepageOptions{HideAppDescriptions: true}
	require.True(t, itemTemplates(t, hidden).HideDescriptions())
	require.False(t, items.HideDescriptions())
	require.NotEqual(t, state, itemTemplates(t, hidden).State())
	require.Equal(t, state, itemTemplates(t, allowed()).State())
	other := allowed()
	other.Homepage, other.Title = &model.HomepageOptions{DesktopColumns: "6"}, "Another"
	require.Equal(t, state, itemTemplates(t, other).State(), "what an item cannot branch on is no part of it")

	// A changed asset is another address, and another state.
	writeThemeAsset(t, "zelda", "wallpaper.jpg", assetJPEG+"more")
	ReloadUserThemes()
	after := itemTemplates(t, allowed())
	require.NotEqual(t, state, after.State())
	require.Contains(t, renderThemeItem(t, after.Fragment(themetemplate.ItemApp, themetemplate.ItemValues{})), `/theme/zelda/`+userTheme(t, "zelda").Hash+`/assets/`)
}

// An item template that does not compile for a value of an option leaves
// the entries with the markup of the application, is compiled once, and is
// listed for the theme.
func TestThemeItemTemplateThatDoesNotCompileForSomeValues(t *testing.T) {
	userThemeDirectory(t, "zelda")
	resetCompiledThemePages(t)
	writeZeldaTheme(t)
	writeThemeTemplate(t, "zelda", "bookmark.html", `<li><a href="{{.Href}}">{{.Name}}</a>{{if eq .Theme.Options.frames "true"}}<script></script>{{end}}</li>`)
	ReloadUserThemes()
	require.NotNil(t, itemTemplates(t, allowed()).Fragment(themetemplate.ItemBookmark, themetemplate.ItemValues{}))
	require.Empty(t, ThemePageTemplateProblems("zelda"))

	framed := allowed()
	framed.ThemeSettings["zelda"] = model.ThemeSettings{Templates: true, Options: map[string]string{"frames": "true"}}
	require.Nil(t, itemTemplates(t, framed).Fragment(themetemplate.ItemBookmark, themetemplate.ItemValues{}))
	kept := compiledThemePageCount()
	require.Nil(t, itemTemplates(t, framed).Fragment(themetemplate.ItemBookmark, themetemplate.ItemValues{}))
	require.Equal(t, kept, compiledThemePageCount())
	require.Nil(t, itemTemplates(t, framed).Fragment(themetemplate.ItemBookmark, themetemplate.ItemValues{Icon: true}))
	require.Equal(t, []ThemeProblem{{File: "zelda/templates/bookmark.html", Reason: "template_element", Subject: "script"}}, ThemePageTemplateProblems("zelda"))
}
