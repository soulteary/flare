package weather

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/i18n"
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
	if got, want := view.values["TestResult"], i18n.T("zh", "settings_save_failed_config_unreadable"); got != want {
		t.Fatalf("cause not explained: %q", got)
	}
	if _, readErr := data.GetAllSettingsOptions(); readErr == nil {
		t.Fatal("directory error was hidden by cached configuration")
	}
	if removeErr := os.Remove("config.yml"); removeErr != nil {
		t.Fatal(removeErr)
	}
	if renameErr := os.Rename("config-original.yml", "config.yml"); renameErr != nil {
		t.Fatal(renameErr)
	}
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		t.Fatal(err)
	}
	if options.Location != "Saved city" {
		t.Fatal("failed save changed original configuration")
	}
}

func TestWeatherSaveFailureExplainsTheCause(t *testing.T) {
	t.Chdir(t.TempDir())
	flags := define.AppFlags
	defer func() { define.AppFlags = flags }()
	define.AppFlags.DisableLoginMode = true
	// Shared YAML nodes cannot be edited safely from the settings page.
	raw := "Locale: en\nShowWeather: true\nLocation: &city Saved city\nTitle: *city\n"
	if err := os.WriteFile("config.yml", []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	view := &capturedWeatherView{}
	app := fiber.New(fiber.Config{Views: view})
	app.Post("/weather", updateWeatherOptions)
	request := httptest.NewRequest(http.MethodPost, "/weather", strings.NewReader("location=Saved+city&show=0"))
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := app.Test(request, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("failure reported %d", response.StatusCode)
	}
	if view.values["SaveFailed"] != true || view.values["Location"] != "Saved city" || view.values["ShowWeather"] != false {
		t.Fatalf("submitted input lost: %+v", view.values)
	}
	if got, want := view.values["TestResult"], i18n.T("en", "settings_save_failed_yaml_aliases"); got != want {
		t.Fatalf("cause not explained: %q", got)
	}
	if after, readErr := os.ReadFile("config.yml"); readErr != nil || string(after) != raw {
		t.Fatalf("failed save changed the configuration: %q %v", after, readErr)
	}
}
