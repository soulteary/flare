package guide

import (
	"embed"
	"io/fs"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/internal/resources/assets"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/pages/home"
)

const _ASSETS_WEB_URI = "/assets/guide"

//go:embed guide-assets
var IntroAssets embed.FS

func RegisterRouting(e *fiber.App) {
	if introAssets, err := fs.Sub(IntroAssets, "guide-assets"); err == nil {
		e.Use(_ASSETS_WEB_URI, assets.StaticFS(introAssets, _ASSETS_WEB_URI))
	}
	if define.AppFlags.Visibility == "PRIVATE" {
		e.Get(define.RegularPages.Guide.Path, auth.AuthRequired, render)
	} else {
		e.Get(define.RegularPages.Guide.Path, render)
	}
}

// render shows the visitor's own homepage with the guide assets enabled. The
// page is rendered within this request instead of being fetched over loopback
// HTTP, so the visitor's session, visibility rules and Host policy still apply.
func render(c fiber.Ctx) error {
	return home.RenderGuide(c, _ASSETS_WEB_URI)
}
