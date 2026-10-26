package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func searchSettingsToken(t *testing.T, app *fiber.App) (string, *http.Cookie) {
	t.Helper()
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/settings/search", nil))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Equal(t, "no-store", response.Header.Get("Cache-Control"))
	body := nativeFiberBody(t, response)
	match := regexp.MustCompile(`name="?csrf-token"? value="?([A-Za-z0-9_.-]+)`).FindStringSubmatch(body)
	require.Len(t, match, 2, "missing production form token")
	for _, cookie := range response.Cookies() {
		if strings.HasPrefix(cookie.Name, "flare_search_csrf_") {
			require.True(t, cookie.HttpOnly)
			require.Equal(t, http.SameSiteStrictMode, cookie.SameSite)
			require.Equal(t, "/settings/search", cookie.Path)
			require.Empty(t, cookie.Domain)
			return match[1], cookie
		}
	}
	t.Fatal("missing search token cookie")
	return "", nil
}

func TestSearchSettingsSignedToken(t *testing.T) {
	app := newNativeFiberApp(t, false)
	require.NoError(t, data.UpdateSearchOptions(true, true, &model.SearchOptions{DefaultTarget: "local"}))
	token, cookie := searchSettingsToken(t, app)
	otherToken, otherCookie := searchSettingsToken(t, app)
	for _, tc := range []struct {
		name, token string
		cookie      *http.Cookie
		headers     http.Header
		want        int
	}{
		{name: "valid", token: token, cookie: cookie, want: 200},
		{name: "metadata-free client with token", token: token, cookie: cookie, want: 200},
		{name: "TLS offload origin", token: token, cookie: cookie, headers: http.Header{"Origin": {"https://example.com"}, "Sec-Fetch-Site": {"same-origin"}}, want: 200},
		{name: "missing token", cookie: cookie, want: 403},
		{name: "duplicate cookies", token: token, cookie: cookie, headers: http.Header{"Cookie": {cookie.Name + "=" + token}}, want: 403},
		{name: "missing cookie", token: token, want: 403},
		{name: "forged matching token", token: "forged", cookie: &http.Cookie{Name: cookie.Name, Value: "forged"}, want: 403},
		{name: "other cookie", token: token, cookie: otherCookie, want: 403},
		{name: "other token", token: otherToken, cookie: cookie, want: 403},
		{name: "cross-site", token: token, cookie: cookie, headers: http.Header{"Sec-Fetch-Site": {"cross-site"}}, want: 403},
		{name: "same-site sibling", token: token, cookie: cookie, headers: http.Header{"Sec-Fetch-Site": {"same-site"}}, want: 403},
		{name: "opaque origin", token: token, cookie: cookie, headers: http.Header{"Origin": {"null"}}, want: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			values := url.Values{"show-search-component": {"1"}, "csrf-token": {tc.token}}
			req := httptest.NewRequest(http.MethodPost, "http://example.com/settings/search", strings.NewReader(values.Encode()))
			if tc.headers != nil {
				req.Header = tc.headers.Clone()
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			if tc.cookie != nil {
				req.AddCookie(tc.cookie)
			}
			response, err := app.Test(req)
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			require.Equal(t, tc.want, response.StatusCode)
		})
	}
}

func TestAuthenticatedSearchSettingsToken(t *testing.T) {
	app := newNativeFiberApp(t, true)
	client, base := nativeFiberClient(t, app, false)
	response, err := client.PostForm(base+define.MiscPages.Login.Path, url.Values{"username": {"native-user"}, "password": {"native-password"}})
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, response.StatusCode)
	require.NoError(t, response.Body.Close())
	response, err = client.Get(base + "/settings/search")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	body := nativeFiberBody(t, response)
	match := regexp.MustCompile(`name="?csrf-token"? value="?([A-Za-z0-9_.-]+)`).FindStringSubmatch(body)
	require.Len(t, match, 2)
	response, err = client.PostForm(base+"/settings/search", url.Values{"show-search-component": {"1"}, "csrf-token": {match[1]}})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, nativeFiberBody(t, response), "搜索设置已保存")
}

