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
	if introAssets, err := fs.Sub(editorAssets, "editor-assets"); err == nil {
		e.Use(_ASSETS_WEB_URI, static.New("", static.Config{FS: introAssets}))
	}
	e.Get(define.RegularPages.Editor.Path, auth.AuthRequired, render)
	e.Post(define.RegularPages.Editor.Path, auth.AuthRequired, updateData)
}

func updateData(c fiber.Ctx) error {
	var body struct {
		Categories string `form:"categories"`
		Bookmarks  string `form:"bookmarks"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return c.Status(http.StatusForbidden).JSON("提交数据缺失")
	}
	if !data.UpdateBookmarksFromEditor(body.Categories, body.Bookmarks) {
		return c.Status(http.StatusInternalServerError).JSON(map[string]any{"saved": false, "error": "保存失败，请检查数据格式、目录权限和服务端日志。"})
	}
	if c.Get("Accept") == "application/json" {
		return c.Status(http.StatusOK).JSON(map[string]bool{"saved": true})
	}
	return render(c)
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
