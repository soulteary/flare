package weather

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
)

type capturedWeatherView struct{ values map[string]any }

func (*capturedWeatherView) Load() error { return nil }
func (view *capturedWeatherView) Render(out io.Writer, _ string, values any, _ ...string) error {
	view.values = map[string]any{}
	for k, v := range values.(map[string]any) {
		view.values[k] = v
	}
	_, err := io.WriteString(out, "weather")
	return err
}

func TestWeatherSaveFailurePreservesInputAndReportsError(t *testing.T) {
	t.Chdir(t.TempDir())
	flags := define.AppFlags
	defer func() { define.AppFlags = flags }()
	define.AppFlags.DisableLoginMode = true
	if !data.UpdateWeatherAndLocation(true, "Saved city") {
		t.Fatal("initialize config")
	}
	if _, err := data.GetAllSettingsOptions(); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename("config.yml", "config-original.yml"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("config.yml", 0700); err != nil {
		t.Fatal(err)
	}
	// Disk errors must not be hidden by stale cached configuration; preserve the submitted form.
	view := &capturedWeatherView{}
	app := fiber.New(fiber.Config{Views: view})
	app.Post("/weather", updateWeatherOptions)
	request := httptest.NewRequest(http.MethodPost, "/weather", strings.NewReader("location=Pending+city&show=1"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := app.Test(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusInternalServerError {
		t.Fatalf("failure reported %d", response.StatusCode)
	}
	if view.values["SaveFailed"] != true || view.values["Location"] != "Pending city" || view.values["ShowWeather"] != true {
		t.Fatalf("submitted input lost: %+v", view.values)
	}
	if _, err := data.GetAllSettingsOptions(); err == nil {
		t.Fatal("directory error was hidden by cached configuration")
	}
	if err := os.Remove("config.yml"); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename("config-original.yml", "config.yml"); err != nil {
		t.Fatal(err)
	}
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		t.Fatal(err)
	}
	if options.Location != "Saved city" {
		t.Fatal("failed save changed original configuration")
	}
}