func TestSearchSettingsWriteFailureRetainsSubmittedForm(t *testing.T) {
	app := newNativeFiberApp(t, false)
	require.NoError(t, data.UpdateSearchOptions(true, true, &model.SearchOptions{DefaultTarget: "local"}))
	token, cookie := searchSettingsToken(t, app)
	original, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.NoError(t, os.Rename("config.yml", "config-original.yml"))
	require.NoError(t, os.Mkdir("config.yml", 0700))
	t.Cleanup(func() {
		require.NoError(t, os.Remove("config.yml"))
		require.NoError(t, os.Rename("config-original.yml", "config.yml"))
	})
	values := url.Values{"csrf-token": {token}, "show-search-component": {"1"}, "engine-count": {"1"}, "search-mode": {"select"}, "default-target": {"local"}, "action": {"save"}, "engine-0-id": {"custom"}, "engine-0-name": {"Unsaved search engine"}, "engine-0-url": {"https://search.example/?q={query}"}, "engine-0-enabled": {"1"}}
	req := httptest.NewRequest(http.MethodPost, "/settings/search", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	response, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	body := nativeFiberBody(t, response)
	require.Contains(t, body, "Unable to save search settings")
	require.Contains(t, body, "Unsaved search engine")
	require.NotContains(t, body, "搜索设置已保存")
	after, err := os.ReadFile("config-original.yml")
	require.NoError(t, err)
	require.Equal(t, original, after)
	info, err := os.Stat("config.yml")
	require.NoError(t, err)
	require.True(t, info.IsDir())
}

func TestSearchSettingsStaleFormRetainsInputWithoutOverwriting(t *testing.T) {
	app := newNativeFiberApp(t, false)
	require.NoError(t, data.UpdateSearchOptions(true, true, &model.SearchOptions{DefaultTarget: "local"}))
	token, cookie := searchSettingsToken(t, app)
	loaded, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.True(t, data.UpdateThemeName("blackboard"))
	// Ensure a real independent edit even if the theme already matched.
	require.True(t, data.UpdateThemeName("stale-form-test-theme"))
	original, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	values := url.Values{"csrf-token": {token}, "config-revision": {loaded.ConfigRevision}, "show-search-component": {"1"}, "engine-count": {"1"}, "search-mode": {"select"}, "default-target": {"local"}, "action": {"save"}, "engine-0-id": {"custom"}, "engine-0-name": {"Keep my unsaved input"}, "engine-0-url": {"https://search.example/?q={query}"}, "engine-0-enabled": {"1"}}
	req := httptest.NewRequest(http.MethodPost, "/settings/search", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(cookie)
	response, err := app.Test(req)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	body := nativeFiberBody(t, response)
	require.Contains(t, body, "configuration changed since this form was loaded")
	require.Contains(t, body, "Keep my unsaved input")
	require.NotContains(t, body, "搜索设置已保存")
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, original, after)
}

func TestSearchEditingActionsPreserveSubmittedHybridMode(t *testing.T) {
	for _, action := range []string{"add", "preset:baidu", "up:1", "down:0", "delete:1"} {
		t.Run(action, func(t *testing.T) {
			app := newNativeFiberApp(t, false)
			options := &model.SearchOptions{Mode: "select", DefaultTarget: "local", Engines: []model.SearchEngine{
				{ID: "bing", Name: "Bing", URLTemplate: "https://www.bing.com/search?q={query}", Enabled: true},
				{ID: "google", Name: "Google", URLTemplate: "https://www.google.com/search?q={query}", Enabled: true},
			}}
			require.NoError(t, data.UpdateSearchOptions(true, true, options))
			token, cookie := searchSettingsToken(t, app)
			original, err := os.ReadFile("config.yml")
			require.NoError(t, err)
			values := url.Values{"csrf-token": {token}, "show-search-component": {"1"}, "engine-count": {"2"}, "search-mode": {"hybrid"}, "default-target": {"local"}, "action": {action}, "engine-0-id": {"bing"}, "engine-0-name": {"Bing"}, "engine-0-url": {"https://www.bing.com/search?q={query}"}, "engine-0-enabled": {"1"}, "engine-1-id": {"google"}, "engine-1-name": {"Google"}, "engine-1-url": {"https://www.google.com/search?q={query}"}, "engine-1-enabled": {"1"}}
			req := httptest.NewRequest(http.MethodPost, "/settings/search", strings.NewReader(values.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(cookie)
			response, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			body := nativeFiberBody(t, response)
			require.Regexp(t, `name="?search-mode"? value="?hybrid"? checked`, body)
			require.NotRegexp(t, `name="?search-mode"? value="?local-first"? checked`, body)
			after, err := os.ReadFile("config.yml")
			require.NoError(t, err)
			require.Equal(t, original, after)
		})
	}
}
