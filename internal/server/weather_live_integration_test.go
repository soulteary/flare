//go:build integration

package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

// Uses the full production router, login middleware and native listener. The
// socket is explicitly loopback-only, and all configuration is temporary.
func TestLiveNativeWeather(t *testing.T) {
	t.Chdir(t.TempDir())
	original := define.AppFlags
	t.Cleanup(func() { define.AppFlags = original })
	require.True(t, data.UpdateWeatherAndLocation(false, ""))
	flags := model.Flags{Port: 5005, Visibility: "DEFAULT", CookieName: "live-test", CookieSecret: "live-weather-test-signing-key-32-bytes", User: "live-test", Pass: "temporary-test-password"}
	router, err := newRouter(&flags)
	require.NoError(t, err)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	stopped := make(chan error, 1)
	go func() { stopped <- router.Serve(listener) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		require.NoError(t, router.Shutdown(ctx))
		require.NoError(t, <-stopped)
	}()
	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	base := "http://" + listener.Addr().String()
	response, err := client.PostForm(base+"/login", url.Values{"username": {flags.User}, "password": {flags.Pass}})
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, 302, response.StatusCode)
	for _, keyword := range []string{"北京", "北京市", "朝阳", "津市"} {
		response, err = client.Get(base + "/settings/weather/locations?q=" + url.QueryEscape(keyword))
		require.NoError(t, err)
		var candidates []model.WeatherLocation
		raw, err := io.ReadAll(response.Body)
		response.Body.Close()
		require.NoError(t, err)
		require.Equal(t, 200, response.StatusCode, string(raw))
		require.NoError(t, json.Unmarshal(raw, &candidates))
		require.NotEmpty(t, candidates)
		candidate := candidates[0]
		response, err = client.PostForm(base+"/settings/weather", url.Values{"show": {"1"}, "keyword": {keyword}, "selected": {candidate.ID}, "location": {candidate.String()}})
		require.NoError(t, err)
		response.Body.Close()
		require.Equal(t, 200, response.StatusCode)
		response, err = client.PostForm(base+"/settings/weather/test", nil)
		require.NoError(t, err)
		raw, err = io.ReadAll(response.Body)
		response.Body.Close()
		require.NoError(t, err)
		require.Equal(t, 200, response.StatusCode)
		require.True(t, strings.Contains(string(raw), "°C"), string(raw))
		t.Logf("full production router: %s -> %s, saved and queried weather", keyword, candidate.String())
	}
}
