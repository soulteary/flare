package guide

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/pages/home"
	"github.com/soulteary/flare/internal/resources/templates"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

const (
	publicEntry  = "GUIDE-PUBLIC-SENTINEL"
	privateEntry = "GUIDE-PRIVATE-SENTINEL"
	privateLink  = "https://private.guide.example/secret"
	guidePolicy  = "script-src 'self'; script-src-attr 'none'; connect-src 'self'; object-src 'none'; base-uri 'none';"
)

var guideAssets = []string{"/assets/guide/introjs.min.css", "/assets/guide/app.css", "/assets/guide/intro.min.js", "/assets/guide/app.js"}

// guideApp serves the homepage and the guide from the embedded production
// templates behind the persistent authentication middleware. withPrivate adds
// an entry that only an authenticated administrator may see.
func guideApp(t *testing.T, flags model.Flags, withPrivate bool) (*fiber.App, *http.Cookie) {
	t.Helper()
	t.Chdir(t.TempDir())
	original := define.AppFlags
	t.Cleanup(func() { define.AppFlags = original })
	flags.Port, flags.CookieName, flags.EnableOfflineMode, flags.EnableGuide = 5005, "guide-test", true, true
	define.AppFlags = flags
	require.NoError(t, os.WriteFile("config.yml", []byte("Title: Guide test\nShowApps: true\nShowBookmarks: true\nShowDateTime: false\nShowWeather: false\nLocale: en\nTheme: blackboard\n"), 0600))
	apps := model.Bookmarks{Items: []model.Bookmark{{Name: publicEntry, URL: "https://public.guide.example/"}}}
	if withPrivate {
		apps.Items = append(apps.Items, model.Bookmark{Name: privateEntry, URL: privateLink, Private: true})
	}
	require.True(t, data.SaveFavoriteBookmarks(apps))
	require.True(t, data.SaveNormalBookmarks(model.Bookmarks{}))
	t.Cleanup(func() {
		require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{}))
		require.True(t, data.SaveNormalBookmarks(model.Bookmarks{}))
	})
	store, err := security.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.Initialize("admin", "guide test password", "local"))
	token, err := store.CreateSession()
	require.NoError(t, err)
	views, err := templates.NewRenderer()
	require.NoError(t, err)
	app := fiber.New(fiber.Config{Views: views, Immutable: true})
	require.NoError(t, auth.RequestHandle(app, store))
	home.RegisterRouting(app)
	RegisterRouting(app)
	return app, &http.Cookie{Name: auth.RequestHandleSessionName("guide-test", 5005) + "_" + store.Info().InstanceID, Value: token}
}

func guideRequest(t *testing.T, app *fiber.App, path string, cookie *http.Cookie, accept string) (*http.Response, string) {
	t.Helper()
	request := httptest.NewRequest(http.MethodGet, "http://flare.test"+path, nil)
	if cookie != nil {
		request.AddCookie(cookie)
	}
	if accept != "" {
		request.Header.Set("Accept", accept)
	}
	response, err := app.Test(request, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	require.NoError(t, err)
	raw, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	return response, string(raw)
}

// requireGuideDocument checks the markup that the former text substitution got
// wrong: its pattern never matched the unquoted pageview class of the minified
// production template, and it replaced the closing body tag with a second </head>.
func requireGuideDocument(t *testing.T, body string) {
	t.Helper()
	for _, asset := range guideAssets {
		require.Equal(t, 1, strings.Count(body, asset), asset)
	}
	require.Contains(t, body, `id=page-home style="position:inherit;"`)
	require.Equal(t, 1, strings.Count(body, "</head>"))
	require.Equal(t, 1, strings.Count(body, "</body>"))
	require.True(t, strings.HasSuffix(strings.TrimSpace(body), "</body></html>"))
	head, bodyClose := strings.Index(body, "</head>"), strings.Index(body, "</body>")
	require.Less(t, strings.Index(body, "/assets/guide/intro.min.js"), head)
	require.Greater(t, strings.Index(body, "/assets/guide/app.js"), head)
	require.Less(t, strings.Index(body, "/assets/guide/app.js"), bodyClose)
}

func TestGuideRendersHomepageWithAssetsFromProductionTemplate(t *testing.T) {
	app, _ := guideApp(t, model.Flags{DisableLoginMode: true, Visibility: "DEFAULT"}, false)
	require.False(t, define.AppFlags.DebugMode, "the embedded minified templates must be under test")

	response, body := guideRequest(t, app, define.RegularPages.Guide.Path, nil, "text/html")
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, response.Header.Get("Content-Type"), "text/html")
	require.Equal(t, guidePolicy, response.Header.Get("Content-Security-Policy"))
	require.Contains(t, body, publicEntry)
	requireGuideDocument(t, body)

	// The guide must not leak into ordinary homepage responses.
	response, body = guideRequest(t, app, define.RegularPages.Home.Path, nil, "text/html")
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, body, publicEntry)
	require.NotContains(t, body, _ASSETS_WEB_URI)
	require.NotContains(t, body, "position:inherit")
	require.Contains(t, response.Header.Get("Content-Security-Policy"), "script-src 'none'")
}

func TestGuideWithoutContentSecurityPolicy(t *testing.T) {
	app, _ := guideApp(t, model.Flags{DisableLoginMode: true, Visibility: "DEFAULT", DisableCSP: true}, false)
	response, body := guideRequest(t, app, define.RegularPages.Guide.Path, nil, "text/html")
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Empty(t, response.Header.Get("Content-Security-Policy"))
	requireGuideDocument(t, body)
}

func TestGuideAppliesVisitorVisibility(t *testing.T) {
	app, cookie := guideApp(t, model.Flags{Visibility: "DEFAULT"}, true)

	response, body := guideRequest(t, app, define.RegularPages.Guide.Path, nil, "text/html")
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, body, publicEntry)
	require.NotContains(t, body, privateEntry)
	require.NotContains(t, body, privateLink)
	requireGuideDocument(t, body)

	response, body = guideRequest(t, app, define.RegularPages.Guide.Path, cookie, "text/html")
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, body, publicEntry)
	require.Contains(t, body, privateEntry)
	requireGuideDocument(t, body)

	// Markup rendered for the administrator must not be reused for anonymous visitors.
	response, body = guideRequest(t, app, define.RegularPages.Guide.Path, nil, "text/html")
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, body, publicEntry)
	require.NotContains(t, body, privateEntry)
	require.NotContains(t, body, privateLink)
}

func TestGuideOnPrivateSiteRequiresAndUsesTheSession(t *testing.T) {
	app, cookie := guideApp(t, model.Flags{Visibility: "PRIVATE"}, true)

	response, body := guideRequest(t, app, define.RegularPages.Guide.Path, nil, "text/html")
	require.Equal(t, http.StatusFound, response.StatusCode)
	require.Equal(t, "/login?return_to=%2Fguide", response.Header.Get("Location"))
	require.NotContains(t, body, publicEntry)

	response, body = guideRequest(t, app, define.RegularPages.Guide.Path, nil, "")
	require.Equal(t, http.StatusUnauthorized, response.StatusCode)
	require.NotContains(t, body, publicEntry)
	require.NotContains(t, body, privateEntry)

	// Formerly the loopback request carried no session and the page was empty.
	response, body = guideRequest(t, app, define.RegularPages.Guide.Path, cookie, "text/html")
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, body, publicEntry)
	require.Contains(t, body, privateEntry)
	requireGuideDocument(t, body)
}
