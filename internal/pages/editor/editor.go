package editor

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	memfs "github.com/soulteary/memfs-kit"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/pool"
)

var MemFs *memfs.FS

const _ASSETS_BASE_DIR = "assets/editor"
const _ASSETS_WEB_URI = "/" + _ASSETS_BASE_DIR

//go:embed editor-assets
var editorAssets embed.FS

func Init() error {
	MemFs = memfs.New()
	err := MemFs.MkdirAll(_ASSETS_BASE_DIR, 0777)
	if err != nil {
		return err
	}
	return nil
}

func RegisterRouting(e *fiber.App) {
	registerTransfers(e)
	registerEditorClient(e)
	registerEditorDocument(e)
	if introAssets, err := fs.Sub(editorAssets, "editor-assets"); err == nil {
		e.Use(_ASSETS_WEB_URI, static.New("", static.Config{FS: introAssets}))
	}
	e.Get(define.RegularPages.Editor.Path, auth.AuthRequired, render)
	e.Post(define.RegularPages.Editor.Path, auth.AuthRequired, updateData)
}

// Old CSV pages have no revision token and cannot safely overwrite a document.
func updateData(c fiber.Ctx) error {
	return c.Status(http.StatusConflict).JSON(map[string]any{"saved": false, "error": "编辑器已升级，请先保留当前内容，再刷新页面后重新编辑。"})
}

func render(c fiber.Ctx) error {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	dataCategories, dataBookmarks := data.GetBookmarksForEditor()
	m := pool.GetTemplateMap()
	defer pool.PutTemplateMap(m)
	m["PageName"] = "Editor"
	m["PageAppearance"] = define.GetAppBodyStyle()
	m["SettingPages"] = define.SettingPages
	m["DebugMode"] = define.AppFlags.DebugMode
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	m["DataCategories"] = template.HTML(dataCategories)
	m["DataBookmarks"] = template.HTML(dataBookmarks)
	m["OptionTitle"] = options.Title
	m["OptionFooter"] = template.HTML(options.Footer)
	m["OptionOpenAppNewTab"] = options.OpenAppNewTab
	m["OptionOpenBookmarkNewTab"] = options.OpenBookmarkNewTab
	m["OptionShowTitle"] = options.ShowTitle
	m["OptionShowDateTime"] = options.ShowDateTime
	m["OptionShowApps"] = options.ShowApps
	m["OptionShowBookmarks"] = options.ShowBookmarks
	return c.Status(http.StatusOK).Render("editor.html", m)
}
