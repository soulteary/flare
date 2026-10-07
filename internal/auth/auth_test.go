package auth

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v5"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/logger"
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
	t.Chdir(t.TempDir())
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = true
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005

	for _, secret := range []string{"", define.DEFAULT_COOKIE_SECRET, " \t\n"} {
		define.AppFlags.CookieSecret = secret
		e := echo.New()
		require.NoError(t, RequestHandle(e))
		assert.Equal(t, secret, define.AppFlags.CookieSecret, "关闭登录时不应更改密钥配置")
		_, err := os.Stat(".flare-cookie-secret")
		assert.ErrorIs(t, err, os.ErrNotExist, "关闭登录时不应创建密钥文件")
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

func TestRequestHandle_InitializesUnsafeCookieSecrets(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005
	define.AppFlags.User = "testuser"
	define.AppFlags.Pass = "testpass"

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
			t.Chdir(t.TempDir())
			define.AppFlags.CookieSecret = tt.secret
			e := echo.New()
			require.NoError(t, RequestHandle(e), "旧配置升级后应能正常启动")
			key, err := hex.DecodeString(define.AppFlags.CookieSecret)
			require.NoError(t, err)
			require.Len(t, key, 32, "自动密钥应包含 32 字节随机数据")
			saved, err := os.ReadFile(".flare-cookie-secret")
			require.NoError(t, err)
			assert.Equal(t, define.AppFlags.CookieSecret, strings.TrimSpace(string(saved)))
			e.GET("/protected", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") }, AuthRequired)
			cookie := loginForAutoSecretTest(t, e)
			assertProtectedWithCookie(t, e, cookie, http.StatusOK)
		})
	}
}

func TestRequestHandle_AcceptsSufficientCookieSecrets(t *testing.T) {
	t.Chdir(t.TempDir())
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005

	for _, secret := range []string{strings.Repeat("x", 32), testCookieSecret} {
		define.AppFlags.CookieSecret = secret
		e := echo.New()
		require.NoError(t, RequestHandle(e))
		assert.Equal(t, secret, define.AppFlags.CookieSecret, "有效显式密钥应原样使用")
		_, err := os.Stat(".flare-cookie-secret")
		assert.ErrorIs(t, err, os.ErrNotExist, "有效显式密钥无需初始化密钥文件")
		req := httptest.NewRequest(http.MethodPost, define.MiscPages.Login.Path, nil)
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, req)
		assert.Equal(t, http.StatusBadRequest, rec.Code, "有效密钥应注册登录路由")
	}
}

func loginForAutoSecretTest(t *testing.T, handler http.Handler) *http.Cookie {
	t.Helper()
	body := strings.NewReader("username=testuser&password=testpass")
	req := httptest.NewRequest(http.MethodPost, define.MiscPages.Login.Path, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	require.Equal(t, http.StatusFound, rec.Code, "自动密钥初始化后应能正常登录")
	cookies := rec.Result().Cookies()
	require.Len(t, cookies, 1)
	return cookies[0]
}

func assertProtectedWithCookie(t *testing.T, handler http.Handler, cookie *http.Cookie, status int) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	assert.Equal(t, status, rec.Code)
}

