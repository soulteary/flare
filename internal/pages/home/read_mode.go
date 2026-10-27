package home

import (
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
)

type requestSettingsKey struct{}
type settingsSnapshot struct {
	value model.Application
	err   error
}

// The snapshot is read-only and lives only for this request. Cache errors too:
// a later helper must not repair a failed tentative read or mix configurations.
func requestSettings(c fiber.Ctx) (model.Application, error) {
	if snapshot, ok := c.Locals(requestSettingsKey{}).(settingsSnapshot); ok {
		return snapshot.value, snapshot.err
	}
	var snapshot settingsSnapshot
	if auth.BufferedReadTentative(c) {
		snapshot.value, snapshot.err = data.ReadExistingSettingsOptions()
	} else {
		snapshot.value, snapshot.err = data.GetAllSettingsOptions()
	}
	c.Locals(requestSettingsKey{}, snapshot)
	return snapshot.value, snapshot.err
}
func requestReadableCategory(c fiber.Ctx, id string, administrator bool) bool {
	if auth.BufferedReadTentative(c) {
		visible, err := data.ReadableExistingBookmarkCategory(id, administrator)
		return err == nil && visible
	}
	return readableCategory(id, administrator)
}
