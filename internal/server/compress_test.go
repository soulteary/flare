package server

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

func TestResponsesAreCompressedUnlessTheyCanHoldASecret(t *testing.T) {
	original := define.AppFlags
	t.Cleanup(func() { define.AppFlags = original })
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	require.NoError(t, os.WriteFile("config.yml", []byte("ShowApps: true\nShowBookmarks: true\nShowWeather: false\nTheme: blackboard\n"), 0600))
	fixture := model.Bookmarks{}
	for range 40 {
		fixture.Items = append(fixture.Items, model.Bookmark{Name: "COMPRESS-FIXTURE", URL: "https://example.com/compress"})
	}
	require.True(t, data.SaveFavoriteBookmarks(fixture))
	require.True(t, data.SaveNormalBookmarks(fixture))
	t.Cleanup(func() {
		require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{}))
		require.True(t, data.SaveNormalBookmarks(model.Bookmarks{}))
	})
	store, err := security.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.Initialize("admin", "compress integration password", "local"))
	flags := &model.Flags{Port: 5005, CookieName: "compress-test", EnableOfflineMode: true, EnableEditor: true}
	app, err := NewFiberApp(flags, store)
	require.NoError(t, err)
	session, err := store.Authenticate("admin", "compress integration password")
	require.NoError(t, err)
	cookie := &http.Cookie{Name: auth.RequestHandleSessionName(flags.CookieName, flags.Port) + "_" + store.Info().InstanceID, Value: session}

	request := func(method, path string, signedIn bool, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://flare.test"+path, nil)
		if signedIn {
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
	accept := func(coding string) map[string]string { return map[string]string{"Accept-Encoding": coding} }
	unpack := func(body *bytes.Buffer) string {
		reader, err := gzip.NewReader(body)
		require.NoError(t, err)
		plain, err := io.ReadAll(reader)
		require.NoError(t, err)
		return string(plain)
	}

	t.Run("a page for a visitor without a session", func(t *testing.T) {
		plain := request(http.MethodGet, "/bookmarks", false, nil)
		require.Equal(t, http.StatusOK, plain.Code)
		require.Empty(t, plain.Header().Get("Content-Encoding"))
		require.Contains(t, plain.Header().Values("Vary"), "Accept-Encoding")
		require.Contains(t, plain.Body.String(), "COMPRESS-FIXTURE")
		require.NotContains(t, plain.Body.String(), "csrf-token", "the fixture page must carry no token")

		packed := request(http.MethodGet, "/bookmarks", false, accept("gzip"))
		require.Equal(t, http.StatusOK, packed.Code)
		require.Equal(t, "gzip", packed.Header().Get("Content-Encoding"))
		require.Equal(t, "private, no-store", packed.Header().Get("Cache-Control"))
		require.Contains(t, packed.Header().Values("Vary"), "Accept-Encoding")
		// The home page varies by the browser; that line stays as the route wrote it.
		home := request(http.MethodGet, "/", false, accept("gzip"))
		require.ElementsMatch(t, []string{"User-Agent", "Accept-Encoding"}, home.Header().Values("Vary"))
		require.Less(t, packed.Body.Len(), plain.Body.Len()/2)
		require.Equal(t, plain.Body.String(), unpack(packed.Body))

		// By default gzip and deflate are sent, whatever else the client accepts.
		for accepted, coding := range map[string]string{
			"deflate":                    "deflate",
			"gzip, deflate, br, zstd":    "gzip",
			"br;q=1.0, gzip;q=0.5":       "gzip",
			"*":                          "gzip",
			"br, *;q=0.3":                "gzip",
			"deflate, gzip;q=0.2, *":     "deflate",
			"br":                         "",
			"zstd, br":                   "",
			"gzip;q=0, br":               "",
			"gzip;q=0.0, deflate;q=0, *": "",
			"*;q=0":                      "",
			"identity":                   "",
		} {
			other := request(http.MethodGet, "/bookmarks", false, accept(accepted))
			require.Equal(t, http.StatusOK, other.Code, accepted)
			require.Equal(t, coding, other.Header().Get("Content-Encoding"), accepted)
			if coding == "" {
				require.Equal(t, plain.Body.String(), other.Body.String(), accepted)
			} else {
				require.Less(t, other.Body.Len(), plain.Body.Len()/2, accepted)
			}
			require.Contains(t, other.Header().Values("Vary"), "Accept-Encoding", accepted)
		}
		refused := request(http.MethodGet, "/bookmarks", false, accept("gzip;q=0, identity"))
		require.Empty(t, refused.Header().Get("Content-Encoding"))
		require.Equal(t, plain.Body.String(), refused.Body.String())
	})

	t.Run("a page that can hold a secret stays as it is", func(t *testing.T) {
		signedIn := request(http.MethodGet, "/bookmarks", true, accept("gzip, br"))
		require.Equal(t, http.StatusOK, signedIn.Code)
		require.Empty(t, signedIn.Header().Get("Content-Encoding"))
		require.Contains(t, signedIn.Body.String(), "COMPRESS-FIXTURE")

		forged := httptest.NewRequest(http.MethodGet, "http://flare.test/bookmarks", nil)
		forged.AddCookie(&http.Cookie{Name: cookie.Name, Value: "not-a-session"})
		forged.Header.Set("Accept-Encoding", "gzip")
		response, err := app.Test(forged)
		require.NoError(t, err)
		require.Empty(t, response.Header.Get("Content-Encoding"), "a session cookie is not judged here")
		require.NoError(t, response.Body.Close())

		login := request(http.MethodGet, define.SettingPages.Others.Path, false, accept("gzip, br"))
		require.Equal(t, http.StatusOK, login.Code)
		require.Empty(t, login.Header().Get("Content-Encoding"))

		editor := request(http.MethodGet, "/settings/content", true, accept("gzip, br"))
		require.Equal(t, http.StatusOK, editor.Code)
		require.Contains(t, editor.Body.String(), "csrf")
		require.Empty(t, editor.Header().Get("Content-Encoding"))
		require.Greater(t, editor.Body.Len(), compressMinimum)
	})

	t.Run("a public file whoever asks", func(t *testing.T) {
		for _, signedIn := range []bool{false, true} {
			plain := request(http.MethodGet, "/assets/editor-ui/app.js", signedIn, nil)
			require.Equal(t, http.StatusOK, plain.Code)
			tag := plain.Header().Get("ETag")
			require.Regexp(t, `^"[0-9a-f]{64}"$`, tag)

			packed := request(http.MethodGet, "/assets/editor-ui/app.js", signedIn, accept("gzip"))
			require.Equal(t, "gzip", packed.Header().Get("Content-Encoding"))
			require.Equal(t, "public, max-age=0, must-revalidate", packed.Header().Get("Cache-Control"))
			require.Equal(t, "W/"+tag, packed.Header().Get("ETag"))
			require.Equal(t, plain.Body.String(), unpack(packed.Body))

			for _, validator := range []string{tag, "W/" + tag} {
				unchanged := request(http.MethodGet, "/assets/editor-ui/app.js", signedIn, map[string]string{"Accept-Encoding": "gzip", "If-None-Match": validator})
				require.Equal(t, http.StatusNotModified, unchanged.Code, validator)
				require.Empty(t, unchanged.Header().Get("Content-Encoding"), validator)
				require.Empty(t, unchanged.Body.String(), validator)
			}
			partial := request(http.MethodGet, "/assets/editor-ui/app.js", signedIn, map[string]string{"Accept-Encoding": "gzip", "Range": "bytes=0-3"})
			require.Equal(t, http.StatusPartialContent, partial.Code)
			require.Empty(t, partial.Header().Get("Content-Encoding"))
			require.Equal(t, plain.Body.String()[:4], partial.Body.String())
			require.Equal(t, tag, partial.Header().Get("ETag"))

			head := request(http.MethodHead, "/assets/editor-ui/app.js", signedIn, accept("gzip"))
			require.Equal(t, http.StatusOK, head.Code)
			require.Empty(t, head.Header().Get("Content-Encoding"))
		}
	})

	t.Run("a small or packed file stays as it is", func(t *testing.T) {
		icon := request(http.MethodGet, "/favicon.ico", false, accept("gzip, br"))
		require.Equal(t, http.StatusOK, icon.Code)
		require.Greater(t, icon.Body.Len(), compressMinimum)
		require.Empty(t, icon.Header().Get("Content-Encoding"))
		require.Regexp(t, `^"[0-9a-f]{64}"$`, icon.Header().Get("ETag"))

		small := request(http.MethodGet, "/assets/editor-ui/i18n.js", false, accept("gzip, br"))
		require.Equal(t, http.StatusOK, small.Code)
		require.Less(t, small.Body.Len(), compressMinimum)
		require.Empty(t, small.Header().Get("Content-Encoding"))
		require.Regexp(t, `^"[0-9a-f]{64}"$`, small.Header().Get("ETag"))

		missing := request(http.MethodGet, "/assets/editor-ui/missing.js", false, accept("gzip, br"))
		require.Equal(t, http.StatusNotFound, missing.Code)
		require.Empty(t, missing.Header().Get("Content-Encoding"))
	})

	// Runs last: it builds a second application, which replaces the flags.
	t.Run("a page with a token for a visitor without a session", func(t *testing.T) {
		open, err := security.Open(t.TempDir())
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, open.Close()) })
		require.NoError(t, open.Initialize("admin", "compress integration password", "local"))
		trusted, err := NewFiberApp(&model.Flags{Port: 5005, CookieName: "compress-open", DisableLoginMode: true, EnableOfflineMode: true, EnableEditor: true}, open)
		require.NoError(t, err)
		get := func(path, coding string) (*http.Response, []byte) {
			req := httptest.NewRequest(http.MethodGet, "http://flare.test"+path, nil)
			if coding != "" {
				req.Header.Set("Accept-Encoding", coding)
			}
			response, err := trusted.Test(req)
			require.NoError(t, err)
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			require.NoError(t, err)
			return response, body
		}
		response, body := get("/settings/content", "gzip, br")
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Empty(t, response.Header.Get("Content-Encoding"))
		require.Greater(t, len(body), compressMinimum)
		require.Regexp(t, `name="?csrf-token"? content="?[\w-]{20,}`, string(body))

		response, _ = get("/help", "gzip")
		require.Equal(t, http.StatusOK, response.StatusCode)
		require.Equal(t, "gzip", response.Header.Get("Content-Encoding"), "a page without a token")
	})
}

