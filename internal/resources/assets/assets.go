package assets

import (
	"bytes"
	"crypto/sha256"
	"embed"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gofiber/fiber/v3/middleware/static"

	"github.com/soulteary/flare/config/define"
)

//go:embed favicon.ico
var Favicon embed.FS

var faviconVersionOnce sync.Once
var faviconVersionURL string

// FaviconURL bypasses older releases' long-lived unversioned browser entries.
func FaviconURL() string {
	faviconVersionOnce.Do(func() {
		data, err := Favicon.ReadFile("favicon.ico")
		if err != nil {
			faviconVersionURL = "/favicon.ico"
			return
		}
		faviconVersionURL = fmt.Sprintf("/favicon.ico?v=%x", sha256.Sum256(data))
	})
	return faviconVersionURL
}

func RegisterRouting(e *fiber.App) {
	e.Use(optimizeResourceCacheTime())
	registerThemeFavicons(e)
	handler := StaticFS(Favicon, "/")
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
	// This handler serves public resource bytes, so override the authentication
	// middleware's default. Do not relax policy merely from an /assets/ prefix.
	c.Set("Cache-Control", "public, max-age=0, must-revalidate")
	return serveReader(c, name, contentType, fmt.Sprintf(`"%x"`, sha256.Sum256(data)), bytes.NewReader(data))
}

func serveReader(c fiber.Ctx, name, contentType, etag string, reader io.ReadSeeker) error {
	return adaptor.HTTPHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", etag)
		w.Header().Set("Content-Type", contentType)
		http.ServeContent(w, r, name, time.Time{}, reader)
	})(c)
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
			if versionNamesContent(c, path) {
				// The address names these bytes: other bytes get another address.
				c.Set("Cache-Control", versionedPolicy)
			} else if len(c.Response().Header.Peek("Cache-Control")) == 0 {
				// An address without a version is a name, not a content hash. A
				// browser must revalidate it before reuse when a deployment
				// changes its bytes.
				c.Set("Cache-Control", "public, max-age=0, must-revalidate")
			}
		}
		return err
	}
}

const versionedPolicy = "public, max-age=31536000, immutable"

var (
	bundleVersionsMu sync.RWMutex
	bundleVersions   = map[string]string{}
)

// TrustVersion says that an address below prefix with ?v=version names its
// bytes, although the version is not the hash of that one file: it is a hash
// over a set of files that are published together, the file among them.
func TrustVersion(prefix, version string) {
	if prefix == "" || version == "" {
		return
	}
	bundleVersionsMu.Lock()
	bundleVersions[prefix] = version
	bundleVersionsMu.Unlock()
}

// versionNamesContent reports whether the version in the address of a public
// file stands for the bytes that were served: it is the hash in the validator
// of the response, or the version of the set the file belongs to. Any other
// version, such as one an older page still asks for, names nothing and the
// file is revalidated as one without a version.
func versionNamesContent(c fiber.Ctx, path string) bool {
	version := c.Query("v")
	if version == "" || !strings.HasPrefix(string(c.Response().Header.Peek("Cache-Control")), "public") {
		return false
	}
	if tag := string(c.Response().Header.Peek("ETag")); tag == `"`+version+`"` {
		return true
	}
	bundleVersionsMu.RLock()
	defer bundleVersionsMu.RUnlock()
	for prefix, trusted := range bundleVersions {
		if trusted == version && strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
