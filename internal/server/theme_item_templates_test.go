package server

import (
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
)

const (
	itemAppTemplate = `<div class="t-app" data-new="{{.NewTab}}" data-icon="{{.Has.Icon}}" data-desc="{{.Has.Description}}" data-frames="{{.Theme.Options.frames}}">` +
		`<a href="{{.Href}}" title="{{.Name}}">{{template "flare/icon"}}<strong>{{.Name}}</strong><small>{{.Description}}</small><em>{{.Host}}</em><b>{{.Initial}}</b></a></div>`
	itemBookmarkTemplate = `<li class="t-mark" data-new="{{.NewTab}}" data-icon="{{.Has.Icon}}"><a href="{{.Href}}">{{template "flare/icon"}}<span>{{.Name}}</span><em>{{.Host}}</em><b>{{.Initial}}</b></a></li>`
	itemGroupTemplate    = `{{if .Collapsible}}<details class="t-group"><summary>{{.Name}}</summary><ul>{{template "flare/items"}}</ul></details>` +
		`{{else}}<section class="t-group" data-named="{{.Has.Name}}">{{if .Has.Name}}<h3>{{.Name}}</h3>{{end}}<ol>{{template "flare/items"}}</ol></section>{{end}}`
	itemThemeFile = themeFileColors + "options:\n  - id: frames\n    type: boolean\n"
	itemsAllowed  = "ThemeTemplates: true\nThemeSettings:\n    zelda:\n        Templates: true\n"
)

func itemEntries(t *testing.T, categories bool) {
	t.Helper()
	require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{
		{Name: `Alpha & "<one>"`, URL: "https://alpha.example/path?a=1&b=2", Desc: "first <app>", Icon: "github"},
		{Name: " beta", URL: "http://beta.example:8080/"},
	}}))
	bookmarks := model.Bookmarks{Items: []model.Bookmark{
		{Name: `Gamma & <two>`, URL: "https://gamma.example/", Icon: "github", Category: "one"},
		{Name: "delta", URL: "https://delta.example/", Category: "two"},
	}}
	if categories {
		bookmarks.Categories = []model.Category{{ID: "one", Name: `One & <first>`}, {ID: "two", Name: "Two"}}
	} else {
		bookmarks.Items[0].Category, bookmarks.Items[1].Category = "", ""
	}
	require.True(t, data.SaveNormalBookmarks(bookmarks))
	t.Cleanup(func() {
		require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{}))
		require.True(t, data.SaveNormalBookmarks(model.Bookmarks{}))
	})
}

// between returns what stands in a page between two marks.
func between(t *testing.T, page, from, to string) string {
	t.Helper()
	_, rest, found := strings.Cut(page, from)
	require.True(t, found, from)
	part, _, found := strings.Cut(rest, to)
	require.True(t, found, to)
	return part
}

// withoutIcons takes the markup of the icons out, which is the
// application's and long.
func withoutIcons(markup string) string {
	for {
		before, rest, found := strings.Cut(markup, "<svg")
		if !found {
			return markup
		}
		_, after, _ := strings.Cut(rest, "</svg>")
		markup = before + "[icon]" + after
	}
}

