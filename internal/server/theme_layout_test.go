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

	"github.com/soulteary/flare/internal/i18n"
)

const (
	arrangedThemeFile = themeFileColors + "name: Arranged\nlayout:\n  DesktopColumns: 6\n  TitleSize: compact\n  HideAppDescriptions: true\n  CategoryNavigation: tabs\n"
	layoutConfig      = "Locale: en\nTitle: Site\nTheme: arranged\nShowApps: true\nShowBookmarks: true\nHomepage:\n  DesktopColumns: \"4\"\n  TitleSize: prominent\n  NavigationTextSize: large\n"
	layoutBookmarks   = "categories:\n- id: a\n  title: First\n- id: b\n  title: Second\nlinks:\n- name: One\n  link: https://example.com/one\n  category: a\n- name: Two\n  link: https://example.com/two\n  category: b\n"
)

func layoutRouter(t *testing.T, config string) http.Handler {
	t.Helper()
	handler := themeFilesRouter(t, config, map[string]string{"arranged.yml": arrangedThemeFile, "plain.yml": themeFileColors})
	require.NoError(t, os.WriteFile("bookmarks.yml", []byte(layoutBookmarks), 0600))
	return handler
}

// bodyClasses returns the classes of the page body.
func bodyClasses(t *testing.T, page string) []string {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	var classes []string
	walkNodes(document, func(node *html.Node) {
		if node.Type == html.ElementNode && node.Data == "body" {
			classes = strings.Fields(nodeAttribute(node, "class"))
		}
	})
	return classes
}

// hasClass reports whether an element of the page has a class. The page
// carries its stylesheet, so the text of the page names every class.
func hasClass(t *testing.T, page, class string) bool {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	found := false
	walkNodes(document, func(node *html.Node) {
		found = found || nodeHasClass(node, class)
	})
	return found
}

// The parts of the appearance page about the layout of the theme.
type layoutPage struct {
	// all is the selected value of the switch for every option, empty when
	// the page has no such switch.
	all     string
	summary string
	theme   string
	// notes maps the text of each remark to its box: the name of the box
	// followed by " checked" when it is ticked, or nothing without a box.
	notes map[string]string
	// selected holds the selected values of the layout controls, by name.
	selected map[string]string
}

func readLayoutPage(t *testing.T, page string) layoutPage {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	read := layoutPage{notes: map[string]string{}, selected: map[string]string{}}
	walkNodes(document, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		switch {
		case node.Data == "select":
			name := nodeAttribute(node, "name")
			walkNodes(node, func(option *html.Node) {
				if option.Type != html.ElementNode || option.Data != "option" {
					return
				}
				for _, attribute := range option.Attr {
					if attribute.Key == "selected" {
						read.selected[name] = nodeAttribute(option, "value")
					}
				}
			})
			if name == "theme-layout-all" {
				read.all = read.selected[name]
				for sibling := node.NextSibling; sibling != nil; sibling = sibling.NextSibling {
					if nodeHasClass(sibling, "help-text") {
						read.summary = nodeText(sibling)
					}
				}
			}
		case node.Data == "input" && nodeAttribute(node, "name") == "theme-layout-theme":
			read.theme = nodeAttribute(node, "value")
		case nodeHasClass(node, "theme-layout-note"):
			text, box := "", ""
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				if child.Type == html.TextNode {
					text += child.Data
				}
			}
			walkNodes(node, func(part *html.Node) {
				if part.Type != html.ElementNode || part.Data != "input" {
					return
				}
				require.Equal(t, "checkbox", nodeAttribute(part, "type"))
				box = nodeAttribute(part, "name")
				for _, attribute := range part.Attr {
					if attribute.Key == "checked" {
						box += " checked"
					}
				}
			})
			read.notes[strings.TrimSpace(text)] = box
		}
	})
	return read
}

