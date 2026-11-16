package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/security"
)

const (
	choiceCookie     = "flare_theme"
	choiceBlackboard = "--color-background:#1a1a1a;--color-primary:#FFFDEA;--color-accent:#5c5c5c;"
	choiceTron       = "--color-background:#242B33;--color-primary:#EFFBFF;--color-accent:#6EE2FF;"
)

// getWithChoice asks for a page with the cookie of a visitor who chose a
// theme, or without one for an empty name.
func getWithChoice(t *testing.T, handler http.Handler, path, theme string) (string, http.Header) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	if theme != "" {
		req.Header.Set("Cookie", choiceCookie+"="+theme)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code, path)
	return rec.Body.String(), rec.Header()
}

// postChoice submits a choice as a visitor does and returns the response
// with the cookie it sets, or nil when it sets none.
func postChoice(t *testing.T, handler http.Handler, fields url.Values) (*httptest.ResponseRecorder, *http.Cookie) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, "/theme-choice", strings.NewReader(fields.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == choiceCookie {
			return rec, cookie
		}
	}
	return rec, nil
}

func TestVisitorsChooseAmongTheThemesTheAdministratorAllowsHTTP(t *testing.T) {
	// The list names a theme file, a bundled theme, a theme that does not
	// exist, a theme twice and the selected theme.
	handler := themeFilesRouter(t, previewConfig+"ThemeChoices: [forest, tron, gone, forest, blackboard]\n", map[string]string{
		"forest.yml": previewForestFile,
		"hidden.yml": themeFileColors + "  radius: 2px\n",
	})
	stored, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	site := []string{"desktop-columns-4", "app-content-uppercase"}
	pages := []string{"/", "/bookmarks", "/applications", "/help"}

	for _, path := range pages {
		// Without a choice the pages are shown with the selected theme, and
		// say that the cookie decides what the address shows.
		page, header := getWithChoice(t, handler, path, "")
		require.Equal(t, choiceBlackboard, bodyStyle(t, page), path)
		require.Contains(t, header.Values("Vary"), "Cookie", path)
		require.Empty(t, header.Get("Cache-Control"), path)

		// A visitor who chose a theme gets every page with it: its tokens
		// and its layout, with no notice, and for this visitor only.
		page, header = getWithChoice(t, handler, path, "forest")
		require.Equal(t, previewForestStyle, bodyStyle(t, page), path)
		require.Nil(t, readPreviewNotice(t, page), path)
		require.Equal(t, "private", header.Get("Cache-Control"), path)
		require.Contains(t, header.Values("Vary"), "Cookie", path)
		if path != "/help" {
			require.Equal(t, []string{"desktop-columns-6", "hide-app-descriptions", "app-content-uppercase"}, bodyClasses(t, page), path)
		}
		page, _ = getWithChoice(t, handler, path, "tron")
		require.Equal(t, choiceTron, bodyStyle(t, page), path)
		if path != "/help" {
			require.Equal(t, site, bodyClasses(t, page), path)
		}

		// A theme that is not in the list, one that does not exist, the
		// selected one and a value that is no name change nothing.
		for _, name := range []string{"hidden", "gone", "blackboard", "Forest", "forest%20", "%3Cb%3E", "custom"} {
			page, header = getWithChoice(t, handler, path, name)
			require.Equal(t, choiceBlackboard, bodyStyle(t, page), "%s with %s", path, name)
			require.Empty(t, header.Get("Cache-Control"), "%s with %s", path, name)
			if path != "/help" {
				require.Equal(t, site, bodyClasses(t, page), "%s with %s", path, name)
			}
		}
	}

	// The choice is made with a form. It is kept in a cookie of the
	// visitor, and the visitor is sent back to one of the pages.
	rec, cookie := postChoice(t, handler, url.Values{"theme": {"forest"}})
	require.Equal(t, http.StatusSeeOther, rec.Code)
	require.Equal(t, "/", rec.Header().Get("Location"))
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.NotNil(t, cookie)
	require.Equal(t, "forest", cookie.Value)
	require.Equal(t, "/", cookie.Path)
	require.Equal(t, 365*24*60*60, cookie.MaxAge)
	require.True(t, cookie.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	require.Empty(t, cookie.Domain)
	for back, location := range map[string]string{
		"/bookmarks": "/bookmarks", "/applications": "/applications", "/": "/",
		"/settings/theme": "/", "//example.org/": "/", "https://example.org/": "/", "/bookmarks?x=1": "/", "": "/",
	} {
		rec, cookie = postChoice(t, handler, url.Values{"theme": {"tron"}, "back": {back}})
		require.Equal(t, http.StatusSeeOther, rec.Code, back)
		require.Equal(t, location, rec.Header().Get("Location"), back)
		require.Equal(t, "tron", cookie.Value, back)
	}

	// A theme that cannot be chosen is refused, and the cookie is left.
	for _, name := range []string{"hidden", "gone", "custom", "Forest", "<b>", "forest\r\nSet-Cookie: a=b"} {
		rec, cookie = postChoice(t, handler, url.Values{"theme": {name}})
		require.Equal(t, http.StatusBadRequest, rec.Code, name)
		require.Nil(t, cookie, name)
		require.Empty(t, rec.Header().Values("Set-Cookie"), name)
	}

	// No theme, and the selected theme, give the choice up.
	for _, fields := range []url.Values{{}, {"theme": {""}}, {"theme": {"blackboard"}, "back": {"/bookmarks"}}} {
		rec, cookie = postChoice(t, handler, fields)
		require.Equal(t, http.StatusSeeOther, rec.Code)
		require.NotNil(t, cookie)
		require.Empty(t, cookie.Value)
		require.Negative(t, cookie.MaxAge)
	}

	// The address takes a form and nothing else.
	req := httptest.NewRequest(http.MethodGet, "/theme-choice?theme=forest", nil)
	got := httptest.NewRecorder()
	handler.ServeHTTP(got, req)
	require.NotEqual(t, http.StatusSeeOther, got.Code)
	require.Empty(t, got.Header().Values("Set-Cookie"))

	// Nothing of this selected a theme or wrote to the settings.
	selected, _ := define.ThemeSnapshot()
	require.Equal(t, "blackboard", selected)
	raw, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, string(stored), string(raw))

	// When the administrator selects another theme, the theme that was
	// selected before is a choice, because the list names it, and the newly
	// selected one is what every page shows anyway.
	require.NoError(t, data.SaveThemeName("forest"))
	define.SetTheme("forest")
	page, header := getWithChoice(t, handler, "/", "forest")
	require.Equal(t, previewForestStyle, bodyStyle(t, page))
	require.Empty(t, header.Get("Cache-Control"))
	page, header = getWithChoice(t, handler, "/", "blackboard")
	require.Equal(t, choiceBlackboard, bodyStyle(t, page))
	require.Equal(t, site, bodyClasses(t, page))
	require.Equal(t, "private", header.Get("Cache-Control"))

	// A theme file that goes away is no choice any more.
	require.NoError(t, data.SaveThemeName("blackboard"))
	define.SetTheme("blackboard")
	require.NoError(t, os.Remove(define.UserThemeDirectory+"/forest.yml"))
	define.ReloadUserThemes()
	page, _ = getWithChoice(t, handler, "/", "forest")
	require.Equal(t, choiceBlackboard, bodyStyle(t, page))
	rec, _ = postChoice(t, handler, url.Values{"theme": {"forest"}})
	require.Equal(t, http.StatusBadRequest, rec.Code)
}