func TestRequestHandle_AutomaticSecretRotatesOldSessionsAndSurvivesRestart(t *testing.T) {
	t.Chdir(t.TempDir())
	orig := saveAppFlags()
	t.Cleanup(func() { restoreAppFlags(orig) })
	define.AppFlags = model.Flags{
		Port: 5005, CookieName: "flare", CookieSecret: define.DEFAULT_COOKIE_SECRET,
		User: "testuser", Pass: "testpass",
	}

	oldStore := sessions.NewCookieStore([]byte(define.DEFAULT_COOKIE_SECRET))
	oldRequest := httptest.NewRequest(http.MethodGet, "/", nil)
	oldSession, err := oldStore.New(oldRequest, RequestHandleSessionName("flare", 5005))
	require.NoError(t, err)
	oldSession.Values[SESSION_KEY_USER_NAME] = "testuser"
	oldRecorder := httptest.NewRecorder()
	require.NoError(t, oldSession.Save(oldRequest, oldRecorder))
	oldCookies := oldRecorder.Result().Cookies()
	require.Len(t, oldCookies, 1)

	newApp := func() *echo.Echo {
		e := echo.New()
		require.NoError(t, RequestHandle(e))
		e.GET("/protected", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") }, AuthRequired)
		return e
	}
	e := newApp()
	firstKey := define.AppFlags.CookieSecret
	assertProtectedWithCookie(t, e, oldCookies[0], http.StatusFound)
	cookie := loginForAutoSecretTest(t, e)
	assertProtectedWithCookie(t, e, cookie, http.StatusOK)

	define.AppFlags.CookieSecret = define.DEFAULT_COOKIE_SECRET
	restarted := newApp()
	assert.Equal(t, firstKey, define.AppFlags.CookieSecret, "重启应复用已保存的自动密钥")
	assertProtectedWithCookie(t, restarted, cookie, http.StatusOK)
	assertProtectedWithCookie(t, restarted, oldCookies[0], http.StatusFound)

	define.AppFlags.CookieSecret = testCookieSecret
	explicit := newApp()
	assert.Equal(t, testCookieSecret, define.AppFlags.CookieSecret, "有效显式配置应优先于自动保存的密钥")
	assertProtectedWithCookie(t, explicit, cookie, http.StatusFound)
	assertProtectedWithCookie(t, explicit, loginForAutoSecretTest(t, explicit), http.StatusOK)
}

func TestLogin_RecoversAfterCookieSecretRotation(t *testing.T) {
	orig := saveAppFlags()
	t.Cleanup(func() { restoreAppFlags(orig) })
	tests := []struct {
		name       string
		oldSecret  string
		configured string
	}{
		{name: "automatic_upgrade", oldSecret: define.DEFAULT_COOKIE_SECRET, configured: define.DEFAULT_COOKIE_SECRET},
		{name: "explicit_rotation", oldSecret: testCookieSecret, configured: strings.Repeat("n", 32)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			define.AppFlags = model.Flags{
				Port: 5005, CookieName: "flare", CookieSecret: tt.configured,
				User: "testuser", Pass: "testpass",
			}
			oldStore := sessions.NewCookieStore([]byte(tt.oldSecret))
			oldRequest := httptest.NewRequest(http.MethodGet, "/", nil)
			oldSession, err := oldStore.New(oldRequest, RequestHandleSessionName("flare", 5005))
			require.NoError(t, err)
			oldSession.Values[SESSION_KEY_USER_NAME] = "testuser"
			oldRecorder := httptest.NewRecorder()
			require.NoError(t, oldSession.Save(oldRequest, oldRecorder))
			oldCookies := oldRecorder.Result().Cookies()
			require.Len(t, oldCookies, 1)

			e := echo.New()
			require.NoError(t, RequestHandle(e))
			e.GET("/protected", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") }, AuthRequired)
			assertProtectedWithCookie(t, e, oldCookies[0], http.StatusFound)

			login := func(password string) *httptest.ResponseRecorder {
				body := strings.NewReader("username=testuser&password=" + password)
				req := httptest.NewRequest(http.MethodPost, define.MiscPages.Login.Path, body)
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
				req.AddCookie(oldCookies[0])
				rec := httptest.NewRecorder()
				e.ServeHTTP(rec, req)
				return rec
			}
			wrongPassword := login("wrong")
			assert.Equal(t, http.StatusBadRequest, wrongPassword.Code, "旧 Cookie 不应绕过密码校验")
			for _, cookie := range wrongPassword.Result().Cookies() {
				assertProtectedWithCookie(t, e, cookie, http.StatusFound)
			}

			rec := login("testpass")
			require.Equal(t, http.StatusFound, rec.Code, "浏览器保留旧 Cookie 时仍应能够重新登录")
			cookies := rec.Result().Cookies()
			require.Len(t, cookies, 1)
			assertProtectedWithCookie(t, e, cookies[0], http.StatusOK)
		})
	}
}

func TestRequestHandle_LogsAutomaticCookieSecretWithoutExposingKeys(t *testing.T) {
	orig := saveAppFlags()
	t.Cleanup(func() { restoreAppFlags(orig) })
	origLogger := logger.GetLogger()
	t.Cleanup(func() { logger.SetLogger(origLogger) })
	tests := []struct {
		name   string
		secret string
		reason string
	}{
		{name: "empty", reason: "empty"},
		{name: "default", secret: define.DEFAULT_COOKIE_SECRET, reason: "default"},
		{name: "short", secret: "private-custom-short-key", reason: "short"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			var logOutput bytes.Buffer
			logger.SetLogger(slog.New(slog.NewJSONHandler(&logOutput, nil)))
			define.AppFlags = model.Flags{Port: 5005, CookieName: "flare", CookieSecret: tt.secret}
			require.NoError(t, RequestHandle(echo.New()))
			firstKey := define.AppFlags.CookieSecret
			assertAutomaticCookieSecretLog(t, &logOutput, tt.secret, firstKey, tt.reason, "generated", true)

			logOutput.Reset()
			define.AppFlags.CookieSecret = tt.secret
			require.NoError(t, RequestHandle(echo.New()))
			assert.Equal(t, firstKey, define.AppFlags.CookieSecret)
			assertAutomaticCookieSecretLog(t, &logOutput, tt.secret, firstKey, tt.reason, "reused", true)
		})
	}
}