// The entries of the navigation pages are written with the item templates
// of the theme: an application, a bookmark and a group each with its own.
// The name, the description, the host and the initial are filled in
// escaped, the address is the one the application links to, and the link
// has the attributes the application gives it.
func TestEntriesAreRenderedWithTheItemTemplatesOfTheThemeHTTP(t *testing.T) {
	const base = "Locale: en\nTheme: zelda\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\n"
	files := map[string]string{
		"zelda/theme.yml": itemThemeFile, "zelda/templates/app.html": itemAppTemplate,
		"zelda/templates/bookmark.html": itemBookmarkTemplate, "zelda/templates/group.html": itemGroupTemplate,
	}
	handler := themeFilesRouter(t, base+"OpenAppNewTab: true\nOpenBookmarkNewTab: false\n"+itemsAllowed, files)
	itemEntries(t, true)
	configure := func(settings string) {
		t.Helper()
		require.NoError(t, os.WriteFile("config.yml", []byte(base+settings), 0600))
	}
	apps := func(page string) string {
		return withoutIcons(between(t, page, `<div class="apps-container clearfix">`, "</div></div>\n"))
	}
	bookmarks := func(page string) string {
		return withoutIcons(between(t, page, `<div class=bookmark-groups>`, `</div></div><div class=toolbar-container>`))
	}
	alphaName := `Alpha &amp; &#34;&lt;one&gt;&#34;`
	alpha := func(link, described string) string {
		small := `<small>first &lt;app&gt;</small>`
		if described == "false" {
			small = `<small></small>`
		}
		return `<a href="https://alpha.example/path?a=1&amp;b=2" title="` + alphaName + `"` + link + `>[icon]<strong>` + alphaName + `</strong>` + small + `<em>alpha.example</em><b>A</b></a></div>`
	}
	beta := func(link string) string {
		return `<a href="http://beta.example:8080/" title=" beta"` + link + `><strong> beta</strong><small></small><em>beta.example</em><b>b</b></a></div>`
	}
	newTab, sameTab := ` target="_blank" rel="noopener"`, ` rel="noopener"`
	gamma := func(link, opens string) string {
		return `<li class="t-mark" data-new="` + opens + `" data-icon="true"><a href="https://gamma.example/"` + link + `>[icon]<span>Gamma &amp; &lt;two&gt;</span><em>gamma.example</em><b>G</b></a></li>`
	}
	delta := func(link, opens string) string {
		return `<li class="t-mark" data-new="` + opens + `" data-icon="false"><a href="https://delta.example/"` + link + `><span>delta</span><em>delta.example</em><b>d</b></a></li>`
	}

	// Applications open in a new window, bookmarks do not; every category
	// is shown.
	page := getPage(t, handler, "/")
	require.Equal(t,
		`<div class="t-app" data-new="true" data-icon="true" data-desc="true" data-frames="false">`+alpha(newTab, "true")+
			`<div class="t-app" data-new="true" data-icon="false" data-desc="false" data-frames="false">`+beta(newTab), apps(page))
	require.Equal(t,
		`<section class="t-group" data-named="true"><h3>One &amp; &lt;first&gt;</h3><ol>`+gamma(sameTab, "false")+`</ol></section>`+
			`<section class="t-group" data-named="true"><h3>Two</h3><ol>`+delta(sameTab, "false")+`</ol></section>`, bookmarks(page))
	for _, builtin := range []string{`class="app-item"`, `class="app-container"`, `class="bookmark"`, `bookmark-group-container`, `bookmark-list`} {
		require.NotContains(t, apps(page)+bookmarks(page), builtin)
	}
	// The same entries on the pages of their own.
	require.Equal(t, apps(page), apps(getPage(t, handler, "/applications")))
	require.Equal(t, bookmarks(page), bookmarks(getPage(t, handler, "/bookmarks")))

	// Other settings show on the next page: where entries open, hidden
	// descriptions, an option of the theme.
	configure("OpenAppNewTab: false\nOpenBookmarkNewTab: true\nHomepage:\n    HideAppDescriptions: true\n" +
		"ThemeTemplates: true\nThemeSettings:\n    zelda:\n        Templates: true\n        Options:\n            frames: \"true\"\n")
	page = getPage(t, handler, "/")
	require.Equal(t,
		`<div class="t-app" data-new="false" data-icon="true" data-desc="false" data-frames="true">`+alpha(sameTab, "false")+
			`<div class="t-app" data-new="false" data-icon="false" data-desc="false" data-frames="true">`+beta(sameTab), apps(page))
	require.Contains(t, bookmarks(page), gamma(newTab, "true"))
	require.Contains(t, bookmarks(page), delta(newTab, "true"))

	// Categories that are opened and closed, and those that are selected
	// one at a time: the bar that selects them is the application's.
	configure("Homepage:\n    CategoryNavigation: collapse\n" + itemsAllowed)
	require.Equal(t,
		`<details class="t-group"><summary>One &amp; &lt;first&gt;</summary><ul>`+gamma(sameTab, "false")+`</ul></details>`+
			`<details class="t-group"><summary>Two</summary><ul>`+delta(sameTab, "false")+`</ul></details>`, bookmarks(getPage(t, handler, "/")))
	// A search shows every match, so its groups are not closed.
	require.Equal(t, `<section class="t-group" data-named="true"><h3>Two</h3><ol>`+delta(sameTab, "false")+`</ol></section>`, bookmarks(getPage(t, handler, "/?search=delta")))
	configure("Homepage:\n    CategoryNavigation: tabs\n" + itemsAllowed)
	tabs := bookmarks(getPage(t, handler, "/?category=two"))
	require.True(t, strings.HasPrefix(tabs, `<nav class="bookmark-category-nav"`), tabs)
	require.True(t, strings.HasSuffix(tabs, `</nav><section class="t-group" data-named="true"><h3>Two</h3><ol>`+delta(sameTab, "false")+`</ol></section>`), tabs)
	require.NotContains(t, tabs, "gamma.example")

	// A search goes by the entries that are shown, whatever their markup.
	configure(itemsAllowed)
	found := getPage(t, handler, "/?search=alpha")
	require.Contains(t, apps(found), alphaName)
	require.NotContains(t, apps(found), "beta.example")
	require.NotContains(t, found, `<p role=status class=search-empty>`)
	require.Contains(t, getPage(t, handler, "/?search=nothing-of-that-name"), `<p role=status class=search-empty>`)
	found = getPage(t, handler, "/?search=delta")
	require.Contains(t, bookmarks(found), "delta.example")
	require.NotContains(t, found, "alpha.example")
	require.NotContains(t, found, `<p role=status class=search-empty>`)

	// An address that is shown encoded is the encoded one in the template.
	configure("EnableEncryptedLink: true\n" + itemsAllowed)
	encoded := apps(getPage(t, handler, "/"))
	require.NotContains(t, encoded, `href="https://alpha.example`)
	require.Contains(t, encoded, `<a href="/redir/url?`)
	require.Contains(t, encoded, `<em>alpha.example</em>`)
	encoded = bookmarks(getPage(t, handler, "/"))
	require.NotContains(t, encoded, `href="https://delta.example`)
	require.Contains(t, encoded, `data-icon="false"><a href="/redir/url?`)
	require.Contains(t, encoded, `<em>delta.example</em>`)

	// The help page, a page in safe mode and a page whose templates are not
	// confirmed have the markup of the application.
	configure(itemsAllowed)
	for _, path := range []string{"/help", "/?theme-safe-mode"} {
		plain := getPage(t, handler, path)
		require.NotContains(t, plain, "t-app", path)
		require.NotContains(t, plain, "t-mark", path)
		require.Contains(t, plain, `class="app-item"`, path)
	}
	configure("ThemeTemplates: true\n")
	plain := getPage(t, handler, "/")
	require.NotContains(t, plain, "t-app")
	require.Contains(t, plain, `class="app-item"`)
	require.Contains(t, plain, `class="bookmark"`)
	require.Contains(t, plain, `<div class="bookmark-group-container pull-left"><h3 class="bookmark-group-title">`)
	// The administrator can look at them before confirming.
	require.Contains(t, getPage(t, handler, "/?theme-preview=zelda"), `class="t-app"`)
	require.Contains(t, getPage(t, handler, "/bookmarks?theme-preview=zelda"), `class="t-mark"`)
	// The form of the theme lists the item templates with the others.
	fields, shown := templateForm(t, getPage(t, handler, "/settings/theme"), "theme-templates-confirm-form")
	require.True(t, shown)
	require.Equal(t, "app.html,bookmark.html,group.html", fields["theme-templates-files"])
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"action": {"confirm-templates"}, "theme-templates-theme": {"zelda"}, "theme-templates-files": {"app.html,bookmark.html,group.html"}}).Code)
	require.Contains(t, getPage(t, handler, "/"), `class="t-app"`)
	require.Contains(t, getPage(t, handler, "/settings/theme"), strings.ReplaceAll(i18n.T("en", "theme_templates_note_used"), "{files}", "app.html, bookmark.html, group.html"))
}

