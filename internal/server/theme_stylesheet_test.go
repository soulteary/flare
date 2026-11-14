package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/security"
)

const themeStylesheetText = "/* Rounded tiles. */\n.app-item { border-radius: 12px }\n"

var themeStylesheetAddress = regexp.MustCompile(`^/theme/[a-z0-9-]+/[0-9a-f]{16}/theme\.css$`)

// pageStyles lists the styles a page loads, in the order of the document:
// "style" for a style element and the address for a linked stylesheet. The
// order is the order in which their rules apply.
func pageStyles(t *testing.T, page string) []string {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	var styles []string
	walkNodes(document, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		switch {
		case node.Data == "style":
			require.Equal(t, "head", node.Parent.Data)
			styles = append(styles, "style")
		case node.Data == "link" && nodeAttribute(node, "rel") == "stylesheet":
			require.Equal(t, "head", node.Parent.Data)
			styles = append(styles, nodeAttribute(node, "href"))
		}
	})
	return styles
}

func getStylesheet(handler http.Handler, method, address string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(method, address, nil))
	return rec
}

// A theme that is a directory can bring a stylesheet. The navigation pages
// link it after the stylesheet of the application and before the tokens of
// the theme; the settings do not load it.
func TestThemeStylesheetHTTP(t *testing.T) {
	paired := themeFileColors + "modes:\n  light:\n    background: \"#f6f3ea\"\n    primary: \"#2b2a27\"\n"
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\nShowApps: true\n", map[string]string{
		"styled/theme.yml": themeFileColors,
		"styled/theme.css": themeStylesheetText,
		"paired/theme.yml": paired,
		"paired/theme.css": ".app-title { letter-spacing: 1px }\n",
		"bare/theme.yml":   themeFileColors,
		"plain.yml":        themeFileColors,
		"remote/theme.yml": themeFileColors,
		"remote/theme.css": "a { color: red }\n.app-item { background: url(https://example.test/a.png) }\n",
		// The name of a function is taken from the stylesheet with its
		// escapes resolved, and can be anything.
		"markup/theme.yml": themeFileColors,
		"markup/theme.css": `a { width: \3c img\20 src\3d x\3e (1) }`,
	})
	navigation := []string{"/", "/applications", "/bookmarks", "/help"}
	settings := []string{"/settings/theme", "/settings/appearance"}

	// A bundled theme has no stylesheet: the pages load what they loaded.
	for _, path := range append(navigation, settings...) {
		require.Equal(t, []string{"style"}, pageStyles(t, getPage(t, handler, path)), path)
	}

	// The theme page says which themes bring a stylesheet, and for a
	// stylesheet that is not accepted where and why, as text.
	raw := getPage(t, handler, "/settings/theme")
	page := readThemeFilePage(t, raw)
	note := i18n.T("en", "theme_stylesheet_note")
	about := map[string]string{}
	for _, card := range page.cards {
		about[card.value] = card.about
	}
	require.Equal(t, map[string]string{"bare": "", "paired": note, "plain": "", "styled": note}, about)
	require.Equal(t, []string{
		"markup/theme.css " + i18n.T("en", "theme_file_problem_style_function") + " line 1: <img src=x>()",
		"remote/theme.css " + i18n.T("en", "theme_file_problem_style_url") + " line 2: url()",
	}, page.lineTexts)
	require.NotContains(t, raw, "<img src=x>")
	require.Contains(t, raw, "&lt;img src=x&gt;()")
	require.Zero(t, page.elements["img"])

	// Selected, the navigation pages link the stylesheet after the styles
	// of the application. The settings keep the colors and load no
	// stylesheet of a theme.
	require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"styled"}}).Code)
	address := ""
	for _, path := range navigation {
		body := getPage(t, handler, path)
		styles := pageStyles(t, body)
		require.Len(t, styles, 2, path)
		require.Equal(t, "style", styles[0], path)
		require.Regexp(t, themeStylesheetAddress, styles[1], path)
		require.True(t, strings.HasPrefix(styles[1], "/theme/styled/"), path)
		require.Equal(t, themeFileStyle, bodyStyle(t, body), path)
		if address == "" {
			address = styles[1]
		}
		require.Equal(t, address, styles[1], path)
	}
	for _, path := range settings {
		body := getPage(t, handler, path)
		require.Equal(t, []string{"style"}, pageStyles(t, body), path)
		require.Equal(t, themeFileStyle, bodyStyle(t, body), path)
		require.NotContains(t, body, "/theme/styled/", path)
	}

	// The address serves the file as it is, as a stylesheet and nothing
	// else, and for good: other content has another address.
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		served := getStylesheet(handler, method, address)
		require.Equal(t, http.StatusOK, served.Code, method)
		require.Equal(t, "text/css; charset=utf-8", served.Header().Get("Content-Type"), method)
		require.Equal(t, "nosniff", served.Header().Get("X-Content-Type-Options"), method)
		require.Equal(t, "public, max-age=31536000, immutable", served.Header().Get("Cache-Control"), method)
		if method == http.MethodGet {
			require.Equal(t, themeStylesheetText, served.Body.String())
		} else {
			require.Empty(t, served.Body.String())
		}
	}
	require.Equal(t, http.StatusMethodNotAllowed, getStylesheet(handler, http.MethodPost, address).Code)
	// A copy that is still held is confirmed without sending the content.
	tag := getStylesheet(handler, http.MethodGet, address).Header().Get("ETag")
	require.NotEmpty(t, tag)
	conditional := httptest.NewRequest(http.MethodGet, address, nil)
	conditional.Header.Set("If-None-Match", tag)
	confirmed := httptest.NewRecorder()
	handler.ServeHTTP(confirmed, conditional)
	require.Equal(t, http.StatusNotModified, confirmed.Code)
	require.Empty(t, confirmed.Body.String())

	// Nothing else is served there: no other hash, no other file, no theme
	// without a stylesheet or with one that is not accepted.
	hash := strings.Split(address, "/")[3]
	require.NoError(t, os.WriteFile("secret.css", []byte("SECRET-STYLESHEET"), 0600))
	for _, wrong := range []string{
		"/theme/styled/" + strings.Repeat("0", 16) + "/theme.css",
		"/theme/styled/" + strings.ToUpper(hash) + "/theme.css",
		"/theme/styled/" + hash + "/other.css",
		"/theme/styled/" + hash + "/theme.yml",
		"/theme/styled/" + hash + "/theme.css/",
		"/theme/styled/" + hash + "/",
		"/theme/styled/theme.css",
		"/theme/bare/" + hash + "/theme.css",
		"/theme/plain/" + hash + "/theme.css",
		"/theme/remote/" + hash + "/theme.css",
		"/theme/blackboard/" + hash + "/theme.css",
		"/theme/..%2Fsecret/" + hash + "/theme.css",
		"/theme/styled/" + hash + "/..%2F..%2F..%2Fsecret.css",
		"/theme/styled/" + hash + "/%2e%2e/%2e%2e/%2e%2e/config.yml",
	} {
		served := getStylesheet(handler, http.MethodGet, wrong)
		require.Equal(t, http.StatusNotFound, served.Code, wrong)
		require.NotContains(t, served.Body.String(), "SECRET", wrong)
		require.NotContains(t, served.Body.String(), "Theme:", wrong)
		require.NotContains(t, served.Body.String(), "border-radius", wrong)
	}

	// A preview links the stylesheet of the previewed theme, for that
	// response alone. With different values for light and dark the tokens
	// are rules, and they follow the stylesheet.
	previewed := pageStyles(t, getPage(t, handler, "/?theme-preview=paired"))
	require.Len(t, previewed, 3)
	require.Equal(t, "style", previewed[0])
	require.True(t, strings.HasPrefix(previewed[1], "/theme/paired/"), previewed[1])
	require.Equal(t, "style", previewed[2])
	require.Equal(t, http.StatusOK, getStylesheet(handler, http.MethodGet, previewed[1]).Code)
	require.Equal(t, []string{"style"}, pageStyles(t, getPage(t, handler, "/?theme-preview=bare")))
	require.Equal(t, []string{"style", address}, pageStyles(t, getPage(t, handler, "/")))

	// An edit of the stylesheet is applied by the next page under another
	// address, and the earlier address is gone.
	edited := themeStylesheetText + ".app-title { letter-spacing: 1px }\n"
	writeThemeFile(t, "styled/theme.css", edited)
	styles := pageStyles(t, getPage(t, handler, "/"))
	require.Len(t, styles, 2)
	require.NotEqual(t, address, styles[1])
	require.Regexp(t, themeStylesheetAddress, styles[1])
	require.Equal(t, edited, getStylesheet(handler, http.MethodGet, styles[1]).Body.String())
	require.Equal(t, http.StatusNotFound, getStylesheet(handler, http.MethodGet, address).Code)

	// A stylesheet that is not accepted leaves the pages without the theme:
	// the colors of the stylesheet of the application, and no stylesheet of
	// the theme. The theme page names the theme and the reason.
	writeThemeFile(t, "styled/theme.css", "@import \"https://example.test/a.css\";\n"+themeStylesheetText)
	for _, path := range navigation {
		body := getPage(t, handler, path)
		require.Equal(t, []string{"style"}, pageStyles(t, body), path)
		require.Empty(t, bodyStyle(t, body), path)
	}
	require.Equal(t, http.StatusNotFound, getStylesheet(handler, http.MethodGet, styles[1]).Code)
	page = readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Equal(t, []string{i18n.Format("en", "theme_unavailable", map[string]any{"name": "styled"})}, page.alerts)
	require.Contains(t, page.lineTexts, "styled/theme.css "+i18n.T("en", "theme_file_problem_style_rule")+" line 1: @import")

	// Corrected, the theme is back, and without the file it has no stylesheet.
	writeThemeFile(t, "styled/theme.css", themeStylesheetText)
	require.Equal(t, []string{"style", address}, pageStyles(t, getPage(t, handler, "/")))
	require.Empty(t, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).alerts)
	require.NoError(t, os.Remove("themes/styled/theme.css"))
	body := getPage(t, handler, "/")
	require.Equal(t, []string{"style"}, pageStyles(t, body))
	require.Equal(t, themeFileStyle, bodyStyle(t, body))

	// The reasons are given in the interface language.
	require.NoError(t, os.WriteFile("config.yml", []byte("Locale: zh\nTheme: styled\nShowApps: true\n"), 0600))
	page = readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Contains(t, page.lineTexts, "remote/theme.css "+i18n.T("zh", "theme_file_problem_style_url")+" line 2: url()")
}

