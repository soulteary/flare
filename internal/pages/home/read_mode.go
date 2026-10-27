package home

import (
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
)

func requestSettings(c fiber.Ctx) (model.Application, error) {
	if auth.BufferedReadTentative(c) {
		return data.ReadExistingSettingsOptions()
	}
	return data.GetAllSettingsOptions()
}
func requestReadableCategory(c fiber.Ctx, id string, administrator bool) bool {
	if auth.BufferedReadTentative(c) {
		visible, err := data.ReadableExistingBookmarkCategory(id, administrator)
		return err == nil && visible
	}
	return readableCategory(id, administrator)
}
