//go:build fiber

package settings

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/define"
)

func RegisterRouting(e *fiber.App) {
	e.Get(define.RegularPages.Settings.Path, pageHome)
	e.Get(define.RegularPages.Settings.Path+"/", pageHome)
}

func pageHome(c fiber.Ctx) error {
	return c.Redirect().Status(http.StatusFound).To(define.SettingPages.Theme.Path)
}
