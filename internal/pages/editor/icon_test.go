package editor

import (
	"encoding/xml"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/resources/mdi"
	"github.com/stretchr/testify/require"
)

func TestEditorIconHTTP(t *testing.T) {
	flags, theme, color := define.AppFlags, define.ThemeCurrent, define.ThemePrimaryColor
	t.Cleanup(func() {
		define.AppFlags, define.ThemeCurrent, define.ThemePrimaryColor = flags, theme, color
	})
	define.AppFlags.DisableLoginMode = true
	for _, minimumRequest := range []bool{false, true} {
		t.Run(fmt.Sprintf("minimum_request=%t", minimumRequest), func(t *testing.T) {
			define.AppFlags.EnableMinimumRequest = minimumRequest
			define.ThemeCurrent, define.ThemePrimaryColor = "pumpkin", "#123456"
			// Preview the editor directly, before the homepage has generated any assets.
			require.NoError(t, mdi.Init())
			app := fiber.New()
			registerEditorIcon(app)
			request := func(name, validator string) *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodGet, iconPreviewPath+"?name="+url.QueryEscape(name), nil)
				if validator != "" {
					req.Header.Set("If-None-Match", validator)
				}
				rec := httptest.NewRecorder()
				serveTransferHTTP(t, app, rec, req)
				return rec
			}

			first := request("serverNetwork", "")
			require.Equal(t, http.StatusOK, first.Code)
			require.Equal(t, "image/svg+xml", first.Header().Get("Content-Type"))
			require.Equal(t, "no-store", first.Header().Get("Cache-Control"))
			var svg struct {
				XMLName xml.Name
				Path    struct {
					Data  string `xml:"d,attr"`
					Style string `xml:"style,attr"`
				} `xml:"path"`
			}
			require.NoError(t, xml.Unmarshal(first.Body.Bytes(), &svg))
			require.Equal(t, "svg", svg.XMLName.Local)
			require.Equal(t, "http://www.w3.org/2000/svg", svg.XMLName.Space)
			require.NotEmpty(t, svg.Path.Data)
			require.Contains(t, svg.Path.Style, "fill:#123456;")
			require.NotContains(t, first.Body.String(), "var(--color-primary)")
			lowercase := request("servernetwork", "")
			require.Equal(t, http.StatusOK, lowercase.Code)
			require.Equal(t, first.Body.String(), lowercase.Body.String())

			define.ThemeCurrent, define.ThemePrimaryColor = "cloud", "#abcdef"
			validator := first.Header().Get("ETag")
			if validator == "" {
				validator = `"previous-theme"`
			}
			changed := request("serverNetwork", validator)
			require.Equal(t, http.StatusOK, changed.Code)
			require.Equal(t, "no-store", changed.Header().Get("Cache-Control"))
			require.Contains(t, changed.Body.String(), "fill:#abcdef;")
			require.NotEqual(t, first.Body.String(), changed.Body.String())

			for _, name := range []string{"", "unknown-icon", "../serverNetwork", "serverNetwork/..", `serverNetwork\..`, "serverNetwork.svg", `<svg onload="alert(1)">`} {
				rec := request(name, validator)
				require.Equal(t, http.StatusNotFound, rec.Code, "icon name %q", name)
				require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
				require.NotContains(t, rec.Body.String(), "<svg")
			}
		})
	}
}

func TestEditorIconRequiresAuthentication(t *testing.T) {
	flags := define.AppFlags
	t.Cleanup(func() { define.AppFlags = flags })
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieSecret = strings.Repeat("s", 32)
	app := fiber.New()
	require.NoError(t, auth.RequestHandle(app))
	registerEditorIcon(app)
	req := httptest.NewRequest(http.MethodGet, iconPreviewPath+"?name=serverNetwork", nil)
	rec := httptest.NewRecorder()
	serveTransferHTTP(t, app, rec, req)
	require.Equal(t, http.StatusFound, rec.Code)
	require.Equal(t, define.SettingPages.Others.Path, rec.Header().Get("Location"))
	require.NotContains(t, rec.Body.String(), "<svg")
}
