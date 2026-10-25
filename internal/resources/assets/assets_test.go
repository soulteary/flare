package assets

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestResourceCacheResponses checks headers on the final HTTP response rather
// than treating every request under /assets/ as a successfully served file.
func TestResourceCacheResponses(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		err       error
		cacheable bool
		implicit  bool
	}{
		{name: "success", status: http.StatusOK, cacheable: true},
		{name: "implicit success", status: http.StatusOK, cacheable: true, implicit: true},
		{name: "range", status: http.StatusPartialContent, cacheable: true},
		{name: "not modified", status: http.StatusNotModified, cacheable: true},
		{name: "redirect", status: http.StatusMovedPermanently},
		{name: "missing", status: http.StatusNotFound},
		{name: "invalid range", status: http.StatusRequestedRangeNotSatisfiable},
		{name: "server error", status: http.StatusInternalServerError},
		{name: "returned error", status: http.StatusNotFound, err: fiber.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := fiber.New(fiber.Config{CaseSensitive: true, StrictRouting: true})
			e.Use(optimizeResourceCacheTime())
			called := false
			e.Get("/assets/fixture.svg", func(c fiber.Ctx) error {
				called = true
				c.Set("ETag", `"fixture"`)
				c.Set("Last-Modified", "Mon, 01 Jan 2024 00:00:00 GMT")
				if tt.err != nil {
					return tt.err
				}
				if tt.implicit {
					_, err := c.Write([]byte("fixture"))
					return err
				}
				return c.SendStatus(tt.status)
			})
			request := httptest.NewRequest(http.MethodGet, "/assets/fixture.svg", nil)
			request.Header.Set("If-None-Match", "*")
			response := httptest.NewRecorder()
			serveHTTP(e, response, request)
			require.True(t, called, "cache middleware must let the handler check the resource")
			require.Equal(t, tt.status, response.Code)
			if tt.cacheable {
				assert.Equal(t, "public, max-age=0, must-revalidate", response.Header().Get("Cache-Control"))
				assert.Equal(t, `"fixture"`, response.Header().Get("ETag"))
			} else {
				assert.NotContains(t, response.Header().Get("Cache-Control"), "max-age")
				if tt.status >= http.StatusBadRequest {
					assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
					assert.Empty(t, response.Header().Get("ETag"))
					assert.Empty(t, response.Header().Get("Last-Modified"))
				}
			}
		})
	}
}

// TestResourceCachePreservesHandlerPolicy keeps an explicit resource policy and
// leaves normal application routes outside the static-resource cache policy.
func TestResourceCachePreservesHandlerPolicy(t *testing.T) {
	for _, path := range []string{"/assets/private.svg", "/settings"} {
		e := fiber.New(fiber.Config{CaseSensitive: true, StrictRouting: true})
		e.Use(optimizeResourceCacheTime())
		e.Get(path, func(c fiber.Ctx) error {
			c.Set("Cache-Control", "private, max-age=60")
			return c.SendString("fixture")
		})
		response := httptest.NewRecorder()
		serveHTTP(e, response, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "private, max-age=60", response.Header().Get("Cache-Control"))
	}
}

