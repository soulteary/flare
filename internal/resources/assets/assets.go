//go:build !fiber

package assets

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v5"

	"github.com/soulteary/flare/config/define"
)

//go:embed favicon.ico
var Favicon embed.FS

func RegisterRouting(e *echo.Echo) {
	e.Use(optimizeResourceCacheTime())

	e.Match([]string{http.MethodGet, http.MethodHead}, "/favicon.ico", func(c *echo.Context) error {
		data, err := fs.ReadFile(Favicon, "favicon.ico")
		if err != nil {
			return err
		}
		c.Response().Header().Set("ETag", fmt.Sprintf(`"%x"`, sha256.Sum256(data)))
		c.Response().Header().Set("Content-Type", "image/x-icon")
		http.ServeContent(c.Response(), c.Request(), "favicon.ico", time.Time{}, bytes.NewReader(data))
		return nil
	})

	if define.AppFlags.DebugMode {
		e.Static("/assets/css", "embed/assets/css")
	}
}

// optimizeResourceCacheTime caches successful asset responses without deciding
// whether a resource exists. Validators and conditional requests belong to the
// resource handler; a process-wide ETag can incorrectly turn missing files into 304s.
func optimizeResourceCacheTime() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			path := c.Request().URL.Path
			if !strings.HasPrefix(path, "/assets/") && path != "/favicon.ico" {
				return next(c)
			}
			original := c.Response()
			response := &resourceCacheWriter{ResponseWriter: original}
			c.SetResponse(response)
			defer c.SetResponse(original)
			err := next(c)
			if err != nil && !response.committed {
				// Echo writes returned errors after this middleware unwinds.
				disableResourceCache(response.Header())
			}
			return err
		}
	}
}

// resourceCacheWriter selects the cache policy when the final status is known,
// without buffering or copying response bodies.
type resourceCacheWriter struct {
	http.ResponseWriter
	committed bool
}

// WriteHeader keeps long-lived caching off errors and preserves a handler's
// explicit policy for successful responses.
func (w *resourceCacheWriter) WriteHeader(status int) {
	if !w.committed && status >= http.StatusOK {
		w.committed = true
		switch {
		case status < http.StatusMultipleChoices || status == http.StatusNotModified:
			if w.Header().Get("Cache-Control") == "" {
				w.Header().Set("Cache-Control", "public, max-age=31536000")
			}
		case status >= http.StatusBadRequest:
			disableResourceCache(w.Header())
		}
	}
	w.ResponseWriter.WriteHeader(status)
}

// Write also applies the policy when a handler implicitly sends HTTP 200.
func (w *resourceCacheWriter) Write(data []byte) (int, error) {
	if !w.committed {
		w.WriteHeader(http.StatusOK)
	}
	return w.ResponseWriter.Write(data)
}

// Unwrap preserves Echo status inspection and net/http response-controller access.
func (w *resourceCacheWriter) Unwrap() http.ResponseWriter {
	return w.ResponseWriter
}

// disableResourceCache prevents transient failures from being reused as assets.
func disableResourceCache(header http.Header) {
	header.Set("Cache-Control", "no-store")
	header.Del("ETag")
	header.Del("Last-Modified")
}
