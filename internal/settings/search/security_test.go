package search

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/stretchr/testify/require"
)

func TestSearchSettingsOriginGuard(t *testing.T) {
	original := define.AppFlags
	t.Cleanup(func() { define.AppFlags = original })
	for _, tc := range []struct {
		name    string
		headers http.Header
		secure  bool
		want    int
	}{
		{name: "legacy client", want: 204},
		{name: "same origin", headers: http.Header{"Origin": {"http://flare.example:5005"}, "Sec-Fetch-Site": {"same-origin"}}, want: 204},
		{name: "cross-site form", headers: http.Header{"Origin": {"https://attacker.example"}, "Sec-Fetch-Site": {"cross-site"}}, want: 403},
		{name: "same-site sibling", headers: http.Header{"Origin": {"http://sibling.example:5005"}, "Sec-Fetch-Site": {"same-site"}}, want: 403},
		{name: "same-site metadata only", headers: http.Header{"Sec-Fetch-Site": {"same-site"}}, want: 403},
		{name: "cross-site metadata only", headers: http.Header{"Sec-Fetch-Site": {"cross-site"}}, want: 403},
		{name: "origin mismatch", headers: http.Header{"Origin": {"http://attacker.example:5005"}}, want: 403},
		{name: "port mismatch", headers: http.Header{"Origin": {"http://flare.example:5006"}}, want: 403},
		{name: "TLS termination with signed token", headers: http.Header{"Origin": {"https://flare.example:5005"}}, want: 204},
		{name: "null origin", headers: http.Header{"Origin": {"null"}}, want: 403},
		{name: "origin has path", headers: http.Header{"Origin": {"http://flare.example:5005/path"}}, want: 403},
		{name: "origin has credentials", headers: http.Header{"Origin": {"http://user@flare.example:5005"}}, want: 403},
		{name: "multiple origins", headers: http.Header{"Origin": {"http://flare.example:5005", "http://attacker.example"}}, want: 403},
		{name: "same origin referer", headers: http.Header{"Referer": {"http://flare.example:5005/settings/search"}}, want: 204},
		{name: "cross origin referer", headers: http.Header{"Referer": {"http://attacker.example/settings/search"}}, want: 403},
		{name: "conflicting referer", headers: http.Header{"Origin": {"http://flare.example:5005"}, "Referer": {"http://attacker.example/settings/search"}}, want: 403},
		{name: "spoofed forwarded scheme", headers: http.Header{"Origin": {"https://flare.example:5005"}, "X-Forwarded-Proto": {"https"}, "Forwarded": {"proto=https"}}, want: 204},
		{name: "explicit secure proxy", headers: http.Header{"Origin": {"https://flare.example:5005"}}, secure: true, want: 204},
	} {
		t.Run(tc.name, func(t *testing.T) {
			define.AppFlags.CookieSecure = tc.secure
			app := fiber.New()
			guard := newSearchSecurity()
			app.Get("/settings/search", guard.attach, func(c fiber.Ctx) error { return c.SendString(guard.formToken(c)) })
			app.Post("/settings/search", guard.attach, guard.protect, func(c fiber.Ctx) error { return c.SendStatus(http.StatusNoContent) })
			get, getErr := app.Test(httptest.NewRequest(http.MethodGet, "http://flare.example:5005/settings/search", nil))
			require.NoError(t, getErr)
			cookies := get.Cookies()
			require.Len(t, cookies, 1)
			require.NoError(t, get.Body.Close())
			req := httptest.NewRequest(http.MethodPost, "http://flare.example:5005/settings/search", strings.NewReader("csrf-token="+url.QueryEscape(cookies[0].Value)))
			req.Header = tc.headers
			if req.Header == nil {
				req.Header = make(http.Header)
			}
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.AddCookie(cookies[0])
			response, err := app.Test(req)
			require.NoError(t, err)
			defer response.Body.Close()
			require.Equal(t, tc.want, response.StatusCode)
		})
	}
}

func TestSearchSettingsTokenExpiryAndInstanceIsolation(t *testing.T) {
	original := define.AppFlags
	t.Cleanup(func() { define.AppFlags = original })
	define.AppFlags.Port = 5005
	first := newSearchSecurity()
	samePort := newSearchSecurity()
	require.NotEqual(t, first.cookieName, samePort.cookieName, "containers with the same internal port must not replace each other's cookies")
	define.AppFlags.Port = 5006
	second := newSearchSecurity()
	require.NotEqual(t, first.cookieName, second.cookieName)
	app := fiber.New()
	app.Get("/settings/search", first.attach, func(c fiber.Ctx) error { return c.SendString(first.formToken(c)) })
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/settings/search", nil))
	require.NoError(t, err)
	defer response.Body.Close()
	require.Len(t, response.Cookies(), 1)
	token := response.Cookies()[0].Value
	require.True(t, first.validToken(token, time.Now()))
	require.False(t, samePort.validToken(token, time.Now()))
	require.False(t, second.validToken(token, time.Now()))
	require.False(t, first.validToken(token, time.Now().Add(9*time.Hour)))
	require.False(t, first.validToken(token+"tampered", time.Now()))
}