func TestNoThemeIsChosenUnlessTheAdministratorAllowsItHTTP(t *testing.T) {
	for name, config := range map[string]string{
		"no list":                 previewConfig,
		"an empty list":           previewConfig + "ThemeChoices: []\n",
		"only the selected theme": previewConfig + "ThemeChoices: [blackboard]\n",
		"only unknown themes":     previewConfig + "ThemeChoices: [gone, \"<b>\"]\n",
	} {
		t.Run(name, func(t *testing.T) {
			handler := themeFilesRouter(t, config, map[string]string{"forest.yml": previewForestFile})
			for _, path := range []string{"/", "/bookmarks", "/applications", "/help"} {
				for _, theme := range []string{"", "forest", "tron", "blackboard", "gone"} {
					page, header := getWithChoice(t, handler, path, theme)
					require.Equal(t, choiceBlackboard, bodyStyle(t, page), "%s with %s", path, theme)
					require.NotContains(t, header.Values("Vary"), "Cookie", "%s with %s", path, theme)
					require.Empty(t, header.Get("Cache-Control"), "%s with %s", path, theme)
				}
			}
			rec, cookie := postChoice(t, handler, url.Values{"theme": {"forest"}})
			require.Equal(t, http.StatusBadRequest, rec.Code)
			require.Nil(t, cookie)
			// A cookie from a time when there was a choice can be given up.
			rec, cookie = postChoice(t, handler, url.Values{})
			require.Equal(t, http.StatusSeeOther, rec.Code)
			require.Negative(t, cookie.MaxAge)
		})
	}
}

