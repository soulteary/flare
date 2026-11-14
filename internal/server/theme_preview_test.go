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
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/security"
)

const (
	previewForestFile  = themeFileColors + "  radius: 10px\nname:\n  en: Forest at dusk\n  zh: 暮色森林\nlayout:\n  DesktopColumns: 6\n  HideAppDescriptions: true\n"
	previewForestStyle = themeFileStyle + "--flare-radius:10px;"
	previewConfig      = "Locale: en\nTitle: Site\nTheme: blackboard\nShowApps: true\nShowBookmarks: true\nHomepage:\n  DesktopColumns: \"4\"\n"
)

// The notice a page shows while it previews a theme.
type previewNotice struct {
	text   string
	action string
	theme  string
	token  string
	use    string
	back   string
	href   string
}

// readPreviewNotice returns the notice of a page, or nil when it has none.
func readPreviewNotice(t *testing.T, page string) *previewNotice {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	var notice *previewNotice
	walkNodes(document, func(node *html.Node) {
		if !nodeHasClass(node, "theme-preview-bar") {
			return
		}
		require.Nil(t, notice, "the page has two notices")
		notice = &previewNotice{}
		require.Equal(t, "status", nodeAttribute(node, "role"))
		walkNodes(node, func(part *html.Node) {
			if part.Type != html.ElementNode {
				return
			}
			switch part.Data {
			case "span":
				notice.text = nodeText(part)
			case "form":
				require.Equal(t, "POST", nodeAttribute(part, "method"))
				notice.action = nodeAttribute(part, "action")
			case "input":
				require.Equal(t, "hidden", nodeAttribute(part, "type"))
				switch nodeAttribute(part, "name") {
				case "theme":
					notice.theme = nodeAttribute(part, "value")
				case "csrf-token":
					notice.token = nodeAttribute(part, "value")
				default:
					t.Fatalf("unexpected field %q in the notice", nodeAttribute(part, "name"))
				}
			case "button":
				notice.use = nodeText(part)
			case "a":
				notice.back, notice.href = nodeText(part), nodeAttribute(part, "href")
			}
		})
	})
	return notice
}

func previewText(locale, name string) string {
	return i18n.Format(locale, "theme_preview_notice", map[string]any{"name": name})
}

