package home

import (
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/logger"
)

// navigationDataError keeps saved-data failures visible without exposing local
// filenames or parser details to page visitors. Operators get the original error.
func navigationDataError(module, locale string, err error) string {
	logger.GetLogger().Error("Cannot render navigation module", "module", module, "error", err.Error())
	key := "navigation.bookmarks_error"
	if module == "applications" {
		key = "navigation.applications_error"
	}
	// Return plain text; home.html owns markup and normal template escaping.
	return i18n.T(locale, key)
}
