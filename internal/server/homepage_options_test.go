package server

import (
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
)

func TestHomepageLayoutHTTP(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	post := func(value string) *httptest.ResponseRecorder {
		form := url.Values{"mobile-bookmark-layout": {value}, "title": {"Layout"}}
		req := httptest.NewRequest(http.MethodPost, "/settings/appearance", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	require.Equal(t, 200, post("compact").Code)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Contains(t, rec.Body.String(), "mobile-bookmarks-compact")
	before, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, 400, post("unknown").Code)
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Equal(t, 200, post("").Code)
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "compact", options.Homepage.MobileBookmarkLayout)
}
