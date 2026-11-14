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
// that link it. Those pages are for anyone, or for the administrator alone on
// an instance that is private, and so is the stylesheet.
func registerThemeStylesheet(e *fiber.App) {
	handlers := []any{serveThemeStylesheet}
	if define.AppFlags.Visibility == "PRIVATE" {
		handlers = []any{auth.AuthRequired, serveThemeStylesheet}
	}
	e.Get(define.ThemeStylesheetRoute, handlers[0], handlers[1:]...)
	e.Head(define.ThemeStylesheetRoute, handlers[0], handlers[1:]...)
}

// serveThemeStylesheet sends a stylesheet that was checked when the themes
// directory was read. The address holds a hash of the content, so the
// response can be kept for good: other content has another address, and an
// address of content the theme no longer has is not found.
func serveThemeStylesheet(c fiber.Ctx) error {
	hash := c.Params("hash")
	stylesheet, known := define.ThemeStylesheet(c.Params("name"), hash)
	if !known {
		return c.Status(http.StatusNotFound).SendString("Not Found")
	}
	visibility := "public"
	if define.AppFlags.Visibility == "PRIVATE" {
		visibility = "private"
	}
	c.Set("Cache-Control", visibility+", max-age=31536000, immutable")
	c.Set("X-Content-Type-Options", "nosniff")
	return adaptor.HTTPHandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"`+hash+`"`)
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		http.ServeContent(w, r, "theme.css", time.Time{}, strings.NewReader(stylesheet))
	})(c)
}
