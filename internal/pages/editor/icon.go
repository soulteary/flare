package editor

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/resources/mdi"
)

const iconPreviewPath = "/editor/icon"

// registerEditorIcon keeps on-demand previews behind the editor's authentication.
func registerEditorIcon(e *fiber.App) {
	e.Get(iconPreviewPath, auth.AuthRequired, getEditorIcon)
	e.Get("/editor/icons", auth.AuthRequired, searchEditorIcons)
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

// searchEditorIcons bounds result size and uses only names from the bundled map.
func searchEditorIcons(c fiber.Ctx) error {
	c.Set("Cache-Control", "no-store")
	query := strings.TrimSpace(c.Query("q"))
	if len(query) > 100 {
		return c.Status(http.StatusBadRequest).JSON(map[string]string{"error": editorText(editorLocale(), "搜索文字过长")})
	}
	offset := 0
	if raw := c.Query("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return c.Status(http.StatusBadRequest).JSON(map[string]string{"error": editorText(editorLocale(), "无效分页位置")})
		}
		offset = parsed
	}
	names, total := mdi.SearchIconNames(query, offset, 48)
	return c.JSON(map[string]any{"names": names, "total": total, "offset": offset, "limit": 48})
}
