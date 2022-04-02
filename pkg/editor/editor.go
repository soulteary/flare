package editor

import (
	"embed"
	"html/template"
	"io/fs"
	"net/http"

	FlareAuth "github.com/soulteary/flare/pkg/auth"
	FlareState "github.com/soulteary/flare/state"

	FlareData "github.com/soulteary/flare/data"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/memfs"
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

func RegisterRouting(router *gin.Engine) {
	introAssets, _ := fs.Sub(editorAssets, "editor-assets")
	router.StaticFS(_ASSETS_WEB_URI, http.FS(introAssets))

	router.GET(FlareState.RegularPages.Editor.Path, FlareAuth.AuthRequired, render)
	router.POST(FlareState.RegularPages.Editor.Path, FlareAuth.AuthRequired, updateData)
}

func updateData(c *gin.Context) {

	type UpdateBody struct {
		Categories string `form:"categories"`
		Bookmarks  string `form:"bookmarks"`
	}

	var body UpdateBody
	if c.ShouldBind(&body) != nil {
		c.PureJSON(http.StatusForbidden, "提交数据缺失")
		return
	}

	FlareData.UpdateBookmarksFromEditor(body.Categories, body.Bookmarks)
	render(c)
}

func render(c *gin.Context) {
	options := FlareData.GetAllSettingsOptions()

	dataCategories, dataBookmarks := FlareData.GetBookmarksForEditor()
	c.HTML(
		http.StatusOK,
		"editor.html",
		gin.H{
			"PageName":       "Editor",
			"PageAppearance": FlareState.GetAppBodyStyle(),
			"SettingPages":   FlareState.SettingPages,

			"DebugMode":       FlareState.AppFlags.DebugMode,
			"PageInlineStyle": FlareState.GetPageInlineStyle(),

			"DataCategories": template.HTML(dataCategories),
			"DataBookmarks":  template.HTML(dataBookmarks),

			"OptionTitle":              options.Title,
			"OptionFooter":             template.HTML(options.Footer),
			"OptionOpenAppNewTab":      options.OpenAppNewTab,
			"OptionOpenBookmarkNewTab": options.OpenBookmarkNewTab,
			"OptionShowTitle":          options.ShowTitle,
			"OptionShowDateTime":       options.ShowDateTime,
			"OptionShowApps":           options.ShowApps,
			"OptionShowBookmarks":      options.ShowBookmarks,
		},
	)
}
