package home

import (
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/auth"
)

// registerThemeStylesheet serves the stylesheet a theme brings to the pages
// that link it, and the images and fonts that stylesheet refers to. Those
// pages are for anyone, or for the administrator alone on an instance that is
// private, and so is what a theme brings to them.
func registerThemeStylesheet(e *fiber.App) {
	for _, served := range []struct {
		route   string
		handler fiber.Handler
	}{
		{define.ThemeStylesheetRoute, serveThemeStylesheet},
		{define.ThemeAssetRoute, serveThemeAsset},
	} {
		handlers := []any{served.handler}
		if define.AppFlags.Visibility == "PRIVATE" {
			handlers = []any{auth.AuthRequired, served.handler}
		}
		e.Get(served.route, handlers[0], handlers[1:]...)
		e.Head(served.route, handlers[0], handlers[1:]...)
	}
}

// keepThemeFile lets a browser keep what it got from an address of a theme
// for good. The address holds a hash of what the theme brings, so another
// state has another address.
func keepThemeFile(c fiber.Ctx) {
	visibility := "public"
	if define.AppFlags.Visibility == "PRIVATE" {
		visibility = "private"
	}
	c.Set("Cache-Control", visibility+", max-age=31536000, immutable")
	c.Set("X-Content-Type-Options", "nosniff")
}

// serveThemeStylesheet sends a stylesheet that was checked when the themes
// directory was read. The address holds a hash of what the theme brings, so
// the response can be kept for good: another state has another address, and
// an address of a state the theme no longer has is not found.
func serveThemeStylesheet(c fiber.Ctx) error {
	hash := c.Params("hash")
	stylesheet, known := define.ThemeStylesheet(c.Params("name"), hash)
	if !known {
		return c.Status(http.StatusNotFound).SendString("Not Found")
	}
	keepThemeFile(c)
	return adaptor.HTTPHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"`+hash+`"`)
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		http.ServeContent(w, r, "theme.css", time.Time{}, strings.NewReader(stylesheet))
	})(c)
}

// serveThemeAsset sends an image or a font of the assets of a theme. Only a
// file that was looked at when the themes directory was read is sent, under
// the type its name stands for, and only while it is the file that was looked
// at: the address holds a hash that covers its size and modification time.
func serveThemeAsset(c fiber.Ctx) error {
	hash := c.Params("hash")
	file, asset, known := define.OpenThemeAsset(c.Params("name"), hash, c.Params("file"))
	if !known {
		return c.Status(http.StatusNotFound).SendString("Not Found")
	}
	keepThemeFile(c)
	return adaptor.HTTPHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer file.Close()
		w.Header().Set("ETag", `"`+hash+`"`)
		w.Header().Set("Content-Type", asset.Type)
		http.ServeContent(w, r, asset.Name, time.Time{}, file)
	})(c)
}
