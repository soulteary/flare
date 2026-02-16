package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v5"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareModel "github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/assert"
)

func saveAppFlags() FlareModel.Flags {
	return FlareDefine.AppFlags
}

func restoreAppFlags(f FlareModel.Flags) {
	FlareDefine.AppFlags = f
}

func TestAuthRequired_DisableLoginMode(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	FlareDefine.AppFlags.DisableLoginMode = true

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
	FlareDefine.AppFlags.DisableLoginMode = true

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
	FlareDefine.AppFlags.DisableLoginMode = true

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
	FlareDefine.AppFlags.DisableLoginMode = true

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
	FlareDefine.AppFlags.DisableLoginMode = true
	FlareDefine.AppFlags.CookieName = "flare"
	FlareDefine.AppFlags.Port = 5005

	e := echo.New()
	RequestHandle(e)
	// 未注册 login/logout 路由时不应 panic；仅验证可调用
	assert.NotNil(t, e)
}
