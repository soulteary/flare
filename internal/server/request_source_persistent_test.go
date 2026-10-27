package server

import (
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/stretchr/testify/require"
)

func TestPersistentWriteSourcePolicy(t *testing.T) {
	original := define.AppFlags
	t.Cleanup(func() { define.AppFlags = original })
	define.AppFlags.ExternalURL = "https://flare.test"
	define.AppFlags.AllowedHosts = "flare.test"
	define.AppFlags.TrustedProxies = ""
	define.AppFlags.CookieSecure = true
	middleware, err := NewWriteSourceProtection()
	require.NoError(t, err)
	app := fiber.New()
	app.Use(middleware)
	app.Post("/write", func(c fiber.Ctx) error { return c.SendStatus(200) })
	for _, origin := range []string{"http://flare.test", "https://evil.test", "null"} {
		req := httptest.NewRequest("POST", "http://flare.test/write", nil)
		req.Header.Set("Origin", origin)
		req.Header.Set("X-Forwarded-Host", "evil.test")
		req.Header.Set("X-Forwarded-Proto", "http")
		r, e := app.Test(req)
		require.NoError(t, e)
		require.Equal(t, 403, r.StatusCode)
		require.NotEmpty(t, r.Header.Get("X-Request-ID"))
		require.Equal(t, "private, no-store", r.Header.Get("Cache-Control"))
	}
	req := httptest.NewRequest("POST", "http://flare.test/write", nil)
	req.Header.Set("Origin", "https://flare.test:443")
	r, e := app.Test(req)
	require.NoError(t, e)
	require.Equal(t, 200, r.StatusCode)
	req = httptest.NewRequest("GET", "http://evil.test/write", nil)
	r, e = app.Test(req)
	require.NoError(t, e)
	require.Equal(t, 400, r.StatusCode)
}
