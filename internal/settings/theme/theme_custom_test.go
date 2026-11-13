package theme

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/valyala/fasthttp"
)

const defaultThemeConfig = "Theme: blackboard\n"

// isolateTheme runs a test in its own data directory with the default theme
// and no stored palette, and leaves that state behind for the next test.
func isolateTheme(t *testing.T) *fiber.App {
	t.Helper()
	t.Chdir(t.TempDir())
	flags := define.AppFlags
	reset := func() {
		_ = os.RemoveAll("config.yml")
		if err := os.WriteFile("config.yml", []byte(defaultThemeConfig), 0600); err != nil {
			t.Fatal(err)
		}
		define.UpdatePagePalettes()
	}
	// Registered after t.Chdir, so it runs while the test directory is current.
	t.Cleanup(func() {
		reset()
		define.AppFlags = flags
	})
	define.AppFlags.DisableLoginMode = true
	reset()
	app := fiber.New(fiber.Config{Immutable: true, Views: themeTestViews{}})
	RegisterRouting(app)
	return app
}

func postTheme(t *testing.T, app *fiber.App, form url.Values) (int, string) {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, define.SettingPages.Theme.Path, strings.NewReader(form.Encode()))
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
	return res.StatusCode, string(body)
}

func customForm(background, primary, accent string) url.Values {
	return url.Values{"action": {actionSaveCustom}, "custom-background": {background}, "custom-primary": {primary}, "custom-accent": {accent}}
}

func TestCustomPaletteIsSavedAndApplied(t *testing.T) {
	app := isolateTheme(t)
	want := model.Palette{Background: "#10241b", Primary: "#e8f1e4", Accent: "#f2b84b"}
	if status, body := postTheme(t, app, customForm(" #10241B", "#E8F1E4 ", "#f2b84b")); status != http.StatusOK {
		t.Fatalf("save: %d %s", status, body)
	}
	name, primary := define.ThemeSnapshot()
	if name != model.ThemeCustomName || primary != want.Primary {
		t.Fatalf("active theme %q %q", name, primary)
	}
	if theme, stored := data.GetThemeSelection(); theme != model.ThemeCustomName || stored == nil || *stored != want {
		t.Fatalf("stored selection %q %+v", theme, stored)
	}

	// A built-in theme keeps the palette, and the custom card returns to it.
	if status, body := postTheme(t, app, url.Values{"theme": {"cloud"}}); status != http.StatusOK {
		t.Fatalf("built-in theme: %d %s", status, body)
	}
	if theme, stored := data.GetThemeSelection(); theme != "cloud" || stored == nil || *stored != want {
		t.Fatalf("stored selection after a built-in theme %q %+v", theme, stored)
	}
	if status, body := postTheme(t, app, url.Values{"theme": {model.ThemeCustomName}}); status != http.StatusOK {
		t.Fatalf("custom theme: %d %s", status, body)
	}
	if name, primary = define.ThemeSnapshot(); name != model.ThemeCustomName || primary != want.Primary {
		t.Fatalf("active theme after returning %q %q", name, primary)
	}
	if define.CurrentThemePalette() != want {
		t.Fatalf("palette in use %+v", define.CurrentThemePalette())
	}
}

// The colors outlive the request that submitted them: they are written into
// later pages and icons. They must not alias request storage that the server
// reuses for the next request.
func TestCustomPaletteSurvivesFormBufferReuse(t *testing.T) {
	app := isolateTheme(t)
	app.Post("/reuse", func(c fiber.Ctx) error {
		var body struct {
			Primary string `form:"custom-primary"`
		}
		if err := c.Bind().Body(&body); err != nil {
			return err
		}
		return c.SendStatus(http.StatusOK)
	})
	handler := app.Handler()
	var ctx fasthttp.RequestCtx
	send := func(path, background, primary, accent string) {
		t.Helper()
		ctx.Request.Reset()
		ctx.Response.Reset()
		ctx.Request.Header.SetMethod(http.MethodPost)
		ctx.Request.SetRequestURI(path)
		ctx.Request.Header.SetContentType("application/x-www-form-urlencoded")
		ctx.Request.SetBodyString(customForm(background, primary, accent).Encode())
		handler(&ctx)
		if ctx.Response.StatusCode() != http.StatusOK {
			t.Fatalf("request %s: %d %s", path, ctx.Response.StatusCode(), ctx.Response.Body())
		}
	}

	want := model.Palette{Background: "#10241b", Primary: "#e8f1e4", Accent: "#f2b84b"}
	send(define.SettingPages.Theme.Path, want.Background, want.Primary, want.Accent)
	borrowed := ctx.Request.PostArgs().Peek("custom-primary")
	firstStorage := &borrowed[0]
	send("/reuse", "#ffffff", "#000000", "#777777")
	if second := ctx.Request.PostArgs().Peek("custom-primary"); &second[0] != firstStorage || string(borrowed) != "#000000" {
		t.Fatal("test did not overwrite the first request's form buffer")
	}
	if got := define.CurrentThemePalette(); got != want {
		t.Fatalf("retained palette changed after form buffer reuse: %+v", got)
	}
	if _, primary := define.ThemeSnapshot(); primary != want.Primary {
		t.Fatalf("retained icon color changed after form buffer reuse: %q", primary)
	}
}

func TestRejectedCustomPaletteChangesNothing(t *testing.T) {
	app := isolateTheme(t)
	style := define.GetAppBodyStyle()
	for name, form := range map[string]url.Values{
		"custom theme before a palette is stored": {"theme": {model.ThemeCustomName}},
		"color name":                 customForm("red", "#e8f1e4", "#f2b84b"),
		"short color":                customForm("#10241b", "#fff", "#f2b84b"),
		"missing color":              customForm("#10241b", "#e8f1e4", ""),
		"color carrying declaration": customForm("#000;background:url(//example.test/)", "#e8f1e4", "#f2b84b"),
		"unreadable primary color":   customForm("#202020", "#2a2a2a", "#f2b84b"),
	} {
		if status, body := postTheme(t, app, form); status != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, status, body)
		}
		current, _ := define.ThemeSnapshot()
		if current != "blackboard" || define.GetAppBodyStyle() != style || define.IsKnownTheme(model.ThemeCustomName) {
			t.Errorf("%s: theme state changed to %q %q", name, current, define.GetAppBodyStyle())
		}
		if raw, err := os.ReadFile("config.yml"); err != nil || string(raw) != defaultThemeConfig {
			t.Errorf("%s: config.yml changed: %q %v", name, raw, err)
		}
	}
}

func TestFailedCustomPaletteSavePreservesRuntimeState(t *testing.T) {
	app := isolateTheme(t)
	style := define.GetAppBodyStyle()
	if err := os.Remove("config.yml"); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir("config.yml", 0700); err != nil {
		t.Fatal(err)
	}
	status, _ := postTheme(t, app, customForm("#10241b", "#e8f1e4", "#f2b84b"))
	current, _ := define.ThemeSnapshot()
	if status != http.StatusInternalServerError || current != "blackboard" || define.GetAppBodyStyle() != style || define.IsKnownTheme(model.ThemeCustomName) {
		t.Fatalf("failed save status=%d current=%q", status, current)
	}
}
