package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func saveAppFlags() model.Flags {
	return define.AppFlags
}

func restoreAppFlags(f model.Flags) {
	define.AppFlags = f
}

func TestAuthRequired_DisableLoginMode(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = true

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	called := false
	next := func(c *echo.Context) error {
		called = true
		return nil
	}
	handler := AuthRequired(next)
	err := handler(c)
	assert.NoError(t, err)
	assert.True(t, called, "AuthRequired(DisableLoginMode=true) should call next")
}

func TestCheckUserIsLogin_DisableLoginMode(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = true

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	ok := CheckUserIsLogin(c)
	assert.True(t, ok, "CheckUserIsLogin(DisableLoginMode=true) should return true")
}

func TestGetUserName_DisableLoginMode(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = true

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	name := GetUserName(c)
	assert.Empty(t, name, "GetUserName(DisableLoginMode=true) should return empty")
}

func TestGetUserLoginDate_DisableLoginMode(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = true

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	date := GetUserLoginDate(c)
	assert.Empty(t, date, "GetUserLoginDate(DisableLoginMode=true) should return empty")
}

func TestRequestHandle_DisableLoginMode(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = true
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005

	e := echo.New()
	RequestHandle(e)
	// 未注册 login/logout 路由时不应 panic；仅验证可调用
	assert.NotNil(t, e)
}

// TestAuthRequired_LoginRequired_RedirectsWhenNoSession 验证启用登录且无 session 时重定向到设置页
func TestAuthRequired_LoginRequired_RedirectsWhenNoSession(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005

	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)

	next := func(c *echo.Context) error {
		return nil
	}
	handler := AuthRequired(next)
	err := handler(c)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusFound, rec.Code, "应返回 302 重定向")
	assert.Equal(t, define.SettingPages.Others.Path, rec.Header().Get("Location"), "应重定向到设置页")
}

// TestLogin_Success_RedirectsAndSetsSession 验证正确用户名密码登录后重定向并设置 session
func TestLogin_Success_RedirectsAndSetsSession(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005
	define.AppFlags.User = "testuser"
	define.AppFlags.Pass = "testpass"
	define.AppFlags.CookieSecret = "test-secret-for-session"

	e := echo.New()
	RequestHandle(e)
	e.GET("/protected", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") }, AuthRequired)

	// 登录
	loginBody := strings.NewReader("username=testuser&password=testpass")
	req := httptest.NewRequest(http.MethodPost, define.MiscPages.Login.Path, loginBody)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code, "登录成功应 302")
	assert.Equal(t, define.SettingPages.Others.Path, rec.Header().Get("Location"))
	cookie := rec.Header().Get("Set-Cookie")
	require.NotEmpty(t, cookie, "应返回 Set-Cookie")

	// 带 session 请求受保护路由应 200
	req2 := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req2.Header.Set("Cookie", cookie)
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	assert.Equal(t, http.StatusOK, rec2.Code, "带 session 访问受保护路由应 200")
}

// TestLogin_WrongPassword_Returns400 验证错误用户名或密码时返回 400 且不设置 session
func TestLogin_WrongPassword_Returns400(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005
	define.AppFlags.User = "u"
	define.AppFlags.Pass = "p"
	define.AppFlags.CookieSecret = "wrong-pw-test-secret"

	e := echo.New()
	RequestHandle(e)

	body := strings.NewReader("username=u&password=wrong")
	req := httptest.NewRequest(http.MethodPost, define.MiscPages.Login.Path, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "错误密码应返回 400")
	// 不应通过 Set-Cookie 建立有效 session（可能无 Set-Cookie 或仅为清空）
	assert.Contains(t, rec.Body.String(), "请填写正确的用户名和密码", "响应体应包含错误提示")
}

// TestLogin_EmptyCredentials_Returns400 验证空用户名或密码时返回 400
func TestLogin_EmptyCredentials_Returns400(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005
	define.AppFlags.CookieSecret = "empty-test-secret"

	e := echo.New()
	RequestHandle(e)

	body := strings.NewReader("username=&password=any")
	req := httptest.NewRequest(http.MethodPost, define.MiscPages.Login.Path, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code, "空用户名应返回 400")
	assert.Contains(t, rec.Body.String(), "用户名或密码不能为空", "响应体应包含空值提示")
}

// TestLogout_ClearsSession 验证登出后 session 清除，再访问受保护路由被重定向
func TestLogout_ClearsSession(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005
	define.AppFlags.User = "u"
	define.AppFlags.Pass = "p"
	define.AppFlags.CookieSecret = "logout-test-secret"

	e := echo.New()
	RequestHandle(e)
	e.GET("/protected", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") }, AuthRequired)

	// 先登录拿到 cookie
	loginBody := strings.NewReader("username=u&password=p")
	reqLogin := httptest.NewRequest(http.MethodPost, define.MiscPages.Login.Path, loginBody)
	reqLogin.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	recLogin := httptest.NewRecorder()
	e.ServeHTTP(recLogin, reqLogin)
	require.Equal(t, http.StatusFound, recLogin.Code)
	cookie := recLogin.Header().Get("Set-Cookie")
	require.NotEmpty(t, cookie)

	// 登出
	reqLogout := httptest.NewRequest(http.MethodPost, define.MiscPages.Logout.Path, nil)
	reqLogout.Header.Set("Cookie", cookie)
	recLogout := httptest.NewRecorder()
	e.ServeHTTP(recLogout, reqLogout)
	assert.Equal(t, http.StatusFound, recLogout.Code)
	cookieAfterLogout := recLogout.Header().Get("Set-Cookie")
	require.NotEmpty(t, cookieAfterLogout, "登出响应应返回更新后的 Set-Cookie")

	// 使用登出后的 cookie 再访问受保护路由应被重定向（session 已空）
	reqGet := httptest.NewRequest(http.MethodGet, "/protected", nil)
	reqGet.Header.Set("Cookie", cookieAfterLogout)
	recGet := httptest.NewRecorder()
	e.ServeHTTP(recGet, reqGet)
	assert.Equal(t, http.StatusFound, recGet.Code, "登出后带更新后的 cookie 访问应 302")
	assert.Equal(t, define.SettingPages.Others.Path, recGet.Header().Get("Location"))
}
