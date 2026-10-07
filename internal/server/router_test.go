package server

import (
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRouter_Smoke(t *testing.T) {
	origWd, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(origWd)
	}()

	origEnv := os.Getenv("FLARE_BASELINE")
	os.Setenv("FLARE_BASELINE", "1")
	defer func() {
		if origEnv == "" {
			_ = os.Unsetenv("FLARE_BASELINE")
		} else {
			_ = os.Setenv("FLARE_BASELINE", origEnv)
		}
	}()

	env := define.GetDefaultEnvVars()
	flags := model.Flags{
		Port:              env.Port,
		EnableGuide:       false,
		EnableEditor:      false,
		EnableOfflineMode: true,
		DisableLoginMode:  true,
		Visibility:        "DEFAULT",
		DebugMode:         false,
		CookieName:        env.CookieName,
		CookieSecret:      "test-cookie-secret-with-at-least-32-bytes",
	}

	handler, err := NewRouter(&flags)
	require.NoError(t, err)
	require.NotNil(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code, "GET / 应返回 200")
}

// TestNewRouter_PrivateVisibility_RedirectsWhenNoAuth 验证 PRIVATE 可见性且未登录时 GET / 重定向到设置页
func TestNewRouter_PrivateVisibility_RedirectsWhenNoAuth(t *testing.T) {
	origWd, err := os.Getwd()
	require.NoError(t, err)
	tmpDir := t.TempDir()
	err = os.Chdir(tmpDir)
	require.NoError(t, err)
	defer func() {
		_ = os.Chdir(origWd)
	}()

	origEnv := os.Getenv("FLARE_BASELINE")
	os.Setenv("FLARE_BASELINE", "1")
	defer func() {
		if origEnv == "" {
			_ = os.Unsetenv("FLARE_BASELINE")
		} else {
			_ = os.Setenv("FLARE_BASELINE", origEnv)
		}
	}()

	env := define.GetDefaultEnvVars()
	flags := model.Flags{
		Port:              env.Port,
		EnableGuide:       false,
		EnableEditor:      false,
		EnableOfflineMode: true,
		DisableLoginMode:  false,
		Visibility:        "PRIVATE",
		DebugMode:         false,
		CookieName:        env.CookieName,
		CookieSecret:      "test-cookie-secret-with-at-least-32-bytes",
	}

	handler, err := NewRouter(&flags)
	require.NoError(t, err)
	require.NotNil(t, handler)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	assert.Equal(t, http.StatusFound, rec.Code, "未登录访问 PRIVATE 首页应 302")
	assert.Equal(t, define.SettingPages.Others.Path, rec.Header().Get("Location"), "应重定向到设置页")
}

func TestNewRouter_InitializesUnsafeCookieSecretsAndKeepsServing(t *testing.T) {
	orig := define.AppFlags
	t.Cleanup(func() { define.AppFlags = orig })
	t.Setenv("FLARE_BASELINE", "1")

	tests := []struct {
		name   string
		secret string
	}{
		{name: "empty", secret: ""},
		{name: "default", secret: define.DEFAULT_COOKIE_SECRET},
		{name: "whitespace", secret: " \t\n"},
		{name: "short", secret: strings.Repeat("x", 31)},
		{name: "padded_short", secret: " \t" + strings.Repeat("x", 31) + "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			flags := model.Flags{
				Port:              5005,
				EnableEditor:      true,
				EnableOfflineMode: true,
				DisableLoginMode:  false,
				Visibility:        "PRIVATE",
				CookieName:        "flare",
				CookieSecret:      tt.secret,
				User:              "testuser",
				Pass:              "testpass",
			}
			handler, err := NewRouter(&flags)
			require.NoError(t, err, "旧密钥配置升级后应继续提供服务")
			require.NotNil(t, handler)
			key, err := hex.DecodeString(flags.CookieSecret)
			require.NoError(t, err)
			require.Len(t, key, 32)
			assert.Equal(t, define.AppFlags.CookieSecret, flags.CookieSecret, "启动结果应同步有效密钥到传入配置")
			firstKey := flags.CookieSecret

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)
			assert.Equal(t, http.StatusFound, rec.Code, "初始化密钥后仍应保护私人首页")

			body := strings.NewReader("username=testuser&password=testpass")
			loginReq := httptest.NewRequest(http.MethodPost, define.MiscPages.Login.Path, body)
			loginReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			loginRec := httptest.NewRecorder()
			handler.ServeHTTP(loginRec, loginReq)
			require.Equal(t, http.StatusFound, loginRec.Code, "升级后应能正常登录")
			cookies := loginRec.Result().Cookies()
			require.Len(t, cookies, 1)

			flags.CookieSecret = tt.secret
			restarted, err := NewRouter(&flags)
			require.NoError(t, err)
			require.NotNil(t, restarted)
			assert.Equal(t, firstKey, flags.CookieSecret, "重启应复用保存的自动密钥")
			authedReq := httptest.NewRequest(http.MethodGet, "/", nil)
			authedReq.AddCookie(cookies[0])
			authedRec := httptest.NewRecorder()
			restarted.ServeHTTP(authedRec, authedReq)
			assert.Equal(t, http.StatusOK, authedRec.Code, "重启后应保留升级后的登录状态")
		})
	}
}
