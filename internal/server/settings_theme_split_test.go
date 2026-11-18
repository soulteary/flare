package server

import (
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/i18n"
)

// themePageForms lists the forms of a page of the settings by their class,
// with the address each submits to. The form that chooses a theme has no
// class and is listed as "choose".
func themePageForms(t *testing.T, page string) map[string]string {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	forms := map[string]string{}
	walkNodes(document, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "form" || nodeAttribute(node, "method") != "POST" {
			return
		}
		name := nodeAttribute(node, "class")
		if name == "" {
			name = "choose"
		}
		require.NotContains(t, forms, name, "the page has the form twice")
		forms[name] = nodeAttribute(node, "action")
	})
	return forms
}

const splitThemeConfig = "Locale: en\nTheme: zelda\nThemeTemplates: true\nShowApps: true\nShowBookmarks: true\n"

func splitThemeFiles() map[string]string {
	return map[string]string{
		"zelda/theme.yml":           themeFileColors + "name: Zelda\noptions:\n  - id: wide\n    type: boolean\n    default: false\n    label: Wide\nmodes:\n  dark:\n    background: \"#000000\"\n",
		"zelda/templates/home.html": themeShellTemplate,
		"rejected.yml":              themeFileColors + "  radius: 4em\n",
	}
}

// The forms about themes stand on three pages: choosing a theme and what
// belongs to the selected one, the files of the themes, and what themes and
// visitors are allowed. Each form submits to the page it stands on.
func TestSettingsOfThemesAreOnThreePagesHTTP(t *testing.T) {
	handler := themeFilesRouter(t, splitThemeConfig, splitThemeFiles())
	want := map[string]map[string]string{
		"/settings/theme": {
			"choose": "/settings/theme", "theme-mode-form": "/settings/theme", "theme-options-form": "/settings/theme",
			"theme-options-reset-form": "/settings/theme", "theme-custom-form": "/settings/theme",
		},
		"/settings/themes": {
			"choose": "/settings/theme", "theme-import-form": "/settings/themes", "theme-install-form": "/settings/themes",
		},
		"/settings/theme-access": {
			"theme-templates-form": "/settings/theme-access", "theme-templates-confirm-form": "/settings/theme-access", "theme-choices-form": "/settings/theme-access",
		},
	}
	seen := map[string]string{}
	for path, forms := range want {
		page := getPage(t, handler, path)
		require.Equal(t, forms, themePageForms(t, page), path)
		for name := range forms {
			if name != "choose" {
				require.Empty(t, seen[name], "%s is on two pages", name)
				seen[name] = path
			}
		}
	}
	require.Len(t, seen, 9)

	// The theme page lists every theme. The theme library lists those the
	// instance brings itself, and says where the others are: a bundled
	// theme and the custom colors have no card there.
	bundled := regexp.MustCompile(`class="?pull-left theme theme-[a-z]+"?`)
	require.Len(t, bundled.FindAllString(getPage(t, handler, "/settings/theme"), -1), len(define.ThemePalettes))
	library := getPage(t, handler, "/settings/themes")
	require.Empty(t, bundled.FindAllString(library, -1))
	require.NotContains(t, library, "export?name="+define.ThemePalettes[0].Name)
	require.Regexp(t, `<p class="theme-help theme-bundled-elsewhere">`+regexp.QuoteMeta(i18n.T("en", "settings_themes_bundled"))+` <a href="?/settings/theme"?>`+i18n.T("en", "nav_theme")+`</a>`, library)
	require.NotContains(t, getPage(t, handler, "/settings/theme"), "theme-bundled-elsewhere\"")
	// Both list the themes of the instance with preview and download, and
	// the remarks about files that are not used. The page of what is
	// allowed lists no theme to choose.
	cards := themeCardLinks(t, getPage(t, handler, "/settings/theme"))
	require.Contains(t, cards, "zelda")
	require.Equal(t, cards, themeCardLinks(t, getPage(t, handler, "/settings/themes")))
	require.Len(t, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).lineTexts, 1)
	require.Equal(t, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).lineTexts, readThemeFilePage(t, getPage(t, handler, "/settings/themes")).lineTexts)
	require.Empty(t, themeCardLinks(t, getPage(t, handler, "/settings/theme-access")))

	// The theme page says where the rest is.
	page := getPage(t, handler, "/settings/theme")
	require.Regexp(t, regexp.QuoteMeta(i18n.T("en", "settings_theme_more_library"))+` <a href="?/settings/themes"?>`+i18n.T("en", "nav_themes")+`</a>`+
		regexp.QuoteMeta(i18n.T("en", "settings_theme_more_access"))+` <a href="?/settings/theme-access"?>`+i18n.T("en", "nav_theme_access")+`</a>`, page)
	require.Contains(t, getPage(t, handler, "/settings/theme-access"), i18n.T("en", "settings_theme_access_help"))
	// Each page begins with a title of its own: that of the theme page for
	// choosing a theme, and for the two others their entry of the menu with
	// a sentence about the page.
	first := regexp.MustCompile(`<div class="?setting-group-container"?>\s*(?:<form[^>]*>\s*(?:<input[^>]*>\s*)*)?<h2>([^<]*)</h2>(?:\s*<p class="help-text settings-page-help">([^<]*))?`)
	for path, want := range map[string][2]string{
		"/settings/theme":        {i18n.T("en", "settings_theme"), ""},
		"/settings/themes":       {i18n.T("en", "nav_themes"), i18n.T("en", "settings_themes_help")},
		"/settings/theme-access": {i18n.T("en", "nav_theme_access"), i18n.T("en", "settings_theme_access_help")},
	} {
		found := first.FindStringSubmatch(getPage(t, handler, path))
		require.NotNil(t, found, path)
		require.Equal(t, want, [2]string{found[1], found[2]}, path)
	}
	require.NotContains(t, getPage(t, handler, "/settings/theme"), i18n.T("en", "settings_themes_help"))
	require.Equal(t, 1, strings.Count(getPage(t, handler, "/settings/themes"), "<h2>"+i18n.T("en", "nav_themes")+"</h2>"))
	require.NotContains(t, getPage(t, handler, "/settings/themes"), "<h2>"+i18n.T("en", "settings_theme")+"</h2>")
	// The link to the repository of themes stands with the import.
	require.NotContains(t, page, define.ThemeRepositoryURL)
	require.Contains(t, getPage(t, handler, "/settings/themes"), define.ThemeRepositoryURL)
}

