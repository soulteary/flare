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
	// preview is the theme this one response is shown with instead of the
	// selected theme, or empty.
	preview string
}

// previewParameter names a theme to show one response with. It is honored
// for the administrator only: which theme visitors see is not theirs to
// choose with an address.
const previewParameter = "theme-preview"

// previewedTheme returns the theme the request asks to preview, if it may.
func previewedTheme(c fiber.Ctx) string {
	name := c.Query(previewParameter)
	if name == "" || !auth.CheckUserIsLogin(c) {
		return ""
	}
	// A preview is rare, and it should show the file as it is now, whether
	// or not the settings were opened since the file was edited.
	define.ReloadUserThemes()
	// The selected theme is what the page shows anyway.
	if selected, _ := define.ThemeSnapshot(); name == selected || !define.IsKnownTheme(name) {
		return ""
	}
	return name
}

// requestPreview returns the theme this response previews, or an empty
// name. It is known once requestSettings has run.
func requestPreview(c fiber.Ctx) string {
	if snapshot, taken := c.Locals(requestSettingsKey{}).(settingsSnapshot); taken {
		return snapshot.preview
	}
	return ""
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
		if snapshot.preview = previewedTheme(c); snapshot.preview != "" {
			snapshot.value.Homepage = define.HomepageInEffectFor(snapshot.value, snapshot.preview)
		} else {
			snapshot.value.Homepage = define.HomepageInEffect(snapshot.value)
		}
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
