package settings

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/soulteary/flare/config/define"
)

func RegisterRouting(e *echo.Echo) {
	e.GET(define.RegularPages.Settings.Path, pageHome)
	e.GET(define.RegularPages.Settings.Path+"/", pageHome)
}

func pageHome(c *echo.Context) error {
	return c.Redirect(http.StatusFound, define.SettingPages.Theme.Path)
}
