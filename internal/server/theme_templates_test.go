package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
)

// The theme page shows, on the card of a theme, the page templates it brings
// and that they are not applied, and lists a template that is not accepted.
// The pages of a theme with templates are those of the application.
func TestThemePageTemplatesHTTP(t *testing.T) {
	parts := `{{template "flare/search"}}{{template "flare/clock"}}{{template "flare/greeting"}}{{template "flare/weather"}}` +
		`{{template "flare/subpage-header"}}{{template "flare/apps"}}{{template "flare/bookmarks"}}{{template "flare/toolbar"}}{{template "flare/footer"}}`
	files := map[string]string{
		"zelda/theme.yml":                themeFileColors,
		"zelda/templates/home.html":      `<div class="theme-shell">` + parts + `</div>`,
		"zelda/templates/bookmarks.html": `<div onclick="x()">` + parts + `</div>`,
		"plain/theme.yml":                themeFileColors,
	}
	handler := themeFilesRouter(t, "Locale: en\nTheme: zelda\nShowApps: true\nShowBookmarks: true\n", files)

	raw := getPage(t, handler, "/settings/theme")
	note := `<p class="theme-about theme-templates-note">` + strings.ReplaceAll(i18n.T("en", "theme_templates_note"), "{files}", "home.html") + `</p>`
	require.Equal(t, 1, strings.Count(raw, note), "the card of the theme with templates has the note, and no other card")
	page := readThemeFilePage(t, raw)
	require.Equal(t, []string{"zelda/templates/bookmarks.html " + i18n.T("en", "theme_file_problem_template_attribute") + " onclick"}, page.lineTexts)

	// Templates of themes are not applied: the pages have the markup of
	// the application and nothing of the template.
	withTemplates := getPage(t, handler, "/")
	require.NotContains(t, withTemplates, "theme-shell")
	require.Contains(t, withTemplates, "id=hero-container")
	for _, path := range []string{"/applications", "/bookmarks"} {
		require.NotContains(t, getPage(t, handler, path), "theme-shell", path)
	}
}

func postThemeForm(t *testing.T, handler http.Handler, fields url.Values) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/settings/theme", strings.NewReader(fields.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func getResponse(handler http.Handler, path string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

// templateForm reads a form of the theme page by its class: the value of
// each field, " checked" or nothing for a box, and the text of the button.
func templateForm(t *testing.T, page, class string) (map[string]string, bool) {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	var fields map[string]string
	walkNodes(document, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "form" || nodeAttribute(node, "class") != class {
			return
		}
		fields = map[string]string{}
		walkNodes(node, func(input *html.Node) {
			if input.Type != html.ElementNode || input.Data != "input" {
				return
			}
			switch nodeAttribute(input, "type") {
			case "submit":
				fields["submit"] = nodeAttribute(input, "value")
			case "checkbox":
				fields[nodeAttribute(input, "name")] = ""
				for _, attribute := range input.Attr {
					if attribute.Key == "checked" {
						fields[nodeAttribute(input, "name")] = " checked"
					}
				}
			default:
				if name := nodeAttribute(input, "name"); name != "csrf-token" {
					fields[name] = nodeAttribute(input, "value")
				}
			}
		})
	})
	return fields, fields != nil
}

const themeShellTemplate = `<div class="theme-shell" data-kind="{{.Page.Kind}}">{{if .Page.Searching}}<em>searching</em>{{end}}<h1 class="theme-title-shown">{{.Site.Title}}</h1>` +
	`<main>{{template "flare/subpage-header"}}{{template "flare/search"}}{{template "flare/apps"}}{{template "flare/bookmarks"}}</main>` +
	`<aside>{{template "flare/clock"}}{{template "flare/greeting"}}{{template "flare/weather"}}</aside>{{template "flare/toolbar"}}{{template "flare/footer"}}</div>`

