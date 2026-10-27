package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	version "github.com/soulteary/version-kit/v4"
	"html"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/formvalue"
	"github.com/soulteary/flare/internal/logger"
	"github.com/soulteary/flare/internal/requestsource"
	"github.com/soulteary/flare/internal/security"
)

type persistentKey struct{}
type preauthKey struct{}
type requestIdentityKey struct{}

// requestIdentity is confined to one Fiber request. The proof has a durable
// authorization point; every reuse still checks the live store for revocation,
// failure and expiry. Neither the Cookie token nor the proof is shared between
// requests or stored on persistentAuth.
type requestIdentity struct {
	owner    *persistentAuth
	token    string
	username string
	proof    *security.SessionProof
}
type persistentAuth struct {
	store   *security.Store
	name    string
	csrfKey [32]byte
	mu      sync.Mutex
	buckets map[string]loginBucket
	source  *requestsource.Policy
}
type loginBucket struct {
	failures []time.Time
	last     time.Time
	until    time.Time
	strikes  int
}

// RequestHandleWithSecurity installs server-side sessions. A nil store is a
// configuration error rather than permission to fall back to signed cookies.
func RequestHandleWithSecurity(app *fiber.App, store *security.Store) error {
	if store == nil {
		return errors.New("persistent security store is required")
	}
	source, err := requestsource.New(define.AppFlags.ExternalURL, define.AppFlags.AllowedHosts, define.AppFlags.TrustedProxies, define.AppFlags.CookieSecure)
	if err != nil {
		return err
	}
	info := store.Info()
	a := &persistentAuth{store: store, name: RequestHandleSessionName(define.AppFlags.CookieName, define.AppFlags.Port) + "_" + info.InstanceID, buckets: make(map[string]loginBucket), source: source}
	if _, err := rand.Read(a.csrfKey[:]); err != nil {
		return err
	}
	app.Use(func(c fiber.Ctx) error {
		c.Locals(persistentKey{}, a)
		c.Set("Cache-Control", "private, no-store")
		// Native same-origin form POSTs must retain a non-opaque Origin.
		// no-referrer makes Chromium/WebKit send Origin:null even on local forms.
		// same-origin still suppresses Referer when navigating to other origins.
		c.Set("Referrer-Policy", "same-origin")
		_, initialSessionError := a.session(c, false)
		initiallyAuthenticated := initialSessionError == nil
		if initialSessionError != nil && !errors.Is(initialSessionError, security.ErrInvalidSession) {
			return securityError(c, initialSessionError)
		}

		var result error
		if c.Method() == http.MethodGet && c.Path() == "/account/cookie-check" {
			if define.AppFlags.DisableLoginMode {
				return securityResponse(c, 403, "FORBIDDEN")
			}
			if _, err := a.session(c, false); err != nil {
				if errors.Is(err, security.ErrInvalidSession) {
					return securityResponse(c, 503, "COOKIE_NOT_ESTABLISHED")
				}
				return securityError(c, err)
			}
			return c.Redirect().Status(303).To(safeReturn(c.Query("return_to")))
		}
		if c.Method() == http.MethodGet && (c.Path() == "/login" || c.Path() == define.SettingPages.Others.Path) {
			result = a.page(c)
		} else {
			result = c.Next()
		}
		foreground := foregroundActivity(c)
		headView := c.Method() == http.MethodHead && safeReturn(c.Path()) == c.Path()
		if result == nil && c.Response().StatusCode() < 400 && (foreground || headView) && a.token(c) != "" {
			// Observe and durably record the real clock again before releasing a
			// successful view. Request proofs never hide a revoke or rollback
			// discovered after rendering. HEAD validates without extending idle.
			if _, err := a.store.Session(a.token(c), foreground); err != nil {
				read := c.Method() == http.MethodGet || c.Method() == http.MethodHead
				if !errors.Is(err, security.ErrInvalidSession) || initiallyAuthenticated && read {
					return securityError(c, err)
				}
				// Logout and credential rotation intentionally revoke the POST's
				// original cookie; their successful redirects remain successful.
			}
		}
		return result
	})
	if !define.AppFlags.DisableLoginMode {
		app.Post(define.MiscPages.Login.Path, a.login)
		app.Post(define.MiscPages.Logout.Path, a.logout)
		app.Post("/logout-all", a.logoutAll)
		app.Post("/account/reauthenticate", a.reauthenticate)
		app.Post("/account/password", a.changePassword)
		app.Post("/account/local", a.takeLocal)
	}
	return nil
}
func persistent(c fiber.Ctx) *persistentAuth {
	a, ok := c.Locals(persistentKey{}).(*persistentAuth)
	if !ok {
		return nil
	}
	return a
}
func (a *persistentAuth) identity(c fiber.Ctx) *requestIdentity {
	if identity, ok := c.Locals(requestIdentityKey{}).(*requestIdentity); ok && identity.owner == a {
		return identity
	}
	identity := &requestIdentity{owner: a}
	count := 0
	for k, v := range c.Request().Header.Cookies() {
		if string(k) == a.name {
			identity.token = string(v)
			count++
		}
	}
	if count != 1 {
		identity.token = ""
	}
	c.Locals(requestIdentityKey{}, identity)
	return identity
}
func (a *persistentAuth) token(c fiber.Ctx) string { return a.identity(c).token }
func (a *persistentAuth) cookie(c fiber.Ctx, token string) {
	cookie := &fiber.Cookie{Name: a.name, Value: token, Path: "/", HTTPOnly: true, Secure: a.source.Secure(c), SameSite: "Lax"}
	if token == "" {
		cookie.MaxAge = -1
		cookie.Expires = time.Unix(1, 0)
	}
	c.Cookie(cookie)
}
func (a *persistentAuth) session(c fiber.Ctx, touch bool) (security.Session, error) {
	identity := a.identity(c)
	if !touch && identity.proof != nil {
		return a.store.GuardSession(identity.proof)
	}
	session, proof, err := a.store.SessionSnapshot(identity.token, touch)
	// Invalid-session proofs still guard live store failures on anonymous reads.
	identity.proof = proof
	if err != nil {
		return session, err
	}
	identity.username = a.store.Info().Username
	return session, nil
}

