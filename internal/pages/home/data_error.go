package home

import (
	"html"
	"html/template"

	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/logger"
)

// navigationDataError keeps saved-data failures visible without exposing local
// filenames or parser details to page visitors. Operators get the original error.
func navigationDataError(module, locale string, err error) template.HTML {
	logger.GetLogger().Error("Cannot render navigation module", "module", module, "error", err.Error())
	key := "navigation.bookmarks_error"
	if module == "applications" {
		key = "navigation.applications_error"
	}
	// Escape translated text before embedding it in fixed markup.
	return template.HTML(`<p class="navigation-data-error" role="alert">` + html.EscapeString(i18n.T(locale, key)) + `</p>`)
}
