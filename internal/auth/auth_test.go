package auth

import (
	"bytes"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/adaptor"
	"github.com/gorilla/sessions"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRequestHandle_DisableLoginMode(t *testing.T) {
	t.Chdir(t.TempDir())
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = true
	define.AppFlags.CookieName = "flare"
	define.AppFlags.Port = 5005

	for _, secret := range []string{"", define.DEFAULT_COOKIE_SECRET, " \t\n"} {
		define.AppFlags.CookieSecret = secret
		e := newAuthTestApp()
		require.NoError(t, RequestHandle(e.App))
		assert.Equal(t, secret, define.AppFlags.CookieSecret, "关闭登录时不应更改密钥配置")
		_, err := os.Stat(".flare-cookie-secret")
		assert.ErrorIs(t, err, os.ErrNotExist, "关闭登录时不应创建密钥文件")
		for _, path := range []string{define.MiscPages.Login.Path, define.MiscPages.Logout.Path} {
			req := httptest.NewRequest(http.MethodPost, path, nil)
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusNotFound, rec.Code, "登录关闭时不应注册 %s", path)
		}
		e.GET("/protected", func(c fiber.Ctx) error { return c.Status(http.StatusOK).SendString("ok") }, AuthRequired)
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
			e := newAuthTestApp()
			require.NoError(t, RequestHandle(e.App), "旧配置升级后应能正常启动")
			key, err := hex.DecodeString(define.AppFlags.CookieSecret)
			require.NoError(t, err)
			require.Len(t, key, 32, "自动密钥应包含 32 字节随机数据")
			saved, err := os.ReadFile(".flare-cookie-secret")
			require.NoError(t, err)
			assert.Equal(t, define.AppFlags.CookieSecret, strings.TrimSpace(string(saved)))
			e.GET("/protected", func(c fiber.Ctx) error { return c.Status(http.StatusOK).SendString("ok") }, AuthRequired)
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
		e := newAuthTestApp()
		require.NoError(t, RequestHandle(e.App))
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

	newApp := func() *authTestApp {
		e := newAuthTestApp()
		require.NoError(t, RequestHandle(e.App))
		e.GET("/protected", func(c fiber.Ctx) error { return c.Status(http.StatusOK).SendString("ok") }, AuthRequired)
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

			e := newAuthTestApp()
			require.NoError(t, RequestHandle(e.App))
			e.GET("/protected", func(c fiber.Ctx) error { return c.Status(http.StatusOK).SendString("ok") }, AuthRequired)
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
			require.NoError(t, RequestHandle(newAuthTestApp().App))
			firstKey := define.AppFlags.CookieSecret
			assertAutomaticCookieSecretLog(t, &logOutput, tt.secret, firstKey, tt.reason, "generated", true)

			logOutput.Reset()
			define.AppFlags.CookieSecret = tt.secret
			require.NoError(t, RequestHandle(newAuthTestApp().App))
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
	e := newAuthTestApp()
	require.NoError(t, RequestHandle(e.App), "无法保存自动密钥时应继续启动")
	e.GET("/protected", func(c fiber.Ctx) error { return c.Status(http.StatusOK).SendString("ok") }, AuthRequired)
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
	restarted := newAuthTestApp()
	require.NoError(t, RequestHandle(restarted.App))
	restarted.GET("/protected", func(c fiber.Ctx) error { return c.Status(http.StatusOK).SendString("ok") }, AuthRequired)
	assert.NotEqual(t, firstKey, define.AppFlags.CookieSecret, "无法持久化的密钥只能用于当前进程")
	assertProtectedWithCookie(t, restarted, cookie, http.StatusFound)
	assertProtectedWithCookie(t, restarted, loginForAutoSecretTest(t, restarted), http.StatusOK)
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

	e := newAuthTestApp()
	require.NoError(t, RequestHandle(e.App))
	e.GET("/protected", func(c fiber.Ctx) error { return c.Status(http.StatusOK).SendString("ok") }, AuthRequired)

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

	e := newAuthTestApp()
	require.NoError(t, RequestHandle(e.App))

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

	e := newAuthTestApp()
	require.NoError(t, RequestHandle(e.App))

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

	e := newAuthTestApp()
	require.NoError(t, RequestHandle(e.App))
	e.GET("/protected", func(c fiber.Ctx) error { return c.Status(http.StatusOK).SendString("ok") }, AuthRequired)

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

	e := newAuthTestApp()
	require.NoError(t, RequestHandle(e.App))
	called := false
	e.GET("/protected", func(c fiber.Ctx) error {
		called = true
		return c.Status(http.StatusOK).SendString("ok")
	}, AuthRequired)

	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.AddCookie(oldCookies[0])
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusFound, rec.Code)
	assert.Equal(t, define.SettingPages.Others.Path, rec.Header().Get("Location"))
	assert.False(t, called, "旧默认密钥签名的 Cookie 不应通过鉴权")
}

// This bridge is confined to tests; production uses Fiber's native listener.
type authTestApp struct {
	*fiber.App
	handler http.Handler
}

func newAuthTestApp() *authTestApp {
	app := fiber.New(fiber.Config{Immutable: true})
	return &authTestApp{App: app, handler: adaptor.FiberApp(app)}
}

func (app *authTestApp) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	app.handler.ServeHTTP(w, r)
}

