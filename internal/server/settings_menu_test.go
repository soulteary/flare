package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/security"
)

// The menu of a settings page: its groups in their order, each with its
// title and the entries under it as name and address, and the entry that
// is marked as the page that is shown.
type settingsMenu struct {
	groups  [][]string
	current []string
}

func readSettingsMenu(t *testing.T, page string) *settingsMenu {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	var menu *settingsMenu
	walkNodes(document, func(node *html.Node) {
		if node.Type != html.ElementNode || node.Data != "nav" || !nodeHasClass(node, "sidebar") {
			return
		}
		require.Nil(t, menu, "the page has the menu twice")
		menu = &settingsMenu{}
		walkNodes(node, func(group *html.Node) {
			if !nodeHasClass(group, "sidebar-group") {
				return
			}
			var entries []string
			walkNodes(group, func(part *html.Node) {
				switch {
				case nodeHasClass(part, "sidebar-title"):
					entries = append(entries, strings.TrimSpace(nodeText(part)))
				case part.Type == html.ElementNode && part.Data == "a":
					address := nodeAttribute(part, "href")
					entries = append(entries, strings.TrimSpace(nodeText(part))+" "+address)
					if nodeAttribute(part, "aria-current") == "page" {
						require.True(t, nodeHasClass(part, "active"), address)
						menu.current = append(menu.current, address)
					} else {
						require.False(t, nodeHasClass(part, "active"), address)
					}
				}
			})
			menu.groups = append(menu.groups, entries)
		})
	})
	return menu
}

func settingsMenuGroups(locale string, account, editor bool) [][]string {
	text := func(key string) string { return i18n.T(locale, key) }
	site := []string{text("nav_group_site"), text("nav_site") + " /settings/site"}
	if account {
		site = append(site, text("nav_account")+" /settings/application")
	}
	groups := [][]string{}
	if editor {
		groups = append(groups, []string{text("nav_group_content"), text("nav_editor") + " /settings/content", text("nav_transfer") + " /settings/transfer"})
	}
	return append(groups, [][]string{
		{text("nav_group_home"), text("nav_modules") + " /settings/modules", text("nav_search") + " /settings/search", text("nav_weather") + " /settings/weather",
			text("nav_clock") + " /settings/clock", text("nav_links") + " /settings/links"},
		{text("nav_group_look"), text("nav_theme") + " /settings/theme", text("nav_layout") + " /settings/layout",
			text("nav_themes") + " /settings/themes", text("nav_theme_access") + " /settings/theme-access"},
		append(site, text("nav_about")+" /settings/about"),
	}...)
}

func TestSettingsMenuIsGroupedHTTP(t *testing.T) {
	for _, editor := range []bool{true, false} {
		locale := map[bool]string{true: "zh", false: "en"}[editor]
		t.Run(locale, func(t *testing.T) {
			customThemeDirectory(t, "Locale: "+locale+"\nTheme: blackboard\n")
			handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true, EnableEditor: editor})
			require.NoError(t, err)
			// An instance that needs no sign-in has no account page to
			// list, and one without the editor no pages for the content.
			groups := settingsMenuGroups(locale, false, editor)
			pages := []string{"/settings/modules", "/settings/search", "/settings/weather", "/settings/clock", "/settings/links", "/settings/theme", "/settings/layout", "/settings/themes", "/settings/theme-access", "/settings/site", "/settings/about"}
			if editor {
				// The pages for the content stand in the same frame.
				pages = append(pages, "/settings/content", "/settings/transfer")
			}
			for _, path := range pages {
				page := getPage(t, handler, path)
				menu := readSettingsMenu(t, page)
				require.NotNil(t, menu, path)
				require.Equal(t, groups, menu.groups, path)
				require.Equal(t, []string{path}, menu.current, path)
				require.NotContains(t, page, "sidebar-out", path)
				// The switch of the menu, for a narrow window, names the
				// group of the page.
				require.Contains(t, page, `id=settings-nav-toggle`, path)
			}
			// Every entry of the menu leads to a page.
			for _, group := range groups {
				for _, entry := range group[1:] {
					address := entry[strings.LastIndex(entry, " ")+1:]
					rec := getResponse(handler, address)
					require.Equal(t, http.StatusOK, rec.Code, address)
				}
			}
		})
	}
}

// The footer of the site stands under every page of the settings, once,
// after the menu and the page.
func TestSettingsPagesShowTheFooterHTTP(t *testing.T) {
	handler := customThemeRouter(t, "Locale: en\nTheme: blackboard\nFooter: <p>Powered by <b>an example</b></p>\n")
	for _, path := range []string{"/settings/modules", "/settings/search", "/settings/weather", "/settings/clock", "/settings/links", "/settings/theme", "/settings/layout", "/settings/themes", "/settings/theme-access", "/settings/about"} {
		page := getPage(t, handler, path)
		footer := `<p>Powered by <b>an example</b></p>`
		require.Equal(t, 1, strings.Count(page, footer), path)
		require.Regexp(t, `<div class="?footer-container"?>\s*<p>Powered by`, page, path)
		require.Greater(t, strings.Index(page, footer), strings.Index(page, "setting-group-container"), path)
		require.Greater(t, strings.Index(page, footer), strings.Index(page, "sidebar-groups"), path)
	}
}

