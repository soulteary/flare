package server

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	searchcore "github.com/soulteary/flare/internal/search"
	"github.com/stretchr/testify/require"
)

func TestSearchTargetsAndEscapedInput(t *testing.T) {
	app := newNativeFiberApp(t, false)
	opts := &model.SearchOptions{Mode: "select", DefaultTarget: "bing", OpenNewTab: true, EnableKeywords: true, Engines: searchcore.Presets()}
	require.NoError(t, data.UpdateSearchOptions(true, true, opts))
	request := func(values url.Values) *http.Response {
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(values.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response, err := app.Test(req)
		require.NoError(t, err)
		return response
	}
	response := request(url.Values{"search": {"机器人 C++ & Go"}, "target": {"bing"}})
	require.Equal(t, http.StatusSeeOther, response.StatusCode)
	location, err := url.Parse(response.Header.Get("Location"))
	require.NoError(t, err)
	require.Equal(t, "机器人 C++ & Go", location.Query().Get("q"))
	require.Equal(t, "no-referrer", response.Header.Get("Referrer-Policy"))
	response.Body.Close()
	// A missing target remains local even when the configured default is external.
	response = request(url.Values{"search": {"example"}})
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Empty(t, response.Header.Get("Location"))
	response.Body.Close()
	response = request(url.Values{"search": {"<script>alert(1)</script>"}, "target": {"missing"}})
	body := nativeFiberBody(t, response)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Contains(t, body, "search target is unavailable")
	require.NotContains(t, body, "value=\"<script>")
	require.Contains(t, body, "&lt;script&gt;")
	require.Contains(t, response.Header.Get("Content-Security-Policy"), "script-src 'none'")
	response = request(url.Values{"search": {"!g Flare"}, "target": {"local"}})
	require.Equal(t, http.StatusSeeOther, response.StatusCode)
	require.Contains(t, response.Header.Get("Location"), "https://www.google.com/search?")
	response.Body.Close()
}

func TestSearchSettingsActionsAndLegacyPreservation(t *testing.T) {
	app := newNativeFiberApp(t, false)
	original := &model.SearchOptions{DefaultTarget: "bing", Engines: searchcore.Presets()}
	require.NoError(t, data.UpdateSearchOptions(true, true, original))
	submit := func(values url.Values) string {
		token, cookie := searchSettingsToken(t, app)
		values.Set("csrf-token", token)
		req := httptest.NewRequest(http.MethodPost, "/settings/search", strings.NewReader(values.Encode()))
		req.AddCookie(cookie)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response, err := app.Test(req)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		return nativeFiberBody(t, response)
	}
	submit(url.Values{"show-search-component": {"1"}, "disabled-search-auto-focus": {"0"}})
	saved, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.Len(t, saved.Search.Engines, 4)
	require.Equal(t, "bing", saved.Search.DefaultTarget)
	values := url.Values{"show-search-component": {"1"}, "engine-count": {"0"}, "search-mode": {"select"}, "default-target": {"local"}, "action": {"preset:bing"}}
	body := submit(values)
	require.Contains(t, body, "https://www.bing.com/search?q={query}")
	saved, err = data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.Len(t, saved.Search.Engines, 4, "working form actions must not save")
	values.Set("action", "save")
	values.Set("default-target", "missing")
	body = submit(values)
	require.Contains(t, body, "default search target must be local or an enabled engine")
	saved, err = data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.Len(t, saved.Search.Engines, 4, "invalid save must retain persisted settings")
}

func TestPrivateSearchRejectsAnonymousSubmission(t *testing.T) {
	originalFlags := define.AppFlags
	t.Cleanup(func() { define.AppFlags = originalFlags })
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	app, err := NewFiberApp(&model.Flags{Port: 5005, Visibility: "PRIVATE", EnableOfflineMode: true, CookieName: "search-private", CookieSecret: "search-private-signing-secret-at-least-32-bytes", User: "search-user", Pass: "search-password"})
	require.NoError(t, err)
	require.NoError(t, data.UpdateSearchOptions(true, true, &model.SearchOptions{DefaultTarget: "bing", Engines: searchcore.Presets()}))
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("search=private&target=bing"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := app.Test(req)
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, http.StatusFound, response.StatusCode)
	require.Equal(t, define.SettingPages.Others.Path, response.Header.Get("Location"))
}
