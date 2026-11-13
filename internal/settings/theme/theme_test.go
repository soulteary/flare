package theme

import (
	"github.com/soulteary/flare/config/data"
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
	"github.com/valyala/fasthttp"
)

type themeTestViews struct{}

func (themeTestViews) Load() error { return nil }
func (themeTestViews) Render(out io.Writer, _ string, _ any, _ ...string) error {
	_, err := io.WriteString(out, "theme")
	return err
}

func TestThemeSurvivesFormBufferReuse(t *testing.T) {
	t.Chdir(t.TempDir())
	flags := define.AppFlags
	current, primary := define.ThemeCurrent, define.ThemePrimaryColor
	cachedPrimary := define.CACHE_APP_CURRENT_THEME_PRIMARY_COLOR
	defer func() {
		define.AppFlags = flags
		define.ThemeCurrent, define.ThemePrimaryColor = current, primary
		define.CACHE_APP_CURRENT_THEME_PRIMARY_COLOR = cachedPrimary
	}()
	define.AppFlags.DisableLoginMode = true

	app := fiber.New(fiber.Config{Immutable: true, Views: themeTestViews{}})
	RegisterRouting(app)
	app.Post("/reuse", func(c fiber.Ctx) error {
		var body struct {
			Theme string `form:"theme"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return err
		}
		return c.SendStatus(http.StatusOK)
	})
	handler := app.Handler()
	var ctx fasthttp.RequestCtx
	send := func(path, value string) {
		t.Helper()
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(http.MethodPost)
		ctx.Request.SetRequestURI(path)
		ctx.Request.Header.SetContentType("application/x-www-form-urlencoded")
		ctx.Request.SetBodyString("theme=" + value)
		handler(&ctx)
		if ctx.Response.StatusCode() != http.StatusOK {
			t.Fatalf("request %s: %d %s", path, ctx.Response.StatusCode(), ctx.Response.Body())
		}
	}

	send(define.SettingPages.Theme.Path, "blackboard")
	borrowed := ctx.Request.PostArgs().Peek("theme")
	firstStorage := &borrowed[0]
	send("/reuse", "abcdefghij")
	if second := ctx.Request.PostArgs().Peek("theme"); &second[0] != firstStorage || string(borrowed) != "abcdefghij" {
		t.Fatal("test did not overwrite the first request's form buffer")
	}
	if define.ThemeCurrent != "blackboard" {
		t.Fatalf("retained theme changed after form buffer reuse: %q", define.ThemeCurrent)
	}
}

func TestFailedThemeSavePreservesRuntimeState(t *testing.T) {
	t.Chdir(t.TempDir())
	flags := define.AppFlags
	defer func() { define.AppFlags = flags }()
	define.AppFlags.DisableLoginMode = true
	if !data.UpdateThemeName("blackboard") {
		t.Fatal("prepare settings")
	}
	define.SetTheme("blackboard")
	originalStyle := define.GetAppBodyStyle()
	if err := os.Remove("config.yml"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("config.yml", 0700); err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{Views: themeTestViews{}})
	RegisterRouting(app)
	req := httptest.NewRequest("POST", define.SettingPages.Theme.Path, strings.NewReader("theme=cloud"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	current, _ := define.ThemeSnapshot()
	if res.StatusCode != 500 || current != "blackboard" || define.GetAppBodyStyle() != originalStyle {
		t.Fatalf("failed save status=%d current=%q", res.StatusCode, current)
	}
}

func TestFailedThemeSaveExplainsTheCause(t *testing.T) {
	t.Chdir(t.TempDir())
	flags := define.AppFlags
	defer func() { define.AppFlags = flags }()
	define.AppFlags.DisableLoginMode = true
	// Shared YAML nodes cannot be edited safely from the settings page.
	raw := "Theme: &shared blackboard\nOtherData: *shared\n"
	if err := os.WriteFile("config.yml", []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	define.SetTheme("blackboard")
	app := fiber.New(fiber.Config{Views: themeTestViews{}})
	RegisterRouting(app)
	req := httptest.NewRequest("POST", define.SettingPages.Theme.Path, strings.NewReader("theme=cloud"))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	want := i18n.T("zh", "settings_save_failed") + " " + i18n.T("zh", "settings_save_failed_yaml_aliases")
	if res.StatusCode != http.StatusConflict || string(body) != want {
		t.Fatalf("status=%d body=%q", res.StatusCode, body)
	}
	if current, _ := define.ThemeSnapshot(); current != "blackboard" {
		t.Fatalf("failed save changed the active theme to %q", current)
	}
	if after, readErr := os.ReadFile("config.yml"); readErr != nil || string(after) != raw {
		t.Fatalf("failed save changed the configuration: %q %v", after, readErr)
	}
}
