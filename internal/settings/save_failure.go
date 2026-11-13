package settings

import (
	"net/http"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/logger"
)

// SaveFailure is what a settings page reports when a change was not saved.
// Both texts are in the site language.
type SaveFailure struct {
	// Status is the HTTP status for handlers that do not re-render their form.
	Status int
	// Summary states that nothing was saved.
	Summary string
	// Detail gives the cause and what to do about it.
	Detail string
}

// Text is the complete message for pages with a single message slot.
func (f SaveFailure) Text() string { return f.Summary + " " + f.Detail }

var saveFailureStatus = map[data.SaveFailureReason]int{
	data.SaveFailureInvalidInput:     http.StatusBadRequest,
	data.SaveFailureConfigUnreadable: http.StatusInternalServerError,
	data.SaveFailureConfigChanged:    http.StatusConflict,
	data.SaveFailureNewerVersion:     http.StatusConflict,
	data.SaveFailureSharedNodes:      http.StatusConflict,
	data.SaveFailureNotRegularFile:   http.StatusConflict,
	data.SaveFailureEngineExtension:  http.StatusConflict,
	data.SaveFailureWrite:            http.StatusInternalServerError,
}

// ExplainSaveFailure turns a failed settings save into a localized message
// that names the cause and the remedy. The underlying error, which may contain
// paths and parser detail, goes to the server log instead of the page.
func ExplainSaveFailure(err error) SaveFailure {
	reason := data.SaveFailureReasonOf(err)
	logger.GetLogger().Warn("settings were not saved", "reason", string(reason), "error", err.Error())
	// Never create a missing configuration merely to choose a language.
	locale := "zh"
	if options, readErr := data.ReadExistingSettingsOptions(); readErr == nil {
		locale = options.Locale
	}
	status, known := saveFailureStatus[reason]
	key := "settings_save_failed_" + string(reason)
	if !known {
		status, key = http.StatusInternalServerError, "settings_save_failed_unknown"
	}
	return SaveFailure{Status: status, Summary: i18n.T(locale, "settings_save_failed"), Detail: i18n.T(locale, key)}
}

// ConfigUnreadable classifies a configuration read error met by a handler
// before it could attempt the save.
func ConfigUnreadable(err error) error {
	return &data.SaveError{Reason: data.SaveFailureConfigUnreadable, Err: err}
}