// A form is taken at the address of the page it stands on, and that page is
// shown afterwards. Sent to another of the three addresses it is refused,
// and nothing is stored.
func TestFormsAboutThemesShowThePageTheyStandOnHTTP(t *testing.T) {
	handler := themeFilesRouter(t, splitThemeConfig, splitThemeFiles())
	forms := map[string]struct {
		fields url.Values
		page   string
		status int
	}{
		"choosing a theme":       {url.Values{"theme": {"tron"}}, "Theme", http.StatusOK},
		"light and dark":         {url.Values{"action": {"save-mode"}, "theme-mode": {"dark"}}, "Theme", http.StatusOK},
		"custom colors":          {url.Values{"action": {"save-custom"}, "custom-background": {"#101010"}, "custom-primary": {"#f0f0f0"}, "custom-accent": {"#808080"}}, "Theme", http.StatusOK},
		"an import without file": {url.Values{"action": {"import"}}, "ThemeLibrary", http.StatusBadRequest},
		"an address to fetch":    {url.Values{"action": {"fetch-theme"}, "theme-address": {"http://example.org/a.yml"}}, "ThemeLibrary", 0},
		"allowing templates":     {url.Values{"action": {"save-templates"}, "theme-templates-allowed": {"1"}}, "ThemeAccess", http.StatusOK},
		"themes for visitors":    {url.Values{"action": {"save-choices"}, "theme-choice-tron": {"1"}}, "ThemeAccess", http.StatusOK},
	}
	names := make([]string, 0, len(forms))
	for name := range forms {
		names = append(names, name)
	}
	sort.Strings(names)
	current := map[string]string{"Theme": "/settings/theme", "ThemeLibrary": "/settings/themes", "ThemeAccess": "/settings/theme-access"}
	for _, name := range names {
		form := forms[name]
		for _, address := range []string{"/settings/theme", "/settings/themes", "/settings/theme-access"} {
			if address != current[form.page] {
				selected, _ := define.ThemeSnapshot()
				rec := postSettingsForm(t, handler, address, form.fields)
				require.Equal(t, http.StatusBadRequest, rec.Code, "%s sent to %s", name, address)
				now, _ := define.ThemeSnapshot()
				require.Equal(t, selected, now, "%s sent to %s", name, address)
				continue
			}
			rec := postSettingsForm(t, handler, address, form.fields)
			if form.status != 0 {
				require.Equal(t, form.status, rec.Code, "%s sent to %s", name, address)
			}
			menu := readSettingsMenu(t, rec.Body.String())
			require.NotNil(t, menu, "%s sent to %s", name, address)
			require.Equal(t, []string{address}, menu.current, "%s sent to %s", name, address)
		}
	}
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.True(t, options.ThemeTemplates)
	require.Equal(t, []string{"tron"}, options.ThemeChoices)
	// The forms that were taken stored what they submit.
	selected, _ := define.ThemeSnapshot()
	require.Equal(t, "custom", selected)
}