func TestThemePreviewHTTP(t *testing.T) {
	handler := themeFilesRouter(t, previewConfig, map[string]string{
		"forest.yml":   previewForestFile,
		"paired.yml":   pairedThemeFile,
		"rejected.yml": themeFileColors + "  radius: 4em\n",
		"markup.yml":   themeFileColors + "name: '<b>Bold</b> & \"quoted\"'\n",
	})
	get := func(path string) (string, http.Header) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, rec.Code, path)
		return rec.Body.String(), rec.Header()
	}
	blackboard := "--color-background:#1a1a1a;--color-primary:#FFFDEA;--color-accent:#5c5c5c;"
	site := []string{"desktop-columns-4", "app-content-uppercase"}
	stored, err := os.ReadFile("config.yml")
	require.NoError(t, err)

	// One response is shown with the previewed theme: its tokens, its
	// layout, and a notice that says so and offers the two ways on.
	for _, path := range []string{"/", "/bookmarks", "/applications", "/help"} {
		page, header := get(path + "?theme-preview=forest")
		require.Equal(t, previewForestStyle, bodyStyle(t, page), path)
		notice := readPreviewNotice(t, page)
		require.NotNil(t, notice, path)
		require.Equal(t, previewNotice{
			text: previewText("en", "Forest at dusk"), action: "/settings/theme", theme: "forest",
			use: i18n.T("en", "theme_preview_use"), back: i18n.T("en", "theme_preview_back"), href: "/settings/theme",
		}, *notice, path)
		require.Contains(t, header.Get("Cache-Control"), "no-store", path)
		if path != "/help" {
			require.Equal(t, []string{"desktop-columns-6", "hide-app-descriptions", "app-content-uppercase"}, bodyClasses(t, page), path)
		}

		// The next response, and every response without the parameter, is
		// shown with the selected theme: nothing was selected or stored.
		page, _ = get(path)
		require.Equal(t, blackboard, bodyStyle(t, page), path)
		require.Nil(t, readPreviewNotice(t, page), path)
		if path != "/help" {
			require.Equal(t, site, bodyClasses(t, page), path)
		}
	}
	selected, _ := define.ThemeSnapshot()
	require.Equal(t, "blackboard", selected)
	raw, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, string(stored), string(raw))

	// A bundled theme and a theme with light and dark values are previewed
	// in the same way.
	page, _ := get("/?theme-preview=tron")
	require.Equal(t, "--color-background:#242B33;--color-primary:#EFFBFF;--color-accent:#6EE2FF;", bodyStyle(t, page))
	require.Equal(t, previewText("en", "tron"), readPreviewNotice(t, page).text)
	page, _ = get("/?theme-preview=paired")
	require.Empty(t, bodyStyle(t, page))
	styles := readThemedPage(t, page).styles
	require.Equal(t, pairedRules, styles[len(styles)-1])

	// What is not a theme in use is not previewed, and neither is the
	// selected theme, which the page shows anyway.
	for _, name := range []string{"", "missing", "rejected", "blackboard", "custom", "Forest", "forest.yml", "../config", "<script>alert(1)</script>", strings.Repeat("a", 4096)} {
		page, _ = get("/?theme-preview=" + url.QueryEscape(name))
		require.Equal(t, blackboard, bodyStyle(t, page), "%q", name)
		require.Nil(t, readPreviewNotice(t, page), "%q", name)
		require.NotContains(t, page, "<script>alert(1)", "%q", name)
	}

	// The name a theme gives itself is shown as text.
	page, _ = get("/?theme-preview=markup")
	require.Equal(t, previewText("en", `<b>Bold</b> & "quoted"`), readPreviewNotice(t, page).text)
	require.NotContains(t, page, "<b>Bold</b>")

	// The settings pages are not previewed.
	page, _ = get("/settings/theme?theme-preview=forest")
	require.Equal(t, blackboard, bodyStyle(t, page))
	require.Nil(t, readPreviewNotice(t, page))

	// A preview shows the file as it is now, also when the theme is not
	// the selected one and the settings were not opened in between.
	writeThemeFile(t, "forest.yml", strings.Replace(previewForestFile, "radius: 10px", "radius: 2px", 1))
	page, _ = get("/?theme-preview=forest")
	require.Equal(t, themeFileStyle+"--flare-radius:2px;", bodyStyle(t, page))

	// The form of the notice selects the previewed theme.
	notice := readPreviewNotice(t, page)
	rec := themeRequest(handler, url.Values{"theme": {notice.theme}})
	require.Equal(t, http.StatusOK, rec.Code)
	page, _ = get("/")
	require.Equal(t, themeFileStyle+"--flare-radius:2px;", bodyStyle(t, page))
	require.Nil(t, readPreviewNotice(t, page))
	// Now that it is selected, its own preview is the page itself.
	page, _ = get("/?theme-preview=forest")
	require.Nil(t, readPreviewNotice(t, page))

	// A name that is not a theme leaves the page entirely to the selected
	// theme, its layout included.
	themed := []string{"desktop-columns-6", "hide-app-descriptions", "app-content-uppercase"}
	for _, name := range []string{"missing", "rejected", "../config"} {
		page, _ = get("/?theme-preview=" + url.QueryEscape(name))
		require.Equal(t, themeFileStyle+"--flare-radius:2px;", bodyStyle(t, page), name)
		require.Equal(t, themed, bodyClasses(t, page), name)
		require.Nil(t, readPreviewNotice(t, page), name)
	}
	// And a preview of another theme replaces the layout for that page.
	page, _ = get("/?theme-preview=tron")
	require.Equal(t, site, bodyClasses(t, page))
}