// The whole way of a theme with templates, as the administrator goes it on
// the theme page: the site allows templates, the templates of the selected
// theme are confirmed, and the navigation pages are rendered with them.
// Each of the two can be taken back, and the pages are the application's
// own again.
func TestThemeTemplatesAreUsedOnceAllowedAndConfirmedHTTP(t *testing.T) {
	files := map[string]string{
		"zelda/theme.yml":           themeFileColors,
		"zelda/templates/home.html": themeShellTemplate,
		"plain/theme.yml":           themeFileColors,
		"other/theme.yml":           themeFileColors,
		"other/templates/home.html": strings.Replace(themeShellTemplate, "theme-shell", "theme-shell of-another", 1),
	}
	handler := themeFilesRouter(t, "Locale: en\nTitle: Mine & <yours>\nTheme: plain\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowDateTime: true\nShowTitle: true\n", files)
	note := func(page, key string) bool {
		return strings.Contains(page, `<p class="theme-about theme-templates-note">`+strings.ReplaceAll(i18n.T("en", key), "{files}", "home.html")+`</p>`)
	}
	themed := func(path string) bool {
		page := getPage(t, handler, path)
		shell, hero := strings.Contains(page, `<div class="theme-shell"`), strings.Contains(page, "id=hero-container")
		require.NotEqual(t, shell, hero, "%s is rendered with the template or with the markup of the application", path)
		return shell
	}
	navigation := []string{"/", "/applications", "/bookmarks"}

	// Nothing is allowed: the page offers the switch, turned off, and no
	// form about a theme.
	page := getPage(t, handler, "/settings/theme")
	require.True(t, note(page, "theme_templates_note"))
	fields, shown := templateForm(t, page, "theme-templates-form")
	require.True(t, shown)
	require.Equal(t, map[string]string{"action": "save-templates", "theme-templates-allowed": "", "submit": i18n.T("en", "theme_custom_save")}, fields)
	_, shown = templateForm(t, page, "theme-templates-confirm-form")
	require.False(t, shown)
	// Confirming is not possible before the site allows templates.
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"theme": {"zelda"}}).Code)
	require.Equal(t, http.StatusBadRequest, postThemeForm(t, handler, url.Values{"action": {"confirm-templates"}, "theme-templates-theme": {"zelda"}, "theme-templates-files": {"home.html"}}).Code)
	for _, path := range navigation {
		require.False(t, themed(path), path)
	}
	require.False(t, themed("/?theme-preview=zelda"))

	// The site allows templates. The selected theme has a form that lists
	// its templates and confirms them; until then the pages are unchanged.
	rec := postThemeForm(t, handler, url.Values{"action": {"save-templates"}, "theme-templates-allowed": {"1"}})
	require.Equal(t, http.StatusOK, rec.Code)
	page = rec.Body.String()
	require.Contains(t, page, `<p class=theme-notice role=status>`+i18n.T("en", "theme_templates_saved")+`</p>`)
	require.True(t, note(page, "theme_templates_note_unconfirmed"))
	fields, _ = templateForm(t, page, "theme-templates-form")
	require.Equal(t, " checked", fields["theme-templates-allowed"])
	fields, shown = templateForm(t, page, "theme-templates-confirm-form")
	require.True(t, shown)
	require.Equal(t, map[string]string{"action": "confirm-templates", "theme-templates-theme": "zelda", "theme-templates-files": "home.html", "submit": i18n.T("en", "theme_templates_confirm_submit")}, fields)
	require.Contains(t, page, strings.ReplaceAll(i18n.T("en", "theme_templates_confirm_files"), "{files}", "home.html"))
	require.Contains(t, page, i18n.T("en", "theme_templates_confirm_help"))
	fields, shown = templateForm(t, page, "theme-templates-preview-form")
	require.True(t, shown)
	require.Equal(t, map[string]string{"theme-preview": "zelda", "submit": i18n.T("en", "theme_templates_preview")}, fields)
	require.Contains(t, page, `<form action="/" class=theme-templates-preview-form>`, "the form is sent with GET to the home page")
	require.NotContains(t, page, i18n.T("en", "theme_templates_in_use"))
	for _, path := range navigation {
		require.False(t, themed(path), path)
	}
	// The administrator can look at the page with the templates first. That
	// response says what it is and is kept by nothing.
	preview := getResponse(handler, "/?theme-preview=zelda")
	require.Contains(t, preview.Body.String(), `<div class="theme-shell" data-kind="home">`)
	require.Contains(t, preview.Body.String(), i18n.T("en", "theme_templates_preview_page"))
	require.Equal(t, "private, no-store", preview.Header().Get("Cache-Control"))
	require.NotContains(t, getPage(t, handler, "/"), i18n.T("en", "theme_templates_preview_page"))
	// The preview of a theme that is not selected shows its templates
	// before they are confirmed, too.
	require.False(t, themed("/?theme-preview=plain"))
	require.Contains(t, getPage(t, handler, "/?theme-preview=other"), `<div class="theme-shell of-another" data-kind="home">`)
	require.Contains(t, getPage(t, handler, "/bookmarks?theme-preview=other"), `<div class="theme-shell of-another" data-kind="bookmarks">`)

	// A confirmation is about the templates that were listed.
	rec = postThemeForm(t, handler, url.Values{"action": {"confirm-templates"}, "theme-templates-theme": {"zelda"}, "theme-templates-files": {"home.html,bookmarks.html"}})
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Contains(t, rec.Body.String(), i18n.T("en", "theme_templates_changed"))
	require.Equal(t, http.StatusBadRequest, postThemeForm(t, handler, url.Values{"action": {"confirm-templates"}, "theme-templates-theme": {"plain"}, "theme-templates-files": {"home.html"}}).Code)
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.False(t, options.ThemeSettings["zelda"].Templates)

	rec = postThemeForm(t, handler, url.Values{"action": {"confirm-templates"}, "theme-templates-theme": {"zelda"}, "theme-templates-files": {"home.html"}})
	require.Equal(t, http.StatusOK, rec.Code)
	page = rec.Body.String()
	require.Contains(t, page, `<p class=theme-notice role=status>`+i18n.T("en", "theme_templates_confirm_done")+`</p>`)
	require.True(t, note(page, "theme_templates_note_used"))
	require.Contains(t, page, i18n.T("en", "theme_templates_in_use"))
	require.NotContains(t, page, "theme-templates-preview-form")
	fields, _ = templateForm(t, page, "theme-templates-confirm-form")
	require.Equal(t, map[string]string{"action": "withdraw-templates", "theme-templates-theme": "zelda", "theme-templates-files": "home.html", "submit": i18n.T("en", "theme_templates_withdraw_submit")}, fields)

	// The navigation pages are rendered with the template, each for itself;
	// the parts are those of the application, and the title is escaped.
	home := getResponse(handler, "/")
	require.Equal(t, http.StatusOK, home.Code)
	require.NotEqual(t, "private, no-store", home.Header().Get("Cache-Control"))
	body := home.Body.String()
	require.Contains(t, body, `<div class="theme-shell" data-kind="home"><h1 class="theme-title-shown">Mine &amp; &lt;yours&gt;</h1><main>`)
	for _, part := range []string{"id=search-container", "<p id=clock ", "id=plugin-greetings", "id=container-apps", "id=container-bookmarks", "class=toolbar-container", "class=footer-container"} {
		require.Equal(t, 1, strings.Count(body, part), part)
	}
	require.NotContains(t, body, i18n.T("en", "theme_templates_preview_page"))
	require.Less(t, strings.Index(body, "id=container-bookmarks"), strings.Index(body, "<p id=clock "), "the parts stand where the template has them")
	for path, kind := range map[string]string{"/applications": "applications", "/bookmarks": "bookmarks"} {
		body = getPage(t, handler, path)
		require.Contains(t, body, `<div class="theme-shell" data-kind="`+kind+`">`, path)
		require.Contains(t, body, "class=sub-page-header", path)
		require.NotContains(t, body, "id=search-container", path)
	}
	require.Contains(t, getPage(t, handler, "/?search=nothing-of-that-name"), `data-kind="home"><em>searching</em>`)
	// The help page keeps the markup of the application, and so does a page
	// in safe mode and the preview of another theme.
	for _, path := range []string{"/help", "/?theme-safe-mode", "/?theme-preview=plain", "/?theme-preview=blackboard"} {
		require.False(t, themed(path), path)
	}
	// The settings pages are no pages of a theme.
	require.NotContains(t, page, "theme-shell")

	// An edit of the template shows on the next page.
	writeThemeFile(t, "zelda/templates/home.html", strings.Replace(themeShellTemplate, "theme-shell", "theme-shell edited", 1))
	require.Contains(t, getPage(t, handler, "/"), `<div class="theme-shell edited" data-kind="home">`)
	// A template that is no longer accepted leaves the page to the
	// application, and one that is removed does too.
	writeThemeFile(t, "zelda/templates/home.html", `<div class="theme-shell"><script>x()</script></div>`)
	require.NotContains(t, getPage(t, handler, "/"), "theme-shell")
	require.Contains(t, getPage(t, handler, "/"), "id=hero-container")
	writeThemeFile(t, "zelda/templates/home.html", themeShellTemplate)
	require.True(t, themed("/"))

	// Another theme is selected: its pages are the application's, and the
	// confirmation of the first is kept for when it is selected again.
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"theme": {"plain"}}).Code)
	for _, path := range navigation {
		require.False(t, themed(path), path)
	}
	require.True(t, themed("/?theme-preview=zelda"))
	_, shown = templateForm(t, getPage(t, handler, "/settings/theme"), "theme-templates-confirm-form")
	require.False(t, shown, "a theme without templates has nothing to confirm")
	require.Equal(t, http.StatusBadRequest, postThemeForm(t, handler, url.Values{"action": {"confirm-templates"}, "theme-templates-theme": {"plain"}, "theme-templates-files": {""}}).Code)
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"theme": {"zelda"}}).Code)
	require.True(t, themed("/"))

	// The confirmation is withdrawn, then given again and the switch is
	// turned off: either way the pages are the application's.
	rec = postThemeForm(t, handler, url.Values{"action": {"withdraw-templates"}, "theme-templates-theme": {"zelda"}})
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), i18n.T("en", "theme_templates_withdraw_done"))
	require.True(t, note(rec.Body.String(), "theme_templates_note_unconfirmed"))
	for _, path := range navigation {
		require.False(t, themed(path), path)
	}
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"action": {"confirm-templates"}, "theme-templates-theme": {"zelda"}, "theme-templates-files": {"home.html"}}).Code)
	require.True(t, themed("/"))
	rec = postThemeForm(t, handler, url.Values{"action": {"save-templates"}})
	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, note(rec.Body.String(), "theme_templates_note"))
	_, shown = templateForm(t, rec.Body.String(), "theme-templates-confirm-form")
	require.False(t, shown)
	for _, path := range append(navigation, "/?theme-preview=zelda") {
		require.False(t, themed(path), path)
	}
	raw, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Contains(t, string(raw), "ThemeTemplates: false")
	require.Contains(t, string(raw), "Templates: true")
}

