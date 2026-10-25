package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/logger"
	"github.com/stretchr/testify/require"
)

// TestNewFiberAppAccessLogModes exercises the production router wiring, rather
// than a separately configured middleware. Application logs must remain visible
// even when successful access logs or all access logs are disabled.
func TestNewFiberAppAccessLogModes(t *testing.T) {
	originalFlags := define.AppFlags
	originalTheme, originalColor := define.ThemeCurrent, define.ThemePrimaryColor
	originalLogger := logger.GetLogger()
	t.Cleanup(func() {
		define.AppFlags = originalFlags
		define.ThemeCurrent, define.ThemePrimaryColor = originalTheme, originalColor
		logger.SetLogger(originalLogger)
	})
	t.Setenv("FLARE_BASELINE", "")
	for _, tc := range []struct {
		name, mode string
		successLog bool
		failureLog bool
	}{
		{name: "default", failureLog: true},
		{name: "errors", mode: "errors", failureLog: true},
		{name: "all", mode: "all", successLog: true, failureLog: true},
		{name: "off", mode: "off"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			records := make(chan []byte, 8)
			logger.SetLogger(slog.New(slog.NewJSONHandler(errorResponseLogWriter{records: records}, nil)))
			app, err := NewFiberApp(&model.Flags{
				DisableLoginMode: true, EnableOfflineMode: true, AccessLog: tc.mode,
			})
			require.NoError(t, err)
			app.Get("/__access-success", func(c fiber.Ctx) error { return c.SendString("ok") })
			app.Get("/__access-failure", func(fiber.Ctx) error { return errors.New("access log test failure") })
			app.Get("/__access-panic", func(fiber.Ctx) error { panic("access log test panic") })

			logger.GetLogger().Info("application startup test")
			select {
			case raw := <-records:
				var record map[string]any
				require.NoError(t, json.Unmarshal(raw, &record))
				require.Equal(t, "application startup test", record["msg"])
				require.Equal(t, "INFO", record["level"])
			case <-time.After(time.Second):
				t.Fatal("access log mode suppressed an application INFO log")
			}

			for _, request := range []struct {
				path   string
				status int
				logged bool
			}{
				{path: "/__access-success", status: http.StatusOK, logged: tc.successLog},
				{path: "/ping", status: http.StatusOK},
				{path: "/__access-missing", status: http.StatusNotFound, logged: tc.failureLog},
				{path: "/__access-failure", status: http.StatusInternalServerError, logged: tc.failureLog},
				{path: "/__access-panic", status: http.StatusInternalServerError, logged: tc.failureLog},
			} {
				response, err := app.Test(httptest.NewRequest(http.MethodGet, request.path, nil))
				require.NoError(t, err)
				require.Equal(t, request.status, response.StatusCode)
				require.NoError(t, response.Body.Close())
				if !request.logged {
					select {
					case raw := <-records:
						t.Fatalf("unexpected access log for %s: %s", request.path, raw)
					case <-time.After(20 * time.Millisecond):
					}
					continue
				}
				select {
				case raw := <-records:
					var record map[string]any
					require.NoError(t, json.Unmarshal(raw, &record))
					require.Equal(t, request.path, record["path"])
					require.Equal(t, float64(request.status), record["status"])
				case <-time.After(time.Second):
					t.Fatalf("missing access log for %s", request.path)
				}
			}
		})
	}
}
