//go:build fiber

package home

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/assert"
)

func saveAppFlags() model.Flags { return define.AppFlags }
func restoreAppFlags(f model.Flags) {
	define.AppFlags = f
}

func TestSetCSPHeader_WhenDisableCSPFalse_SetsHeader(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableCSP = false

	e := fiber.New()
	e.Get("/", func(c fiber.Ctx) error { setCSPHeader(c); return c.SendStatus(http.StatusOK) })
	rec, err := e.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer rec.Body.Close()

	assert.Equal(t, _cspValue, rec.Header.Get("Content-Security-Policy"), "应设置 CSP 头")
}

func TestSetCSPHeader_WhenDisableCSPTrue_NoHeader(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	define.AppFlags.DisableCSP = true

	e := fiber.New()
	e.Get("/", func(c fiber.Ctx) error { setCSPHeader(c); return c.SendStatus(http.StatusOK) })
	rec, err := e.Test(httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer rec.Body.Close()

	assert.Empty(t, rec.Header.Get("Content-Security-Policy"), "禁用 CSP 时不应设置头")
}