// An instance that sends no Content-Security-Policy uses no templates of
// themes, whatever config.yml says, and says so instead of offering the
// setting. The same holds for an instance in safe mode.
func TestThemeTemplatesAreNotUsedWithoutThePolicyHTTP(t *testing.T) {
	files := map[string]string{"zelda/theme.yml": themeFileColors, "zelda/templates/home.html": themeShellTemplate}
	config := "Locale: en\nTheme: zelda\nShowApps: true\nThemeTemplates: true\nThemeSettings:\n    zelda:\n        Templates: true\n"
	handler := themeFilesRouter(t, config, files)
	require.Contains(t, getPage(t, handler, "/"), `<div class="theme-shell"`)

	define.AppFlags.DisableCSP = true
	t.Cleanup(func() { define.AppFlags.DisableCSP = false })
	for _, path := range []string{"/", "/applications", "/bookmarks", "/?theme-preview=zelda"} {
		page := getPage(t, handler, path)
		require.NotContains(t, page, "theme-shell", path)
		require.Contains(t, page, "id=hero-container", path)
	}
	page := getPage(t, handler, "/settings/theme")
	require.Contains(t, page, `<p class="theme-notice theme-templates-unavailable" role=status>`+i18n.T("en", "theme_templates_unavailable_csp")+`</p>`)
	require.Contains(t, page, strings.ReplaceAll(i18n.T("en", "theme_templates_note_unavailable"), "{files}", "home.html"))
	require.NotContains(t, page, "theme-templates-allowed")
	_, shown := templateForm(t, page, "theme-templates-confirm-form")
	require.False(t, shown)
	require.Equal(t, http.StatusBadRequest, postThemeForm(t, handler, url.Values{"action": {"save-templates"}}).Code)
	require.Equal(t, http.StatusBadRequest, postThemeForm(t, handler, url.Values{"action": {"confirm-templates"}, "theme-templates-theme": {"zelda"}, "theme-templates-files": {"home.html"}}).Code)
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.True(t, options.ThemeTemplates, "what is stored is left as it is")
	// Withdrawing a confirmation stays possible.
	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"action": {"withdraw-templates"}, "theme-templates-theme": {"zelda"}}).Code)
	define.AppFlags.DisableCSP = false
	require.NotContains(t, getPage(t, handler, "/"), "theme-shell")

	define.AppFlags.ThemeSafeMode = true
	t.Cleanup(func() { define.AppFlags.ThemeSafeMode = false })
	page = getPage(t, handler, "/settings/theme")
	require.Contains(t, page, `<p class="theme-notice theme-templates-unavailable" role=status>`+i18n.T("en", "theme_templates_unavailable_safe_mode")+`</p>`)
	require.NotContains(t, page, "theme-templates-allowed")
	require.Equal(t, http.StatusBadRequest, postThemeForm(t, handler, url.Values{"action": {"save-templates"}, "theme-templates-allowed": {"1"}}).Code)
}