// Revalidate checks server-side revocation again immediately before a write.
func Revalidate(c fiber.Ctx) bool {
	if guarded, ok := c.Locals(authorizedWriteKey{}).(bool); ok && guarded {
		return true
	}
	if a := persistent(c); a != nil {
		if define.AppFlags.DisableLoginMode {
			return true
		}
		_, err := a.store.Session(a.token(c), false)
		return err == nil
	}
	return CheckUserIsLogin(c)
}
func RecentAuthentication(c fiber.Ctx) bool {
	if guarded, ok := c.Locals(authorizedWriteKey{}).(bool); ok && guarded {
		s, ok := c.Locals(authorizedSessionKey{}).(security.Session)
		return ok && time.Since(s.VerifiedAt) <= security.ReauthTimeout
	}
	if a := persistent(c); a != nil {
		s, err := a.store.Session(a.token(c), false)
		return err == nil && time.Since(s.VerifiedAt) <= security.ReauthTimeout
	}
	return false
}
func RequireRecentAuthentication(c fiber.Ctx) error {
	if !RecentAuthentication(c) {
		return fiber.NewError(403, "REAUTH_REQUIRED")
	}
	return nil
}
func securityResponse(c fiber.Ctx, status int, code string) error {
	c.Set("Cache-Control", "private, no-store")
	idBytes := make([]byte, 12)
	_, _ = rand.Read(idBytes)
	id := base64.RawURLEncoding.EncodeToString(idBytes)
	c.Set("X-Request-ID", id)
	logger.GetLogger().Warn("security request rejected", "code", code, "request_id", id, "status", status)
	if strings.Contains(c.Get("Accept"), "application/json") {
		return c.Status(status).JSON(fiber.Map{"error": fiber.Map{"code": code, "message": code}, "request_id": id})
	}
	return c.Status(status).Type("html").SendString(`<!doctype html><html lang="en"><meta name="viewport" content="width=device-width"><title>Flare</title><p>` + html.EscapeString(code) + `</p><p>Request: ` + id + `</p><a href="/login">Login / 登录</a></html>`)
}
func securityError(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, security.ErrInvalidCredentials):
		return securityResponse(c, 401, "INVALID_CREDENTIALS")
	case errors.Is(err, security.ErrInvalidSession):
		return securityResponse(c, 401, "AUTH_REQUIRED")
	case errors.Is(err, security.ErrDeploymentManaged):
		return securityResponse(c, 403, "CREDENTIAL_EXTERNALLY_MANAGED")
	case errors.Is(err, security.ErrPasswordPolicy):
		return securityResponse(c, 400, "INVALID_INPUT")
	case errors.Is(err, security.ErrBusy):
		c.Set("Retry-After", "10")
		return securityResponse(c, 429, "RATE_LIMITED")
	default:
		return securityResponse(c, 503, "STORAGE_UNAVAILABLE")
	}
}
func (a *persistentAuth) csrf(c fiber.Ctx) string {
	// Bind the token to the opaque session, or to a separate random browser
	// state before login. The signing key never reaches the browser.
	binding := a.token(c)
	if binding == "" {
		name := a.name + "_preauth"
		binding = c.Cookies(name)
		if cached, ok := c.Locals(preauthKey{}).(string); ok {
			binding = cached
		}
		if !validPreauth(binding) {
			raw := make([]byte, 32)
			_, _ = rand.Read(raw)
			binding = base64.RawURLEncoding.EncodeToString(raw) + "." + strconv.FormatInt(time.Now().Add(30*time.Minute).Unix(), 10)
		}
		c.Locals(preauthKey{}, binding)
		c.Cookie(&fiber.Cookie{Name: name, Value: binding, Path: "/", HTTPOnly: true, Secure: a.source.Secure(c), SameSite: "Lax", MaxAge: 1800})
	}
	mac := hmac.New(sha256.New, a.csrfKey[:])
	_, _ = mac.Write([]byte(binding))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
func (a *persistentAuth) validCSRF(c fiber.Ctx) bool {
	if c.Request().URI().QueryArgs().Has("csrf-token") {
		return false
	}
	if !a.source.Valid(c) {
		return false
	}
	binding := a.token(c)
	if binding == "" {
		binding = c.Cookies(a.name + "_preauth")
		if !validPreauth(binding) {
			return false
		}
	}
	if binding == "" {
		return false
	}
	mac := hmac.New(sha256.New, a.csrfKey[:])
	_, _ = mac.Write([]byte(binding))
	expected := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	token := formvalue.Get(c, "csrf-token")
	if token == "" {
		token = c.Get("X-CSRF-Token")
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(expected)) == 1
}

// ValidateCSRF protects content writes using the same session/browser binding.
func ValidateCSRF(c fiber.Ctx) bool { a := persistent(c); return a != nil && a.validCSRF(c) }
func CSRFToken(c fiber.Ctx) string {
	if a := persistent(c); a != nil {
		return a.csrf(c)
	}
	return ""
}
func (a *persistentAuth) check(c fiber.Ctx, authenticated bool) (bool, error) {
	for _, key := range []string{"username", "password", "new_password", "confirm_password"} {
		if c.Request().URI().QueryArgs().Has(key) {
			return false, securityResponse(c, 400, "INVALID_INPUT")
		}
	}
	if len(c.Body()) > 8192 {
		return false, securityResponse(c, 413, "PAYLOAD_TOO_LARGE")
	}
	if !a.validCSRF(c) {
		return false, securityResponse(c, 403, "CSRF_INVALID")
	}
	if authenticated {
		if _, err := a.session(c, false); err != nil {
			return false, securityError(c, err)
		}
	}
	return true, nil
}

func (a *persistentAuth) bucketKeys(c fiber.Ctx) []string {
	username := ""
	if c.Path() == define.MiscPages.Login.Path {
		username = formvalue.Get(c, "username")
	}
	if username == "" {
		username = a.store.Info().Username
	}
	digest := sha256.Sum256([]byte(username))
	return []string{"ip:" + a.source.ClientIP(c), "account:" + base64.RawURLEncoding.EncodeToString(digest[:])}
}
func (a *persistentAuth) allow(c fiber.Ctx) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for key, b := range a.buckets {
		if now.Sub(b.last) >= 15*time.Minute {
			delete(a.buckets, key)
		}
	}
	keys := a.bucketKeys(c)
	for _, key := range keys {
		b, exists := a.buckets[key]
		if !exists && len(a.buckets) >= 10000 {
			c.Set("Retry-After", "30")
			return false
		}
		if now.Before(b.until) {
			seconds := int(time.Until(b.until).Seconds()) + 1
			c.Set("Retry-After", strconv.Itoa(seconds))
			return false
		}
	}
	return true
}
func (a *persistentAuth) failed(c fiber.Ctx, err error) {
	if !errors.Is(err, security.ErrInvalidCredentials) {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	now := time.Now()
	for _, key := range a.bucketKeys(c) {
		b := a.buckets[key]
		if now.Before(b.until) {
			continue
		}
		if len(a.buckets) >= 10000 && b.last.IsZero() {
			continue
		}
		fresh := b.failures[:0]
		for _, t := range b.failures {
			if now.Sub(t) < 5*time.Minute {
				fresh = append(fresh, t)
			}
		}
		b.failures = append(fresh, now)
		b.last = now
		if len(b.failures) >= 10 {
			delay := 30 * time.Second
			for i := 0; i < b.strikes && delay < 15*time.Minute; i++ {
				delay *= 2
			}
			if delay > 15*time.Minute {
				delay = 15 * time.Minute
			}
			b.until = now.Add(delay)
			if b.strikes < 5 {
				b.strikes++
			}
		}
		a.buckets[key] = b
	}
}
func (a *persistentAuth) succeeded(c fiber.Ctx) {
	a.mu.Lock()
	defer a.mu.Unlock()
	keys := a.bucketKeys(c)
	delete(a.buckets, keys[1])
}
func (a *persistentAuth) login(c fiber.Ctx) error {
	if allowed, err := a.check(c, false); !allowed {
		return err
	}
	if !a.allow(c) {
		return securityResponse(c, 429, "RATE_LIMITED")
	}
	username, password := formvalue.Get(c, "username"), formvalue.Get(c, "password")
	if username == "" || password == "" {
		return securityResponse(c, 400, "INVALID_INPUT")
	}
	token, err := a.store.Authenticate(username, password)
	if err != nil {
		a.failed(c, err)
		return securityError(c, err)
	}
	if old := a.token(c); old != "" {
		if err := a.store.Revoke(old); err != nil {
			return securityError(c, err)
		}
	}
	a.succeeded(c)
	a.cookie(c, token)
	return c.Redirect().Status(303).To("/account/cookie-check?return_to=" + url.QueryEscape(safeReturn(formvalue.Get(c, "return_to"))))
}
func safeReturn(value string) string {
	if strings.ContainsAny(value, "%\\\r\n\t?#") {
		return "/"
	}
	for _, r := range value {
		if r < 32 || r == 127 {
			return "/"
		}
	}
	// Only navigation pages are valid destinations; auth/redirect/API endpoints
	// cannot become a second action after a successful login.
	allowed := map[string]bool{"/": true, "/settings": true, "/editor": true, "/settings/appearance": true, "/settings/application": true, "/settings/bookmark": true, "/settings/search": true, "/settings/theme": true, "/settings/weather": true, "/applications": true, "/bookmarks": true, "/help": true, "/guide": true, "/icons": true}
	if !allowed[value] {
		return "/"
	}
	return value
}
func (a *persistentAuth) logout(c fiber.Ctx) error {
	if allowed, err := a.check(c, true); !allowed {
		return err
	}
	if err := a.store.Revoke(a.token(c)); err != nil {
		return securityError(c, err)
	}
	a.cookie(c, "")
	return c.Redirect().Status(303).To("/login")
}
func (a *persistentAuth) logoutAll(c fiber.Ctx) error {
	if allowed, err := a.check(c, true); !allowed {
		return err
	}
	if err := a.store.RevokeAll(); err != nil {
		return securityError(c, err)
	}
	a.cookie(c, "")
	return c.Redirect().Status(303).To("/login")
}
func (a *persistentAuth) reauthenticate(c fiber.Ctx) error {
	if allowed, err := a.check(c, true); !allowed {
		return err
	}
	if !a.allow(c) {
		return securityResponse(c, 429, "RATE_LIMITED")
	}
	token, err := a.store.Reauthenticate(a.token(c), formvalue.Get(c, "password"))
	if err != nil {
		a.failed(c, err)
		return securityError(c, err)
	}
	a.succeeded(c)
	a.cookie(c, token)
	return c.Redirect().Status(303).To(define.SettingPages.Others.Path)
}
func (a *persistentAuth) changePassword(c fiber.Ctx) error {
	if allowed, err := a.check(c, true); !allowed {
		return err
	}
	if !a.allow(c) {
		return securityResponse(c, 429, "RATE_LIMITED")
	}
	if formvalue.Get(c, "new_password") != formvalue.Get(c, "confirm_password") {
		return securityResponse(c, 400, "INVALID_INPUT")
	}
	token, err := a.store.ChangePasswordAndCreateSession(a.token(c), formvalue.Get(c, "password"), formvalue.Get(c, "new_password"))
	if err != nil {
		a.failed(c, err)
		return securityError(c, err)
	}
	a.succeeded(c)
	a.cookie(c, token)
	return c.Redirect().Status(303).To(define.SettingPages.Others.Path)
}
func (a *persistentAuth) takeLocal(c fiber.Ctx) error {
	if allowed, err := a.check(c, true); !allowed {
		return err
	}
	if !a.allow(c) {
		return securityResponse(c, 429, "RATE_LIMITED")
	}
	if formvalue.Get(c, "confirm") != "local" {
		return securityResponse(c, 400, "INVALID_INPUT")
	}
	token, err := a.store.TakeLocalControl(a.token(c), formvalue.Get(c, "password"))
	if err != nil {
		a.failed(c, err)
		return securityError(c, err)
	}
	a.succeeded(c)
	a.cookie(c, token)
	return c.Redirect().Status(303).To(define.SettingPages.Others.Path)
}
func (a *persistentAuth) page(c fiber.Ctx) error {
	if define.AppFlags.DisableLoginMode {
		return c.Next()
	}
	token := html.EscapeString(a.csrf(c))
	hidden := `<input type="hidden" name="csrf-token" value="` + token + `">`
	style := `<meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><style>body{font:1.1rem system-ui;max-width:36rem;margin:2rem auto;padding:1rem;overflow-wrap:anywhere}input,button{font:inherit;max-width:100%;box-sizing:border-box}label{display:block;margin-top:1rem}button{margin-top:1rem}form{margin-bottom:2rem}</style>`
	body := `<!doctype html><html lang="en"><head>` + style + `<title>Flare account / 账号</title></head><body><main>`
	if s, err := a.session(c, false); err == nil {
		_ = s
		info := a.store.Info()
		if err := a.store.CheckWritable(); err != nil {
			return securityError(c, err)
		}
		body += `<h1>Account / 账号</h1><p>` + html.EscapeString(info.Username) + `</p><p>Credential management / 密码管理：` + html.EscapeString(info.Mode) + `</p>`
		origin := ""
		if u, e := a.source.External(c); e == nil {
			origin = u.Scheme + "://" + u.Host
		}
		body += `<section aria-label="Diagnostics"><h2>Diagnostics / 诊断</h2><dl><dt>Site visibility / 站点范围</dt><dd>` + html.EscapeString(define.AppFlags.Visibility) + `</dd><dt>Visibility source / 范围来源</dt><dd>` + html.EscapeString(define.AppFlags.InputSources["FLARE_VISIBILITY"]) + `</dd><dt>Login source / 登录配置来源</dt><dd>` + html.EscapeString(define.AppFlags.InputSources["FLARE_DISABLE_LOGIN"]) + `</dd><dt>External origin / 外部来源</dt><dd>` + html.EscapeString(origin) + `</dd><dt>Cookie</dt><dd>host-only; Path=/; HttpOnly; SameSite=Lax; Secure=` + strconv.FormatBool(a.source.Secure(c)) + `</dd><dt>Instance / 实例</dt><dd>` + html.EscapeString(info.InstanceID) + `</dd><dt>Security state / 安全状态</dt><dd>ready; writable / 已就绪；可写</dd><dt>Version / 版本</dt><dd>` + html.EscapeString(version.Version) + ` / ` + html.EscapeString(version.Commit) + `</dd></dl></section>`

		for _, p := range []struct{ url, label string }{{"/logout", "Logout / 退出当前"}, {"/logout-all", "Logout all / 退出全部"}} {
			body += `<form method="post" action="` + p.url + `">` + hidden + `<button>` + p.label + `</button></form>`
		}
		body += `<form method="post" action="/account/reauthenticate">` + hidden + `<h2>Verify password / 重新验证</h2><label>Password / 密码 <input type="password" name="password" autocomplete="current-password" required></label><button>Verify / 验证</button></form>`
		if info.Mode == "local" {
			body += `<form method="post" action="/account/password">` + hidden + `<h2>Change password / 修改密码</h2><label>Current password / 当前密码 <input type="password" name="password" autocomplete="current-password" required></label><label>New password / 新密码 <input type="password" name="new_password" autocomplete="new-password" required></label><label>Confirm / 确认 <input type="password" name="confirm_password" autocomplete="new-password" required></label><p>15–128 characters / 字符</p><button>Save / 保存</button></form>`
		} else {
			body += `<form method="post" action="/account/local">` + hidden + `<p>After switching, Compose password changes no longer apply. / 切换后旧 Compose 密码不再生效。</p><label>Current password / 当前密码 <input type="password" name="password" autocomplete="current-password" required></label><label><input type="checkbox" name="confirm" value="local" required> Manage password in application / 以后在应用管理密码</label><button>Confirm switch / 确认切换</button></form>`
		}
	} else {
		body += `<h1>Login / 登录</h1><form method="post" action="/login">` + hidden + `<input type="hidden" name="return_to" value="` + html.EscapeString(safeReturn(c.Query("return_to"))) + `"><label>Username / 用户名 <input name="username" autocomplete="username" required></label><label>Password / 密码 <input type="password" name="password" autocomplete="current-password" required></label><button>Login / 登录</button></form>`
	}
	if options, err := data.GetAllSettingsOptions(); err == nil {
		body += loginFooter(options.LoginFooter)
	}
	body += `<p><a href="/">Home / 首页</a></p></main></body></html>`
	return c.Type("html").SendString(body)
}

// PersistentSecurity distinguishes the explicitly configured production flow.
func PersistentSecurity(c fiber.Ctx) bool { return persistent(c) != nil }

// SessionBinding is suitable for binding a server-side preview ticket, not for
// authentication. It never exposes the session cookie to rendered pages.
func SessionBinding(c fiber.Ctx) string {
	a := persistent(c)
	if a == nil {
		return ""
	}
	binding := a.token(c)
	if binding == "" {
		binding = c.Cookies(a.name + "_preauth")
	}
	if binding == "" {
		a.csrf(c)
		state, ok := c.Locals(preauthKey{}).(string)
		if !ok {
			return ""
		}
		binding = state
	}
	if binding == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(binding))
	return base64.RawURLEncoding.EncodeToString(digest[:])
}

