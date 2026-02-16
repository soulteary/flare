package assets

import (
	"crypto/md5" //#nosec
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

	e.GET("/favicon.ico", func(c *echo.Context) error {
		c.Response().Header().Set("Cache-Control", "public, max-age=31536000")
		data, err := fs.ReadFile(Favicon, "favicon.ico")
		if err != nil {
			return err
		}
		return c.Blob(http.StatusOK, "image/x-icon", data)
	})

	if define.AppFlags.DebugMode {
		e.Static("/assets/css", "embed/assets/css")
	}
}

// optimizeResourceCacheTime sets cache headers for assets and supports 304.
func optimizeResourceCacheTime() echo.MiddlewareFunc {
	data := []byte(time.Now().String())
	/* #nosec */
	etag := fmt.Sprintf("W/%x", md5.Sum(data))
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			uri := c.Request().RequestURI
			if strings.HasPrefix(uri, "/assets/") || strings.HasPrefix(uri, "/favicon.ico") {
				c.Response().Header().Set("Cache-Control", "public, max-age=31536000")
				c.Response().Header().Set("ETag", etag)
				if match := c.Request().Header.Get("If-None-Match"); match != "" && strings.Contains(match, etag) {
					return c.NoContent(http.StatusNotModified)
				}
			}
			return next(c)
		}
	}
}