// What a template is told about the page is what the page shows: which
// parts are on it, the layout in effect, the values of the options of the
// theme and the language.
func TestThemeTemplateIsToldWhatThePageShowsHTTP(t *testing.T) {
	told := `<div class="told" data-lang="{{.Site.Lang}}" data-kind="{{.Page.Kind}}" data-searching="{{.Page.Searching}}" data-shown="` +
		`{{if .Modules.search}}search {{end}}{{if .Modules.clock}}clock {{end}}{{if .Modules.greeting}}greeting {{end}}{{if .Modules.weather}}weather {{end}}` +
		`{{if index .Modules "subpage-header"}}subpage-header {{end}}{{if .Modules.apps}}apps {{end}}{{if .Modules.bookmarks}}bookmarks {{end}}{{if .Modules.toolbar}}toolbar {{end}}{{if .Modules.footer}}footer {{end}}"` +
		` data-layout="{{.Layout.MobileBookmarkLayout}} {{.Layout.TitleSize}} {{.Layout.HideAppDescriptions}} {{.Layout.CategoryNavigation}} {{.Layout.BookmarkLayout}} {{.Layout.DesktopColumns}}"` +
		` data-options="{{.Theme.Options.frames}} {{.Theme.Options.density}}" title="{{T "apps"}}">` +
		`{{template "flare/search"}}{{template "flare/clock"}}{{template "flare/greeting"}}{{template "flare/weather"}}{{template "flare/subpage-header"}}{{template "flare/apps"}}{{template "flare/bookmarks"}}{{template "flare/toolbar"}}{{template "flare/footer"}}</div>`
	theme := themeFileColors + "layout:\n  BookmarkLayout: sections\n  TitleSize: compact\noptions:\n  - id: frames\n    type: boolean\n  - id: density\n    type: enum\n    choices:\n      - value: normal\n      - value: compact\n        layout: { DesktopColumns: 6 }\n"
	files := map[string]string{"zelda/theme.yml": theme, "zelda/templates/home.html": told}
	allowed := "ThemeTemplates: true\nThemeSettings:\n    zelda:\n        Templates: true\n"
	element := func(handler http.Handler, path string) string {
		page := getPage(t, handler, path)
		_, rest, found := strings.Cut(page, `<div class="told" `)
		require.True(t, found, path)
		attributes, _, found := strings.Cut(rest, ">")
		require.True(t, found, path)
		return attributes
	}

	everything := "Locale: en\nTheme: zelda\nFooter: mine\nShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowDateTime: true\nShowTitle: true\n" + allowed
	handler := themeFilesRouter(t, everything, files)
	// The weather is not shown on an instance that is offline.
	require.Equal(t, `data-lang="en" data-kind="home" data-searching="false" data-shown="search clock greeting apps bookmarks toolbar footer "`+
		` data-layout="classic compact false all sections legacy" data-options="false normal" title="`+i18n.T("en", "apps")+`"`, element(handler, "/"))
	require.Equal(t, `data-lang="en" data-kind="home" data-searching="true" data-shown="search clock greeting apps bookmarks toolbar footer "`+
		` data-layout="classic compact false all sections legacy" data-options="false normal" title="`+i18n.T("en", "apps")+`"`, element(handler, "/?search=x"))
	require.Equal(t, `data-lang="en" data-kind="applications" data-searching="false" data-shown="subpage-header apps toolbar "`+
		` data-layout="classic compact false all sections legacy" data-options="false normal" title="`+i18n.T("en", "apps")+`"`, element(handler, "/applications"))
	require.Equal(t, `data-lang="en" data-kind="bookmarks" data-searching="false" data-shown="subpage-header bookmarks toolbar "`+
		` data-layout="classic compact false all sections legacy" data-options="false normal" title="`+i18n.T("en", "apps")+`"`, element(handler, "/bookmarks"))

	// Other settings, without a restart: parts turned off, another
	// language, layout of the site where the theme gives none and where
	// its layout is turned off, and values chosen for the options.
	other := "Locale: zh\nTheme: zelda\nFooter: \"\"\nShowApps: false\nShowBookmarks: true\nShowSearchComponent: false\nShowDateTime: false\nShowTitle: false\nHideSettingButton: true\nHideHelpButton: true\n" +
		"Homepage:\n    MobileBookmarkLayout: compact\n    TitleSize: prominent\n    HideAppDescriptions: true\n    CategoryNavigation: tabs\n" +
		"ThemeTemplates: true\nThemeSettings:\n    zelda:\n        Templates: true\n        IgnoreLayout: [TitleSize]\n        Options:\n            frames: \"true\"\n            density: compact\n"
	require.NoError(t, os.WriteFile("config.yml", []byte(other), 0600))
	require.Equal(t, `data-lang="zh" data-kind="home" data-searching="false" data-shown="bookmarks "`+
		` data-layout="compact prominent true tabs sections 6" data-options="true compact" title="`+i18n.T("zh", "apps")+`"`, element(handler, "/"))
}