// The theme page links to a preview of every theme it lists.
func TestThemePageLinksToPreviews(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\n"+customThemeBlock, map[string]string{"forest.yml": previewForestFile})
	document, err := html.Parse(strings.NewReader(getPage(t, handler, "/settings/theme")))
	require.NoError(t, err)
	var links []string
	walkNodes(document, func(node *html.Node) {
		if nodeHasClass(node, "theme-preview-link") {
			require.Equal(t, i18n.T("en", "theme_preview"), nodeText(node))
			links = append(links, nodeAttribute(node, "href"))
		}
	})
	want := make([]string, 0, len(define.ThemePalettes)+2)
	for _, theme := range define.ThemePalettes {
		want = append(want, "/?theme-preview="+theme.Name)
	}
	want = append(want, "/?theme-preview=custom", "/?theme-preview=forest")
	require.Equal(t, want, links)

	// The custom colors are previewed like any theme.
	page := getPage(t, handler, "/?theme-preview=custom")
	require.Contains(t, page, customThemeStyle)
	require.Equal(t, previewText("en", i18n.T("en", "theme_custom")), readPreviewNotice(t, page).text)
}

// Which theme a visitor sees is not for the visitor to choose with an
// address: the parameter is honored for the administrator only, and the
// notice then carries the token of the session.
func TestThemePreviewRequiresTheAdministrator(t *testing.T) {
	customThemeDirectory(t, previewConfig)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(define.UserThemeDirectory)) })
	require.NoError(t, os.Mkdir(define.UserThemeDirectory, 0700))
	writeThemeFile(t, "forest.yml", previewForestFile)
	store, err := security.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	require.NoError(t, store.Initialize("admin", "legacy", "local"))
	app, err := NewFiberApp(&model.Flags{Port: 5005, CookieName: "settings", Visibility: "DEFAULT", StartupStatus: "ready"}, store)
	require.NoError(t, err)
	token, err := store.Authenticate("admin", "legacy")
	require.NoError(t, err)
	session := &http.Cookie{Name: auth.RequestHandleSessionName("settings", 5005) + "_" + store.Info().InstanceID, Value: token}
	request := func(method, path string, values url.Values, signedIn bool) (int, string) {
		var body io.Reader
		if values != nil {
			body = strings.NewReader(values.Encode())
		}
		req := httptest.NewRequest(method, "http://flare.test"+path, body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if signedIn {
			req.AddCookie(session)
		}
		res, e := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		require.NoError(t, e)
		raw, e := io.ReadAll(res.Body)
		res.Body.Close()
		require.NoError(t, e)
		return res.StatusCode, string(raw)
	}
	blackboard := "--color-background:#1a1a1a;--color-primary:#FFFDEA;--color-accent:#5c5c5c;"

	for _, path := range []string{"/", "/bookmarks", "/applications", "/help"} {
		status, page := request(http.MethodGet, path+"?theme-preview=forest", nil, false)
		require.Equal(t, http.StatusOK, status, path)
		require.Equal(t, blackboard, bodyStyle(t, page), "%s: a visitor chose the theme", path)
		require.Nil(t, readPreviewNotice(t, page), path)
		if path != "/help" {
			require.Equal(t, []string{"desktop-columns-4", "app-content-uppercase"}, bodyClasses(t, page), path)
		}

		status, page = request(http.MethodGet, path+"?theme-preview=forest", nil, true)
		require.Equal(t, http.StatusOK, status, path)
		require.Equal(t, previewForestStyle, bodyStyle(t, page), path)
		notice := readPreviewNotice(t, page)
		require.NotNil(t, notice, path)
		require.NotEmpty(t, notice.token, "%s: the notice carries no token", path)
	}

	// The form of the notice is an ordinary change of the settings: it
	// needs the session and its token.
	_, page := request(http.MethodGet, "/?theme-preview=forest", nil, true)
	notice := readPreviewNotice(t, page)
	status, _ := request(http.MethodPost, notice.action, url.Values{"theme": {notice.theme}}, true)
	require.Equal(t, http.StatusForbidden, status, "no token")
	status, _ = request(http.MethodPost, notice.action, url.Values{"theme": {notice.theme}, "csrf-token": {notice.token}}, false)
	require.Equal(t, http.StatusUnauthorized, status, "no session")
	selected, _ := define.ThemeSnapshot()
	require.Equal(t, "blackboard", selected)
	status, page = request(http.MethodPost, notice.action, url.Values{"theme": {notice.theme}, "csrf-token": {notice.token}}, true)
	require.Equal(t, http.StatusOK, status, page)
	selected, _ = define.ThemeSnapshot()
	require.Equal(t, "forest", selected)
}
