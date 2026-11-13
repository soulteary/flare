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
	"regexp"
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

// Versions 0.6.5–0.6.14 stored "single" whenever the appearance form was saved,
// so that value cannot be told apart from an untouched default.
func TestHomepageMobileLayoutDefaultHTTP(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	require.NoError(t, os.WriteFile("config.yml", []byte("Title: Legacy\nHomepage:\n  MobileBookmarkLayout: single\n"), 0600))
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	get := func(path string) string {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, 200, rec.Code)
		return rec.Body.String()
	}
	post := func(value string) int {
		form := url.Values{"mobile-bookmark-layout": {value}, "title": {"Legacy"}}
		req := httptest.NewRequest(http.MethodPost, "/settings/appearance", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec.Code
	}
	stored := func() string {
		options, err := data.GetAllSettingsOptions()
		require.NoError(t, err)
		require.NotNil(t, options.Homepage)
		return options.Homepage.MobileBookmarkLayout
	}
	selected := func(value string) *regexp.Regexp {
		return regexp.MustCompile(`<option value="?` + value + `"? selected`)
	}
	// The inline stylesheet names every layout class, so inspect the body element.
	bodyClass := regexp.MustCompile(`<body [^>]*class="([^"]*)"`)
	layoutClasses := func(path string) string {
		match := bodyClass.FindStringSubmatch(get(path))
		require.Len(t, match, 2)
		return match[1]
	}

	// The stored legacy value renders the classic layout and is offered as such.
	require.NotContains(t, layoutClasses("/"), "mobile-bookmarks-")
	settings := get("/settings/appearance")
	require.Regexp(t, selected("classic"), settings)
	require.NotRegexp(t, selected("single-column"), settings)
	require.NotContains(t, settings, `value=single `)
	require.NotContains(t, settings, `value="single"`)
	require.Equal(t, "single", stored())

	// Saving the rendered form writes the current name of the default.
	require.Equal(t, 200, post("classic"))
	require.Equal(t, "classic", stored())
	require.NotContains(t, layoutClasses("/"), "mobile-bookmarks-")

	// The single-column layout is an explicit choice with its own class.
	require.Equal(t, 200, post("single-column"))
	require.Equal(t, "single-column", stored())
	for _, path := range []string{"/", "/bookmarks", "/applications"} {
		require.Contains(t, layoutClasses(path), "mobile-bookmarks-single-column ")
	}
	require.Regexp(t, selected("single-column"), get("/settings/appearance"))

	// A form rendered before the upgrade still submits the legacy value.
	require.Equal(t, 200, post("single"))
	require.Equal(t, "classic", stored())
	require.NotContains(t, layoutClasses("/"), "mobile-bookmarks-")
	require.Regexp(t, selected("classic"), get("/settings/appearance"))
}
