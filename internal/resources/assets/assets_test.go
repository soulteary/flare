package assets

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
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
		{name: "returned error", status: http.StatusNotFound, err: echo.ErrNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := echo.New()
			e.Use(optimizeResourceCacheTime())
			called := false
			e.GET("/assets/fixture.svg", func(c *echo.Context) error {
				called = true
				c.Response().Header().Set("ETag", `"fixture"`)
				c.Response().Header().Set("Last-Modified", "Mon, 01 Jan 2024 00:00:00 GMT")
				if tt.err != nil {
					return tt.err
				}
				if tt.implicit {
					_, err := c.Response().Write([]byte("fixture"))
					return err
				}
				return c.NoContent(tt.status)
			})
			request := httptest.NewRequest(http.MethodGet, "/assets/fixture.svg", nil)
			request.Header.Set("If-None-Match", "*")
			response := httptest.NewRecorder()
			e.ServeHTTP(response, request)
			require.True(t, called, "cache middleware must let the handler check the resource")
			require.Equal(t, tt.status, response.Code)
			if tt.cacheable {
				assert.Equal(t, "public, max-age=31536000", response.Header().Get("Cache-Control"))
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
		e := echo.New()
		e.Use(optimizeResourceCacheTime())
		e.GET(path, func(c *echo.Context) error {
			c.Response().Header().Set("Cache-Control", "private, max-age=60")
			return c.String(http.StatusOK, "fixture")
		})
		response := httptest.NewRecorder()
		e.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		assert.Equal(t, http.StatusOK, response.Code)
		assert.Equal(t, "private, max-age=60", response.Header().Get("Cache-Control"))
	}
}

// TestFaviconConditionalRequests ensures the favicon has its own stable
// validator and that this validator cannot turn a missing asset into a 304.
func TestFaviconConditionalRequests(t *testing.T) {
	e := echo.New()
	RegisterRouting(e)
	get := httptest.NewRecorder()
	e.ServeHTTP(get, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	require.Equal(t, http.StatusOK, get.Code)
	require.NotEmpty(t, get.Body.Bytes())
	assert.Equal(t, "image/x-icon", get.Header().Get("Content-Type"))
	etag := get.Header().Get("ETag")
	require.NotEmpty(t, etag)

	for _, method := range []string{http.MethodGet, http.MethodHead} {
		request := httptest.NewRequest(method, "/favicon.ico", nil)
		request.Header.Set("If-None-Match", etag)
		response := httptest.NewRecorder()
		e.ServeHTTP(response, request)
		assert.Equal(t, http.StatusNotModified, response.Code)
		assert.Empty(t, response.Body.String())
		assert.Equal(t, etag, response.Header().Get("ETag"))
	}

	head := httptest.NewRecorder()
	e.ServeHTTP(head, httptest.NewRequest(http.MethodHead, "/favicon.ico", nil))
	assert.Equal(t, http.StatusOK, head.Code)
	assert.Empty(t, head.Body.String())
	assert.Equal(t, get.Header().Get("Content-Length"), head.Header().Get("Content-Length"))

	otherInstance := echo.New()
	RegisterRouting(otherInstance)
	second := httptest.NewRecorder()
	otherInstance.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	assert.Equal(t, etag, second.Header().Get("ETag"), "identical content must keep its validator across instances")

	for _, validator := range []string{etag, "*"} {
		request := httptest.NewRequest(http.MethodGet, "/assets/missing.svg", nil)
		request.Header.Set("If-None-Match", validator)
		response := httptest.NewRecorder()
		e.ServeHTTP(response, request)
		assert.Equal(t, http.StatusNotFound, response.Code)
		assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
		assert.Empty(t, response.Header().Get("ETag"))
	}

	invalidRange := httptest.NewRequest(http.MethodGet, "/favicon.ico", nil)
	invalidRange.Header.Set("Range", "bytes=999999999-")
	response := httptest.NewRecorder()
	e.ServeHTTP(response, invalidRange)
	assert.Equal(t, http.StatusRequestedRangeNotSatisfiable, response.Code)
	assert.Equal(t, "no-store", response.Header().Get("Cache-Control"))
	assert.Empty(t, response.Header().Get("ETag"))
	assert.False(t, strings.Contains(response.Header().Get("Cache-Control"), "max-age"))
}
