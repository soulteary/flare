package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/resources/assets"
	"github.com/soulteary/flare/internal/resources/mdi"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

func TestPersistentStorePublicAssetCacheAndPrivateResponses(t *testing.T) {
	for _, mode := range []struct {
		name                    string
		disabled, authenticated bool
	}{
		{name: "login disabled", disabled: true},
		{name: "anonymous"},
		{name: "authenticated", authenticated: true},
	} {
		t.Run(mode.name, func(t *testing.T) {
			original := define.AppFlags
			t.Cleanup(func() { define.AppFlags = original })
			t.Chdir(t.TempDir())
			t.Setenv("FLARE_BASELINE", "1")
			require.NoError(t, os.WriteFile("config.yml", []byte("ShowApps: true\nShowBookmarks: true\nShowWeather: false\nTheme: blackboard\n"), 0600))
			fixture := model.Bookmarks{Items: []model.Bookmark{
				{Name: "PUBLIC-CACHE-FIXTURE", URL: "https://example.com/public"},
				{Name: "PRIVATE-CACHE-SENTINEL", URL: "https://example.com/private", Private: true},
			}}
			require.True(t, data.SaveFavoriteBookmarks(fixture))
			require.True(t, data.SaveNormalBookmarks(fixture))
			// Navigation caches are process-wide. Invalidate the fixture before
			// t.Chdir restores the directory used by the next HTTP test.
			t.Cleanup(func() {
				require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{}))
				require.True(t, data.SaveNormalBookmarks(model.Bookmarks{}))
			})
			store, err := security.Open(t.TempDir())
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			require.NoError(t, store.Initialize("admin", "cache integration password", "local"))
			flags := &model.Flags{Port: 5005, CookieName: "cache-test", DisableLoginMode: mode.disabled, EnableOfflineMode: true, EnableEditor: true}
			app, err := NewFiberApp(flags, store)
			require.NoError(t, err)
			cookie := &http.Cookie{Name: auth.RequestHandleSessionName(flags.CookieName, flags.Port) + "_" + store.Info().InstanceID}
			if mode.authenticated {
				cookie.Value, err = store.Authenticate("admin", "cache integration password")
				require.NoError(t, err)
			}
			request := func(method, path string, headers map[string]string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(method, "http://flare.test"+path, nil)
				if cookie.Value != "" {
					req.AddCookie(cookie)
				}
				for name, value := range headers {
					req.Header.Set(name, value)
				}
				response, err := app.Test(req)
				require.NoError(t, err)
				defer response.Body.Close()
				rec := httptest.NewRecorder()
				for name, values := range response.Header {
					rec.Header()[name] = values
				}
				rec.WriteHeader(response.StatusCode)
				_, err = io.Copy(rec.Body, response.Body)
				require.NoError(t, err)
				return rec
			}
			require.NotEmpty(t, mdi.GetIconByName("server"))
			paths := []string{"/favicon.ico", "/assets/editor-ui/editor.css", "/assets/editor-ui/app.js", "/assets/mdi/server.svg", assets.ThemeFaviconURL()}
			for _, path := range paths {
				policy := "public, max-age=0, must-revalidate"
				if path == assets.ThemeFaviconURL() {
					policy = "public, max-age=31536000, immutable"
				}
				get := request(http.MethodGet, path, nil)
				require.Equal(t, http.StatusOK, get.Code, path)
				require.Equal(t, policy, get.Header().Get("Cache-Control"), path)
				require.Empty(t, get.Header().Values("Set-Cookie"), path)
				require.NotContains(t, get.Body.String(), "PRIVATE-CACHE-SENTINEL", path)
				etag := get.Header().Get("ETag")
				require.NotEmpty(t, etag, path)
				for _, method := range []string{http.MethodGet, http.MethodHead} {
					conditional := request(method, path, map[string]string{"If-None-Match": etag})
					require.Equal(t, http.StatusNotModified, conditional.Code, path)
					require.Equal(t, policy, conditional.Header().Get("Cache-Control"), path)
					require.Equal(t, etag, conditional.Header().Get("ETag"), path)
					require.Empty(t, conditional.Body.String(), path)
				}
				head := request(http.MethodHead, path, nil)
				require.Equal(t, http.StatusOK, head.Code, path)
				require.Equal(t, policy, head.Header().Get("Cache-Control"), path)
				require.Empty(t, head.Body.String(), path)
				partial := request(http.MethodGet, path, map[string]string{"Range": "bytes=0-3"})
				require.Equal(t, http.StatusPartialContent, partial.Code, path)
				require.Equal(t, policy, partial.Header().Get("Cache-Control"), path)
				require.Equal(t, get.Body.String()[:4], partial.Body.String(), path)
				invalid := request(http.MethodGet, path, map[string]string{"Range": "bytes=999999999-"})
				require.Equal(t, http.StatusRequestedRangeNotSatisfiable, invalid.Code, path)
				require.Equal(t, "no-store", invalid.Header().Get("Cache-Control"), path)
				require.Empty(t, invalid.Header().Get("ETag"), path)
			}
			missing := request(http.MethodGet, "/assets/editor-ui/missing.js", map[string]string{"If-None-Match": "*"})
			require.Equal(t, http.StatusNotFound, missing.Code)
			require.Equal(t, "no-store", missing.Header().Get("Cache-Control"))
			require.Empty(t, missing.Header().Get("ETag"))
			for _, path := range []string{"/", "/applications", "/bookmarks"} {
				page := request(http.MethodGet, path, map[string]string{"If-None-Match": "*"})
				require.Equal(t, http.StatusOK, page.Code, path)
				require.Equal(t, "private, no-store", page.Header().Get("Cache-Control"), path)
				if mode.disabled {
					// Without login a collection that holds a private entry is
					// withheld as a whole; the module reports why.
					require.Regexp(t, `class="?navigation-data-error"? role="?alert"?`, page.Body.String(), path)
					require.NotContains(t, page.Body.String(), "PUBLIC-CACHE-FIXTURE", path)
					require.NotContains(t, page.Body.String(), "PRIVATE-CACHE-SENTINEL", path)
					continue
				}
				require.Contains(t, page.Body.String(), "PUBLIC-CACHE-FIXTURE", path)
				if mode.authenticated {
					require.Contains(t, page.Body.String(), "PRIVATE-CACHE-SENTINEL", path)
				} else {
					require.NotContains(t, page.Body.String(), "PRIVATE-CACHE-SENTINEL", path)
				}
			}
			if mode.disabled || mode.authenticated {
				preview := request(http.MethodGet, "/settings/content/icon?name=server", nil)
				require.Equal(t, http.StatusOK, preview.Code)
				require.Equal(t, "no-store", preview.Header().Get("Cache-Control"))
			}
			for _, path := range []string{"/settings/content", "/settings/content/csrf", "/settings/content/document"} {
				page := request(http.MethodGet, path, map[string]string{"If-None-Match": "*"})
				if mode.disabled && path == "/settings/content/document" {
					require.Equal(t, http.StatusConflict, page.Code, path)
					require.NotContains(t, page.Body.String(), "PRIVATE-CACHE-SENTINEL", path)
				} else if mode.disabled || mode.authenticated {
					require.Equal(t, http.StatusOK, page.Code, path)
				} else {
					require.NotEqual(t, http.StatusOK, page.Code, path)
					require.NotContains(t, page.Body.String(), "PRIVATE-CACHE-SENTINEL", path)
				}
				require.Contains(t, page.Header().Get("Cache-Control"), "no-store", path)
				require.NotContains(t, page.Header().Get("Cache-Control"), "public", path)
				require.Empty(t, page.Header().Get("ETag"), path)
			}
			if mode.authenticated {
				require.NoError(t, store.RevokeAll())
				page := request(http.MethodGet, "/", nil)
				require.Equal(t, "private, no-store", page.Header().Get("Cache-Control"))
				require.NotContains(t, page.Body.String(), "PRIVATE-CACHE-SENTINEL")
			}
		})
	}
}

