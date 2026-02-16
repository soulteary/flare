package editor

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/soulteary/memfs"

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

func Init() {
	MemFs = memfs.New()
	err := MemFs.MkdirAll(_ASSETS_BASE_DIR, 0777)
	if err != nil {
		panic(err)
	}
}

func RegisterRouting(e *echo.Echo) {
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
	data.UpdateBookmarksFromEditor(body.Categories, body.Bookmarks)
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