// A template that passed its check and does not compile for the settings a
// page has leaves that page to the application, and the theme page lists
// the file with the reason, until the settings or the template change.
func TestThemeTemplateThatDoesNotCompileForTheSettingsIsListedHTTP(t *testing.T) {
	theme := themeFileColors + "options:\n  - id: frames\n    type: boolean\n"
	source := `<div class="theme-shell">{{if eq .Theme.Options.frames "true"}}<iframe></iframe>{{end}}` + strings.TrimSuffix(strings.SplitN(themeShellTemplate, "</h1>", 2)[1], "</div>") + `</div>`
	files := map[string]string{"zelda/theme.yml": theme, "zelda/templates/home.html": source}
	config := "Locale: en\nTheme: zelda\nShowApps: true\nThemeTemplates: true\nThemeSettings:\n    zelda:\n        Templates: true\n"
	handler := themeFilesRouter(t, config, files)
	require.Contains(t, getPage(t, handler, "/"), `<div class="theme-shell">`)
	require.Empty(t, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).lineTexts)

	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"action": {"save-options"}, "theme-options-theme": {"zelda"}, "option-frames": {"true"}}).Code)
	// Nothing is listed before a page was rendered with these settings.
	require.Empty(t, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).lineTexts)
	page := getPage(t, handler, "/")
	require.NotContains(t, page, "theme-shell")
	require.Contains(t, page, "id=hero-container")
	listed := []string{"zelda/templates/home.html " + i18n.T("en", "theme_file_problem_template_element") + " iframe"}
	require.Equal(t, listed, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).lineTexts)
	// The other pages have the same reason, which is listed once.
	getPage(t, handler, "/applications")
	getPage(t, handler, "/?search=x")
	require.Equal(t, listed, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).lineTexts)

	require.Equal(t, http.StatusOK, postThemeForm(t, handler, url.Values{"action": {"reset-options"}, "theme-options-theme": {"zelda"}}).Code)
	require.Contains(t, getPage(t, handler, "/"), `<div class="theme-shell">`)
}