func TestAVersionThatNamesTheContentIsKeptWithoutAsking(t *testing.T) {
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true, EnableEditor: true})
	require.NoError(t, err)
	request := func(method, path, etag string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, path, nil)
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}
	page := request(http.MethodGet, "/settings/content", "")
	require.Equal(t, http.StatusOK, page.Code)
	references := regexp.MustCompile(`/assets/editor-ui/[a-zA-Z0-9._-]+`).FindAllString(page.Body.String(), -1)
	paths := regexp.MustCompile(`/assets/editor-ui/[a-zA-Z0-9._-]+\?v=[a-f0-9]{64}`).FindAllString(page.Body.String(), -1)
	require.NotEmpty(t, references)
	require.Len(t, paths, len(references), "all editor styles and scripts, including i18n, must bypass an older fresh cache entry")
	const kept, revalidated = "public, max-age=31536000, immutable", "public, max-age=0, must-revalidate"
	for _, path := range paths {
		get := request(http.MethodGet, path, "")
		require.Equal(t, http.StatusOK, get.Code, path)
		require.Equal(t, kept, get.Header().Get("Cache-Control"), path)
		etag := get.Header().Get("ETag")
		require.NotEmpty(t, etag, path)
		unchanged := request(http.MethodGet, path, etag)
		require.Equal(t, http.StatusNotModified, unchanged.Code, path)
		require.Equal(t, kept, unchanged.Header().Get("Cache-Control"), path)
		head := request(http.MethodHead, path, "")
		require.Equal(t, http.StatusOK, head.Code, path)
		require.Equal(t, kept, head.Header().Get("Cache-Control"), path)
		require.Empty(t, head.Body.String(), path)

		// The same file under an address that does not name its bytes: without
		// a version, with the version of another release, with no value, and
		// with the version under another name.
		name, version, _ := strings.Cut(path, "?v=")
		for _, other := range []string{name, name + "?v=" + strings.Repeat("0", 64), name + "?v=", name + "?version=" + version} {
			asked := request(http.MethodGet, other, "")
			require.Equal(t, http.StatusOK, asked.Code, other)
			require.Equal(t, asked.Body.String(), get.Body.String(), other)
			require.Equal(t, revalidated, asked.Header().Get("Cache-Control"), other)
		}
		// The hash of the file names its bytes as the version of the editor does.
		own := request(http.MethodGet, name+"?v="+strings.Trim(etag, `"`), "")
		require.Equal(t, kept, own.Header().Get("Cache-Control"), path)
		missing := request(http.MethodGet, strings.Replace(path, "?", ".missing?", 1), "")
		require.Equal(t, http.StatusNotFound, missing.Code, path)
		require.Equal(t, "no-store", missing.Header().Get("Cache-Control"), path)
	}

	// A bundled icon and the icon of the site carry the hash of their file.
	require.NotEmpty(t, mdi.GetIconByName("server"))
	home := request(http.MethodGet, "/", "")
	require.Equal(t, http.StatusOK, home.Code)
	icons := regexp.MustCompile(`/assets/mdi/[a-z0-9]+\.svg\?v=[a-f0-9]{64}`).FindAllString(home.Body.String(), -1)
	require.NotEmpty(t, icons)
	for _, path := range append(icons[:1:1], assets.FaviconURL()) {
		get := request(http.MethodGet, path, "")
		require.Equal(t, http.StatusOK, get.Code, path)
		require.Equal(t, kept, get.Header().Get("Cache-Control"), path)
		_, version, _ := strings.Cut(path, "?v=")
		require.Equal(t, `"`+version+`"`, get.Header().Get("ETag"), path)
		name, _, _ := strings.Cut(path, "?")
		for _, other := range []string{name, name + "?v=" + strings.Repeat("0", 64)} {
			asked := request(http.MethodGet, other, "")
			require.Equal(t, http.StatusOK, asked.Code, other)
			require.Equal(t, revalidated, asked.Header().Get("Cache-Control"), other)
		}
	}
	// A version on a page changes nothing: the page is not a public file.
	_, editorVersion, _ := strings.Cut(paths[0], "?v=")
	page = request(http.MethodGet, "/settings/content?v="+editorVersion, "")
	require.Equal(t, http.StatusOK, page.Code)
	require.NotContains(t, page.Header().Get("Cache-Control"), "immutable")
	require.NotContains(t, page.Header().Get("Cache-Control"), "public")
	// Batch assets are generated by the normal build hook. Check those resources
	// when available; plain Go-only source checkouts retain a placeholder file.
	for _, path := range []string{"/assets/editor/handsontable.full.min.js", "/assets/editor/handsontable.full.min.css", "/assets/editor/zh-CN.min.js"} {
		get := request(http.MethodGet, path, "")
		if get.Code == http.StatusNotFound {
			t.Logf("generated asset absent: %s (run go run build/build.go)", path)
			continue
		}
		require.Equal(t, http.StatusOK, get.Code, path)
		require.Equal(t, revalidated, get.Header().Get("Cache-Control"), path)
		require.NotEmpty(t, get.Header().Get("ETag"), path)
		require.Equal(t, http.StatusNotModified, request(http.MethodGet, path, get.Header().Get("ETag")).Code, path)
	}
	for _, path := range []string{"/favicon.ico", "/assets/mdi/server.svg"} {
		if path != "/favicon.ico" {
			require.NotEmpty(t, mdi.GetIconByName("server"))
		}
		get := request(http.MethodGet, path, "")
		require.Equal(t, http.StatusOK, get.Code, path)
		require.Equal(t, "public, max-age=0, must-revalidate", get.Header().Get("Cache-Control"), path)
		require.NotEmpty(t, get.Header().Get("ETag"), path)
	}
	preview := request(http.MethodGet, "/settings/content/icon?name=server", "")
	require.Equal(t, http.StatusOK, preview.Code)
	require.Equal(t, "no-store", preview.Header().Get("Cache-Control"), "theme-dependent previews must not survive a theme change")
}
