package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

type persistentBrowser struct {
	t       *testing.T
	app     *fiber.App
	cookies map[string]*http.Cookie
}

func (b *persistentBrowser) request(method, path string, values url.Values, headers map[string]string) *http.Response {
	var body io.Reader
	if values != nil {
		body = strings.NewReader(values.Encode())
	}
	req := httptest.NewRequest(method, "http://flare.test"+path, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, c := range b.cookies {
		req.AddCookie(c)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := b.app.Test(req, fiber.TestConfig{Timeout: 30 * time.Second})
	require.NoError(b.t, err)
	for _, c := range resp.Cookies() {
		if c.MaxAge < 0 {
			delete(b.cookies, c.Name)
		} else {
			b.cookies[c.Name] = c
		}
	}
	return resp
}
func (b *persistentBrowser) csrf() string {
	r := b.request("GET", "/login", nil, nil)
	require.Equal(b.t, 200, r.StatusCode)
	body, _ := io.ReadAll(r.Body)
	r.Body.Close()
	match := regexp.MustCompile(`name="csrf-token" value="([^"]+)"`).FindStringSubmatch(string(body))
	require.Len(b.t, match, 2)
	return match[1]
}
func persistentFixture(t *testing.T, mode string) (*security.Store, *persistentBrowser) {
	t.Helper()
	orig := define.AppFlags
	t.Cleanup(func() { define.AppFlags = orig })
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005
	define.AppFlags.CookieSecure = false
	store, err := security.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	require.NoError(t, store.Initialize("admin", "legacy short", mode))
	app := fiber.New()
	require.NoError(t, RequestHandle(app, store))
	app.Get("/protected", AuthRequired, func(c fiber.Ctx) error { return c.SendString(GetUserName(c)) })
	app.Post("/write", func(c fiber.Ctx) error {
		if !ValidateCSRF(c) {
			return c.SendStatus(403)
		}
		if !Revalidate(c) {
			return c.SendStatus(401)
		}
		return c.SendStatus(200)
	})
	return store, &persistentBrowser{t, app, make(map[string]*http.Cookie)}
}
func (b *persistentBrowser) login() {
	token := b.csrf()
	r := b.request("POST", "/login", url.Values{"username": {"admin"}, "password": {"legacy short"}, "csrf-token": {token}}, nil)
	require.Equal(b.t, 303, r.StatusCode)
}
func TestPersistentLoginRequiresCSRFAndSameOrigin(t *testing.T) {
	_, b := persistentFixture(t, "local")
	r := b.request("POST", "/login", url.Values{"username": {"admin"}, "password": {"legacy short"}}, nil)
	require.Equal(t, 403, r.StatusCode)
	require.Equal(t, 401, b.request("GET", "/protected", nil, nil).StatusCode)
	token := b.csrf()
	for _, origin := range []string{"null", "http://evil.test", "https://flare.test"} {
		r = b.request("POST", "/login", url.Values{"username": {"admin"}, "password": {"legacy short"}, "csrf-token": {token}}, map[string]string{"Origin": origin})
		require.Equal(t, 403, r.StatusCode)
	}
	r = b.request("POST", "/login", url.Values{"username": {"admin"}, "password": {"legacy short"}, "csrf-token": {token}, "return_to": {"//evil.test"}}, map[string]string{"Origin": "http://flare.test"})
	require.Equal(t, 303, r.StatusCode)
	require.Equal(t, "/account/cookie-check?return_to=%2F", r.Header.Get("Location"))
	check := b.request("GET", r.Header.Get("Location"), nil, nil)
	require.Equal(t, 303, check.StatusCode)
	require.Equal(t, "/", check.Header.Get("Location"))
	require.Equal(t, 200, b.request("GET", "/protected", nil, nil).StatusCode)
	for _, cookie := range r.Cookies() {
		require.True(t, cookie.HttpOnly)
		require.Equal(t, "/", cookie.Path)
		require.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
		require.Empty(t, cookie.Domain)
		require.Contains(t, cookie.Name, "flare_5005_")
	}
}
func TestPersistentCurrentAndAllLogout(t *testing.T) {
	_, b := persistentFixture(t, "local")
	b.login()
	other := &persistentBrowser{t, b.app, make(map[string]*http.Cookie)}
	other.login()
	old := &persistentBrowser{t, b.app, make(map[string]*http.Cookie)}
	for k, v := range b.cookies {
		old.cookies[k] = v
	}
	token := b.csrf()
	r := b.request("POST", "/logout", url.Values{"csrf-token": {token}}, nil)
	require.Equal(t, 303, r.StatusCode)
	require.Equal(t, 401, old.request("GET", "/protected", nil, nil).StatusCode)
	require.Equal(t, 200, other.request("GET", "/protected", nil, nil).StatusCode)
	b.login()
	token = other.csrf()
	require.Equal(t, 303, other.request("POST", "/logout-all", url.Values{"csrf-token": {token}}, nil).StatusCode)
	require.Equal(t, 401, b.request("GET", "/protected", nil, nil).StatusCode)
}
func TestPersistentPasswordChangeRevokesOtherSessions(t *testing.T) {
	store, b := persistentFixture(t, "local")
	b.login()
	other := &persistentBrowser{t, b.app, make(map[string]*http.Cookie)}
	other.login()
	token := b.csrf()
	r := b.request("POST", "/account/password", url.Values{"csrf-token": {token}, "password": {"legacy short"}, "new_password": {"A long unique new phrase 42!"}, "confirm_password": {"A long unique new phrase 42!"}}, nil)
	require.Equal(t, 303, r.StatusCode)
	require.NoError(t, store.Verify("admin", "A long unique new phrase 42!"))
	require.Error(t, store.Verify("admin", "legacy short"))
	require.Equal(t, 200, b.request("GET", "/protected", nil, nil).StatusCode)
	require.Equal(t, 401, other.request("GET", "/protected", nil, nil).StatusCode)
}
func TestPersistentDeploymentTakeLocalRequiresConfirmation(t *testing.T) {
	store, b := persistentFixture(t, "deployment")
	b.login()
	token := b.csrf()
	require.Equal(t, 400, b.request("POST", "/account/local", url.Values{"csrf-token": {token}, "password": {"legacy short"}}, nil).StatusCode)
	require.Equal(t, "deployment", store.Info().Mode)
	require.Equal(t, 303, b.request("POST", "/account/local", url.Values{"csrf-token": {token}, "password": {"legacy short"}, "confirm": {"local"}}, nil).StatusCode)
	require.Equal(t, "local", store.Info().Mode)
}
func TestPersistentReauthRotatesSessionWithoutExtendingDeadline(t *testing.T) {
	store, b := persistentFixture(t, "local")
	b.login()
	var original string
	for k, v := range b.cookies {
		if !strings.HasSuffix(k, "_preauth") {
			original = v.Value
		}
	}
	before, err := store.Session(original, false)
	require.NoError(t, err)
	token := b.csrf()
	require.Equal(t, 303, b.request("POST", "/account/reauthenticate", url.Values{"csrf-token": {token}, "password": {"legacy short"}}, nil).StatusCode)
	_, err = store.Session(original, false)
	require.Error(t, err)
	var current string
	for k, v := range b.cookies {
		if !strings.HasSuffix(k, "_preauth") {
			current = v.Value
		}
	}
	after, err := store.Session(current, false)
	require.NoError(t, err)
	require.Equal(t, before.ExpiresAt, after.ExpiresAt)
}
func TestPersistentLoginBodyLimitAndFooterIsolation(t *testing.T) {
	_, b := persistentFixture(t, "local")
	token := b.csrf()
	r := b.request("POST", "/login", url.Values{"username": {"admin"}, "password": {strings.Repeat("x", 9000)}, "csrf-token": {token}}, nil)
	require.Equal(t, 413, r.StatusCode)
	r = b.request("GET", define.SettingPages.Others.Path, nil, nil)
	body, _ := io.ReadAll(r.Body)
	require.NotContains(t, string(body), "COMMIT")
	require.Equal(t, "private, no-store", r.Header.Get("Cache-Control"))
	require.NotContains(t, string(body), "legacy short")
}

func TestPersistentSessionsSurviveServerRestart(t *testing.T) {
	store, b := persistentFixture(t, "local")
	b.login()
	app := fiber.New()
	require.NoError(t, RequestHandle(app, store))
	app.Get("/protected", AuthRequired, func(c fiber.Ctx) error { return c.SendString("ok") })
	b.app = app
	require.Equal(t, 200, b.request("GET", "/protected", nil, nil).StatusCode)
	require.NoError(t, store.RevokeAll())
	require.Equal(t, 401, b.request("GET", "/protected", nil, nil).StatusCode)
}
func TestPersistentLoginRateLimitedByDirectPeer(t *testing.T) {
	_, b := persistentFixture(t, "local")
	token := b.csrf()
	for i := 0; i < 10; i++ {
		r := b.request("POST", "/login", url.Values{"csrf-token": {token}, "username": {"unknown"}, "password": {"wrong"}}, map[string]string{"X-Forwarded-For": "1.2.3.4"})
		require.Equal(t, 401, r.StatusCode)
	}
	r := b.request("POST", "/login", url.Values{"csrf-token": {token}, "username": {"admin"}, "password": {"legacy short"}}, map[string]string{"X-Forwarded-For": "5.6.7.8"})
	require.Equal(t, 429, r.StatusCode)
	require.Contains(t, []string{"30", "31"}, r.Header.Get("Retry-After"))
}
func TestPersistentCSRFTokenBoundToSession(t *testing.T) {
	_, b := persistentFixture(t, "local")
	b.login()
	other := &persistentBrowser{t, b.app, make(map[string]*http.Cookie)}
	other.login()
	token := b.csrf()
	require.Equal(t, 403, other.request("POST", "/logout-all", url.Values{"csrf-token": {token}}, nil).StatusCode)
	require.Equal(t, 200, b.request("GET", "/protected", nil, nil).StatusCode)
	require.Equal(t, 200, other.request("GET", "/protected", nil, nil).StatusCode)
}
func TestPersistentTrustedModeUsesPreauthWithoutAccount(t *testing.T) {
	_, b := persistentFixture(t, "local")
	define.AppFlags.DisableLoginMode = true
	// Rebuild routes in trusted mode. Content remains writable, account routes do
	// not acquire administrator identity from the anonymous management mode.
	app := fiber.New()
	a := persistentAuthFixtureStore(t)
	require.NoError(t, RequestHandle(app, a))
	app.Get("/form", func(c fiber.Ctx) error {
		token := CSRFToken(c)
		binding := SessionBinding(c)
		require.NotEmpty(t, binding)
		require.Equal(t, token, CSRFToken(c))
		return c.SendString(token)
	})
	app.Post("/write", func(c fiber.Ctx) error {
		if !ValidateCSRF(c) {
			return c.SendStatus(403)
		}
		require.Empty(t, GetUserName(c))
		return c.SendStatus(200)
	})
	b.app = app
	b.cookies = make(map[string]*http.Cookie)
	r := b.request("GET", "/form", nil, nil)
	token, _ := io.ReadAll(r.Body)
	require.Equal(t, 403, b.request("POST", "/write", nil, nil).StatusCode)
	require.Equal(t, 200, b.request("POST", "/write", url.Values{"csrf-token": {string(token)}}, nil).StatusCode)
	require.Equal(t, 404, b.request("POST", "/account/password", url.Values{"csrf-token": {string(token)}}, nil).StatusCode)
}
func persistentAuthFixtureStore(t *testing.T) *security.Store {
	t.Helper()
	s, e := security.Open(t.TempDir())
	require.NoError(t, e)
	t.Cleanup(func() { s.Close() })
	require.NoError(t, s.Initialize("admin", "legacy short", "local"))
	return s
}

func TestSafeReturnOnlyNavigationPages(t *testing.T) {
	for _, unsafe := range []string{"https://evil.test", "//evil.test", "/\\evil.test", "/%2fevil.test", "/%252fevil.test", "/settings?token=secret", "/login", "/logout", "/account/password", "/redir/url", "/\x00", "/\n"} {
		require.Equal(t, "/", safeReturn(unsafe), unsafe)
	}
	for _, safe := range []string{"/", "/editor", "/settings", "/settings/search"} {
		require.Equal(t, safe, safeReturn(safe))
	}
}
func TestLoginFooterIsolatedAllowlist(t *testing.T) {
	result := loginFooter(`<p>Contact <a href="https://support.test/?a=1&amp;b=2" onclick="evil()">Support</a><img src="https://track.test"><script>leak()</script><a href="javascript:alert(1)">unsafe</a></p>`)
	require.Contains(t, result, `href="https://support.test/?a=1&amp;b=2"`)
	require.Contains(t, result, "Support")
	for _, unsafe := range []string{"onclick", "track.test", "javascript:", "leak()", "<img", "<script"} {
		require.NotContains(t, result, unsafe)
	}
}

func TestPersistentLoginEmptyFieldsAndCookieHandshake(t *testing.T) {
	_, b := persistentFixture(t, "local")
	token := b.csrf()
	for _, values := range []url.Values{{"username": {""}, "password": {"legacy short"}, "csrf-token": {token}}, {"username": {"admin"}, "password": {""}, "csrf-token": {token}}} {
		require.Equal(t, 400, b.request("POST", "/login", values, nil).StatusCode)
	}
	b.login()
	check := b.request("GET", "/account/cookie-check?return_to=/editor", nil, nil)
	require.Equal(t, 303, check.StatusCode)
	require.Equal(t, "/editor", check.Header.Get("Location"))
	noCookies := &persistentBrowser{t, b.app, make(map[string]*http.Cookie)}
	check = noCookies.request("GET", "/account/cookie-check?return_to=/editor", nil, nil)
	require.Equal(t, 503, check.StatusCode)
	require.Empty(t, check.Header.Get("Location"))
	body, _ := io.ReadAll(check.Body)
	require.Contains(t, string(body), "COOKIE_NOT_ESTABLISHED")
}
func TestPersistentDiagnosticsOnlyAuthenticated(t *testing.T) {
	store, b := persistentFixture(t, "local")
	r := b.request("GET", "/login", nil, nil)
	body, _ := io.ReadAll(r.Body)
	require.NotContains(t, string(body), store.Info().InstanceID)
	require.NotContains(t, string(body), "Diagnostics")
	b.login()
	r = b.request("GET", define.SettingPages.Others.Path, nil, nil)
	body, _ = io.ReadAll(r.Body)
	require.Contains(t, string(body), "Diagnostics")
	require.Contains(t, string(body), store.Info().InstanceID)
	require.Contains(t, string(body), "SameSite=Lax")
	require.NotContains(t, string(body), "legacy short")
}

func TestPersistentFormsKeepSameOriginReferrerPolicy(t *testing.T) {
	_, browser := persistentFixture(t, "local")
	response := browser.request("GET", "/login", nil, nil)
	require.Equal(t, "same-origin", response.Header.Get("Referrer-Policy"))
	response.Body.Close()
	// Opaque origins remain rejected; fixing native form policy must not weaken
	// the separate Origin and CSRF checks.
	token := browser.csrf()
	response = browser.request("POST", "/login", url.Values{"username": {"admin"}, "password": {"legacy short"}, "csrf-token": {token}}, map[string]string{"Origin": "null"})
	require.Equal(t, 403, response.StatusCode)
	response.Body.Close()
}
