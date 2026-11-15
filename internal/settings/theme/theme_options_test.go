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

// revocableTheme is the handler of the theme page behind sessions. Between
// the check of the route and the handler is a step that can revoke every
// session, as a sign-out on another device does while a request is on its
// way.
type revocableTheme struct {
	t      *testing.T
	app    *fiber.App
	store  *security.Store
	revoke atomic.Bool
}

// newRevocableTheme puts the handler behind sessions. It follows isolateTheme,
// whose cleanup gives the flags back.
func newRevocableTheme(t *testing.T) *revocableTheme {
	t.Helper()
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
	guarded := &revocableTheme{t: t, store: store, app: fiber.New(fiber.Config{Immutable: true, Views: themeTestViews{}})}
	if err = auth.RequestHandle(guarded.app, store); err != nil {
		t.Fatal(err)
	}
	guarded.app.Get("/token", auth.AuthRequired, func(c fiber.Ctx) error { return c.SendString(auth.CSRFToken(c)) })
	guarded.app.Post(define.SettingPages.Theme.Path, auth.AuthRequired, func(c fiber.Ctx) error {
		if guarded.revoke.Load() {
			if e := store.RevokeAll(); e != nil {
				return e
			}
		}
		return c.Next()
	}, updateThemes)
	return guarded
}

func (guarded *revocableTheme) send(req *http.Request) (int, string) {
	guarded.t.Helper()
	res, err := guarded.app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	if err != nil {
		guarded.t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		guarded.t.Fatal(err)
	}
	return res.StatusCode, string(body)
}

// post submits a form to the theme page. Every request has a session of its
// own, with the token of that session, which body is given to put into the
// form: a session that was revoked is not let in a second time. With revoked
// set, every session is revoked after the request was let in.
func (guarded *revocableTheme) post(body func(csrf string) (io.Reader, string), revoked bool) (int, string) {
	guarded.t.Helper()
	token, err := guarded.store.Authenticate("admin", "legacy short")
	if err != nil {
		guarded.t.Fatal(err)
	}
	session := &http.Cookie{Name: auth.RequestHandleSessionName(define.AppFlags.CookieName, define.AppFlags.Port) + "_" + guarded.store.Info().InstanceID, Value: token}
	req := httptest.NewRequest(http.MethodGet, "http://flare.test/token", nil)
	req.AddCookie(session)
	status, csrf := guarded.send(req)
	if status != http.StatusOK || csrf == "" {
		guarded.t.Fatalf("token of the session: %d %q", status, csrf)
	}
	content, kind := body(csrf)
	req = httptest.NewRequest(http.MethodPost, "http://flare.test"+define.SettingPages.Theme.Path, content)
	req.Header.Set("Content-Type", kind)
	req.AddCookie(session)
	guarded.revoke.Store(revoked)
	defer guarded.revoke.Store(false)
	return guarded.send(req)
}

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

	guarded := newRevocableTheme(t)
	submit := func(form url.Values, revoked bool) (int, string) {
		t.Helper()
		return guarded.post(func(csrf string) (io.Reader, string) {
			form.Set("csrf-token", csrf)
			form.Set(optionsTheme, "zelda")
			return strings.NewReader(form.Encode()), "application/x-www-form-urlencoded"
		}, revoked)
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