func TestRequestHandle_ContinuesWithoutPersistingCookieSecret(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.Mkdir(".flare-cookie-secret", 0700))
	orig := saveAppFlags()
	t.Cleanup(func() { restoreAppFlags(orig) })
	define.AppFlags = model.Flags{
		Port: 5005, CookieName: "flare", CookieSecret: "private-custom-short-key",
		User: "testuser", Pass: "testpass",
	}
	var logOutput bytes.Buffer
	origLogger := logger.GetLogger()
	t.Cleanup(func() { logger.SetLogger(origLogger) })
	logger.SetLogger(slog.New(slog.NewJSONHandler(&logOutput, nil)))
	e := echo.New()
	require.NoError(t, RequestHandle(e), "无法保存自动密钥时应继续启动")
	e.GET("/protected", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") }, AuthRequired)
	cookie := loginForAutoSecretTest(t, e)
	assertProtectedWithCookie(t, e, cookie, http.StatusOK)
	firstKey := define.AppFlags.CookieSecret
	assertAutomaticCookieSecretLog(t, &logOutput, "private-custom-short-key", define.AppFlags.CookieSecret, "short", "generated", false)
	assert.Contains(t, logOutput.String(), "WARN")
	assert.Contains(t, logOutput.String(), "未持久化")
	assert.Contains(t, logOutput.String(), "重新登录")

	info, err := os.Stat(".flare-cookie-secret")
	require.NoError(t, err)
	assert.True(t, info.IsDir(), "不应替换阻止密钥保存的现有目录")

	logOutput.Reset()
	define.AppFlags.CookieSecret = "private-custom-short-key"
	restarted := echo.New()
	require.NoError(t, RequestHandle(restarted))
	restarted.GET("/protected", func(c *echo.Context) error { return c.String(http.StatusOK, "ok") }, AuthRequired)
	assert.NotEqual(t, firstKey, define.AppFlags.CookieSecret, "无法持久化的密钥只能用于当前进程")
	assertProtectedWithCookie(t, restarted, cookie, http.StatusFound)
	assertProtectedWithCookie(t, restarted, loginForAutoSecretTest(t, restarted), http.StatusOK)
}

func assertAutomaticCookieSecretLog(t *testing.T, output *bytes.Buffer, configured, effective, reason, action string, persisted bool) {
	t.Helper()
	var record struct {
		Message   string `json:"msg"`
		Reason    string `json:"reason"`
		Action    string `json:"action"`
		Persisted bool   `json:"persisted"`
	}
	require.NoError(t, json.NewDecoder(strings.NewReader(output.String())).Decode(&record))
	assert.NotEmpty(t, record.Message, "启动日志应说明自动初始化行为")
	assert.Equal(t, reason, record.Reason)
	assert.Equal(t, action, record.Action)
	assert.Equal(t, persisted, record.Persisted)
	assert.NotContains(t, output.String(), effective, "日志不应泄漏有效密钥")
	if configured == define.DEFAULT_COOKIE_SECRET {
		// The file name also contains "secret"; reject the actual JSON value instead.
		encoded, err := json.Marshal(configured)
		require.NoError(t, err)
		assert.NotContains(t, output.String(), string(encoded), "日志不应输出配置中的密钥值")
	} else if configured != "" {
		assert.NotContains(t, output.String(), configured, "日志不应泄漏配置中的密钥值")
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