func TestChosenThemeIsShownWithItsConfirmedTemplatesHTTP(t *testing.T) {
	config := "Locale: en\nTheme: plain\nShowApps: true\nShowBookmarks: true\nThemeTemplates: true\n" +
		"ThemeChoices: [zelda, other]\nThemeSettings:\n  zelda:\n    IgnoreLayout: []\n    Templates: true\n"
	handler := themeFilesRouter(t, config, map[string]string{
		"plain/theme.yml": themeFileColors,
		"zelda/theme.yml": themeFileColors, "zelda/templates/home.html": themeShellTemplate, "zelda/templates/app.html": itemAppTemplate,
		"other/theme.yml": themeFileColors, "other/templates/home.html": themeShellTemplate, "other/templates/app.html": itemAppTemplate,
	})
	shown := func(path, theme string) (bool, bool) {
		page, _ := getWithChoice(t, handler, path, theme)
		require.NotContains(t, page, i18n.T("en", "theme_templates_preview_page"), "%s with %s", path, theme)
		require.NotContains(t, page, i18n.T("en", "theme_preview_use"), "%s with %s", path, theme)
		shell := strings.Contains(page, `<div class="theme-shell"`)
		require.NotEqual(t, shell, strings.Contains(page, "id=hero-container"), "%s with %s", path, theme)
		return shell, strings.Contains(page, `class="t-app"`)
	}
	// The pages are asked for in turn, so that no page of one visitor is
	// what another gets from what the instance keeps of rendered entries.
	for range 2 {
		for _, path := range []string{"/", "/applications", "/bookmarks"} {
			apps := path != "/bookmarks"
			// The selected theme has no templates.
			page, item := shown(path, "")
			require.False(t, page, path)
			require.False(t, item, path)
			// The templates of the chosen theme are confirmed.
			page, item = shown(path, "zelda")
			require.True(t, page, path)
			require.Equal(t, apps, item, path)
			// Those of this one are not: it is shown without them, as it
			// is when it is selected, and not as the administrator sees a
			// preview.
			page, item = shown(path, "other")
			require.False(t, page, path)
			require.False(t, item, path)
		}
	}

	// A confirmation that is withdrawn holds for the visitors who chose the
	// theme as well.
	require.NoError(t, data.SaveThemeTemplatesConfirmed("zelda", false))
	page, item := shown("/", "zelda")
	require.False(t, page)
	require.False(t, item)
	// And so does the switch of the site.
	require.NoError(t, data.SaveThemeTemplatesConfirmed("zelda", true))
	page, _ = shown("/", "zelda")
	require.True(t, page)
	require.NoError(t, data.SaveThemeTemplatesAllowed(false))
	page, item = shown("/", "zelda")
	require.False(t, page)
	require.False(t, item)
}

func TestSafeModeLeavesNoThemeOfTheDirectoryToChooseHTTP(t *testing.T) {
	customThemeDirectory(t, previewConfig+"ThemeChoices: [forest, tron]\n")
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(define.UserThemeDirectory)) })
	require.NoError(t, os.Mkdir(define.UserThemeDirectory, 0700))
	writeThemeFile(t, "forest.yml", previewForestFile)
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true, ThemeSafeMode: true})
	require.NoError(t, err)
	page, _ := getWithChoice(t, handler, "/", "forest")
	require.Equal(t, choiceBlackboard, bodyStyle(t, page))
	rec, cookie := postChoice(t, handler, url.Values{"theme": {"forest"}})
	require.Equal(t, http.StatusBadRequest, rec.Code)
	require.Nil(t, cookie)
	// A bundled theme is read in safe mode, and can be chosen.
	page, header := getWithChoice(t, handler, "/", "tron")
	require.Equal(t, choiceTron, bodyStyle(t, page))
	require.Equal(t, "private", header.Get("Cache-Control"))
}

