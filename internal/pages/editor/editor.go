package editor

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v5"
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

func RegisterRouting(e *echo.Echo) {
	registerTransfers(e)
	if introAssets, err := fs.Sub(editorAssets, "editor-assets"); err == nil {
		e.StaticFS(_ASSETS_WEB_URI, introAssets)
	}
	e.GET(define.RegularPages.Editor.Path, render, auth.AuthRequired)
	e.POST(define.RegularPages.Editor.Path, updateData, auth.AuthRequired)
}

func updateData(c *echo.Context) error {
	var body struct {
		Categories string `form:"categories"`
		Bookmarks  string `form:"bookmarks"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusForbidden, "提交数据缺失")
	}
	if !data.UpdateBookmarksFromEditor(body.Categories, body.Bookmarks) {
		return c.JSON(http.StatusInternalServerError, map[string]any{"saved": false, "error": "保存失败，请检查数据格式、目录权限和服务端日志。"})
	}
	if c.Request().Header.Get("Accept") == "application/json" {
		return c.JSON(http.StatusOK, map[string]bool{"saved": true})
	}
	return render(c)
}

func render(c *echo.Context) error {
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.String(http.StatusInternalServerError, "config error")
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
	return c.Render(http.StatusOK, "editor.html", m)
}
