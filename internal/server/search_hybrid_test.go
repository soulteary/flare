package server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func TestHybridSearchHTTPAndCSP(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	opts := &model.SearchOptions{Mode: "hybrid", DefaultTarget: "example", Engines: []model.SearchEngine{{ID: "example", Name: "Example", URLTemplate: "https://search.example/?q={query}", Enabled: true}}}
	require.NoError(t, data.UpdateSearchOptions(true, true, opts))
	settings, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	settings.ShowApps = true
	settings.ShowBookmarks = true
	settings.Clock = &model.ClockOptions{Dynamic: true, TimezoneMode: "custom", Timezone: "UTC"}
	settings.ShowDateTime = true
	require.True(t, data.UpdateAppearance(settings))
	require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "Needle", URL: "https://needle.example"}}}))
	get := func() *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
		require.Equal(t, 200, rec.Code)
		return rec
	}
	for _, withClock := range []bool{true, false} {
		settings.ShowDateTime = withClock
		require.True(t, data.UpdateAppearance(settings))
		rec := get()
		scripts := regexp.MustCompile(`<script>([\s\S]*?)</script>`).FindAllStringSubmatch(rec.Body.String(), -1)
		expected := 1
		if withClock {
			expected++
		}
		require.Len(t, scripts, expected)
		for _, script := range scripts {
			sum := sha256.Sum256([]byte(script[1]))
			require.Contains(t, rec.Header().Get("Content-Security-Policy"), "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
		}
		require.Contains(t, rec.Header().Get("Content-Security-Policy"), "connect-src 'self'")
		require.NotContains(t, rec.Header().Get("Content-Security-Policy"), "unsafe-inline")
	}
	suggest := func(query string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/search/suggest", strings.NewReader(url.Values{"search": {query}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	rec := suggest("NEEDLE")
	require.Equal(t, 200, rec.Code)
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	var results []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &results))
	require.Len(t, results, 1)
	require.Equal(t, "Needle", results[0]["name"])
	require.Equal(t, 400, suggest(strings.Repeat("x", 513)).Code)
	opts.Mode = "select"
	require.NoError(t, data.UpdateSearchOptions(true, true, opts))
	require.Equal(t, 404, suggest("needle").Code)
	require.NotContains(t, get().Body.String(), "<script>")
}

func TestPrivateHybridSuggestionsRequireAuthentication(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, EnableOfflineMode: true, Visibility: "PRIVATE", User: "test", Pass: "test-password"})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/search/suggest", strings.NewReader("search=secret"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.NotEqual(t, http.StatusOK, rec.Code)
	require.NotContains(t, rec.Body.String(), "secret")
}

func TestHybridDebugTemplateHashes(t *testing.T) {
	root, err := filepath.Abs("../..")
	require.NoError(t, err)
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	require.NoError(t, os.Symlink(filepath.Join(root, "embed"), "embed"))
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true, DebugMode: true})
	require.NoError(t, err)
	opts := &model.SearchOptions{Mode: "hybrid", DefaultTarget: "example", Engines: []model.SearchEngine{{ID: "example", Name: "Example", URLTemplate: "https://search.example/?q={query}", Enabled: true}}}
	require.NoError(t, data.UpdateSearchOptions(true, true, opts))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, 200, rec.Code)
	scripts := regexp.MustCompile(`<script>([\s\S]*?)</script>`).FindAllStringSubmatch(rec.Body.String(), -1)
	require.NotEmpty(t, scripts)
	for _, script := range scripts {
		sum := sha256.Sum256([]byte(script[1]))
		require.Contains(t, rec.Header().Get("Content-Security-Policy"), "'sha256-"+base64.StdEncoding.EncodeToString(sum[:])+"'")
	}
}
