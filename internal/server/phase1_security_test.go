package server

import (
	"github.com/gofiber/fiber/v3"
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

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/security"
	"github.com/soulteary/flare/internal/yamlutil"
	"github.com/stretchr/testify/require"
)

func TestPhase1StartupGuardBlocksAllBusinessSurfaces(t *testing.T) {
	original := define.AppFlags
	t.Cleanup(func() { define.AppFlags = original })
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	require.NoError(t, os.WriteFile("bookmarks.yml", []byte("links:\n  - name: DO-NOT-LEAK-SENTINEL\n    link: https://private.example.com\n"), 0600))
	for _, status := range []string{"new", "attention_required", "faulted", "migrating"} {
		app, e := NewFiberApp(&model.Flags{StartupStatus: status, EnableEditor: true, EnableOfflineMode: true})
		require.NoError(t, e)
		for _, path := range []string{"/", "/login", "/editor", "/settings/others", "/search?keyword=DO-NOT-LEAK-SENTINEL", "/health", "/bookmarks.yml", "/logout"} {
			response, e := app.Test(httptest.NewRequest("GET", "http://flare.test"+path, nil))
			require.NoError(t, e)
			raw, e := io.ReadAll(response.Body)
			response.Body.Close()
			require.NoError(t, e)
			require.Equal(t, 503, response.StatusCode, status+path)
			require.Equal(t, "private, no-store", response.Header.Get("Cache-Control"))
			require.NotContains(t, string(raw), "DO-NOT-LEAK-SENTINEL")
		}
	}
}
func TestPhase1PersistentStoreAuthorizationAndRevocation(t *testing.T) {
	original := define.AppFlags
	t.Cleanup(func() { define.AppFlags = original })
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	require.NoError(t, os.WriteFile("apps.yml", []byte("links: []\n"), 0600))
	require.NoError(t, os.WriteFile("bookmarks.yml", []byte("categories:\n  - id: public\n    title: PUBLIC-CATEGORY\n  - id: secret\n    title: PRIVATE-CATEGORY-SENTINEL\n    private: true\nlinks:\n  - name: PUBLIC-BOOKMARK-SENTINEL\n    link: https://example.com/public\n    category: public\n  - name: PRIVATE-ITEM-SENTINEL\n    link: https://example.com/secret\n    category: public\n    private: true\n  - name: PRIVATE-INHERITED-SENTINEL\n    link: https://example.com/inherited\n    category: secret\n    private: false\n"), 0600))
	// The application supports one data directory per process. Saving the fixture
	// explicitly invalidates navigation caches retained by earlier router tests.
	rawFixture, e := os.ReadFile("bookmarks.yml")
	require.NoError(t, e)
	var bookmarkFixture model.Bookmarks
	require.NoError(t, yamlutil.DecodeBookmarks(rawFixture, &bookmarkFixture, false))
	require.True(t, data.SaveNormalBookmarks(bookmarkFixture))
	require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{}))
	require.NoError(t, os.WriteFile("config.yml", []byte("ShowBookmarks: true\nShowApps: true\nShowWeather: false\nTheme: blackboard\n"), 0600))
	store, e := security.Open(filepath.Join(t.TempDir(), "state"))
	require.NoError(t, e)
	t.Cleanup(func() { store.Close() })
	require.NoError(t, store.Initialize("admin", "legacy short", "local"))
	flags := &model.Flags{Port: 5005, CookieName: "phase1", Visibility: "DEFAULT", StartupStatus: "ready", EnableOfflineMode: true, EnableEditor: true}
	app, e := NewFiberApp(flags, store)
	require.NoError(t, e)
	cookieName := auth.RequestHandleSessionName("phase1", 5005) + "_" + store.Info().InstanceID
	request := func(path, token string) (int, string) {
		req := httptest.NewRequest("GET", "http://flare.test"+path, nil)
		if token != "" {
			req.AddCookie(&http.Cookie{Name: cookieName, Value: token})
		}
		response, requestErr := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		require.NoError(t, requestErr)
		raw, readErr := io.ReadAll(response.Body)
		response.Body.Close()
		require.NoError(t, readErr)
		require.Equal(t, "private, no-store", response.Header.Get("Cache-Control"))
		return response.StatusCode, string(raw)
	}
	status, raw := request("/", "")
	require.Equal(t, 200, status)
	require.Contains(t, raw, "PUBLIC-BOOKMARK-SENTINEL")
	for _, secret := range []string{"PRIVATE-ITEM-SENTINEL", "PRIVATE-INHERITED-SENTINEL", "PRIVATE-CATEGORY-SENTINEL"} {
		require.NotContains(t, raw, secret)
	}
	token, e := store.Authenticate("admin", "legacy short")
	require.NoError(t, e)
	status, raw = request("/", token)
	require.Equal(t, 200, status)
	require.Contains(t, raw, "PRIVATE-ITEM-SENTINEL")
	require.Contains(t, raw, "PRIVATE-INHERITED-SENTINEL")
	require.NoError(t, store.RevokeAll())
	_, raw = request("/", token)
	require.NotContains(t, raw, "PRIVATE-ITEM-SENTINEL")
	status, _ = request("/editor", token)
	require.Equal(t, 401, status)
}

