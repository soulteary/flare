package guide

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/stretchr/testify/require"
)

func TestGuideInjectsAssetsAndKeepsHomepageContent(t *testing.T) {
	before := define.AppFlags
	t.Cleanup(func() { define.AppFlags = before })
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `<html><head></head><body><main class="pageview">Homepage sentinel</main></body></html>`)
	}))
	_, port, err := net.SplitHostPort(upstream.Listener.Addr().String())
	require.NoError(t, err)
	define.AppFlags.Port, err = strconv.Atoi(port)
	require.NoError(t, err)
	define.AppFlags.Visibility = "DEFAULT"
	app := fiber.New()
	RegisterRouting(app)
	resp, err := app.Test(httptest.NewRequest("GET", define.RegularPages.Guide.Path, nil))
	require.NoError(t, err)
	raw, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, 200, resp.StatusCode)
	for _, expected := range []string{"Homepage sentinel", "/assets/guide/introjs.min.css", "/assets/guide/app.js", `style="position:inherit;"`} {
		require.Contains(t, string(raw), expected)
	}
	upstream.Close()
	require.Empty(t, getUserHomePage())
	define.AppFlags.Visibility = "PRIVATE"
	define.AppFlags.DisableLoginMode = false
	private := fiber.New()
	RegisterRouting(private)
	resp, err = private.Test(httptest.NewRequest("GET", define.RegularPages.Guide.Path, nil))
	require.NoError(t, err)
	defer resp.Body.Close()
	require.Equal(t, 302, resp.StatusCode)
	require.Equal(t, define.SettingPages.Others.Path, resp.Header.Get("Location"))
}
