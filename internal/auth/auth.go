package auth

import (
	"crypto/subtle"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/gorilla/sessions"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/formvalue"
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

func RequestHandle(app *fiber.App) error {
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

var commonText = `<a href="` + define.SettingPages.Others.Path + `">返回重试</a></p><p>或前往 <a href="https://github.com/soulteary/docker-flare/issues/" target="_blank">https://github.com/soulteary/docker-flare/issues/</a> 反馈使用中的问题，谢谢！`
var internalErrorInput = []byte(`<html><p>请填写正确的用户名和密码 ` + commonText + `</html>`)
var internalErrorEmpty = []byte(`<html><p>用户名或密码不能为空 ` + commonText + `</html>`)
var internalErrorSave = []byte(`<html><p>程序内部错误，保存登陆状态失败 ` + commonText + `</html>`)

func AuthRequired(c fiber.Ctx) error {
	if !define.AppFlags.DisableLoginMode {
		state, err := getSession(c)
		if err != nil || state.session.Values[SESSION_KEY_USER_NAME] == nil {
			return c.Redirect().Status(http.StatusFound).To(define.SettingPages.Others.Path)
		}
	}
	return c.Next()
}

func CheckUserIsLogin(c fiber.Ctx) bool {
	if define.AppFlags.DisableLoginMode {
		return true
	}
	state, err := getSession(c)
	return err == nil && state.session.Values[SESSION_KEY_USER_NAME] != nil
}

func GetUserName(c fiber.Ctx) string {
	if !define.AppFlags.DisableLoginMode {
		if state, err := getSession(c); err == nil {
			value, _ := state.session.Values[SESSION_KEY_USER_NAME].(string)
			return value
		}
	}
	return ""
}

func GetUserLoginDate(c fiber.Ctx) string {
	if !define.AppFlags.DisableLoginMode {
		if state, err := getSession(c); err == nil {
			value, _ := state.session.Values[SESSION_KEY_LOGIN_DATE].(string)
			return value
		}
	}
	return ""
}

func authError(c fiber.Ctx, body []byte) error {
	c.Set(fiber.HeaderContentType, fiber.MIMETextHTMLCharsetUTF8)
	return c.Status(http.StatusBadRequest).Send(body)
}

func login(c fiber.Ctx) error {
	state, err := getSession(c)
	if state == nil || state.session == nil {
		return authError(c, internalErrorSave)
	}
	sess := state.session
	if err != nil {
		sess.Values = make(map[interface{}]interface{})
		sess.IsNew = true
	}
	username, password := formvalue.Get(c, "username"), formvalue.Get(c, "password")
	if strings.Trim(username, " ") == "" || strings.Trim(password, " ") == "" {
		return authError(c, internalErrorEmpty)
	}
	if subtle.ConstantTimeCompare([]byte(username), []byte(define.AppFlags.User)) != 1 ||
		subtle.ConstantTimeCompare([]byte(password), []byte(define.AppFlags.Pass)) != 1 {
		return authError(c, internalErrorInput)
	}
	sess.Values[SESSION_KEY_USER_NAME] = username
	sess.Values[SESSION_KEY_LOGIN_DATE] = time.Now().Format("2006年01月02日 15:04:05 CST")
	if err := saveSession(c, sess); err != nil {
		return authError(c, internalErrorSave)
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
		return authError(c, internalErrorSave)
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
