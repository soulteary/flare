package mdi

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/static"
	memfs "github.com/soulteary/memfs-kit"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/resources/assets"
)

var MemFs *memfs.FS

const _ASSETS_BASE_DIR = "assets/mdi"
const _ASSETS_WEB_URI = "/" + _ASSETS_BASE_DIR

var _CACHE_MDI_ICON_EXIST map[string]bool
var _CACHE_MDI_ICON_DATA map[string]string

// iconCacheMu keeps cache initialization and icon generation consistent with
// concurrent reads of the cache maps and generated files.
var iconCacheMu sync.RWMutex

//go:embed mdi-cheat-sheets
var MdiExampleAssets embed.FS

func Init() error {
	iconCacheMu.Lock()
	defer iconCacheMu.Unlock()

	MemFs = memfs.New()
	err := MemFs.MkdirAll(_ASSETS_BASE_DIR, 0777)
	if err != nil {
		return err
	}
	_CACHE_MDI_ICON_EXIST = make(map[string]bool)
	_CACHE_MDI_ICON_DATA = make(map[string]string)
	return nil
}

func RegisterRouting(e *fiber.App) {
	iconCacheMu.RLock()
	icons, err := fs.Sub(MemFs, _ASSETS_BASE_DIR)
	iconCacheMu.RUnlock()
	if err == nil {
		handler := iconFileHandler(icons)
		e.Get(_ASSETS_WEB_URI+"/:name", handler)
		e.Head(_ASSETS_WEB_URI+"/:name", handler)
	}
	if mdiExample, err := fs.Sub(MdiExampleAssets, "mdi-cheat-sheets"); err == nil {
		e.Use(define.RegularPages.Icons.Path, static.New("", static.Config{FS: mdiExample, ByteRange: true}))
	}
}

// iconFileHandler serves generated SVGs with a content-derived ETag and an
// independent reader per request so HTTP ranges and repeated requests stay
// consistent. Only direct SVG filenames inside the icon subtree are accepted.
func iconFileHandler(icons fs.FS) fiber.Handler {
	return func(c fiber.Ctx) error {
		name := c.Params("name")
		if !fs.ValidPath(name) || strings.ContainsAny(name, `/\`) || !strings.HasSuffix(name, ".svg") {
			return fiber.ErrNotFound
		}

		iconCacheMu.RLock()
		data, err := fs.ReadFile(icons, name)
		iconCacheMu.RUnlock()
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return fiber.ErrNotFound
			}
			return fmt.Errorf("read MDI icon %q: %w", name, err)
		}

		// Validate existence before setting a validator. A different resource's
		// ETag must never turn a missing icon into a successful 304 response.
		return assets.ServeContent(c, name, "image/svg+xml", data)
	}
}

const _EMPTY_ICON = ""

// GetIconSVGByName renders a standalone preview without relying on generated
// asset files or minimum-request mode's inline CSS variables.
func GetIconSVGByName(name string) string {
	icon := iconMap[strings.ToLower(name)]
	if icon == "" {
		return _EMPTY_ICON
	}
	_, color := define.ThemeSnapshot()
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="` + icon + `" style="fill:` + color + `;"></path></svg>`
}

func GetIconByName(name string) string {
	if name == "" {
		return _EMPTY_ICON
	}
	icon := iconMap[strings.ToLower(name)]
	if icon == "" {
		return _EMPTY_ICON
	}
	iconCacheMu.Lock()
	defer iconCacheMu.Unlock()

	content := ""
	if define.AppFlags.EnableMinimumRequest {
		if !_CACHE_MDI_ICON_EXIST[name] {
			content = `<svg viewBox="0 0 24 24"><path d="` + icon + `" style="fill: var(--color-primary);"></path></svg>`
			_CACHE_MDI_ICON_DATA[name] = content
			_CACHE_MDI_ICON_EXIST[name] = true
		}
		return _CACHE_MDI_ICON_DATA[name]
	}
	theme, color := define.ThemeSnapshot()
	svgFile := filepath.ToSlash(filepath.Join(_ASSETS_BASE_DIR, (theme + "-" + name + ".svg")))
	if !_CACHE_MDI_ICON_EXIST[theme+"-"+name] {
		content = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="` + icon + `" style="fill:` + color + `;"></path></svg>`
		err := MemFs.WriteFile(svgFile, []byte(content), 0755)
		if err != nil {
			log.Println("缓存内置图标出错:", err)
			return _EMPTY_ICON
		}
		_, err = fs.ReadFile(MemFs, svgFile)
		if err != nil {
			log.Println("读取内置图标缓存出错:", err)
			return _EMPTY_ICON
		}
		_CACHE_MDI_ICON_EXIST[theme+"-"+name] = true
	}
	return `<img src="/` + svgFile + `" width="68" height="68" alt="">`
}
