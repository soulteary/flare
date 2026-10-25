package editor

import (
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/resources/mdi"
)

const iconPreviewPath = "/editor/icon"

// registerEditorIcon keeps on-demand previews behind the editor's authentication.
func registerEditorIcon(e *fiber.App) {
	e.Get(iconPreviewPath, auth.AuthRequired, getEditorIcon)
}

// getEditorIcon resolves bundled names directly, including icons that have never
// appeared on the homepage. Theme-dependent responses must not outlive a theme change.
func getEditorIcon(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	svg := mdi.GetIconSVGByName(c.Query("name"))
	if svg == "" {
		return fiber.ErrNotFound
	}
	c.Set("Content-Type", "image/svg+xml")
	return c.Status(http.StatusOK).SendString(svg)
}
