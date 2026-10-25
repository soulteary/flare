package assets

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedResourceContentTypesAndHTTPValidators(t *testing.T) {
	tests := []struct{ name, body, contentType string }{
		{"favicon.ico", "icon fixture", "image/x-icon"},
		{"FAVICON.ICO", "icon fixture", "image/x-icon"},
		{"app.js", "const answer = 42;", "text/javascript; charset=utf-8"},
		{"APP.JS", "const answer = 42;", "text/javascript; charset=utf-8"},
		{"module.mjs", "export const answer = 42;", "text/javascript; charset=utf-8"},
		{"style.css", "body { color: blue; }", "text/css; charset=utf-8"},
		{"index.html", "<!doctype html><html></html>", "text/html; charset=utf-8"},
		{"icon.svg", "<svg xmlns=\"http://www.w3.org/2000/svg\"></svg>", "image/svg+xml"},
		{"font.woff2", "wOF2font fixture", "font/woff2"},
		{"font.woff", "wOFFfont fixture", "font/woff"},
		{"messages.json", "{\"answer\":42}", "application/json"},
		{"app.js.map", "{\"version\":3}", "application/json"},
		{"notes.txt", "plain text fixture", "text/plain; charset=utf-8"},
		{"unknown.flare-fixture", "<!doctype html><html></html>", "text/html; charset=utf-8"},
		{"no-extension", "ordinary text fixture", "text/plain; charset=utf-8"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{CaseSensitive: true})
			app.Use(optimizeResourceCacheTime())
			app.Use("/assets/fixture", StaticFS(fstest.MapFS{test.name: {Data: []byte(test.body)}}, "/assets/fixture"))
			request := func(method string, headers map[string]string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(method, "/assets/fixture/"+test.name, nil)
				for k, v := range headers {
					req.Header.Set(k, v)
				}
				rec := httptest.NewRecorder()
				serveHTTP(app, rec, req)
				return rec
			}
			get := request(http.MethodGet, nil)
			require.Equal(t, http.StatusOK, get.Code)
			require.Equal(t, test.contentType, get.Header().Get("Content-Type"))
			require.Equal(t, test.body, get.Body.String())
			etag := get.Header().Get("ETag")
			require.NotEmpty(t, etag)
			head := request(http.MethodHead, nil)
			require.Equal(t, http.StatusOK, head.Code)
			require.Equal(t, test.contentType, head.Header().Get("Content-Type"))
			require.Equal(t, fmt.Sprint(len(test.body)), head.Header().Get("Content-Length"))
			require.Empty(t, head.Body.String())
			conditional := request(http.MethodGet, map[string]string{"If-None-Match": etag})
			require.Equal(t, http.StatusNotModified, conditional.Code)
			require.Empty(t, conditional.Body.String())
			require.Equal(t, etag, conditional.Header().Get("ETag"))
			partial := request(http.MethodGet, map[string]string{"Range": "bytes=0-2", "If-Range": etag})
			require.Equal(t, http.StatusPartialContent, partial.Code)
			require.Equal(t, test.body[:3], partial.Body.String())
			require.Equal(t, fmt.Sprintf("bytes 0-2/%d", len(test.body)), partial.Header().Get("Content-Range"))
			full := request(http.MethodGet, map[string]string{"Range": "bytes=0-2", "If-Range": "\"another-resource\""})
			require.Equal(t, http.StatusOK, full.Code)
			require.Equal(t, test.body, full.Body.String())
		})
	}
}