func TestTheOptionSelectsTheCodings(t *testing.T) {
	original := define.AppFlags
	t.Cleanup(func() { define.AppFlags = original })
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	get := func(app interface {
		Test(*http.Request, ...fiber.TestConfig) (*http.Response, error)
	}, coding string) (*http.Response, int) {
		req := httptest.NewRequest(http.MethodGet, "http://flare.test/help", nil)
		req.Header.Set("Accept-Encoding", coding)
		response, err := app.Test(req)
		require.NoError(t, err)
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.Equal(t, http.StatusOK, response.StatusCode)
		return response, len(body)
	}
	build := func(mode string) *fiber.App {
		app, err := NewFiberApp(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true, Compression: mode})
		require.NoError(t, err)
		return app
	}
	_, plain := get(build("off"), "identity")
	require.Greater(t, plain, compressMinimum)

	for _, mode := range []string{"", "gzip", " GZip ", "unknown"} {
		app := build(mode)
		for accepted, coding := range map[string]string{"gzip, br, zstd": "gzip", "br, zstd": "", "deflate, br": "deflate"} {
			response, size := get(app, accepted)
			require.Equal(t, coding, response.Header.Get("Content-Encoding"), mode+": "+accepted)
			require.Equal(t, coding == "", size == plain, mode+": "+accepted)
		}
	}
	all := build("all")
	for accepted, coding := range map[string]string{"gzip, br, zstd": "br", "zstd, gzip": "zstd", "zstd": "zstd", "br": "br", "gzip": "gzip", "deflate": "deflate", "*": "br"} {
		response, size := get(all, accepted)
		require.Equal(t, coding, response.Header.Get("Content-Encoding"), accepted)
		require.Less(t, size, plain/2, accepted)
	}
	off := build("off")
	for _, accepted := range []string{"gzip", "gzip, br, zstd", "*"} {
		response, size := get(off, accepted)
		require.Empty(t, response.Header.Get("Content-Encoding"), accepted)
		require.Equal(t, plain, size, accepted)
		require.NotContains(t, response.Header.Values("Vary"), "Accept-Encoding", accepted)
	}
}
