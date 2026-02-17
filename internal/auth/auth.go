package auth

import (
	"crypto/subtle"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	session "github.com/labstack/echo-contrib/v5/session"
	"github.com/labstack/echo/v5"

	"github.com/soulteary/flare/config/define"
)

const (
	SESSION_KEY_USER_NAME  = "USER_NAME"
	SESSION_KEY_LOGIN_DATE = "LOGIN_TIME"
)

// sessionName is set by RequestHandle and used by session.Get. Prefer passing name via RequestHandleSessionName.
var sessionName string

// RequestHandleSessionName returns the session name for the given cookie name and port (for testing or explicit wiring).
func RequestHandleSessionName(cookieName string, port int) string {
	return fmt.Sprintf("%s_%d", cookieName, port)
}

func RequestHandle(e *echo.Echo) {
	sessionName = RequestHandleSessionName(define.AppFlags.CookieName, define.AppFlags.Port)
	if !define.AppFlags.DisableLoginMode {
		if define.AppFlags.CookieSecret == define.DEFAULT_COOKIE_SECRET {
			log.Println("[auth] 警告: 已启用登录但 CookieSecret 仍为默认值，生产环境请通过 FLARE_COOKIE_SECRET 或 --cookie-secret 设置强密钥")
		}
		store := sessions.NewCookieStore([]byte(define.AppFlags.CookieSecret))
		e.Use(session.Middleware(store))
		e.POST(define.MiscPages.Login.Path, login)
		e.POST(define.MiscPages.Logout.Path, logout)
	}
}

var commonText = `<a href="` + define.SettingPages.Others.Path + `">返回重试</a></p><p>或前往 <a href="https://github.com/soulteary/docker-flare/issues/" target="_blank">https://github.com/soulteary/docker-flare/issues/</a> 反馈使用中的问题，谢谢！`
var internalErrorInput = []byte(`<html><p>请填写正确的用户名和密码 ` + commonText + `</html>`)
var internalErrorEmpty = []byte(`<html><p>用户名或密码不能为空 ` + commonText + `</html>`)
var internalErrorSave = []byte(`<html><p>程序内部错误，保存登陆状态失败 ` + commonText + `</html>`)

func AuthRequired(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if !define.AppFlags.DisableLoginMode {
			sess, err := session.Get(sessionName, c)
			if err != nil {
				return c.Redirect(http.StatusFound, define.SettingPages.Others.Path)
			}
			user := sess.Values[SESSION_KEY_USER_NAME]
			if user == nil {
				return c.Redirect(http.StatusFound, define.SettingPages.Others.Path)
			}
		}
		return next(c)
	}
}

func CheckUserIsLogin(c *echo.Context) bool {
	if !define.AppFlags.DisableLoginMode {
		sess, err := session.Get(sessionName, c)
		if err != nil {
			return false
		}
		user := sess.Values[SESSION_KEY_USER_NAME]
		return user != nil
	}
	return true
}

func GetUserName(c *echo.Context) string {
	if !define.AppFlags.DisableLoginMode {
		sess, err := session.Get(sessionName, c)
		if err != nil {
			return ""
		}
		if v, ok := sess.Values[SESSION_KEY_USER_NAME].(string); ok {
			return v
		}
	}
	return ""
}

func GetUserLoginDate(c *echo.Context) string {
	if !define.AppFlags.DisableLoginMode {
		sess, err := session.Get(sessionName, c)
		if err != nil {
			return ""
		}
		if v, ok := sess.Values[SESSION_KEY_LOGIN_DATE].(string); ok {
			return v
		}
	}
	return ""
}

func login(c *echo.Context) error {
	sess, err := session.Get(sessionName, c)
	if err != nil {
		return c.HTMLBlob(http.StatusBadRequest, internalErrorSave)
	}
	username := c.FormValue("username")
	password := c.FormValue("password")

	if strings.Trim(username, " ") == "" || strings.Trim(password, " ") == "" {
		return c.HTMLBlob(http.StatusBadRequest, internalErrorEmpty)
	}

	if subtle.ConstantTimeCompare([]byte(username), []byte(define.AppFlags.User)) != 1 ||
		subtle.ConstantTimeCompare([]byte(password), []byte(define.AppFlags.Pass)) != 1 {
		return c.HTMLBlob(http.StatusBadRequest, internalErrorInput)
	}

	sess.Values[SESSION_KEY_USER_NAME] = username
	sess.Values[SESSION_KEY_LOGIN_DATE] = time.Now().Format("2006年01月02日 15:04:05 CST")

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return c.HTMLBlob(http.StatusBadRequest, internalErrorSave)
	}

	return c.Redirect(http.StatusFound, define.SettingPages.Others.Path)
}

func logout(c *echo.Context) error {
	sess, err := session.Get(sessionName, c)
	if err != nil {
		return c.Redirect(http.StatusFound, define.SettingPages.Others.Path)
	}
	if sess.Values[SESSION_KEY_USER_NAME] == nil {
		return c.Redirect(http.StatusFound, define.SettingPages.Others.Path)
	}
	delete(sess.Values, SESSION_KEY_USER_NAME)
	delete(sess.Values, SESSION_KEY_LOGIN_DATE)

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return c.HTMLBlob(http.StatusBadRequest, internalErrorSave)
	}
	return c.Redirect(http.StatusFound, define.SettingPages.Others.Path)
}
