package auth

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"html"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gorilla/sessions"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/formvalue"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/security"
)

const (
	SESSION_KEY_USER_NAME     = "USER_NAME"
	SESSION_KEY_LOGIN_DATE    = "LOGIN_TIME"
	minimumCookieSecretLength = 32
)

var sessionName string

type sessionStoreKey struct{}
type sessionStateKey struct{}

type cookieStore struct {
	store *sessions.CookieStore
	name  string
}

type sessionState struct {
	request *http.Request
	session *sessions.Session
	err     error
}

// RequestHandleSessionName preserves the cookie names used by the Echo version.
func RequestHandleSessionName(cookieName string, port int) string {
	return fmt.Sprintf("%s_%d", cookieName, port)
}

func RequestHandle(app *fiber.App, stores ...*security.Store) error {
	if len(stores) > 0 {
		return RequestHandleWithSecurity(app, stores[0])
	}
	if !define.AppFlags.DisableLoginMode {
		secret, err := initializeCookieSecret(define.AppFlags.CookieSecret)
		if err != nil {
			return err
		}
		define.AppFlags.CookieSecret = secret
	}
	sessionName = RequestHandleSessionName(define.AppFlags.CookieName, define.AppFlags.Port)
	if !define.AppFlags.DisableLoginMode {
		store := sessions.NewCookieStore([]byte(define.AppFlags.CookieSecret))
		store.Options = &sessions.Options{
			Path: "/", MaxAge: 30 * 24 * 60 * 60, HttpOnly: true,
			SameSite: http.SameSiteLaxMode, Secure: define.AppFlags.CookieSecure,
		}
		store.MaxAge(store.Options.MaxAge)
		configured := cookieStore{store: store, name: sessionName}
		app.Use(func(c fiber.Ctx) error {
			c.Locals(sessionStoreKey{}, &configured)
			return c.Next()
		})
		app.Post(define.MiscPages.Login.Path, login)
		app.Post(define.MiscPages.Logout.Path, logout)
	}
	return nil
}

// getSession adapts only the Cookie header when a handler actually needs a
// session. The cached state replaces Gorilla's net/http request registry without
// adapting the entire request or changing the signed cookie format.
func getSession(c fiber.Ctx) (*sessionState, error) {
	if state, ok := c.Locals(sessionStateKey{}).(*sessionState); ok {
		return state, state.err
	}
	configured, ok := c.Locals(sessionStoreKey{}).(*cookieStore)
	if !ok {
		return nil, errors.New("session store is not configured")
	}
	request := &http.Request{Header: http.Header{"Cookie": {c.Get(fiber.HeaderCookie)}}}
	session, err := configured.store.New(request, configured.name)
	state := &sessionState{request: request, session: session, err: err}
	c.Locals(sessionStateKey{}, state)
	return state, err
}

func AuthRequired(c fiber.Ctx) error {
	if a := persistent(c); a != nil {
		if define.AppFlags.DisableLoginMode {
			if unsafeMethod(c.Method()) && !a.validCSRF(c) {
				return securityResponse(c, 403, "CSRF_INVALID")
			}
			return c.Next()
		}
		if _, err := a.session(c, false); err != nil {
			if errors.Is(err, security.ErrInvalidSession) && strings.Contains(c.Get("Accept"), "text/html") && c.Method() == http.MethodGet {
				return c.Redirect().Status(302).To("/login?return_to=" + url.QueryEscape(c.OriginalURL()))
			}
			return securityError(c, err)
		}
		if unsafeMethod(c.Method()) && !a.validCSRF(c) {
			return securityResponse(c, 403, "CSRF_INVALID")
		}
		return c.Next()
	}
	if !define.AppFlags.DisableLoginMode {
		state, err := getSession(c)
		if err != nil || state.session.Values[SESSION_KEY_USER_NAME] == nil {
			return c.Redirect().Status(http.StatusFound).To(define.SettingPages.Others.Path)
		}
	}
	return c.Next()
}

func CheckUserIsLogin(c fiber.Ctx) bool {
	if guarded, ok := c.Locals(authorizedWriteKey{}).(bool); ok && guarded {
		return true
	}
	if a := persistent(c); a != nil {
		if define.AppFlags.DisableLoginMode {
			return true
		}
		_, err := a.session(c, false)
		return err == nil
	}
	if define.AppFlags.DisableLoginMode {
		return true
	}
	state, err := getSession(c)
	return err == nil && state.session.Values[SESSION_KEY_USER_NAME] != nil
}