// A theme can bring any of the three templates alone. What it brings none
// for has the markup of the application, and the two fit together: a
// bookmark is an item of a list either way, and a group holds a list.
func TestThemeBringsSomeOfTheItemTemplatesHTTP(t *testing.T) {
	config := "Locale: en\nTheme: zelda\nShowApps: true\nShowBookmarks: true\n" + itemsAllowed
	files := map[string]string{"zelda/theme.yml": itemThemeFile, "zelda/templates/bookmark.html": itemBookmarkTemplate}
	handler := themeFilesRouter(t, config, files)
	itemEntries(t, true)
	lists := func() string {
		return withoutIcons(between(t, getPage(t, handler, "/"), `<div class=bookmark-groups>`, `</div></div><div class=toolbar-container>`))
	}
	mark := `<li class="t-mark" data-new="false" data-icon="false"><a href="https://delta.example/" rel="noopener"><span>delta</span><em>delta.example</em><b>d</b></a></li>`

	page := getPage(t, handler, "/")
	require.Contains(t, page, `class="app-item"`)
	require.Contains(t, lists(), `<div class="bookmark-group-container pull-left"><h3 class="bookmark-group-title">Two</h3><ul class="bookmark-list">`+mark+`</ul></div>`)

	// The template of a group is added and the one of a bookmark removed,
	// while the instance runs.
	writeThemeFile(t, "zelda/templates/group.html", itemGroupTemplate)
	require.NoError(t, os.Remove("themes/zelda/templates/bookmark.html"))
	require.Contains(t, lists(), `<section class="t-group" data-named="true"><h3>Two</h3><ol><li><a rel="noopener" href="https://delta.example/" class="bookmark"><span>delta</span></a></li></ol></section>`)

	// Only applications.
	require.NoError(t, os.Remove("themes/zelda/templates/group.html"))
	writeThemeFile(t, "zelda/templates/app.html", itemAppTemplate)
	page = getPage(t, handler, "/")
	require.Contains(t, page, `class="t-app"`)
	require.NotContains(t, page, `class="app-item"`)
	require.Contains(t, lists(), `<ul class="bookmark-list"><li><a rel="noopener" href="https://delta.example/" class="bookmark">`)

	// An edit shows on the next page, and so does a template that is no
	// longer accepted: its entries are the application's again, and the
	// theme page says why.
	writeThemeFile(t, "zelda/templates/app.html", strings.Replace(itemAppTemplate, "t-app", "t-app edited", 1))
	require.Contains(t, getPage(t, handler, "/"), `class="t-app edited"`)
	writeThemeFile(t, "zelda/templates/app.html", `<div><strong>{{.Name}}</strong></div>`)
	page = getPage(t, handler, "/")
	require.NotContains(t, page, "t-app")
	require.Contains(t, page, `class="app-item"`)
	require.Equal(t, []string{"zelda/templates/app.html " + i18n.T("en", "theme_file_problem_template_link_count") + " 0"}, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).lineTexts)
}