func appearanceRequest(handler http.Handler, form url.Values) *httptest.ResponseRecorder {
	// What the form of the page submits for the options the tests look at.
	body := url.Values{"title": {"Site"}, "show-apps": {"1"}, "show-bookmarks": {"1"}, "desktop-columns": {"4"}, "title-size": {"prominent"}, "navigation-text-size": {"large"}}
	for name, values := range form {
		body[name] = values
	}
	req := httptest.NewRequest(http.MethodPost, "/settings/appearance", strings.NewReader(body.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func note(key, value string) string {
	return i18n.Format("en", key, map[string]any{"value": value})
}

func TestThemeLayoutHTTP(t *testing.T) {
	handler := layoutRouter(t, layoutConfig)
	themed := []string{"desktop-columns-6", "navigation-text-large", "homepage-title-compact", "hide-app-descriptions", "app-content-uppercase"}
	site := []string{"desktop-columns-4", "navigation-text-large", "homepage-title-prominent", "app-content-uppercase"}

	// The pages take the options the theme specifies from the theme and the
	// rest, the text size among it, from the site.
	for _, path := range []string{"/", "/bookmarks", "/applications"} {
		require.Equal(t, themed, bodyClasses(t, getPage(t, handler, path)), path)
	}
	// The category navigation is one of them, and it changes the markup.
	home := getPage(t, handler, "/")
	require.True(t, hasClass(t, home, "bookmark-category-nav"), "bookmark-category-nav")
	require.False(t, hasClass(t, home, "bookmark-collapsible"), "bookmark-collapsible")

	// The appearance page keeps showing the settings of the site, and says
	// next to each option what the theme sets instead.
	page := readLayoutPage(t, getPage(t, handler, "/settings/appearance"))
	require.Equal(t, "apply", page.all)
	require.Equal(t, "arranged", page.theme)
	require.Equal(t, i18n.Format("en", "theme_layout_help", map[string]any{"options": "Homepage title size, Application descriptions, Bookmark category navigation, Desktop columns"}), page.summary)
	require.Equal(t, map[string]string{
		note("theme_layout_note", "Compact"):        "theme-layout-ignore-TitleSize",
		note("theme_layout_note", "Hide"):           "theme-layout-ignore-HideAppDescriptions",
		note("theme_layout_note", "Category links"): "theme-layout-ignore-CategoryNavigation",
		note("theme_layout_note", "6"):              "theme-layout-ignore-DesktopColumns",
	}, page.notes)
	require.Equal(t, "4", page.selected["desktop-columns"])
	require.Equal(t, "prominent", page.selected["title-size"])
	require.Equal(t, "all", page.selected["category-navigation"])

	// The card of the theme names what it sets before it is selected.
	cards := readThemeFilePage(t, getPage(t, handler, "/settings/theme")).cards
	require.Len(t, cards, 2)
	require.Equal(t, "arranged", cards[0].value)
	require.Contains(t, getPage(t, handler, "/settings/theme"), i18n.Format("en", "theme_layout_sets", map[string]any{"options": "Homepage title size, Application descriptions, Bookmark category navigation, Desktop columns"}))

	// Single options can be switched back to the settings of the site.
	saved := appearanceRequest(handler, url.Values{
		"theme-layout-all": {"apply"}, "theme-layout-theme": {"arranged"},
		"theme-layout-ignore-DesktopColumns": {"1"}, "theme-layout-ignore-CategoryNavigation": {"1"},
	})
	require.Equal(t, http.StatusOK, saved.Code)
	page = readLayoutPage(t, saved.Body.String())
	require.Equal(t, "theme-layout-ignore-DesktopColumns checked", page.notes[note("theme_layout_note", "6")])
	require.Equal(t, "theme-layout-ignore-CategoryNavigation checked", page.notes[note("theme_layout_note", "Category links")])
	require.Equal(t, "theme-layout-ignore-TitleSize", page.notes[note("theme_layout_note", "Compact")])
	home = getPage(t, handler, "/")
	require.Equal(t, []string{"desktop-columns-4", "navigation-text-large", "homepage-title-compact", "hide-app-descriptions", "app-content-uppercase"}, bodyClasses(t, home))
	require.False(t, hasClass(t, home, "bookmark-category-nav"), "bookmark-category-nav")
	stored, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Contains(t, string(stored), "ThemeSettings:")
	require.Contains(t, string(stored), "- DesktopColumns")
	// The settings of the site were not rewritten with values of the theme.
	require.Contains(t, string(stored), "TitleSize: prominent")
	require.Contains(t, string(stored), "HideAppDescriptions: false")

	// All of it can be turned off at once. The options then have no boxes,
	// and the choice made per option is kept for later.
	saved = appearanceRequest(handler, url.Values{"theme-layout-all": {"ignore"}})
	require.Equal(t, http.StatusOK, saved.Code)
	page = readLayoutPage(t, saved.Body.String())
	require.Equal(t, "ignore", page.all)
	require.Empty(t, page.theme)
	require.Equal(t, map[string]string{
		note("theme_layout_note_off", "Compact"):        "",
		note("theme_layout_note_off", "Hide"):           "",
		note("theme_layout_note_off", "Category links"): "",
		note("theme_layout_note_off", "6"):              "",
	}, page.notes)
	for _, path := range []string{"/", "/bookmarks", "/applications"} {
		require.Equal(t, site, bodyClasses(t, getPage(t, handler, path)), path)
	}
	saved = appearanceRequest(handler, url.Values{"theme-layout-all": {"apply"}})
	require.Equal(t, http.StatusOK, saved.Code)
	page = readLayoutPage(t, saved.Body.String())
	require.Equal(t, "theme-layout-ignore-DesktopColumns checked", page.notes[note("theme_layout_note", "6")])

	// Unticking every box applies the whole layout of the theme again.
	require.Equal(t, http.StatusOK, appearanceRequest(handler, url.Values{"theme-layout-all": {"apply"}, "theme-layout-theme": {"arranged"}}).Code)
	home = getPage(t, handler, "/")
	require.Equal(t, themed, bodyClasses(t, home))
	require.True(t, hasClass(t, home, "bookmark-category-nav"), "bookmark-category-nav")

	// A change of the layout in the file shows on the next page, in the
	// markup as well: nothing rendered for the previous layout is served.
	writeThemeFile(t, "arranged.yml", strings.Replace(arrangedThemeFile, "CategoryNavigation: tabs", "CategoryNavigation: collapse", 1))
	home = getPage(t, handler, "/")
	require.True(t, hasClass(t, home, "bookmark-collapsible"), "bookmark-collapsible")
	require.False(t, hasClass(t, home, "bookmark-category-nav"), "bookmark-category-nav")
	writeThemeFile(t, "arranged.yml", strings.Replace(arrangedThemeFile, "  CategoryNavigation: tabs\n", "", 1))
	home = getPage(t, handler, "/")
	require.False(t, hasClass(t, home, "bookmark-collapsible"), "bookmark-collapsible")
	require.False(t, hasClass(t, home, "bookmark-category-nav"), "bookmark-category-nav")
	require.Equal(t, themed, bodyClasses(t, home))

	// A theme without a layout, and a bundled theme, leave the pages to the
	// settings of the site, and the appearance page has nothing to say.
	for _, name := range []string{"plain", "blackboard"} {
		require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {name}}).Code, name)
		require.Equal(t, site, bodyClasses(t, getPage(t, handler, "/")), name)
		page = readLayoutPage(t, getPage(t, handler, "/settings/appearance"))
		require.Empty(t, page.all, name)
		require.Empty(t, page.notes, name)
		require.Empty(t, page.theme, name)
	}

	// Selecting the theme again applies its layout again: the settings of
	// the site were never changed by it.
	require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"arranged"}}).Code)
	require.Equal(t, themed, bodyClasses(t, getPage(t, handler, "/")))
}

