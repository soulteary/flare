package home

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/clock"
	"github.com/soulteary/flare/internal/searchui"
	"github.com/stretchr/testify/require"
)

func TestGuideScriptPolicyAllowsOnlyOwnAndHashedScripts(t *testing.T) {
	orig := saveAppFlags()
	defer restoreAppFlags(orig)
	clockHash := strings.TrimPrefix(strings.SplitN(clock.Policy(), ";", 2)[0], "script-src ")
	require.Regexp(t, `^'sha256-[A-Za-z0-9+/=]+'$`, clockHash)
	const rest = "; script-src-attr 'none'; connect-src 'self'; object-src 'none'; base-uri 'none';"
	for _, tc := range []struct {
		name                          string
		disabled, withClock, withFind bool
		want                          string
	}{
		{name: "guide scripts only", want: "script-src 'self'" + rest},
		{name: "clock", withClock: true, want: "script-src 'self' " + clockHash + rest},
		{name: "search", withFind: true, want: "script-src 'self' " + searchui.Hash() + rest},
		{name: "search and clock", withClock: true, withFind: true, want: "script-src 'self' " + searchui.Hash() + " " + clockHash + rest},
		{name: "policy disabled", disabled: true, withClock: true, withFind: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			define.AppFlags.DisableCSP = tc.disabled
			app := fiber.New()
			app.Get("/", func(c fiber.Ctx) error {
				// The guide replaces the script-free homepage policy set earlier in the request.
				setCSPHeader(c)
				applyGuideScriptPolicy(c, tc.withClock, tc.withFind)
				return c.SendStatus(http.StatusOK)
			})
			response, err := app.Test(httptest.NewRequest(http.MethodGet, "/", nil))
			require.NoError(t, err)
			require.NoError(t, response.Body.Close())
			policies := response.Header.Values("Content-Security-Policy")
			if tc.disabled {
				require.Empty(t, policies)
				return
			}
			require.Equal(t, []string{tc.want}, policies)
		})
	}
}

func TestGuideAssetsAreRequestScoped(t *testing.T) {
	app := fiber.New()
	app.Get("/", func(c fiber.Ctx) error { return c.SendString(guideAssetsURI(c)) })
	app.Get("/guide", func(c fiber.Ctx) error {
		c.Locals(guideAssetsKey{}, "/assets/guide")
		return c.SendString(guideAssetsURI(c))
	})
	for path, want := range map[string]string{"/guide": "/assets/guide", "/": ""} {
		response, err := app.Test(httptest.NewRequest(http.MethodGet, path, nil))
		require.NoError(t, err)
		body, err := io.ReadAll(response.Body)
		require.NoError(t, err)
		require.NoError(t, response.Body.Close())
		require.Equal(t, want, string(body), path)
	}
}
