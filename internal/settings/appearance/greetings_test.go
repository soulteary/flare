package appearance

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/stretchr/testify/require"
)

type appearanceTestViews struct{}

func (appearanceTestViews) Load() error { return nil }
func (appearanceTestViews) Render(out io.Writer, _ string, _ any, _ ...string) error {
	_, err := io.WriteString(out, "appearance")
	return err
}

// The settings page renders the greetings field only while the title is shown.
// Saving the page with the title hidden must not erase the stored greetings.
func TestGreetingsSurviveSavesWithoutTheField(t *testing.T) {
	multipartForm := func(fields map[string]string) (string, string) {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		for name, value := range fields {
			require.NoError(t, writer.WriteField(name, value))
		}
		require.NoError(t, writer.Close())
		return body.String(), writer.FormDataContentType()
	}
	hiddenMultipart, hiddenType := multipartForm(map[string]string{"title": "Site", "show-title": "0"})
	clearedMultipart, clearedType := multipartForm(map[string]string{"title": "Site", "show-title": "1", "greetings": ""})
	for _, tc := range []struct {
		name, contentType, body, want string
		wantTitleShown                bool
	}{
		{"title hidden, field absent", fiber.MIMEApplicationForm, "title=Site&show-title=0", "stored greeting", false},
		{"older form without the field", fiber.MIMEApplicationForm, "title=Site&show-title=1", "stored greeting", true},
		{"field cleared", fiber.MIMEApplicationForm, "title=Site&show-title=1&greetings=", "", true},
		{"field changed", fiber.MIMEApplicationForm, "title=Site&show-title=1&greetings=new+greeting", "new greeting", true},
		{"multipart, field absent", hiddenType, hiddenMultipart, "stored greeting", false},
		{"multipart, field cleared", clearedType, clearedMultipart, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			flags := define.AppFlags
			t.Cleanup(func() { define.AppFlags = flags })
			define.AppFlags.DisableLoginMode = true
			require.NoError(t, os.WriteFile("config.yml", []byte("Title: Site\nShowTitle: true\nGreetings: stored greeting\n"), 0600))
			app := fiber.New(fiber.Config{Views: appearanceTestViews{}})
			app.Post("/appearance", updateAppearanceOptions)
			req := httptest.NewRequest(http.MethodPost, "/appearance", strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			res, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
			require.NoError(t, err)
			require.NoError(t, res.Body.Close())
			require.Equal(t, http.StatusOK, res.StatusCode)
			options, err := data.GetAllSettingsOptions()
			require.NoError(t, err)
			require.Equal(t, tc.want, options.Greetings)
			require.Equal(t, tc.wantTitleShown, options.ShowTitle)
		})
	}
}

func TestGreetingsLookupFailureDoesNotSave(t *testing.T) {
	t.Chdir(t.TempDir())
	flags := define.AppFlags
	t.Cleanup(func() { define.AppFlags = flags })
	define.AppFlags.DisableLoginMode = true
	raw := []byte("Title: [invalid\n")
	require.NoError(t, os.WriteFile("config.yml", raw, 0600))
	app := fiber.New(fiber.Config{Views: appearanceTestViews{}})
	app.Post("/appearance", updateAppearanceOptions)
	req := httptest.NewRequest(http.MethodPost, "/appearance", strings.NewReader("title=Site&show-title=0"))
	req.Header.Set("Content-Type", fiber.MIMEApplicationForm)
	res, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	require.NoError(t, err)
	require.NoError(t, res.Body.Close())
	require.Equal(t, http.StatusInternalServerError, res.StatusCode)
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, raw, after)
}
