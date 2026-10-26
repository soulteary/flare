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

func TestBookmarkLayoutHTTP(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	postForm := func(form url.Values) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/settings/appearance", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	post := func(value string) *httptest.ResponseRecorder {
		return postForm(url.Values{"bookmark-layout": {value}, "title": {"Layout"}})
	}
	require.Equal(t, 200, postForm(url.Values{
		"bookmark-layout": {"sections"}, "desktop-columns": {"6"},
		"category-navigation": {"tabs"}, "mobile-bookmark-layout": {"compact"},
		"navigation-text-size": {"large"},
	}).Code)
	// Older forms with other appearance fields must retain the new layout.
	require.Equal(t, 200, postForm(url.Values{"mobile-bookmark-layout": {"single"}}).Code)
	require.Equal(t, 200, postForm(url.Values{"desktop-columns": {"8"}}).Code)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Contains(t, rec.Body.String(), "bookmark-layout-sections")
	before, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, 400, post("unknown").Code)
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, before, after)
	require.Equal(t, 200, post("").Code)
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "sections", options.Homepage.BookmarkLayout)
	require.Equal(t, "8", options.Homepage.DesktopColumns)
	require.Equal(t, "tabs", options.Homepage.CategoryNavigation)
	require.Equal(t, "single", options.Homepage.MobileBookmarkLayout)
	require.Equal(t, "large", options.Homepage.NavigationTextSize)
	require.Equal(t, 200, post("groups").Code)
	options, err = data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "groups", options.Homepage.BookmarkLayout)
}
