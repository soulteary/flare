package home

import (
	"errors"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/logger"
)

// navigationDataError keeps saved-data failures visible without exposing local
// filenames or parser details to page visitors. Operators get the original error.
func navigationDataError(module, locale string, err error) string {
	logger.GetLogger().Error("Cannot render navigation module", "module", module, "error", err.Error())
	if errors.Is(err, data.ErrPrivateContentRequiresLogin) {
		// Not a storage fault: say what to change. No entry is named.
		return i18n.T(locale, "navigation.private_requires_login")
	}
	key := "navigation.bookmarks_error"
	if module == "applications" {
		key = "navigation.applications_error"
	}
	// Return plain text; home.html owns markup and normal template escaping.
	return i18n.T(locale, key)
}