// The stylesheet of a theme is for whoever the pages that link it are for:
// anyone on an instance that is open to visitors, and the administrator
// alone on a private one.
func TestThemeStylesheetFollowsTheVisibilityOfThePages(t *testing.T) {
	for visibility, open := range map[string]bool{"DEFAULT": true, "PRIVATE": false} {
		t.Run(visibility, func(t *testing.T) {
			customThemeDirectory(t, "Locale: en\nTheme: styled\n")
			t.Cleanup(func() { require.NoError(t, os.RemoveAll(define.UserThemeDirectory)) })
			writeThemeFile(t, "styled/theme.yml", themeFileColors)
			writeThemeFile(t, "styled/theme.css", themeStylesheetText)
			store, err := security.Open(t.TempDir())
			require.NoError(t, err)
			t.Cleanup(func() { store.Close() })
			require.NoError(t, store.Initialize("admin", "legacy", "local"))
			app, err := NewFiberApp(&model.Flags{Port: 5005, CookieName: "settings", Visibility: visibility, StartupStatus: "ready"}, store)
			require.NoError(t, err)
			token, err := store.Authenticate("admin", "legacy")
			require.NoError(t, err)
			session := &http.Cookie{Name: auth.RequestHandleSessionName("settings", 5005) + "_" + store.Info().InstanceID, Value: token}
			send := func(path string, signedIn bool) (*http.Response, string) {
				req := httptest.NewRequest(http.MethodGet, "http://flare.test"+path, nil)
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

			res, home := send("/", true)
			require.Equal(t, http.StatusOK, res.StatusCode)
			styles := pageStyles(t, home)
			require.Len(t, styles, 2)
			address := styles[1]
			require.Regexp(t, themeStylesheetAddress, address)

			res, body := send(address, true)
			require.Equal(t, http.StatusOK, res.StatusCode)
			require.Equal(t, themeStylesheetText, body)
			res, body = send(address, false)
			if open {
				require.Equal(t, http.StatusOK, res.StatusCode)
				require.Equal(t, themeStylesheetText, body)
				require.Equal(t, "public, max-age=31536000, immutable", res.Header.Get("Cache-Control"))
				return
			}
			require.NotEqual(t, http.StatusOK, res.StatusCode)
			require.NotContains(t, body, "border-radius")
			res, _ = send(address, true)
			require.Equal(t, "private, max-age=31536000, immutable", res.Header.Get("Cache-Control"))
		})
	}
}

// documentedSelectors reads the selectors a theme stylesheet can rely on from
// the table of docs/theme-stylesheets.md.
func documentedSelectors(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "theme-stylesheets.md"))
	require.NoError(t, err)
	_, section, found := strings.Cut(string(raw), "\n## What a stylesheet can rely on\n")
	require.True(t, found)
	section, _, _ = strings.Cut(section, "\n## ")
	var selectors []string
	for _, row := range regexp.MustCompile("(?m)^\\| `([#.][a-z-]+)` \\|").FindAllStringSubmatch(section, -1) {
		selectors = append(selectors, row[1])
	}
	return selectors
}