func TestPreviewAndSafeModeComeBeforeTheChoiceOfAVisitor(t *testing.T) {
	for _, visibility := range []string{"DEFAULT", "PRIVATE"} {
		t.Run(visibility, func(t *testing.T) {
			customThemeDirectory(t, previewConfig+"ThemeChoices: [forest, tron]\n")
			t.Cleanup(func() { require.NoError(t, os.RemoveAll(define.UserThemeDirectory)) })
			require.NoError(t, os.Mkdir(define.UserThemeDirectory, 0700))
			writeThemeFile(t, "forest.yml", previewForestFile)
			store, err := security.Open(t.TempDir())
			require.NoError(t, err)
			t.Cleanup(func() { store.Close() })
			require.NoError(t, store.Initialize("admin", "legacy", "local"))
			app, err := NewFiberApp(&model.Flags{Port: 5005, CookieName: "settings", Visibility: visibility, StartupStatus: "ready"}, store)
			require.NoError(t, err)
			token, err := store.Authenticate("admin", "legacy")
			require.NoError(t, err)
			session := &http.Cookie{Name: auth.RequestHandleSessionName("settings", 5005) + "_" + store.Info().InstanceID, Value: token}
			request := func(method, path string, values url.Values, signedIn bool) (*http.Response, string) {
				var body io.Reader
				if values != nil {
					body = strings.NewReader(values.Encode())
				}
				req := httptest.NewRequest(method, "http://flare.test"+path, body)
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.AddCookie(&http.Cookie{Name: choiceCookie, Value: "forest"})
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

			// The administrator has a choice like any visitor.
			res, page := request(http.MethodGet, "/", nil, true)
			require.Equal(t, http.StatusOK, res.StatusCode)
			require.Equal(t, previewForestStyle, bodyStyle(t, page))
			require.Nil(t, readPreviewNotice(t, page))
			// An instance that lets nothing keep its pages says so for the
			// page of a choice as for every other.
			require.Equal(t, "private, no-store", res.Header.Get("Cache-Control"))
			require.Contains(t, res.Header.Values("Vary"), "Cookie")
			// A preview shows the theme it names, and says so.
			_, page = request(http.MethodGet, "/?theme-preview=tron", nil, true)
			require.Equal(t, choiceTron, bodyStyle(t, page))
			require.NotNil(t, readPreviewNotice(t, page))
			require.Equal(t, []string{"desktop-columns-4", "app-content-uppercase"}, bodyClasses(t, page), "the layout is that of the choice")
			// Safe mode for one page shows no theme of the directory.
			res, page = request(http.MethodGet, "/?theme-safe-mode", nil, true)
			require.Equal(t, choiceBlackboard, bodyStyle(t, page))
			require.Equal(t, []string{"desktop-columns-4", "app-content-uppercase"}, bodyClasses(t, page), "the layout is that of the choice")
			require.Contains(t, res.Header.Get("Cache-Control"), "no-store")

			// A site that is shown to the administrator only takes a choice
			// from the administrator only.
			res, page = request(http.MethodGet, "/", nil, false)
			sets := func(response *http.Response) bool {
				for _, cookie := range response.Cookies() {
					if cookie.Name == choiceCookie {
						return true
					}
				}
				return false
			}
			if visibility == "PRIVATE" {
				require.NotEqual(t, http.StatusOK, res.StatusCode)
				res, _ = request(http.MethodPost, "/theme-choice", url.Values{"theme": {"tron"}}, false)
				require.NotEqual(t, http.StatusSeeOther, res.StatusCode)
				require.False(t, sets(res))
			} else {
				require.Equal(t, http.StatusOK, res.StatusCode)
				require.Equal(t, previewForestStyle, bodyStyle(t, page))
				res, _ = request(http.MethodPost, "/theme-choice", url.Values{"theme": {"tron"}}, false)
				require.Equal(t, http.StatusSeeOther, res.StatusCode)
				require.True(t, sets(res))
			}
			// There the form is a request of the session, and needs its
			// token like every other.
			fields := url.Values{"theme": {"tron"}}
			if visibility == "PRIVATE" {
				res, _ = request(http.MethodPost, "/theme-choice", fields, true)
				require.Equal(t, http.StatusForbidden, res.StatusCode)
				require.False(t, sets(res))
				_, page = request(http.MethodGet, "/?theme-preview=tron", nil, true)
				fields.Set("csrf-token", readPreviewNotice(t, page).token)
			}
			res, _ = request(http.MethodPost, "/theme-choice", fields, true)
			require.Equal(t, http.StatusSeeOther, res.StatusCode)
			require.True(t, sets(res))
		})
	}
}