func TestPhase1SettingsFormsUseSessionCSRF(t *testing.T) {
	for _, mode := range []string{"local", security.ModeTrusted} {
		t.Run(mode, func(t *testing.T) {
			original := define.AppFlags
			t.Cleanup(func() { define.AppFlags = original })
			t.Chdir(t.TempDir())
			t.Setenv("FLARE_BASELINE", "1")
			require.NoError(t, os.WriteFile("config.yml", []byte("ShowWeather: false\nTheme: blackboard\nTitle: original\n"), 0600))
			store, e := security.Open(t.TempDir())
			require.NoError(t, e)
			t.Cleanup(func() { store.Close() })
			if mode == security.ModeTrusted {
				require.NoError(t, store.Initialize("", "", mode))
			} else {
				require.NoError(t, store.Initialize("admin", "legacy", mode))
			}
			flags := &model.Flags{Port: 5005, CookieName: "settings", Visibility: "DEFAULT", StartupStatus: "ready", DisableLoginMode: mode == security.ModeTrusted}
			app, e := NewFiberApp(flags, store)
			require.NoError(t, e)
			cookies := map[string]*http.Cookie{}
			if mode != "trusted" {
				token, e := store.Authenticate("admin", "legacy")
				require.NoError(t, e)
				name := auth.RequestHandleSessionName("settings", 5005) + "_" + store.Info().InstanceID
				cookies[name] = &http.Cookie{Name: name, Value: token}
			}
			request := func(method, path string, values url.Values, origin string) (int, string) {
				var body io.Reader
				if values != nil {
					body = strings.NewReader(values.Encode())
				}
				req := httptest.NewRequest(method, "http://flare.test"+path, body)
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				for _, cookie := range cookies {
					req.AddCookie(cookie)
				}
				if origin != "" {
					req.Header.Set("Origin", origin)
				}
				res, e := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
				require.NoError(t, e)
				for _, cookie := range res.Cookies() {
					cookies[cookie.Name] = cookie
				}
				raw, e := io.ReadAll(res.Body)
				res.Body.Close()
				require.NoError(t, e)
				return res.StatusCode, string(raw)
			}
			for _, path := range []string{"/settings/theme", "/settings/appearance", "/settings/weather", "/settings/search"} {
				status, page := request("GET", path, nil, "")
				require.Equal(t, 200, status, path)
				match := regexp.MustCompile(`name="?csrf-token"? value="?([A-Za-z0-9_.-]+)`).FindStringSubmatch(page)
				require.Len(t, match, 2, path+" must render usable unified token")
				before, e := os.ReadFile("config.yml")
				require.NoError(t, e)
				status, _ = request("POST", path, url.Values{"theme": {"blackboard"}}, "")
				require.Equal(t, 403, status, path)
				status, _ = request("POST", path, url.Values{"csrf-token": {match[1]}, "theme": {"blackboard"}}, "https://evil.example")
				require.Equal(t, 403, status, path)
				after, e := os.ReadFile("config.yml")
				require.NoError(t, e)
				require.Equal(t, before, after, "rejected write changed config")
				status, page = request("POST", path, url.Values{"csrf-token": {match[1]}, "theme": {"blackboard"}, "title": {"preserved"}, "location": {""}}, "")
				require.Equal(t, 200, status, path+page)
			}
		})
	}
}
