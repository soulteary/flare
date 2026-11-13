package settings

import (
	"bytes"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/logger"
	"github.com/stretchr/testify/require"
)

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var out bytes.Buffer
	previous := logger.GetLogger()
	logger.SetLogger(slog.New(slog.NewTextHandler(&out, nil)))
	t.Cleanup(func() { logger.SetLogger(previous) })
	return &out
}

func TestExplainSaveFailureIsLocalizedAndKeepsDetailInTheLog(t *testing.T) {
	for _, tc := range []struct {
		reason data.SaveFailureReason
		status int
	}{
		{data.SaveFailureInvalidInput, http.StatusBadRequest},
		{data.SaveFailureConfigUnreadable, http.StatusInternalServerError},
		{data.SaveFailureConfigChanged, http.StatusConflict},
		{data.SaveFailureNewerVersion, http.StatusConflict},
		{data.SaveFailureSharedNodes, http.StatusConflict},
		{data.SaveFailureNotRegularFile, http.StatusConflict},
		{data.SaveFailureEngineExtension, http.StatusConflict},
		{data.SaveFailureWrite, http.StatusInternalServerError},
	} {
		for _, locale := range []string{"zh", "en"} {
			t.Run(fmt.Sprintf("%s/%s", tc.reason, locale), func(t *testing.T) {
				t.Chdir(t.TempDir())
				require.NoError(t, os.WriteFile("config.yml", []byte("Locale: "+locale+"\n"), 0600))
				log := captureLog(t)
				detail := errors.New("open /private/path/config.yml: internal detail")
				failure := ExplainSaveFailure(fmt.Errorf("wrapped: %w", &data.SaveError{Reason: tc.reason, Err: detail}))
				require.Equal(t, tc.status, failure.Status)
				key := "settings_save_failed_" + string(tc.reason)
				require.Equal(t, i18n.T(locale, "settings_save_failed"), failure.Summary)
				require.Equal(t, i18n.T(locale, key), failure.Detail)
				require.NotEqual(t, key, failure.Detail, "missing translation")
				require.Equal(t, failure.Summary+" "+failure.Detail, failure.Text())
				require.NotContains(t, failure.Text(), "internal detail")
				require.Contains(t, log.String(), "internal detail")
				require.Contains(t, log.String(), "reason="+string(tc.reason))
			})
		}
	}
}

func TestExplainSaveFailureWithoutClassificationOrConfiguration(t *testing.T) {
	t.Chdir(t.TempDir())
	log := captureLog(t)
	failure := ExplainSaveFailure(errors.New("unexpected internal detail"))
	require.Equal(t, http.StatusInternalServerError, failure.Status)
	// Without a readable configuration the site default language applies.
	require.Equal(t, i18n.T("zh", "settings_save_failed"), failure.Summary)
	require.Equal(t, i18n.T("zh", "settings_save_failed_unknown"), failure.Detail)
	require.NotContains(t, failure.Text(), "unexpected internal detail")
	require.Contains(t, log.String(), "unexpected internal detail")
	_, err := os.Stat("config.yml")
	require.ErrorIs(t, err, os.ErrNotExist, "explaining a failure must not create a configuration")

	cause := errors.New("read failed")
	classified := ConfigUnreadable(cause)
	require.ErrorIs(t, classified, cause)
	require.Equal(t, data.SaveFailureConfigUnreadable, data.SaveFailureReasonOf(classified))
}
