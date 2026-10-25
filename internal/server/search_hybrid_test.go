package server

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
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

func TestHybridHiddenMalformedModuleDoesNotBlockVisibleSuggestions(t *testing.T) {
	for _, visibleApps := range []bool{true, false} {
		t.Run(fmt.Sprintf("apps-visible-%t", visibleApps), func(t *testing.T) {
			app := newNativeFiberApp(t, false)
			opts := &model.SearchOptions{Mode: "hybrid", DefaultTarget: "example", Engines: []model.SearchEngine{{ID: "example", Name: "Example", URLTemplate: "https://search.example/?q={query}", Enabled: true}}}
			require.NoError(t, data.UpdateSearchOptions(true, true, opts))
			settings, err := data.GetAllSettingsOptions()
			require.NoError(t, err)
			settings.ShowApps, settings.ShowBookmarks = visibleApps, !visibleApps
			require.True(t, data.UpdateAppearance(settings))
			entries := model.Bookmarks{Items: []model.Bookmark{{Name: "Visible needle", URL: "https://visible.example/"}}}
			require.True(t, data.SaveFavoriteBookmarks(entries))
			require.True(t, data.SaveNormalBookmarks(entries))
			hiddenFile := "bookmarks.yml"
			if !visibleApps {
				hiddenFile = "apps.yml"
			}
			require.NoError(t, os.WriteFile(hiddenFile, []byte("links: ["), 0600))
			req := httptest.NewRequest(http.MethodPost, "/search/suggest", strings.NewReader("search=needle"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			response, err := app.Test(req)
			require.NoError(t, err)
			require.Equal(t, http.StatusOK, response.StatusCode)
			var matches []map[string]any
			require.NoError(t, json.Unmarshal([]byte(nativeFiberBody(t, response)), &matches))
			require.Len(t, matches, 1)
			require.Equal(t, "Visible needle", matches[0]["name"])
		})
	}
}

func TestThreeSearchModes(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	for _, mode := range []string{"hybrid", "local-first", "select"} {
		t.Run(mode, func(t *testing.T) {
			opts := &model.SearchOptions{Mode: mode, DefaultTarget: "example", Engines: []model.SearchEngine{{ID: "example", Name: "Example", URLTemplate: "https://search.example/?q={query}", Enabled: true}}}
			require.NoError(t, data.UpdateSearchOptions(true, true, opts))
			get := httptest.NewRecorder()
			handler.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/", nil))
			require.Equal(t, 200, get.Code)
			if mode == "local-first" {
				require.NotContains(t, get.Body.String(), `id=search-target`)
			} else {
				require.Contains(t, get.Body.String(), `id=search-target`)
			}
			for _, target := range []string{"local", "example"} {
				post := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(url.Values{"search": {"missing-result"}, "target": {target}}.Encode()))
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				handler.ServeHTTP(post, req)
				if target == "example" {
					require.Equal(t, http.StatusSeeOther, post.Code)
					require.Equal(t, "https://search.example/?q=missing-result", post.Header().Get("Location"))
				} else {
					require.Equal(t, http.StatusOK, post.Code)
					if mode == "local-first" {
						require.Contains(t, post.Body.String(), "search-external-actions")
					}
				}
			}
		})
	}
	require.NoError(t, data.UpdateSearchOptions(true, true, &model.SearchOptions{Mode: "local-first", DefaultTarget: "local"}))
	post := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("search=missing-result&target=local"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	handler.ServeHTTP(post, req)
	require.Equal(t, 200, post.Code)
	require.NotContains(t, post.Body.String(), `class="search-external-actions"`)
	require.Contains(t, post.Body.String(), "没有找到匹配的书签或应用。")
}
