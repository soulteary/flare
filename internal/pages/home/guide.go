package home

import (
	"strings"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/clock"
	"github.com/soulteary/flare/internal/searchui"
)

type guideAssetsKey struct{}

// RenderGuide renders the homepage for the interactive guide within the current
// request. The visitor's session, visibility filtering and request-derived
// values therefore apply exactly as they do on the homepage. The template adds
// the guide stylesheets and scripts served under assetsURI.
func RenderGuide(c fiber.Ctx, assetsURI string) error {
	c.Locals(guideAssetsKey{}, assetsURI)
	return render(c, "")
}

func guideAssetsURI(c fiber.Ctx) string {
	if uri, ok := c.Locals(guideAssetsKey{}).(string); ok {
		return uri
	}
	return ""
}

// applyGuideScriptPolicy replaces the script-free homepage policy on the guide,
// which has to load its own same-origin scripts. Inline scripts remain limited
// to the hashed homepage ones, so footer markup cannot run script here either.
// Trusted Types enforcement is left out because the guide library assigns markup.
func applyGuideScriptPolicy(c fiber.Ctx, withClock, withSearch bool) {
	if define.AppFlags.DisableCSP {
		return
	}
	sources := "'self'"
	if withSearch {
		sources += " " + searchui.Hash()
	}
	if withClock {
		sources += " " + strings.TrimPrefix(strings.SplitN(clock.Policy(), ";", 2)[0], "script-src ")
	}
	c.Set("Content-Security-Policy", "script-src "+sources+"; script-src-attr 'none'; connect-src 'self'; object-src 'none'; base-uri 'none';")
}
