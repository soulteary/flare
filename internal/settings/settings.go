package settings

import (
	"net/http"

	"github.com/labstack/echo/v5"

	FlareDefine "github.com/soulteary/flare/config/define"
)

func RegisterRouting(e *echo.Echo) {
	e.GET(FlareDefine.RegularPages.Settings.Path, pageHome)
	e.GET(FlareDefine.RegularPages.Settings.Path+"/", pageHome)
}

func pageHome(c *echo.Context) error {
	return c.Redirect(http.StatusFound, FlareDefine.SettingPages.Theme.Path)
}
