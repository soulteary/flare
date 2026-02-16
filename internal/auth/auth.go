package auth

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/sessions"
	session "github.com/labstack/echo-contrib/v5/session"
	"github.com/labstack/echo/v5"

	FlareDefine "github.com/soulteary/flare/config/define"
)

const (
	SESSION_KEY_USER_NAME  = "USER_NAME"
	SESSION_KEY_LOGIN_DATE = "LOGIN_TIME"
)

// sessionName is set by RequestHandle and used by session.Get.
var sessionName string

func RequestHandle(e *echo.Echo) {
	sessionName = fmt.Sprintf("%s_%d", FlareDefine.AppFlags.CookieName, FlareDefine.AppFlags.Port)
	if !FlareDefine.AppFlags.DisableLoginMode {
		store := sessions.NewCookieStore([]byte(FlareDefine.AppFlags.CookieSecret))
		e.Use(session.Middleware(store))
		e.POST(FlareDefine.MiscPages.Login.Path, login)
		e.POST(FlareDefine.MiscPages.Logout.Path, logout)
	}
}

var commonText = `<a href="` + FlareDefine.SettingPages.Others.Path + `">返回重试</a></p><p>或前往 <a href="https://github.com/soulteary/docker-flare/issues/" target="_blank">https://github.com/soulteary/docker-flare/issues/</a> 反馈使用中的问题，谢谢！`
var internalErrorInput = []byte(`<html><p>请填写正确的用户名和密码 ` + commonText + `</html>`)
var internalErrorEmpty = []byte(`<html><p>用户名或密码不能为空 ` + commonText + `</html>`)
var internalErrorSave = []byte(`<html><p>程序内部错误，保存登陆状态失败 ` + commonText + `</html>`)

func AuthRequired(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c *echo.Context) error {
		if !FlareDefine.AppFlags.DisableLoginMode {
			sess, err := session.Get(sessionName, c)
			if err != nil {
				return c.Redirect(http.StatusFound, FlareDefine.SettingPages.Others.Path)
			}
			user := sess.Values[SESSION_KEY_USER_NAME]
			if user == nil {
				return c.Redirect(http.StatusFound, FlareDefine.SettingPages.Others.Path)
			}
		}
		return next(c)
	}
}

func CheckUserIsLogin(c *echo.Context) bool {
	if !FlareDefine.AppFlags.DisableLoginMode {
		sess, err := session.Get(sessionName, c)
		if err != nil {
			return false
		}
		user := sess.Values[SESSION_KEY_USER_NAME]
		return user != nil
	}
	return true
}

func GetUserName(c *echo.Context) interface{} {
	if !FlareDefine.AppFlags.DisableLoginMode {
		sess, err := session.Get(sessionName, c)
		if err != nil {
			return ""
		}
		if v, ok := sess.Values[SESSION_KEY_USER_NAME]; ok {
			return v
		}
	}
	return ""
}

func GetUserLoginDate(c *echo.Context) interface{} {
	if !FlareDefine.AppFlags.DisableLoginMode {
		sess, err := session.Get(sessionName, c)
		if err != nil {
			return ""
		}
		if v, ok := sess.Values[SESSION_KEY_LOGIN_DATE]; ok {
			return v
		}
	}
	return ""
}

const (
	_HTMLContentType = "text/html; charset=utf-8"
)

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

	if username != FlareDefine.AppFlags.User || password != FlareDefine.AppFlags.Pass {
		return c.HTMLBlob(http.StatusBadRequest, internalErrorInput)
	}

	sess.Values[SESSION_KEY_USER_NAME] = username
	sess.Values[SESSION_KEY_LOGIN_DATE] = time.Now().Format("2006年01月02日 15:04:05 CST")

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return c.HTMLBlob(http.StatusBadRequest, internalErrorSave)
	}

	return c.Redirect(http.StatusFound, FlareDefine.SettingPages.Others.Path)
}

func logout(c *echo.Context) error {
	sess, err := session.Get(sessionName, c)
	if err != nil {
		return c.Redirect(http.StatusFound, FlareDefine.SettingPages.Others.Path)
	}
	if sess.Values[SESSION_KEY_USER_NAME] == nil {
		return c.Redirect(http.StatusFound, FlareDefine.SettingPages.Others.Path)
	}
	delete(sess.Values, SESSION_KEY_USER_NAME)
	delete(sess.Values, SESSION_KEY_LOGIN_DATE)

	if err := sess.Save(c.Request(), c.Response()); err != nil {
		return c.HTMLBlob(http.StatusBadRequest, internalErrorSave)
	}
	return c.Redirect(http.StatusFound, FlareDefine.SettingPages.Others.Path)
}
