package data

import (
	"errors"
	"log"
)

// SaveFailureReason classifies why a settings save was not applied. Handlers
// translate it into an actionable message; the wrapped error keeps the detail
// for the server log.
type SaveFailureReason string

const (
	// SaveFailureInvalidInput means the submitted values failed validation.
	SaveFailureInvalidInput SaveFailureReason = "invalid_input"
	// SaveFailureConfigUnreadable means config.yml cannot be read or parsed.
	SaveFailureConfigUnreadable SaveFailureReason = "config_unreadable"
	// SaveFailureConfigChanged means config.yml changed after it was loaded.
	SaveFailureConfigChanged SaveFailureReason = "config_changed"
	// SaveFailureNewerVersion means config.yml was written by a newer release.
	SaveFailureNewerVersion SaveFailureReason = "newer_version"
	// SaveFailureSharedNodes means YAML aliases or merge keys prevent a safe edit.
	SaveFailureSharedNodes SaveFailureReason = "yaml_aliases"
	// SaveFailureNotRegularFile means config.yml is a symbolic link or special file.
	SaveFailureNotRegularFile SaveFailureReason = "not_regular_file"
	// SaveFailureEngineExtension means an engine ID change would drop unknown fields.
	SaveFailureEngineExtension SaveFailureReason = "engine_extension"
	// SaveFailureWrite means the replacement file could not be written.
	SaveFailureWrite SaveFailureReason = "write_failed"
)

// SaveError is returned by the settings save functions. Its text is the
// underlying error, so existing log output and error matching are unchanged.
type SaveError struct {
	Reason SaveFailureReason
	Err    error
}

func (e *SaveError) Error() string { return e.Err.Error() }
func (e *SaveError) Unwrap() error { return e.Err }

// saveError classifies err. An error that already carries a reason keeps it:
// the innermost check knows the cause best.
func saveError(reason SaveFailureReason, err error) error {
	var classified *SaveError
	if errors.As(err, &classified) {
		return err
	}
	return &SaveError{Reason: reason, Err: err}
}

// SaveFailureReasonOf returns the classified reason, or "" when err has none.
func SaveFailureReasonOf(err error) SaveFailureReason {
	var classified *SaveError
	if errors.As(err, &classified) {
		return classified.Reason
	}
	return ""
}

// reportSave adapts a classified save to the boolean API used by callers that
// only need to know whether the change was applied.
func reportSave(err error) bool {
	if err != nil {
		log.Println(err)
		return false
	}
	return true
}