func validPreauth(binding string) bool {
	parts := strings.Split(binding, ".")
	if len(parts) != 2 {
		return false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil || len(raw) != 32 {
		return false
	}
	expiry, err := strconv.ParseInt(parts[1], 10, 64)
	return err == nil && expiry > time.Now().Unix() && expiry <= time.Now().Add(30*time.Minute).Unix()
}

func foregroundActivity(c fiber.Ctx) bool {
	if c.Method() != http.MethodGet {
		return unsafeMethod(c.Method())
	}
	path := c.Path()
	if path == "/login" {
		return true
	}
	return safeReturn(path) == path
}

func unsafeMethod(method string) bool {
	return method != http.MethodGet && method != http.MethodHead && method != http.MethodOptions
}

type authorizedWriteKey struct{}
type authorizedSessionKey struct{}

// AuthorizedWrite serializes the final session check with a disk commit, so a
// revocation cannot return successfully while an older session commits later.
// fn must only mutate data; rendering or security operations would reenter the
// security store lock.
func AuthorizedWrite(c fiber.Ctx, fn func() error) error {
	a := persistent(c)
	if a == nil {
		return fn()
	}
	trusted := define.AppFlags.DisableLoginMode
	session, sessionErr := a.store.Session(a.token(c), false)
	// Trusted-network writes have no administrator session. Only that expected
	// missing-session result is allowed; storage faults still reject the commit.
	if sessionErr != nil && (!trusted || !errors.Is(sessionErr, security.ErrInvalidSession)) {
		return authorizedWriteError(sessionErr)
	}
	err := a.store.WithAuthorizedWrite(a.token(c), trusted, func() error {
		c.Locals(authorizedSessionKey{}, session)
		c.Locals(authorizedWriteKey{}, true)
		defer c.Locals(authorizedWriteKey{}, false)
		return fn()
	})
	return authorizedWriteError(err)
}

func authorizedWriteError(err error) error {
	if errors.Is(err, security.ErrInvalidSession) {
		return fiber.NewError(401, "AUTH_REQUIRED")
	}
	if errors.Is(err, security.ErrFaulted) {
		return fiber.NewError(503, "STORAGE_UNAVAILABLE")
	}
	return err
}
