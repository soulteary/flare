package server

import (
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestHomepageReadabilityHTTP(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	post := func(form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/settings/appearance", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	require.Equal(t, 200, post(url.Values{"navigation-text-size": {"large"}, "title-size": {"compact"}, "hide-app-descriptions": {"1"}}).Code)
	// The previous layout-only form must retain the newer settings.
	require.Equal(t, 200, post(url.Values{"mobile-bookmark-layout": {"compact"}}).Code)
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "large", options.Homepage.NavigationTextSize)
	require.Equal(t, "compact", options.Homepage.TitleSize)
	require.True(t, options.Homepage.HideAppDescriptions)
	for field, value := range map[string]string{"navigation-text-size": "invalid", "title-size": "invalid", "hide-app-descriptions": "invalid"} {
		require.Equal(t, 400, post(url.Values{field: {value}}).Code)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Contains(t, rec.Body.String(), "navigation-text-large homepage-title-compact hide-app-descriptions")
}