// hasSelector reports whether a page has an element with an id or a class,
// written as #id or .class.
func hasSelector(t *testing.T, page, selector string) bool {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	found := false
	walkNodes(document, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		if strings.HasPrefix(selector, "#") && nodeAttribute(node, "id") == selector[1:] {
			found = true
		}
		if strings.HasPrefix(selector, ".") && nodeHasClass(node, selector[1:]) {
			found = true
		}
	})
	return found
}

// The elements a theme stylesheet can rely on are on the navigation pages
// under the names the documentation gives them. A change of a name there
// breaks the stylesheets of themes, so it has to be a change of the table
// first.
func TestThemeStylesheetNamesAreOnThePages(t *testing.T) {
	selectors := documentedSelectors(t)
	require.Len(t, selectors, 19, "the table of names a stylesheet can rely on changed")
	modules := "ShowApps: true\nShowBookmarks: true\nShowSearchComponent: true\nShowDateTime: true\nShowTitle: true\nShowWeather: false\n"
	bookmarks := "categories:\n- id: a\n  title: First\n- id: b\n  title: Second\nlinks:\n- name: One\n  link: https://example.com/one\n  category: a\n- name: Two\n  link: https://example.com/two\n  category: b\n"
	for layout, sometimes := range map[string]string{
		// The links between the categories are part of the layouts that have them.
		"all":      ".bookmark-category-nav",
		"tabs":     "",
		"collapse": ".bookmark-category-nav",
	} {
		t.Run(layout, func(t *testing.T) {
			customThemeDirectory(t, "Locale: en\nTheme: blackboard\n"+modules+"Homepage:\n    CategoryNavigation: "+layout+"\n")
			require.NoError(t, os.WriteFile("apps.yml", []byte("links:\n- name: Application\n  link: https://example.com/a\n  icon: circle\n  desc: A description\n"), 0600))
			require.NoError(t, os.WriteFile("bookmarks.yml", []byte(bookmarks), 0600))
			handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
			require.NoError(t, err)
			home := getPage(t, handler, "/")
			for _, selector := range selectors {
				require.Equal(t, selector != sometimes, hasSelector(t, home, selector), selector)
			}
		})
	}
}
