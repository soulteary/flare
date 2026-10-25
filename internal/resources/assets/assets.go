package assets

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/soulteary/flare/config/define"
)

//go:embed favicon.ico
var Favicon embed.FS

func RegisterRouting(e *fiber.App) {
	e.Use(optimizeResourceCacheTime())
	handler := func(c fiber.Ctx) error {
		data, err := fs.ReadFile(Favicon, "favicon.ico")
		if err != nil {
			return err
		}
		return ServeContent(c, "favicon.ico", "image/x-icon", data)
	}
	e.Get("/favicon.ico", handler)
	e.Head("/favicon.ico", handler)
	if define.AppFlags.DebugMode {
		e.Use("/assets/css", static.New("embed/assets/css", static.Config{ByteRange: true}))
	}
}

// ServeContent retains net/http's conditional and range semantics for embedded
// files. Only static assets use this adapter; application routes use Fiber
// directly, including routing, middleware, and template rendering.
func ServeContent(c fiber.Ctx, name, contentType string, data []byte) error {
	return adaptor.HTTPHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", fmt.Sprintf(`"%x"`, sha256.Sum256(data)))
		w.Header().Set("Content-Type", contentType)
		http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(data))
	})(c)
}

// StaticFS serves embedded files with content-derived validators. Embedded files
// have no meaningful modification time, so timestamp/size validators can mistake
// a changed, same-size script for an unchanged resource after an upgrade.
func StaticFS(files fs.FS, prefix string) fiber.Handler {
	return func(c fiber.Ctx) error {
		if c.Method() != http.MethodGet && c.Method() != http.MethodHead {
			return c.Next()
		}
		name := strings.TrimPrefix(strings.TrimPrefix(c.Path(), prefix), "/")
		if !fs.ValidPath(name) || strings.Contains(name, "\\") {
			return c.Next()
		}
		info, err := fs.Stat(files, name)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return c.Next()
			}
			return err
		}
		if !info.Mode().IsRegular() {
			return c.Next()
		}
		data, err := fs.ReadFile(files, name)
		if err != nil {
			return err
		}
		contentType := mime.TypeByExtension(path.Ext(name))
		if contentType == "" {
			contentType = http.DetectContentType(data)
		}
		return ServeContent(c, path.Base(name), contentType, data)
	}
}

// optimizeResourceCacheTime applies policy after Fiber has buffered the response,
// so an absent file or invalid range never inherits a successful asset's ETag.
func optimizeResourceCacheTime() fiber.Handler {
	return func(c fiber.Ctx) error {
		path := c.Path()
		if !strings.HasPrefix(path, "/assets/") && path != "/favicon.ico" {
			return c.Next()
		}
		err := c.Next()
		status := c.Response().StatusCode()
		switch {
		case err != nil || status >= http.StatusBadRequest:
			c.Set("Cache-Control", "no-store")
			c.Response().Header.Del("ETag")
			c.Response().Header.Del("Last-Modified")
		case status >= http.StatusOK && status < http.StatusMultipleChoices || status == http.StatusNotModified:
			if len(c.Response().Header.Peek("Cache-Control")) == 0 {
				// Current asset URLs are names, not content hashes. A browser must
				// revalidate them before reuse when a deployment changes their bytes.
				c.Set("Cache-Control", "public, max-age=0, must-revalidate")
			}
		}
		return err
	}
}
