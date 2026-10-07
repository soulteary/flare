package auth

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v5"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testCookieSecret = "test-cookie-secret-with-at-least-32-bytes"

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

	for _, secret := range []string{"", define.DEFAULT_COOKIE_SECRET, " \t\n"} {
		define.AppFlags.CookieSecret = secret
		e := echo.New()
		require.NoError(t, RequestHandle(e))
		for _, path := range []string{define.MiscPages.Login.Path, define.MiscPages.Logout.Path} {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusNotFound, rec.Code, "登录关闭时不应注册 %s", path)
		}
		e.GET("/protected", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") }, AuthRequired)
		req := httptest.NewRequest(http.MethodGet, "/protected", nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusOK, rec.Code, "关闭登录时应继续允许匿名访问")
	}
}

func TestRequestHandle_RejectsUnsafeCookieSecrets(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005

	tests := []struct {
		name   string
		secret string
	}{
		{name: "empty", secret: ""},
		{name: "default", secret: define.DEFAULT_COOKIE_SECRET},
		{name: "whitespace", secret: " \t\n"},
		{name: "padded_default", secret: " \t" + define.DEFAULT_COOKIE_SECRET + "\n"},
		{name: "short", secret: strings.Repeat("x", 31)},
		{name: "padded_short", secret: " \t" + strings.Repeat("x", 31) + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			define.AppFlags.CookieSecret = tt.secret
			e := echo.New()
			require.Error(t, RequestHandle(e))
			for _, path := range []string{define.MiscPages.Login.Path, define.MiscPages.Logout.Path} {
				req := httptest.NewRequest(http.MethodPost, path, nil)
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				assert.Equal(t, http.StatusNotFound, rec.Code, "无效密钥不应注册 %s", path)
			}
		})
	}
}

func TestRequestHandle_AcceptsSufficientCookieSecrets(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005

	for _, secret := range []string{strings.Repeat("x", 32), testCookieSecret} {
		define.AppFlags.CookieSecret = secret
		e := echo.New()
		require.NoError(t, RequestHandle(e))
		req := httptest.NewRequest(http.MethodPost, define.MiscPages.Login.Path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "有效密钥应注册登录路由")
	}
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
	define.AppFlags.CookieSecret = testCookieSecret

	e := echo.New()
	require.NoError(t, RequestHandle(e))
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
	define.AppFlags.CookieSecret = testCookieSecret

	e := echo.New()
	require.NoError(t, RequestHandle(e))

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
	define.AppFlags.CookieSecret = testCookieSecret

	e := echo.New()
	require.NoError(t, RequestHandle(e))

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
	define.AppFlags.CookieSecret = testCookieSecret

	e := echo.New()
	require.NoError(t, RequestHandle(e))
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

func TestAuthRequired_RejectsCookieSignedWithOldDefaultSecret(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005
	define.AppFlags.CookieSecret = testCookieSecret

	// Reproduce an attacker-created session signed with the old public key.
	oldStore := sessions.NewCookieStore([]byte(define.DEFAULT_COOKIE_SECRET))
	oldRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	oldSession, err := oldStore.New(oldRequest, RequestHandleSessionName("flare", 5005))
	require.NoError(t, err)
	oldSession.Values[SESSION_KEY_USER_NAME] = "forged-user"
	oldRecorder := httptest.NewRecorder()
	require.NoError(t, oldSession.Save(oldRequest, oldRecorder))
	oldCookies := oldRecorder.Result().Cookies()
	require.Len(t, oldCookies, 1)

	e := echo.New()
	require.NoError(t, RequestHandle(e))
	called := false
	e.GET("/protected", func(c *echo.Context) error {
		called = true
		return c.String(http.StatusOK, "ok")
	}, AuthRequired)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(oldCookies[0])
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, define.SettingPages.Others.Path, rec.Header().Get("Location"))
	assert.False(t, called, "旧默认密钥签名的 Cookie 不应通过鉴权")
}
