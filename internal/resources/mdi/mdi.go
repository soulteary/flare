package mdi

import (
	"embed"
	"io/fs"
	"log"
	"path/filepath"
	"strings"

	"github.com/labstack/echo/v5"
	"github.com/soulteary/memfs"

	"github.com/soulteary/flare/config/define"
)

var MemFs *memfs.FS

const _ASSETS_BASE_DIR = "assets/mdi"
const _ASSETS_WEB_URI = "/" + _ASSETS_BASE_DIR

var _CACHE_MDI_ICON_EXIST map[string]bool
var _CACHE_MDI_ICON_DATA map[string]string

//go:embed mdi-cheat-sheets
var MdiExampleAssets embed.FS

func Init() error {
	MemFs = memfs.New()
	err := MemFs.MkdirAll(_ASSETS_BASE_DIR, 0777)
	if err != nil {
		return err
	}
	_CACHE_MDI_ICON_EXIST = make(map[string]bool)
	_CACHE_MDI_ICON_DATA = make(map[string]string)
	return nil
}

func RegisterRouting(e *echo.Echo) {
	if weather, err := fs.Sub(MemFs, _ASSETS_BASE_DIR); err == nil {
		e.StaticFS(_ASSETS_WEB_URI, weather)
	}
	if mdiExample, err := fs.Sub(MdiExampleAssets, "mdi-cheat-sheets"); err == nil {
		e.StaticFS(define.RegularPages.Icons.Path, mdiExample)
	}
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
	if define.AppFlags.EnableMinimumRequest {
		if !_CACHE_MDI_ICON_EXIST[name] {
			content = `<svg viewBox="0 0 24 24"><path d="` + icon + `" style="fill: var(--color-primary);"></path></svg>`
			_CACHE_MDI_ICON_DATA[name] = content
			_CACHE_MDI_ICON_EXIST[name] = true
		}
		return _CACHE_MDI_ICON_DATA[name]
	}
	svgFile := filepath.ToSlash(filepath.Join(_ASSETS_BASE_DIR, (define.ThemeCurrent + "-" + name + ".svg")))
	if !_CACHE_MDI_ICON_EXIST[define.ThemeCurrent+"-"+name] {
		content = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="` + icon + `" style="fill:` + define.ThemePrimaryColor + `;"></path></svg>`
		err := MemFs.WriteFile(svgFile, []byte(content), 0755)
		if err != nil {
			log.Println("缓存内置图标出错:", err)
		}
		_, err = fs.ReadFile(MemFs, svgFile)
		if err != nil {
			log.Println("读取内置图标缓存出错:", err)
			return _EMPTY_ICON
		}
		_CACHE_MDI_ICON_EXIST[define.ThemeCurrent+"-"+name] = true
	}
	return `<img src="/` + svgFile + `" width="68" height="68" alt="">`
}
