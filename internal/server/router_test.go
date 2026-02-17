package server

import (
	"net/http"
	"net/http/httptest"
	"os"
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
		CookieSecret:      env.CookieSecret,
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
		CookieSecret:      env.CookieSecret,
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
