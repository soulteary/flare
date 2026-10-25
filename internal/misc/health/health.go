package health

import (
	"net/http"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/define"
)

func RegisterRouting(e *fiber.App) {
	e.Get(define.MiscPages.HealthCheck.Path, func(c fiber.Ctx) error {
		return c.Status(http.StatusOK).JSON(map[string]string{"message": "pong"})
	})
}