// The bookmarks of a site without categories are one group without a name.
func TestBookmarksWithoutCategoriesAreOneGroupHTTP(t *testing.T) {
	config := "Locale: en\nTheme: zelda\nShowApps: true\nShowBookmarks: true\n" + itemsAllowed
	files := map[string]string{"zelda/theme.yml": itemThemeFile, "zelda/templates/bookmark.html": itemBookmarkTemplate, "zelda/templates/group.html": itemGroupTemplate}
	handler := themeFilesRouter(t, config, files)
	itemEntries(t, false)
	lists := withoutIcons(between(t, getPage(t, handler, "/"), `<div class=bookmark-groups>`, `</div></div><div class=toolbar-container>`))
	require.True(t, strings.HasPrefix(lists, `<section class="t-group" data-named="false"><ol><li class="t-mark" data-new="false" data-icon="true"><a href="https://gamma.example/" rel="noopener">[icon]<span>Gamma`), lists)
	require.True(t, strings.HasSuffix(lists, `<b>d</b></a></li></ol></section>`), lists)
	require.Equal(t, 1, strings.Count(lists, "t-group"))

	// Without the template of a group, the list is the application's.
	require.NoError(t, os.Remove("themes/zelda/templates/group.html"))
	lists = withoutIcons(between(t, getPage(t, handler, "/"), `<div class=bookmark-groups>`, `</div></div><div class=toolbar-container>`))
	require.True(t, strings.HasPrefix(lists, `<div class="bookmark-group-container pull-left"><ul class="bookmark-list"><li class="t-mark"`), lists)
	require.True(t, strings.HasSuffix(lists, `</li></ul></div>`), lists)
}