func (app *authTestApp) GET(path string, handler fiber.Handler, middleware ...fiber.Handler) {
	handlers := make([]any, 0, len(middleware)+1)
	for _, h := range middleware {
		handlers = append(handlers, h)
	}
	handlers = append(handlers, handler)
	app.Get(path, handlers[0], handlers[1:]...)
}

func TestAuthHelpers_DisableLoginMode(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = true
	app := newAuthTestApp()
	app.Get("/", AuthRequired, func(c fiber.Ctx) error {
		assert.True(t, CheckUserIsLogin(c))
		assert.Empty(t, GetUserName(c))
		assert.Empty(t, GetUserLoginDate(c))
		return c.SendString("ok")
	})
	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusOK, response.Code)
	assert.Equal(t, "ok", response.Body.String())
}

func TestAuthRequired_LoginRequired_RedirectsWhenNoSession(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableLoginMode = false
	app := newAuthTestApp()
	app.Get("/", AuthRequired, func(c fiber.Ctx) error {
		t.Fatal("anonymous request reached protected handler")
		return nil
	})
	response := httptest.NewRecorder()
	app.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	assert.Equal(t, http.StatusFound, response.Code)
	assert.Equal(t, define.SettingPages.Others.Path, response.Header().Get("Location"))
}

func TestSessionCookiesRemainGorillaCompatible(t *testing.T) {
	original := saveAppFlags()
	t.Cleanup(func() { restoreAppFlags(original) })
	define.AppFlags = model.Flags{
		Port: 5005, CookieName: "flare", CookieSecret: testCookieSecret,
		User: "testuser", Pass: "testpass",
	}
	app := newAuthTestApp()
	require.NoError(t, RequestHandle(app.App))
	app.GET("/protected", func(c fiber.Ctx) error {
		assert.True(t, CheckUserIsLogin(c))
		assert.Equal(t, "testuser", GetUserName(c))
		assert.NotEmpty(t, GetUserLoginDate(c))
		return c.SendString("ok")
	}, AuthRequired)

	// Echo also uses this Gorilla store and serialization. A preexisting
	// session signed with the same key must survive switching frameworks.
	store := sessions.NewCookieStore([]byte(testCookieSecret))
	request := httptest.NewRequest(http.MethodGet, "/", nil)
	sess, err := store.New(request, RequestHandleSessionName("flare", 5005))
	require.NoError(t, err)
	sess.Values[SESSION_KEY_USER_NAME] = "testuser"
	sess.Values[SESSION_KEY_LOGIN_DATE] = "existing login"
	response := httptest.NewRecorder()
	require.NoError(t, sess.Save(request, response))
	assertProtectedWithCookie(t, app, response.Result().Cookies()[0], http.StatusOK)

	// A session produced by Fiber remains decodable by the original store.
	cookie := loginForAutoSecretTest(t, app)
	request = httptest.NewRequest(http.MethodGet, "/", nil)
	request.AddCookie(cookie)
	decoded, err := store.New(request, RequestHandleSessionName("flare", 5005))
	require.NoError(t, err)
	assert.Equal(t, "testuser", decoded.Values[SESSION_KEY_USER_NAME])
	assert.NotEmpty(t, decoded.Values[SESSION_KEY_LOGIN_DATE])
}

func TestSessionCookiePolicy(t *testing.T) {
	original := saveAppFlags()
	t.Cleanup(func() { restoreAppFlags(original) })
	for _, tc := range []struct {
		name, scheme, forwarded string
		forced, secure          bool
	}{
		{name: "plain HTTP", scheme: "http"},
		{name: "direct TLS", scheme: "https", secure: true},
		{name: "forwarded HTTPS is not trusted", scheme: "http", forwarded: "https"},
		{name: "forwarded HTTP cannot weaken TLS", scheme: "https", forwarded: "http", secure: true},
		{name: "forced secure", scheme: "http", forced: true, secure: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			define.AppFlags = model.Flags{
				Port: 5005, CookieName: "flare", CookieSecret: testCookieSecret,
				User: "testuser", Pass: "testpass", CookieSecure: tc.forced,
			}
			app := newAuthTestApp()
			require.NoError(t, RequestHandle(app.App))
			request := httptest.NewRequest(http.MethodPost, tc.scheme+"://flare.test/login", strings.NewReader("username=testuser&password=testpass"))
			request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			request.Header.Set("X-Forwarded-Proto", tc.forwarded)
			request.Header.Set("Forwarded", "proto="+tc.forwarded)
			response := httptest.NewRecorder()
			app.ServeHTTP(response, request)
			require.Equal(t, http.StatusFound, response.Code)
			cookies := response.Result().Cookies()
			require.Len(t, cookies, 1)
			cookie := cookies[0]
			assert.Equal(t, "flare_5005", cookie.Name)
			assert.Equal(t, "/", cookie.Path)
			assert.Equal(t, 30*24*60*60, cookie.MaxAge)
			assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
			assert.True(t, cookie.HttpOnly)
			assert.Equal(t, tc.secure, cookie.Secure)
		})
	}
}
