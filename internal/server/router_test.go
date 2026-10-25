package server

import (
	"encoding/hex"
	"encoding/xml"
	"mime"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/data"
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

// TestNewRouter_MDIIcons follows the resources emitted by the real homepage.
// A successful HTML response alone does not exercise the browser's SVG requests.
func TestNewRouter_MDIIcons(t *testing.T) {
	handler := newMDITestRouter(t)
	iconURL := mdiURLFromHomepage(t, handler)
	icon := httptest.NewRecorder()
	handler.ServeHTTP(icon, httptest.NewRequest(http.MethodGet, iconURL, nil))
	pathData, originalStyle := decodeMDISVG(t, icon)
	originalBody := icon.Body.String()
	etag := icon.Header().Get("ETag")
	require.NotEmpty(t, etag, "successful SVG responses must support cache validation")

	for i := 0; i < 2; i++ {
		repeated := httptest.NewRecorder()
		handler.ServeHTTP(repeated, httptest.NewRequest(http.MethodGet, iconURL, nil))
		require.Equal(t, http.StatusOK, repeated.Code)
		assert.Equal(t, originalBody, repeated.Body.String(), "each request must read the entire SVG")
		assert.Equal(t, etag, repeated.Header().Get("ETag"))
	}

	head := httptest.NewRecorder()
	handler.ServeHTTP(head, httptest.NewRequest(http.MethodHead, iconURL, nil))
	assert.Equal(t, http.StatusOK, head.Code)
	assert.Empty(t, head.Body.String())
	assert.Equal(t, icon.Header().Get("Content-Type"), head.Header().Get("Content-Type"))
	assert.Equal(t, etag, head.Header().Get("ETag"))

	conditionalRequest := httptest.NewRequest(http.MethodGet, iconURL, nil)
	conditionalRequest.Header.Set("If-None-Match", etag)
	conditional := httptest.NewRecorder()
	handler.ServeHTTP(conditional, conditionalRequest)
	assert.Equal(t, http.StatusNotModified, conditional.Code)
	assert.Empty(t, conditional.Body.String())

	// Use the settings route so the next homepage exercises a real theme change.
	themeRequest := httptest.NewRequest(http.MethodPost, define.SettingPages.Theme.Path, strings.NewReader("theme=pumpkin"))
	themeRequest.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	themeResponse := httptest.NewRecorder()
	handler.ServeHTTP(themeResponse, themeRequest)
	require.Equal(t, http.StatusOK, themeResponse.Code)
	themedURL := mdiURLFromHomepage(t, handler)
	require.NotEqual(t, iconURL, themedURL, "theme changes must produce a distinct icon URL")
	themedIcon := httptest.NewRecorder()
	handler.ServeHTTP(themedIcon, httptest.NewRequest(http.MethodGet, themedURL, nil))
	themedPath, themedStyle := decodeMDISVG(t, themedIcon)
	assert.Equal(t, pathData, themedPath, "changing themes must preserve the icon geometry")
	assert.NotEqual(t, originalStyle, themedStyle, "the new SVG must use the selected theme color")
	assert.Contains(t, themedStyle, "#EFFBFF", "pumpkin's primary color must reach the SVG")
	assert.NotEqual(t, etag, themedIcon.Header().Get("ETag"), "different SVG bytes need distinct validators")

	previousIcon := httptest.NewRecorder()
	handler.ServeHTTP(previousIcon, httptest.NewRequest(http.MethodGet, iconURL, nil))
	require.Equal(t, http.StatusOK, previousIcon.Code)
	assert.Equal(t, originalBody, previousIcon.Body.String(), "a cached URL must keep its original theme")

	// The existing minimum-request mode must still render the same icon inline.
	flags := define.AppFlags
	flags.EnableMinimumRequest = true
	inlineHandler, err := NewRouter(&flags)
	require.NoError(t, err)
	inline := httptest.NewRecorder()
	inlineHandler.ServeHTTP(inline, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, inline.Code)
	assert.NotContains(t, inline.Body.String(), "/assets/mdi/")
	assert.Contains(t, inline.Body.String(), `d="`+pathData+`"`, "the configured icon must be present as inline SVG")
}

// TestNewRouter_MissingMDIIsNotCached checks the resource middleware together
// with the MDI route: validators from another resource cannot hide a missing file.
func TestNewRouter_MissingMDIIsNotCached(t *testing.T) {
	handler := newMDITestRouter(t)
	favicon := httptest.NewRecorder()
	handler.ServeHTTP(favicon, httptest.NewRequest(http.MethodGet, "/favicon.ico", nil))
	require.Equal(t, http.StatusOK, favicon.Code)
	etag := favicon.Header().Get("ETag")
	require.NotEmpty(t, etag)

	for _, validator := range []string{"", etag, "*"} {
		request := httptest.NewRequest(http.MethodGet, "/assets/mdi/missing-icon.svg", nil)
		if validator != "" {
			request.Header.Set("If-None-Match", validator)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		assert.Equal(t, http.StatusNotFound, response.Code, "If-None-Match: %q must not turn a missing icon into 304", validator)
		assert.Empty(t, response.Header().Get("ETag"), "errors must not carry a successful resource's validator")
		cacheControl := strings.ToLower(response.Header().Get("Cache-Control"))
		assert.True(t, cacheControl == "" || strings.Contains(cacheControl, "no-store") || strings.Contains(cacheControl, "no-cache"),
			"missing icons must not be cached as successful assets: %q", cacheControl)
	}
}

// newMDITestRouter creates an isolated, offline app with one known MDI bookmark.
// These tests intentionally run serially because the application's settings are global.
func newMDITestRouter(t *testing.T) http.Handler {
	t.Helper()
	originalFlags := define.AppFlags
	originalTheme, originalColor := define.ThemeCurrent, define.ThemePrimaryColor
	t.Cleanup(func() {
		define.AppFlags = originalFlags
		define.ThemeCurrent, define.ThemePrimaryColor = originalTheme, originalColor
	})
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	require.True(t, data.UpdateThemeName("blackboard"))
	require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{
		Name: "MDI regression fixture", URL: "https://example.com", Icon: "serverNetwork",
	}}}))
	require.True(t, data.SaveNormalBookmarks(model.Bookmarks{}))
	flags := model.Flags{
		Port:              5005,
		EnableOfflineMode: true,
		DisableLoginMode:  true,
		Visibility:        "DEFAULT",
		CookieName:        "flare",
		CookieSecret:      "test-cookie-secret-with-at-least-32-bytes",
	}
	handler, err := NewRouter(&flags)
	require.NoError(t, err)
	return handler
}

