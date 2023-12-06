package auth

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"

	FlareState "github.com/soulteary/flare/internal/state"
)

const (
	SESSION_KEY_USER_NAME  = "USER_NAME"
	SESSION_KEY_LOGIN_DATE = "LOGIN_TIME"
)

func RequestHandle(router *gin.Engine) {
	// 共享 store
	// TODO: 剥离逻辑
	// TODO：替换密钥为用户相关数据
	store := cookie.NewStore([]byte("secret"))
	router.Use(sessions.Sessions("flare_"+strconv.Itoa(FlareState.AppFlags.Port), store))

	// 非离线模式注册路由
	if !FlareState.AppFlags.DisableLoginMode {
		router.POST(FlareState.MiscPages.Login.Path, login)
		router.POST(FlareState.MiscPages.Logout.Path, logout)
	}
}

var commonText = `<a href="` + FlareState.SettingPages.Others.Path + `">返回重试</a></p><p>或前往 <a href="https://github.com/soulteary/docker-flare/issues/" target="_blank">https://github.com/soulteary/docker-flare/issues/</a> 反馈使用中的问题，谢谢！`
var internalErrorInput = []byte(`<html><p>请填写正确的用户名和密码 ` + commonText + `</html>`)
var internalErrorEmpty = []byte(`<html><p>用户名或密码不能为空 ` + commonText + `</html>`)
var internalErrorSave = []byte(`<html><p>程序内部错误，保存登陆状态失败 ` + commonText + `</html>`)

func AuthRequired(c *gin.Context) {

	if !FlareState.AppFlags.DisableLoginMode {
		session := sessions.Default(c)
		user := session.Get(SESSION_KEY_USER_NAME)
		if user == nil {
			c.Redirect(http.StatusFound, FlareState.SettingPages.Others.Path)
			c.Abort()
			return
		}
	}

	c.Next()
}

func CheckUserIsLogin(c *gin.Context) bool {
	if !FlareState.AppFlags.DisableLoginMode {
		session := sessions.Default(c)
		user := session.Get(SESSION_KEY_USER_NAME)
		return user != nil
	}
	return true
}

func GetUserName(c *gin.Context) interface{} {
	if !FlareState.AppFlags.DisableLoginMode {
		session := sessions.Default(c)
		data := session.Get(SESSION_KEY_USER_NAME)
		return data
	}
	return ""
}

func GetUserLoginDate(c *gin.Context) interface{} {
	if !FlareState.AppFlags.DisableLoginMode {
		session := sessions.Default(c)
		data := session.Get(SESSION_KEY_LOGIN_DATE)
		return data
	}
	return ""
}

const (
	_HTMLContentType = "text/html; charset=utf-8"
)

// login is a handler that parses a form and checks for specific data
func login(c *gin.Context) {
	session := sessions.Default(c)
	username := c.PostForm("username")
	password := c.PostForm("password")

	if strings.Trim(username, " ") == "" || strings.Trim(password, " ") == "" {
		c.Data(http.StatusBadRequest, _HTMLContentType, internalErrorEmpty)
		c.Abort()
		return
	}

	if username != FlareState.AppFlags.User || password != FlareState.AppFlags.Pass {
		c.Data(http.StatusBadRequest, _HTMLContentType, internalErrorInput)
		c.Abort()
		return
	}

	session.Set(SESSION_KEY_USER_NAME, username)
	session.Set(SESSION_KEY_LOGIN_DATE, time.Now().Format("2006年01月02日 15:04:05 CST"))

	if err := session.Save(); err != nil {
		c.Data(http.StatusBadRequest, _HTMLContentType, internalErrorSave)
		c.Abort()
		return
	}

	c.Redirect(http.StatusFound, FlareState.SettingPages.Others.Path)
	c.Abort()
}

func logout(c *gin.Context) {
	session := sessions.Default(c)
	user := session.Get(SESSION_KEY_USER_NAME)
	if user == nil {
		// 直接跳转登陆页面
		c.Redirect(http.StatusFound, FlareState.SettingPages.Others.Path)
		c.Abort()
		return
	}
	session.Delete(SESSION_KEY_USER_NAME)
	session.Delete(SESSION_KEY_LOGIN_DATE)

	if err := session.Save(); err != nil {
		c.Data(http.StatusBadRequest, _HTMLContentType, internalErrorSave)
		c.Abort()
		return
	}
	c.Redirect(http.StatusFound, FlareState.SettingPages.Others.Path)
	c.Abort()
}
