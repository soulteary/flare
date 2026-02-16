package health

import (
	"net/http"

	"github.com/labstack/echo/v5"

	"github.com/soulteary/flare/config/define"
)

func RegisterRouting(e *echo.Echo) {
	e.GET(define.MiscPages.HealthCheck.Path, func(c *echo.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"message": "pong"})
	})
}
