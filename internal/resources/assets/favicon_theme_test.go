package assets

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/stretchr/testify/require"
)

func TestThemeFavicons(t *testing.T) {
	previous, _ := define.ThemeSnapshot()
	t.Cleanup(func() { define.SetTheme(previous) })
	app := fiber.New()
	RegisterRouting(app)
	seen := map[string]bool{}
	for _, theme := range define.ThemePalettes {
		define.SetTheme(theme.Name)
		url := ThemeFaviconURL()
		require.False(t, seen[url])
		seen[url] = true
		icon := themeFavicons[theme.Name]
		data, err := themedFaviconFiles.ReadFile(icon.name)
		require.NoError(t, err)
		require.Equal(t, fmt.Sprintf(`"%x"`, sha256.Sum256(data)), icon.etag)
		response := httptest.NewRecorder()
		serveHTTP(app, response, httptest.NewRequest(http.MethodGet, url, nil))
		require.Equal(t, http.StatusOK, response.Code)
		require.Equal(t, data, response.Body.Bytes())
		require.Equal(t, "image/x-icon", response.Header().Get("Content-Type"))
		require.Equal(t, "public, max-age=31536000, immutable", response.Header().Get("Cache-Control"))
		require.Equal(t, icon.etag, response.Header().Get("ETag"))
		req := httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("If-None-Match", icon.etag)
		cached := httptest.NewRecorder()
		serveHTTP(app, cached, req)
		require.Equal(t, http.StatusNotModified, cached.Code)
		require.Empty(t, cached.Body.Bytes())
		head := httptest.NewRecorder()
		serveHTTP(app, head, httptest.NewRequest(http.MethodHead, url, nil))
		require.Equal(t, http.StatusOK, head.Code)
		require.Empty(t, head.Body.Bytes())
		req = httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Range", "bytes=0-5")
		partial := httptest.NewRecorder()
		serveHTTP(app, partial, req)
		require.Equal(t, http.StatusPartialContent, partial.Code)
		require.Equal(t, data[:6], partial.Body.Bytes())
		req = httptest.NewRequest(http.MethodGet, url, nil)
		req.Header.Set("Range", "bytes=999999-")
		invalid := httptest.NewRecorder()
		serveHTTP(app, invalid, req)
		require.Equal(t, http.StatusRequestedRangeNotSatisfiable, invalid.Code)
		require.Equal(t, "no-store", invalid.Header().Get("Cache-Control"))
	}
	define.SetTheme("unknown")
	require.Equal(t, themeFavicons[define.ThemePalettes[0].Name].url, ThemeFaviconURL())
	missing := httptest.NewRecorder()
	serveHTTP(app, missing, httptest.NewRequest(http.MethodGet, "/assets/favicon/missing.ico", nil))
	require.Equal(t, http.StatusNotFound, missing.Code)
	require.Equal(t, "no-store", missing.Header().Get("Cache-Control"))
}

func BenchmarkThemeFaviconURL(b *testing.B) {
	b.ReportAllocs()
	for b.Loop() {
		_ = ThemeFaviconURL()
	}
}
