package mdi

import (
	"crypto/sha256"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
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

// iconFragment is the id of the path inside a generated icon file. Pages
// reference it with <use>, so the icon takes its color from the page.
const iconFragment = "i"

type iconCacheKey struct {
	name   string
	inline bool
}

var iconMarkupCache map[iconCacheKey]string

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
	iconMarkupCache = make(map[iconCacheKey]string)
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
		// The index uses relative CSS URLs, which require a directory URL.
		e.Get(define.RegularPages.Icons.Path, func(c fiber.Ctx) error {
			if strings.HasSuffix(c.Path(), "/") {
				return c.Next()
			}
			return c.Redirect().Status(http.StatusFound).To(define.RegularPages.Icons.Path + "/")
		})
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

		// Icon files are stored under lower-case names. Accept the mixed-case
		// spelling used in navigation files so that one canonical file serves both.
		name = strings.ToLower(name)

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
	icon := iconPath(strings.ToLower(name))
	if icon == "" {
		return _EMPTY_ICON
	}
	_, color := define.ThemeSnapshot()
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="` + icon + `" style="fill:` + color + `;"></path></svg>`
}

func GetIconByName(name string) string {
	name = strings.ToLower(name)
	if name == "" {
		return _EMPTY_ICON
	}
	icon := iconPath(name)
	if icon == "" {
		return _EMPTY_ICON
	}
	key := iconCacheKey{name: name, inline: define.AppFlags.EnableMinimumRequest}
	iconCacheMu.RLock()
	markup, exists := iconMarkupCache[key]
	iconCacheMu.RUnlock()
	if exists {
		return markup
	}

	iconCacheMu.Lock()
	defer iconCacheMu.Unlock()
	// A concurrent first use may have generated the same normalized icon.
	if markup, exists = iconMarkupCache[key]; exists {
		return markup
	}
	if key.inline {
		markup = `<svg viewBox="0 0 24 24"><path d="` + icon + `" style="fill: var(--color-primary);"></path></svg>`
	} else {
		// The file carries no color and does not depend on the theme: the page
		// references its path and fills it with the theme color, so a theme or
		// color change needs no new file.
		svgFile := _ASSETS_BASE_DIR + "/" + name + ".svg"
		content := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path id="` + iconFragment + `" d="` + icon + `"/></svg>`
		if err := MemFs.WriteFile(svgFile, []byte(content), 0755); err != nil {
			log.Println("缓存内置图标出错:", err)
			return _EMPTY_ICON
		}
		// The version bypasses a previously cached file after an icon-library
		// update. The query is not an access-control or immutable guarantee.
		version := fmt.Sprintf("%x", sha256.Sum256([]byte(content)))
		markup = `<svg class="icon-ref" viewBox="0 0 24 24" width="68" height="68" style="fill: var(--color-primary);" aria-hidden="true"><use href="/` + svgFile + `?v=` + version + `#` + iconFragment + `"></use></svg>`
	}
	// Do not retain a borrowed request string from a caller using Fiber's
	// mutable request mode. Canonical hot lookups need no additional copy.
	key.name = strings.Clone(key.name)
	iconMarkupCache[key] = markup
	return markup
}
