package server

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
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

// The guide used to fetch the homepage from http://localhost:<port>/. A Host
// allow-list rejects that authority, so the guide was served as an empty page.
func TestGuideRendersBehindHostAllowList(t *testing.T) {
	t.Chdir(t.TempDir())
	original := define.AppFlags
	t.Cleanup(func() { define.AppFlags = original })
	t.Setenv("FLARE_BASELINE", "1")
	require.NoError(t, os.WriteFile("config.yml", []byte("Title: Guide allow-list\nShowApps: true\nShowBookmarks: true\nShowDateTime: false\nShowWeather: false\nLocale: en\nTheme: blackboard\n"), 0600))
	require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "GUIDE-ALLOW-LIST-SENTINEL", URL: "https://example.com/"}}}))
	require.True(t, data.SaveNormalBookmarks(model.Bookmarks{}))
	t.Cleanup(func() {
		require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{}))
		require.True(t, data.SaveNormalBookmarks(model.Bookmarks{}))
	})
	store, err := security.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.Initialize("", "", security.ModeTrusted))
	app, err := NewFiberApp(&model.Flags{Port: 5005, CookieName: "guide-allow-list", DisableLoginMode: true, Visibility: "DEFAULT",
		EnableOfflineMode: true, EnableGuide: true, AllowedHosts: "flare.example", StartupStatus: "ready"}, store)
	require.NoError(t, err)
	get := func(target string) (int, string) {
		response, requestErr := app.Test(httptest.NewRequest(http.MethodGet, target, nil), fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		require.NoError(t, requestErr)
		body, readErr := io.ReadAll(response.Body)
		require.NoError(t, readErr)
		require.NoError(t, response.Body.Close())
		return response.StatusCode, string(body)
	}

	// The loopback authority of the former sub-request is outside the allow-list.
	status, _ := get("http://localhost:5005" + define.RegularPages.Home.Path)
	require.Equal(t, http.StatusBadRequest, status)

	status, body := get("http://flare.example" + define.RegularPages.Guide.Path)
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, "GUIDE-ALLOW-LIST-SENTINEL")
	require.Contains(t, body, "/assets/guide/intro.min.js")
	require.Contains(t, body, "/assets/guide/app.js")
	require.True(t, strings.HasSuffix(strings.TrimSpace(body), "</body></html>"))
}