// The guide keeps the markup of the application: its script finds the
// elements it explains by that markup.
func TestGuideUsesNoThemeTemplateHTTP(t *testing.T) {
	files := map[string]string{"zelda/theme.yml": themeFileColors, "zelda/templates/home.html": themeShellTemplate}
	config := "Locale: en\nTheme: zelda\nShowApps: true\nShowBookmarks: true\nThemeTemplates: true\nThemeSettings:\n    zelda:\n        Templates: true\n"
	themeFilesRouter(t, config, files)
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true, EnableGuide: true})
	require.NoError(t, err)
	require.Contains(t, getPage(t, handler, "/"), `<div class="theme-shell"`)
	guide := getPage(t, handler, define.RegularPages.Guide.Path)
	require.Contains(t, guide, "/assets/guide/app.js")
	require.NotContains(t, guide, "theme-shell")
	require.Contains(t, guide, "id=hero-container")
}

// Templates that are not confirmed are shown to the administrator only. A
// visitor who writes the address of a preview gets the page as the site
// shows it, for the selected theme and for any other.
func TestUnconfirmedThemeTemplatesAreNotShownToVisitorsHTTP(t *testing.T) {
	customThemeDirectory(t, "Locale: en\nTheme: zelda\nShowApps: true\nShowBookmarks: true\nThemeTemplates: true\n")
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(define.UserThemeDirectory)) })
	require.NoError(t, os.Mkdir(define.UserThemeDirectory, 0700))
	for file, content := range map[string]string{
		"zelda/theme.yml": themeFileColors, "zelda/templates/home.html": themeShellTemplate,
		"other/theme.yml": themeFileColors, "other/templates/home.html": themeShellTemplate,
	} {
		writeThemeFile(t, file, content)
	}
	handler, err := NewRouter(&model.Flags{
		Port: 5005, EnableOfflineMode: true, Visibility: "DEFAULT",
		CookieName: "flare", CookieSecret: "template-test-signing-key-at-least-32-bytes", User: "template-user", Pass: "template-pass",
	})
	require.NoError(t, err)
	for _, path := range []string{"/", "/?theme-preview=zelda", "/?theme-preview=other", "/applications?theme-preview=zelda", "/bookmarks?theme-preview=other"} {
		rec := getResponse(handler, path)
		require.Equal(t, http.StatusOK, rec.Code, path)
		require.NotContains(t, rec.Body.String(), "theme-shell", path)
		require.NotContains(t, rec.Body.String(), i18n.T("en", "theme_templates_preview_page"), path)
		require.Contains(t, rec.Body.String(), "id=hero-container", path)
	}
	// Once they are confirmed, they are what every visitor gets.
	require.NoError(t, data.SaveThemeTemplatesConfirmed("zelda", true))
	require.Contains(t, getPage(t, handler, "/"), `<div class="theme-shell"`)
	require.NotContains(t, getPage(t, handler, "/?theme-preview=other"), i18n.T("en", "theme_preview_use"))
}
