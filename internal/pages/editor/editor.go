package editor

import (
	"embed"
	"github.com/soulteary/flare/config/data"
	"io/fs"
	"log/slog"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	memfs "github.com/soulteary/memfs-kit"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/auth"
)

var MemFs *memfs.FS

const _ASSETS_BASE_DIR = "assets/editor"
const _ASSETS_WEB_URI = "/" + _ASSETS_BASE_DIR

//go:embed editor-assets
var editorAssets embed.FS

func Init() error {
	recoveries, err := data.EditorRecoveryDirectories()
	if err != nil {
		return err
	}
	for _, path := range recoveries {
		slog.Warn("Editor recovery copies detected; stop writers, preserve current files, and inspect both YAML files before saving", "recovery_path", path)
	}
	MemFs = memfs.New()
	err = MemFs.MkdirAll(_ASSETS_BASE_DIR, 0777)
	if err != nil {
		return err
	}
	return nil
}

func RegisterRouting(e *fiber.App) {
	registerTransfers(e)
	registerEditorClient(e)
	registerEditorDocument(e)
	registerEditorIcon(e)
	if introAssets, err := fs.Sub(editorAssets, "editor-assets"); err == nil {
		e.Use(_ASSETS_WEB_URI, static.New("", static.Config{FS: introAssets}))
	}
	e.Get(define.RegularPages.Editor.Path, auth.AuthRequired, render)
	e.Get("/editor/batch", auth.AuthRequired, render)
	e.Post(define.RegularPages.Editor.Path, auth.AuthRequired, updateData)
}

// Old CSV pages have no revision token and cannot safely overwrite a document.
func updateData(c fiber.Ctx) error {
	return c.Status(http.StatusConflict).JSON(map[string]any{"saved": false, "error": editorText(editorLocale(), "编辑器已升级，请先保留当前内容，再刷新页面后重新编辑。")})
}

func render(c fiber.Ctx) error {
	return c.Status(http.StatusOK).Render("editor.html", map[string]any{"Locale": editorLocale(), "EditorMessages": editorMessageJSON(editorLocale())})
}