// TestFaviconConditionalRequests ensures the favicon has its own stable
// validator and that this validator cannot turn a missing asset into a 304.
func TestFaviconConditionalRequests(t *testing.T) {
	e := fiber.New(fiber.Config{CaseSensitive: true, StrictRouting: true})
	RegisterRouting(e)
	get := httptest.NewRecorder()
	serveHTTP(e, get, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	require.Equal(t, http.StatusOK, get.Code)
	require.NotEmpty(t, get.Body.Bytes())
	assert.Equal(t, "image/x-icon", get.Header().Get("Content-Type"))
	etag := get.Header().Get("ETag")
	require.NotEmpty(t, etag)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request := httptest.NewRequest(method, "/favicon.ico", nil)
		request.Header.Set("If-None-Match", etag)
		response := httptest.NewRecorder()
		serveHTTP(e, response, request)
		assert.Equal(t, http.StatusNotModified, response.Code)
		assert.Empty(t, response.Body.String())
		assert.Equal(t, etag, response.Header().Get("ETag"))
	}

	head := httptest.NewRecorder()
	serveHTTP(e, head, httptest.NewRequest(http.MethodHead, "/favicon.ico", nil))
	assert.Equal(t, http.StatusOK, head.Code)
	assert.Empty(t, head.Body.String())
	assert.Equal(t, get.Header().Get("Content-Length"), head.Header().Get("Content-Length"))

	otherInstance := fiber.New(fiber.Config{CaseSensitive: true, StrictRouting: true})
	RegisterRouting(otherInstance)
	second := httptest.NewRecorder()
	serveHTTP(otherInstance, second, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	assert.Equal(t, etag, second.Header().Get("ETag"), "identical content must keep its validator across instances")

	for _, validator := range []string{etag, "*"} {
		request := httptest.NewRequest(http.MethodGet, "/assets/missing.svg", nil)
		request.Header.Set("If-None-Match", validator)
		response := httptest.NewRecorder()
		serveHTTP(e, response, request)
		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
		assert.Empty(t, response.Header().Get("ETag"))
	}

	invalidRange := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	invalidRange.Header.Set("Range", "bytes=999999999-")
	response := httptest.NewRecorder()
	serveHTTP(e, response, invalidRange)
	assert.Equal(t, http.StatusRequestedRangeNotSatisfiable, response.Code)
	assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	assert.Empty(t, response.Header().Get("ETag"))
	assert.False(t, strings.Contains(response.Header().Get("Cache-Control"), "max-age"))
}

// serveHTTP bridges the test client only; production requests use fasthttp.
func serveHTTP(app *fiber.App, rec *httptest.ResponseRecorder, req *http.Request) {
	res, err := app.Test(req)
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()
	for key, values := range res.Header {
		rec.Header()[key] = values
	}
	rec.WriteHeader(res.StatusCode)
	if _, err := io.Copy(rec.Body, res.Body); err != nil {
		panic(err)
	}
}

// Same-size replacements with identical embedded timestamps must not reuse the
// old script. This models a browser revalidating the same URL after deployment.
func TestStaticFSUpgradeRevalidation(t *testing.T) {
	newApp := func(content string) *fiber.App {
		e := fiber.New()
		e.Use(optimizeResourceCacheTime())
		e.Use("/assets/editor-ui", StaticFS(fstest.MapFS{
			"app.js": &fstest.MapFile{Data: []byte(content)},
		}, "/assets/editor-ui"))
		return e
	}
	get := func(app *fiber.App, method, etag, byteRange string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "/assets/editor-ui/app.js", nil)
		if etag != "" {
			req.Header.Set("If-None-Match", etag)
		}
		if byteRange != "" {
			req.Header.Set("Range", byteRange)
		}
		rec := httptest.NewRecorder()
		serveHTTP(app, rec, req)
		return rec
	}
	before, after := newApp("old-script"), newApp("new-script")
	original := get(before, http.MethodGet, "", "")
	require.Equal(t, http.StatusOK, original.Code)
	require.Equal(t, "public, max-age=0, must-revalidate", original.Header().Get("Cache-Control"))
	require.NotEmpty(t, original.Header().Get("ETag"))
	cached := get(before, http.MethodGet, original.Header().Get("ETag"), "")
	require.Equal(t, http.StatusNotModified, cached.Code)
	require.Empty(t, cached.Body.String())
	upgraded := get(after, http.MethodGet, original.Header().Get("ETag"), "")
	require.Equal(t, http.StatusOK, upgraded.Code)
	require.Equal(t, "new-script", upgraded.Body.String())
	require.NotEqual(t, original.Header().Get("ETag"), upgraded.Header().Get("ETag"))
	head := get(after, http.MethodHead, "", "")
	require.Equal(t, http.StatusOK, head.Code)
	require.Empty(t, head.Body.String())
	require.Equal(t, upgraded.Header().Get("ETag"), head.Header().Get("ETag"))
	partial := get(after, http.MethodGet, "", "bytes=0-2")
	require.Equal(t, http.StatusPartialContent, partial.Code)
	require.Equal(t, "new", partial.Body.String())
	require.Equal(t, "bytes 0-2/10", partial.Header().Get("Content-Range"))
	invalid := get(after, http.MethodGet, "", "bytes=100-")
	require.Equal(t, http.StatusRequestedRangeNotSatisfiable, invalid.Code)
	require.Equal(t, "no-store", invalid.Header().Get("Cache-Control"))
	require.Empty(t, invalid.Header().Get("ETag"))
}