// mdiURLFromHomepage obtains the actual URL a browser would fetch, rather than
// coupling the regression test to the icon filename construction.
func mdiURLFromHomepage(t *testing.T, handler http.Handler) string {
	t.Helper()
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, response.Code)
	imageSource := regexp.MustCompile(`<img\b[^>]*\bsrc=["']?(/assets/mdi/[^"'\s>]+)`)
	matches := imageSource.FindAllStringSubmatch(response.Body.String(), -1)
	require.Len(t, matches, 1, "the fixture's icon must be emitted by the homepage")
	return matches[0][1]
}

// decodeMDISVG verifies the HTTP response and parses the SVG as a browser resource.
func decodeMDISVG(t *testing.T, response *httptest.ResponseRecorder) (pathData, style string) {
	t.Helper()
	require.Equal(t, http.StatusOK, response.Code, response.Body.String())
	mediaType, _, err := mime.ParseMediaType(response.Header().Get("Content-Type"))
	require.NoError(t, err)
	require.Equal(t, "image/svg+xml", mediaType)
	var document struct {
		XMLName xml.Name `xml:"svg"`
		Paths   []struct {
			Data  string `xml:"d,attr"`
			Style string `xml:"style,attr"`
		} `xml:"path"`
	}
	require.NoError(t, xml.Unmarshal(response.Body.Bytes(), &document))
	require.Equal(t, "http://www.w3.org/2000/svg", document.XMLName.Space)
	require.Len(t, document.Paths, 1)
	require.NotEmpty(t, document.Paths[0].Data)
	return document.Paths[0].Data, document.Paths[0].Style
}