// The page about the application holds what the page at the address of the
// account used to show beside the sign-in, and that address leads to it on
// an instance without sign-in.
func TestAboutPageHTTP(t *testing.T) {
	handler := customThemeRouter(t, "Locale: en\nTheme: blackboard\n")
	page := getPage(t, handler, "/settings/about")
	for _, text := range []string{i18n.T("en", "about_flare"), i18n.T("en", "about_desc_1"), i18n.T("en", "project_link"), i18n.T("en", "version"), i18n.T("en", "build_date"), `href=/help`} {
		require.Contains(t, page, text)
	}
	require.NotContains(t, page, `type=password`)
	for _, path := range []string{"/settings/theme", "/settings/layout"} {
		require.NotContains(t, getPage(t, handler, path), i18n.T("en", "about_desc_1"), path)
	}
	rec := getResponse(handler, "/settings/application")
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, "/settings/about", rec.Header().Get("Location"))
	// The help page sends the question for the version there.
	require.Contains(t, getPage(t, handler, "/help"), `href="/settings/about"`)
}

// On an instance with accounts the menu lists the account page, the page
// about the application needs the session like the other settings, and
// signing in can lead back to it.
func TestSettingsMenuWithAccounts(t *testing.T) {
	customThemeDirectory(t, "Locale: en\nTheme: blackboard\n")
	store, err := security.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	require.NoError(t, store.Initialize("admin", "legacy", "local"))
	app, err := NewFiberApp(&model.Flags{Port: 5005, CookieName: "settings", Visibility: "DEFAULT", StartupStatus: "ready"}, store)
	require.NoError(t, err)
	token, err := store.Authenticate("admin", "legacy")
	require.NoError(t, err)
	session := &http.Cookie{Name: auth.RequestHandleSessionName("settings", 5005) + "_" + store.Info().InstanceID, Value: token}
	request := func(path string, signedIn bool) (*http.Response, string) {
		req := httptest.NewRequest(http.MethodGet, "http://flare.test"+path, nil)
		req.Header.Set("Accept", "text/html")
		if signedIn {
			req.AddCookie(session)
		}
		res, e := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		require.NoError(t, e)
		raw, e := io.ReadAll(res.Body)
		res.Body.Close()
		require.NoError(t, e)
		return res, string(raw)
	}
	for _, path := range []string{"/settings/theme", "/settings/about"} {
		res, page := request(path, true)
		require.Equal(t, http.StatusOK, res.StatusCode, path)
		menu := readSettingsMenu(t, page)
		require.Equal(t, settingsMenuGroups("en", true, false), menu.groups, path)
		require.Equal(t, []string{path}, menu.current, path)
	}
	res, page := request("/settings/about", false)
	require.Equal(t, http.StatusFound, res.StatusCode)
	require.Equal(t, "/login?return_to=%2Fsettings%2Fabout", res.Header.Get("Location"))
	require.NotContains(t, page, i18n.T("en", "about_desc_1"))
	// The account page is the one sign-in has, at the address it always had.
	res, page = request("/settings/application", true)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Contains(t, page, "Account / 账号")
	res, page = request("/login?return_to=%2Fsettings%2Fabout", false)
	require.Equal(t, http.StatusOK, res.StatusCode)
	require.Contains(t, page, `name="return_to" value="/settings/about"`)
	// The pages that hold what the appearance page held need the session
	// as that page did, to be read and to be saved, and signing in leads
	// back to each.
	for _, path := range []string{"/settings/modules", "/settings/clock", "/settings/links", "/settings/layout", "/settings/site", "/settings/themes", "/settings/theme-access"} {
		res, page = request(path, true)
		require.Equal(t, http.StatusOK, res.StatusCode, path)
		require.Equal(t, []string{path}, readSettingsMenu(t, page).current, path)
		res, page = request(path, false)
		require.Equal(t, http.StatusFound, res.StatusCode, path)
		require.Equal(t, "/login?return_to="+url.QueryEscape(path), res.Header.Get("Location"), path)
		require.NotContains(t, page, "<form", path)
		res, page = request("/login?return_to="+url.QueryEscape(path), false)
		require.Equal(t, http.StatusOK, res.StatusCode, path)
		require.Contains(t, page, `name="return_to" value="`+path+`"`, path)
		post := httptest.NewRequest(http.MethodPost, "http://flare.test"+path, strings.NewReader("title=not+stored"))
		post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		answer, e := app.Test(post, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		require.NoError(t, e)
		answer.Body.Close()
		require.NotEqual(t, http.StatusOK, answer.StatusCode, path)
	}
	stored, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.Empty(t, stored.Title)
}
