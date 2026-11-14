package home

import (
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
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
	// The pages are rendered with the layout in effect: the options of the
	// site with the layout of the selected theme in place. Every reader of
	// Homepage below this point, the body classes, the category navigation
	// and the fragment cache key among them, takes that one result.
	if snapshot.err == nil {
		snapshot.value.Homepage = define.HomepageInEffect(snapshot.value)
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
