package editor

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	"github.com/labstack/echo/v5"
	"github.com/soulteary/memfs"

	FlareData "github.com/soulteary/flare/config/data"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareAuth "github.com/soulteary/flare/internal/auth"
	FlarePool "github.com/soulteary/flare/internal/pool"
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
	introAssets, _ := fs.Sub(editorAssets, "editor-assets")
	e.StaticFS(_ASSETS_WEB_URI, introAssets)
	e.GET(FlareDefine.RegularPages.Editor.Path, render, FlareAuth.AuthRequired)
	e.POST(FlareDefine.RegularPages.Editor.Path, updateData, FlareAuth.AuthRequired)
}

func updateData(c *echo.Context) error {
	var body struct {
		Categories string `form:"categories"`
		Bookmarks  string `form:"bookmarks"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusForbidden, "提交数据缺失")
	}
	FlareData.UpdateBookmarksFromEditor(body.Categories, body.Bookmarks)
	return render(c)
}

func render(c *echo.Context) error {
	options := FlareData.GetAllSettingsOptions()
	dataCategories, dataBookmarks := FlareData.GetBookmarksForEditor()
	m := FlarePool.GetTemplateMap()
	defer FlarePool.PutTemplateMap(m)
	m["PageName"] = "Editor"
	m["PageAppearance"] = FlareDefine.GetAppBodyStyle()
	m["SettingPages"] = FlareDefine.SettingPages
	m["DebugMode"] = FlareDefine.AppFlags.DebugMode
	m["PageInlineStyle"] = FlareDefine.GetPageInlineStyle()
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
