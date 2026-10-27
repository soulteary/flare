package server

import (
	"encoding/json"
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

func TestFiberAuthorizationErrorsIncludeRequestID(t *testing.T) {
	for _, existingID := range []string{"", "existing-request-id"} {
		app := fiber.New(fiber.Config{ErrorHandler: fiberErrorHandler})
		app.Get("/", func(c fiber.Ctx) error {
			if existingID != "" {
				c.Set("X-Request-ID", existingID)
			}
			return fiber.NewError(403, "REAUTH_REQUIRED")
		})
		req := httptest.NewRequest("GET", "/", nil)
		req.Header.Set("Accept", "application/json")
		r, err := app.Test(req)
		require.NoError(t, err)
		raw, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, r.Body.Close())
		require.Equal(t, 403, r.StatusCode)
		require.NotEmpty(t, r.Header.Get("X-Request-ID"))
		require.Contains(t, string(raw), r.Header.Get("X-Request-ID"))
		var payload struct {
			Error struct {
				Code string `json:"code"`
			} `json:"error"`
			RequestID string `json:"request_id"`
		}
		require.NoError(t, json.Unmarshal(raw, &payload))
		require.Equal(t, "REAUTH_REQUIRED", payload.Error.Code)
		require.Equal(t, r.Header.Get("X-Request-ID"), payload.RequestID)
		if existingID != "" {
			require.Equal(t, existingID, r.Header.Get("X-Request-ID"))
		}
	}
}

func TestRouterRejectsInvalidPersistentSourceConfiguration(t *testing.T) {
	t.Chdir(t.TempDir())
	flags := define.AppFlags
	t.Cleanup(func() { define.AppFlags = flags })
	store, err := security.Open(t.TempDir())
	require.NoError(t, err)
	defer store.Close()
	input := model.Flags{DisableLoginMode: true, EnableOfflineMode: true, ExternalURL: "https://user:pass@flare.test"}
	_, err = NewRouter(&input, store)
	require.ErrorContains(t, err, "初始化请求来源")
}
