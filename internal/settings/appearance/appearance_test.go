package appearance

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/stretchr/testify/require"
)

func TestFailedAppearanceSaveExplainsTheCause(t *testing.T) {
	for _, locale := range []string{"zh", "en"} {
		t.Run(locale, func(t *testing.T) {
			t.Chdir(t.TempDir())
			flags := define.AppFlags
			t.Cleanup(func() { define.AppFlags = flags })
			define.AppFlags.DisableLoginMode = true
			// Shared YAML nodes cannot be edited safely from the settings page.
			raw := []byte("Locale: " + locale + "\nTitle: &shared original\nFooter: *shared\n")
			require.NoError(t, os.WriteFile("config.yml", raw, 0600))
			app := fiber.New()
			app.Post("/appearance", updateAppearanceOptions)
			req := httptest.NewRequest(http.MethodPost, "/appearance", strings.NewReader("title=Changed"))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			res, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
			require.NoError(t, err)
			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			require.NoError(t, err)
			require.Equal(t, http.StatusConflict, res.StatusCode)
			require.Equal(t, i18n.T(locale, "settings_save_failed")+" "+i18n.T(locale, "settings_save_failed_yaml_aliases"), string(body))
			require.NotContains(t, string(body), "aliases/merge keys cannot be safely edited", "internal error text belongs in the log")
			after, err := os.ReadFile("config.yml")
			require.NoError(t, err)
			require.Equal(t, raw, after)
		})
	}
}
