package theme

import (
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/security"
)

// The values of the options are stored with the checks of every write of the
// theme page: the session is looked at again when they are stored. A session
// that is revoked after the request was let in stores nothing, and neither
// does it remove what is stored.
func TestThemeOptionsAreNotStoredForARevokedSession(t *testing.T) {
	trusted := isolateTheme(t)
	t.Cleanup(func() { _ = os.RemoveAll(define.UserThemeDirectory) })
	if err := os.Mkdir(define.UserThemeDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	writeThemeFile(t, "zelda.yml", themeFileColors+"options:\n  - id: density\n    type: enum\n    choices:\n      - value: normal\n      - value: compact\n  - id: frames\n    type: boolean\n")
	if status, body := postTheme(t, trusted, url.Values{"theme": {"zelda"}}); status != http.StatusOK {
		t.Fatalf("select: %d %s", status, body)
	}

	// The handler of the page behind sessions. Between the check of the route
	// and the handler is a step that can revoke every session, as a sign-out
	// on another device does while a request is on its way.
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005
	define.AppFlags.CookieSecure = false
	store, err := security.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	if err = store.Initialize("admin", "legacy short", "local"); err != nil {
		t.Fatal(err)
	}
	app := fiber.New(fiber.Config{Immutable: true, Views: themeTestViews{}})
	if err = auth.RequestHandle(app, store); err != nil {
		t.Fatal(err)
	}
	var revoke atomic.Bool
	app.Get("/token", auth.AuthRequired, func(c fiber.Ctx) error { return c.SendString(auth.CSRFToken(c)) })
	app.Post(define.SettingPages.Theme.Path, auth.AuthRequired, func(c fiber.Ctx) error {
		if revoke.Load() {
			if e := store.RevokeAll(); e != nil {
				return e
			}
		}
		return c.Next()
	}, updateThemes)

	send := func(req *http.Request) (int, string) {
		t.Helper()
		res, e := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		body, e := io.ReadAll(res.Body)
		if e != nil {
			t.Fatal(e)
		}
		return res.StatusCode, string(body)
	}
	// Every request has a session of its own, with the token of that session:
	// a session that was revoked is not let in a second time.
	submit := func(form url.Values, revoked bool) (int, string) {
		t.Helper()
		token, e := store.Authenticate("admin", "legacy short")
		if e != nil {
			t.Fatal(e)
		}
		session := &http.Cookie{Name: auth.RequestHandleSessionName(define.AppFlags.CookieName, define.AppFlags.Port) + "_" + store.Info().InstanceID, Value: token}
		req := httptest.NewRequest(http.MethodGet, "http://flare.test/token", nil)
		req.AddCookie(session)
		status, csrf := send(req)
		if status != http.StatusOK || csrf == "" {
			t.Fatalf("token of the session: %d %q", status, csrf)
		}
		form.Set("csrf-token", csrf)
		form.Set(optionsTheme, "zelda")
		req = httptest.NewRequest(http.MethodPost, "http://flare.test"+define.SettingPages.Theme.Path, strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.AddCookie(session)
		revoke.Store(revoked)
		defer revoke.Store(false)
		return send(req)
	}
	stored := func() map[string]string {
		t.Helper()
		options, e := data.GetAllSettingsOptions()
		if e != nil {
			t.Fatal(e)
		}
		return options.ThemeSettings["zelda"].Options
	}
	compact := map[string]string{"density": "compact"}

	if status, body := submit(url.Values{"action": {actionSaveOptions}, "option-density": {"compact"}}, false); status != http.StatusOK || !maps.Equal(stored(), compact) {
		t.Fatalf("with a session: %d %s, stored %v", status, body, stored())
	}
	for _, form := range []url.Values{
		{"action": {actionSaveOptions}, "option-density": {"normal"}, "option-frames": {"on"}},
		{"action": {actionResetOptions}},
	} {
		action := form.Get("action")
		if status, body := submit(form, true); status != http.StatusUnauthorized || body != "AUTH_REQUIRED" {
			t.Errorf("%s with a revoked session: %d %s", action, status, body)
		}
		if !maps.Equal(stored(), compact) {
			t.Errorf("%s with a revoked session left %v stored", action, stored())
		}
	}
	// The same request is carried out for a session that holds.
	if status, body := submit(url.Values{"action": {actionResetOptions}}, false); status != http.StatusOK || len(stored()) != 0 {
		t.Fatalf("reset with a session: %d %s, stored %v", status, body, stored())
	}
}

// A browser submits a color in lower case, whatever the case of the theme
// file. A color option that is left alone is therefore not stored as a value
// of its own, and keeps following the theme.
func TestThemeColorOptionOfAnotherCaseIsNotStored(t *testing.T) {
	app := isolateTheme(t)
	t.Cleanup(func() { _ = os.RemoveAll(define.UserThemeDirectory) })
	if err := os.Mkdir(define.UserThemeDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	writeThemeFile(t, "zelda.yml", "contract: 1\ntokens:\n  background: \"#10241B\"\n  primary: \"#E8F1E4\"\n  accent: \"#F2B84B\"\n"+
		"options:\n  - id: highlight\n    type: color\n    token: accent\n  - id: tint\n    type: color\n    token: muted\n    default: \"#A0A0A0\"\n")
	if status, body := postTheme(t, app, url.Values{"theme": {"zelda"}}); status != http.StatusOK {
		t.Fatalf("select: %d %s", status, body)
	}
	for _, tc := range []struct {
		name            string
		highlight, tint string
		want            map[string]string
	}{
		{"as a browser submits the form nothing was changed in", "#f2b84b", "#a0a0a0", nil},
		{"as the theme file has the colors", "#F2B84B", "#A0A0A0", nil},
		{"one color picked", "#00ff88", "#a0a0a0", map[string]string{"highlight": "#00ff88"}},
		{"the other color picked", "#f2b84b", "#A0A0A1", map[string]string{"tint": "#A0A0A1"}},
		{"put back", "#f2b84b", "#a0a0a0", nil},
	} {
		form := url.Values{"action": {actionSaveOptions}, optionsTheme: {"zelda"}, "option-highlight": {tc.highlight}, "option-tint": {tc.tint}}
		if status, body := postTheme(t, app, form); status != http.StatusOK {
			t.Fatalf("%s: %d %s", tc.name, status, body)
		}
		options, err := data.GetAllSettingsOptions()
		if err != nil {
			t.Fatal(err)
		}
		if stored := options.ThemeSettings["zelda"].Options; !maps.Equal(stored, tc.want) {
			t.Errorf("%s: stored %v, want %v", tc.name, stored, tc.want)
		}
	}
}