func GetUserName(c fiber.Ctx) string {
	if a := persistent(c); a != nil {
		if define.AppFlags.DisableLoginMode {
			return ""
		}
		if _, err := a.session(c, false); err == nil {
			return a.identity(c).username
		}
		return ""
	}
	if !define.AppFlags.DisableLoginMode {
		if state, err := getSession(c); err == nil {
			if value, ok := state.session.Values[SESSION_KEY_USER_NAME].(string); ok {
				return value
			}
		}
	}
	return ""
}

func GetUserLoginDate(c fiber.Ctx) string {
	if a := persistent(c); a != nil {
		if s, err := a.session(c, false); err == nil {
			return s.CreatedAt.Format(time.RFC3339)
		}
		return ""
	}
	if !define.AppFlags.DisableLoginMode {
		if state, err := getSession(c); err == nil {
			if value, ok := state.session.Values[SESSION_KEY_LOGIN_DATE].(string); ok {
				return value
			}
		}
	}
	return ""
}

func authError(c fiber.Ctx, key string) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	locale := "zh"
	if options, err := data.GetAllSettingsOptions(); err == nil {
		locale = options.Locale
	}
	body := `<html lang="` + html.EscapeString(i18n.HTMLLang(locale)) + `"><p>` + html.EscapeString(i18n.T(locale, key)) + ` <a href="` + define.SettingPages.Others.Path + `">` + html.EscapeString(i18n.T(locale, "auth.retry")) + `</a></p><p><a href="https://github.com/soulteary/docker-flare/issues/" target="_blank" rel="noopener noreferrer">` + html.EscapeString(i18n.T(locale, "auth.report")) + `</a></p></html>`
	return c.Status(http.StatusBadRequest).SendString(body)
}

func login(c fiber.Ctx) error {
	state, err := getSession(c)
	if state == nil || state.session == nil {
		return authError(c, "auth.save_error")
	}
	sess := state.session
	if err != nil {
		sess.Values = make(map[interface{}]interface{})
		sess.IsNew = true
	}
	username, password := formvalue.Get(c, "username"), formvalue.Get(c, "password")
	if strings.Trim(username, " ") == "" || strings.Trim(password, " ") == "" {
		return authError(c, "auth.empty_error")
	}
	if subtle.ConstantTimeCompare([]byte(username), []byte(define.AppFlags.User)) != 1 ||
		subtle.ConstantTimeCompare([]byte(password), []byte(define.AppFlags.Pass)) != 1 {
		return authError(c, "auth.input_error")
	}
	sess.Values[SESSION_KEY_USER_NAME] = username
	sess.Values[SESSION_KEY_LOGIN_DATE] = time.Now().Format("2006年01月02日 15:04:05 CST")
	if err := saveSession(c, sess); err != nil {
		return authError(c, "auth.save_error")
	}
	return c.Redirect().Status(http.StatusFound).To(define.SettingPages.Others.Path)
}

func logout(c fiber.Ctx) error {
	state, err := getSession(c)
	if err != nil || state.session.Values[SESSION_KEY_USER_NAME] == nil {
		return c.Redirect().Status(http.StatusFound).To(define.SettingPages.Others.Path)
	}
	delete(state.session.Values, SESSION_KEY_USER_NAME)
	delete(state.session.Values, SESSION_KEY_LOGIN_DATE)
	if err := saveSession(c, state.session); err != nil {
		return authError(c, "auth.save_error")
	}
	return c.Redirect().Status(http.StatusFound).To(define.SettingPages.Others.Path)
}

// cookieResponseWriter lets Gorilla construct exactly the same Set-Cookie
// header as before. It is used only when a login or logout saves the session.
type cookieResponseWriter struct{ header http.Header }

func (w cookieResponseWriter) Header() http.Header       { return w.header }
func (cookieResponseWriter) Write(p []byte) (int, error) { return len(p), nil }
func (cookieResponseWriter) WriteHeader(int)             {}

func saveSession(c fiber.Ctx, sess *sessions.Session) error {
	state, err := getSession(c)
	if state == nil {
		return err
	}
	// Options is a per-session copy. Direct TLS never mutates the shared store,
	// and forwarding headers cannot enable or disable the Secure cookie policy.
	sess.Options.Secure = sess.Options.Secure || c.RequestCtx().IsTLS()
	writer := cookieResponseWriter{header: make(http.Header)}
	if err := sess.Save(state.request, writer); err != nil {
		return err
	}
	for _, cookie := range writer.header.Values(fiber.HeaderSetCookie) {
		c.Response().Header.Add(fiber.HeaderSetCookie, cookie)
	}
	return nil
}