// The stored choices are applied when the instance starts.
func TestThemeLayoutAtStartup(t *testing.T) {
	for name, tc := range map[string]struct {
		config string
		want   []string
	}{
		"applied":           {layoutConfig, []string{"desktop-columns-6", "navigation-text-large", "homepage-title-compact", "hide-app-descriptions", "app-content-uppercase"}},
		"one option off":    {layoutConfig + "ThemeSettings:\n  arranged:\n    IgnoreLayout: [TitleSize]\n", []string{"desktop-columns-6", "navigation-text-large", "homepage-title-prominent", "hide-app-descriptions", "app-content-uppercase"}},
		"off for all":       {strings.Replace(layoutConfig, "Homepage:\n", "Homepage:\n  IgnoreThemeLayout: true\n", 1), []string{"desktop-columns-4", "navigation-text-large", "homepage-title-prominent", "app-content-uppercase"}},
		"off for another":   {layoutConfig + "ThemeSettings:\n  plain:\n    IgnoreLayout: [TitleSize, DesktopColumns]\n", []string{"desktop-columns-6", "navigation-text-large", "homepage-title-compact", "hide-app-descriptions", "app-content-uppercase"}},
		"no Homepage block": {"Locale: en\nTheme: arranged\nShowApps: true\n", []string{"desktop-columns-6", "homepage-title-compact", "hide-app-descriptions", "app-content-uppercase"}},
	} {
		t.Run(name, func(t *testing.T) {
			handler := layoutRouter(t, tc.config)
			require.Equal(t, tc.want, bodyClasses(t, getPage(t, handler, "/")))
		})
	}
}
