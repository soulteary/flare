package mdi

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/soulteary/memfs"

	FlareDefine "github.com/soulteary/flare/config/define"
)

var MemFs *memfs.FS

const _ASSETS_BASE_DIR = "assets/mdi"
const _ASSETS_WEB_URI = "/" + _ASSETS_BASE_DIR

// 缓存图标
var _CACHE_MDI_ICON_EXIST map[string]bool
var _CACHE_MDI_ICON_DATA map[string]string

//go:embed mdi-cheat-sheets
var MdiExampleAssets embed.FS

func Init() {
	MemFs = memfs.New()
	err := MemFs.MkdirAll(_ASSETS_BASE_DIR, 0777)

	if err != nil {
		panic(err)
	}

	_CACHE_MDI_ICON_EXIST = make(map[string]bool)
	_CACHE_MDI_ICON_DATA = make(map[string]string)
}

func RegisterRouting(router *gin.Engine) {
	weather, _ := fs.Sub(MemFs, _ASSETS_BASE_DIR)
	router.StaticFS(_ASSETS_WEB_URI, http.FS(weather))

	mdiExample, _ := fs.Sub(MdiExampleAssets, "mdi-cheat-sheets")
	router.StaticFS(FlareDefine.RegularPages.Icons.Path, http.FS(mdiExample))
}

const _EMPTY_ICON = ""

func GetIconByName(name string) string {
	if name == "" {
		return _EMPTY_ICON
	}

	icon := iconMap[strings.ToLower(name)]
	if icon == "" {
		return _EMPTY_ICON
	}

	content := ""

	if FlareDefine.AppFlags.EnableMinimumRequest {
		if !_CACHE_MDI_ICON_EXIST[name] {
			content = `<svg viewBox="0 0 24 24"><path d="` + icon + `" style="fill: var(--color-primary);"></path></svg>`
			_CACHE_MDI_ICON_DATA[name] = content
			_CACHE_MDI_ICON_EXIST[name] = true
		}
		return _CACHE_MDI_ICON_DATA[name]
	}

	svgFile := filepath.ToSlash(filepath.Join(_ASSETS_BASE_DIR, (FlareDefine.ThemeCurrent + "-" + name + ".svg")))
	if !_CACHE_MDI_ICON_EXIST[FlareDefine.ThemeCurrent+"-"+name] {
		content = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="` + icon + `" style="fill:` + FlareDefine.ThemePrimaryColor + `;"></path></svg>`

		err := MemFs.WriteFile(svgFile, []byte(content), 0755)
		if err != nil {
			log.Println("缓存内置图标出错:", err)
		}

		_, err = fs.ReadFile(MemFs, svgFile)
		if err != nil {
			panic(err)
		}
		_CACHE_MDI_ICON_EXIST[FlareDefine.ThemeCurrent+"-"+name] = true
	}

	return `<img src="/` + svgFile + `" width="68" height="68" alt="">`
}
