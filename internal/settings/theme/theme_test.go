package theme

import (
	"io"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
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
