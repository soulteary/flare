package assets

import (
	"embed"
	"fmt"
	"io"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
)

//go:embed favicons/*.ico
var themedFaviconFiles embed.FS

type themeFavicon struct{ url, name, etag string }

// ThemeFaviconURL performs only a bounded lookup; metadata is generated at build time.
func ThemeFaviconURL() string {
	theme, _ := define.ThemeSnapshot()
	if icon, ok := themeFavicons[theme]; ok {
		return icon.url
	}
	return themeFavicons[define.ThemePalettes[0].Name].url
}

func registerThemeFavicons(app *fiber.App) {
	for _, icon := range themeFavicons {
		handler := func(c fiber.Ctx) error {
			file, err := themedFaviconFiles.Open(icon.name)
			if err != nil {
				return err
			}
			defer file.Close()
			c.Set("Cache-Control", "public, max-age=31536000, immutable")
			reader, ok := file.(io.ReadSeeker)
			if !ok {
				return fmt.Errorf("embedded favicon %s does not support seeking", icon.name)
			}
			return serveReader(c, icon.name, "image/x-icon", icon.etag, reader)
		}
		app.Get(icon.url, handler)
		app.Head(icon.url, handler)
	}
}
