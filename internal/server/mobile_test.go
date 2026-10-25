package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func TestMobileSearchAutofocus(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	opts, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	opts.Clock = &model.ClockOptions{TimezoneMode: "server"}
	require.True(t, data.UpdateAppearance(opts))
	require.True(t, data.UpdateSearch(true, false))
	for _, tc := range []struct {
		ua        string
		autofocus bool
	}{
		{"Mozilla/5.0 (X11; Linux x86_64) Chrome/153.0", true},
		{"Mozilla/5.0 (Linux; Android 14) Mobile Safari/537.36", false},
		{"Mozilla/5.0 (iPhone; CPU iPhone OS 18_0) Safari", false},
		{"Mozilla/5.0 (iPad; CPU OS 18_0) Safari", false},
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("User-Agent", tc.ua)
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		require.Equal(t, http.StatusOK, rec.Code)
		require.Equal(t, tc.autofocus, strings.Contains(rec.Body.String(), "autofocus"), tc.ua)
		require.Contains(t, rec.Header().Values("Vary"), "User-Agent")
		require.Contains(t, rec.Header().Get("Content-Security-Policy"), "script-src 'none'")
	}
	require.True(t, data.UpdateSearch(true, true))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.NotContains(t, rec.Body.String(), "autofocus")
}
