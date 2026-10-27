package others

import (
	"io"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/resources/templates"
	version "github.com/soulteary/version-kit/v4"
	"github.com/stretchr/testify/require"
)

func TestHideVersion(t *testing.T) {
	t.Chdir(t.TempDir())
	oldFlags := define.AppFlags
	oldVersion, oldCommit, oldDate := version.Version, version.Commit, version.BuildDate
	t.Cleanup(func() {
		define.AppFlags = oldFlags
		version.Version, version.Commit, version.BuildDate = oldVersion, oldCommit, oldDate
	})
	define.AppFlags.DisableLoginMode = true
	define.AppFlags.DebugMode = false
	version.Version, version.Commit, version.BuildDate = "version-secret-123", "commit-secret-456", "date-secret-789"
	define.Init()
	view, err := templates.NewRenderer()
	require.NoError(t, err)
	app := fiber.New(fiber.Config{Views: view})
	RegisterRouting(app)
	for _, hidden := range []bool{false, true, false} {
		require.True(t, data.UpdateAppearance(model.Application{HideVersion: hidden}))
		saved, err := data.GetAllSettingsOptions()
		require.NoError(t, err)
		require.Equal(t, hidden, saved.HideVersion)
		response, err := app.Test(httptest.NewRequest("GET", define.SettingPages.Others.Path, nil))
		require.NoError(t, err)
		require.Equal(t, 200, response.StatusCode)
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		require.NoError(t, err)
		for _, value := range []string{version.Version, version.Commit, version.BuildDate} {
			if hidden {
				require.NotContains(t, string(body), value)
			} else {
				require.Contains(t, string(body), value)
			}
		}
	}
}
