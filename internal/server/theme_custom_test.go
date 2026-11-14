package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/resources/assets"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

const (
	customThemeStyle     = `style="--color-background:#10241b;--color-primary:#e8f1e4;--color-accent:#f2b84b;"`
	blackboardThemeStyle = `style="--color-background:#1a1a1a;--color-primary:#FFFDEA;--color-accent:#5c5c5c;"`
	customThemeBlock     = "ThemeCustom:\n  Background: '#10241B'\n  Primary: '#e8f1e4'\n  Accent: '#f2b84b'\n"
	customThemeCard      = `theme theme-custom"`
)

// customThemeDirectory runs a test in its own data directory and leaves the
// default theme without a stored palette for the following tests.
func customThemeDirectory(t *testing.T, config string) {
	t.Helper()
	original := define.AppFlags
	t.Chdir(t.TempDir())
	// Registered after t.Chdir, so it runs while the test directory is current.
	t.Cleanup(func() {
		require.NoError(t, os.WriteFile("config.yml", []byte("Theme: blackboard\n"), 0600))
		define.UpdatePagePalettes()
		define.AppFlags = original
	})
	t.Setenv("FLARE_BASELINE", "1")
	require.NoError(t, os.WriteFile("config.yml", []byte(config), 0600))
	require.NoError(t, os.WriteFile("apps.yml", []byte("links:\n  - name: Example\n    link: https://example.com/\n    icon: github\n"), 0600))
}

func customThemeRouter(t *testing.T, config string) http.Handler {
	t.Helper()
	customThemeDirectory(t, config)
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	return handler
}

func themeRequest(handler http.Handler, form url.Values) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/settings/theme", nil)
	if form != nil {
		req = httptest.NewRequest(http.MethodPost, "/settings/theme", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

func customColorForm(background, primary, accent string) url.Values {
	return url.Values{"action": {"save-custom"}, "custom-background": {background}, "custom-primary": {primary}, "custom-accent": {accent}}
}

// colorInput matches one color field of the minified form with its value.
func colorInput(name, value string) *regexp.Regexp {
	return regexp.MustCompile(`<input type="?color"? id="?theme-custom-` + name + `"? name="?custom-` + name + `"? value="` + regexp.QuoteMeta(value) + `">`)
}

func TestCustomThemeColorsHTTP(t *testing.T) {
	handler := customThemeRouter(t, "Locale: en\nTheme: blackboard\nShowApps: true\n")
	get := func(path string) string {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, rec.Code, path)
		return rec.Body.String()
	}

	// Without a stored palette there is no custom card, and the form starts
	// from the colors of the theme in use.
	page := get("/settings/theme")
	require.NotContains(t, page, customThemeCard)
	require.Regexp(t, colorInput("background", "#1a1a1a"), page)
	require.Regexp(t, colorInput("primary", "#fffdea"), page)
	require.Regexp(t, colorInput("accent", "#5c5c5c"), page)
	require.Contains(t, page, i18n.T("en", "theme_custom_help"))

	saved := themeRequest(handler, customColorForm("#10241B", "#E8F1E4", "#f2b84b"))
	require.Equal(t, http.StatusOK, saved.Code)
	page = saved.Body.String()
	require.Contains(t, page, customThemeStyle)
	require.Contains(t, page, customThemeCard)
	require.Contains(t, page, `style="background-color: #10241b;"`)
	require.Regexp(t, `role="?status"?>`+regexp.QuoteMeta(i18n.T("en", "theme_custom_saved")), page)
	require.Regexp(t, colorInput("background", "#10241b"), page)
	// The custom palette has no generated favicon of its own.
	require.Contains(t, page, `href="`+assets.FaviconURL()+`"`)

	// Every themed page uses the palette. Icons are filled with its primary
	// color through the page's variable; the icon file itself carries no color.
	home := get("/")
	require.Contains(t, home, customThemeStyle)
	icon := regexp.MustCompile(`style="fill: var\(--flare-icon\);" aria-hidden="true"><use href="(/assets/mdi/github\.svg)\?v=[0-9a-f]{64}#i">`).FindStringSubmatch(home)
	require.NotNil(t, icon, "home page has no icon that follows the custom theme")
	require.NotContains(t, get(icon[1]), "fill")

	// A rejected palette explains the reason and leaves the saved palette
	// active. Colors that were submitted stay in the form for correction.
	rejected := themeRequest(handler, customColorForm("#202020", "#2a2a2a", "#f2b84b"))
	require.Equal(t, http.StatusBadRequest, rejected.Code)
	page = rejected.Body.String()
	require.Regexp(t, `role="?alert"?>`+regexp.QuoteMeta(i18n.T("en", "theme_custom_low_contrast")), page)
	require.NotRegexp(t, `role="?status"?>`, page)
	require.Contains(t, page, customThemeStyle)
	require.Regexp(t, colorInput("background", "#202020"), page)
	require.Regexp(t, colorInput("primary", "#2a2a2a"), page)

	// Text that is not a color is not written back into the page at all; that
	// field keeps the color in use.
	rejected = themeRequest(handler, customColorForm("#000;background:url(//example.test/)", "#ffffff", "#f2b84b"))
	require.Equal(t, http.StatusBadRequest, rejected.Code)
	page = rejected.Body.String()
	require.Regexp(t, `role="?alert"?>`+regexp.QuoteMeta(i18n.T("en", "theme_custom_invalid_color")), page)
	require.NotContains(t, page, "example.test")
	require.Contains(t, page, customThemeStyle)
	require.Regexp(t, colorInput("background", "#10241b"), page)
	require.Regexp(t, colorInput("primary", "#ffffff"), page)
	require.Contains(t, get("/"), customThemeStyle)

	// A bundled theme keeps the custom card. The form follows the colors in
	// use, so the bundled theme can be adjusted; the card returns to the
	// saved colors.
	require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"cloud"}}).Code)
	page = get("/settings/theme")
	require.NotContains(t, page, customThemeStyle)
	require.Contains(t, page, customThemeCard)
	require.Contains(t, page, `style="background-color: #10241b;"`)
	require.Regexp(t, colorInput("background", "#f1f2f0"), page)
	require.Regexp(t, colorInput("primary", "#35342f"), page)
	require.Regexp(t, colorInput("accent", "#37bbe4"), page)
	require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"custom"}}).Code)
	require.Contains(t, get("/"), customThemeStyle)

	raw, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Contains(t, string(raw), "Theme: custom")
	require.Contains(t, string(raw), "Background: '#10241b'")
}

func TestCustomThemeColorsSurviveRestart(t *testing.T) {
	handler := customThemeRouter(t, "Theme: custom\n"+customThemeBlock)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Contains(t, rec.Body.String(), customThemeStyle)
}

// config.yml can be edited by hand. A ThemeCustom block that is not a usable
// palette never reaches the page and never makes the configuration unreadable:
// the instance starts with the first bundled theme.
func TestUnusableStoredCustomThemeFallsBack(t *testing.T) {
	for name, block := range map[string]string{
		"color carrying a declaration": "ThemeCustom:\n  Background: '#000;background:url(//example.test/)'\n  Primary: '#e8f1e4'\n  Accent: '#f2b84b'\n",
		"unquoted colors are comments": "ThemeCustom:\n  Background: #10241b\n  Primary: #e8f1e4\n  Accent: #f2b84b\n",
		"scalar instead of a block":    "ThemeCustom: example.test\n",
		"list instead of a block":      "ThemeCustom:\n  - '#10241b'\n  - '#e8f1e4'\n",
		"nested value":                 "ThemeCustom:\n  Background:\n    - example.test\n  Primary: '#e8f1e4'\n  Accent: '#f2b84b'\n",
	} {
		t.Run(name, func(t *testing.T) {
			handler := customThemeRouter(t, "Theme: custom\n"+block)
			for _, path := range []string{"/", "/settings/theme"} {
				rec := httptest.NewRecorder()
				handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
				require.Equal(t, http.StatusOK, rec.Code, path)
				page := rec.Body.String()
				require.Contains(t, page, blackboardThemeStyle, path)
				require.NotContains(t, page, "example.test", path)
				require.NotContains(t, page, customThemeCard, path)
			}
			// The unusable palette cannot be selected either.
			require.Equal(t, http.StatusBadRequest, themeRequest(handler, url.Values{"theme": {"custom"}}).Code)
			// Other settings can still be saved, and a block that holds no
			// color at all is left in the file as it was written.
			require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"cloud"}}).Code)
			raw, err := os.ReadFile("config.yml")
			require.NoError(t, err)
			require.Contains(t, string(raw), "Theme: cloud")
			if strings.HasPrefix(name, "scalar") {
				require.Contains(t, string(raw), "ThemeCustom: example.test")
			}
			// Saving colors through the form replaces the block.
			require.Equal(t, http.StatusOK, themeRequest(handler, customColorForm("#10241b", "#e8f1e4", "#f2b84b")).Code)
			raw, err = os.ReadFile("config.yml")
			require.NoError(t, err)
			require.Contains(t, string(raw), "Background: '#10241b'")
			require.NotContains(t, string(raw), "example.test")
		})
	}
}

// The custom card applies the palette in config.yml, not the one last applied
// by this process: the file may have been edited in the meantime, and the
// selection has to match what the next start reads.
func TestCustomThemeCardFollowsConfiguration(t *testing.T) {
	handler := customThemeRouter(t, "Theme: cloud\n"+customThemeBlock)
	page := themeRequest(handler, nil).Body.String()
	require.Contains(t, page, customThemeCard)
	require.Contains(t, page, `style="background-color: #10241b;"`)

	edited := "Theme: cloud\nThemeCustom:\n  Background: '#ffffff'\n  Primary: '#222222'\n  Accent: '#888888'\n"
	require.NoError(t, os.WriteFile("config.yml", []byte(edited), 0600))
	page = themeRequest(handler, nil).Body.String()
	require.Contains(t, page, `style="background-color: #ffffff;"`)
	require.NotContains(t, page, `style="background-color: #10241b;"`)
	selected := themeRequest(handler, url.Values{"theme": {"custom"}})
	require.Equal(t, http.StatusOK, selected.Code)
	require.Contains(t, selected.Body.String(), `style="--color-background:#ffffff;--color-primary:#222222;--color-accent:#888888;"`)

	// Once the block is removed from the file the card is gone and cannot be
	// selected; the colors in use stay.
	require.NoError(t, os.WriteFile("config.yml", []byte("Theme: custom\n"), 0600))
	page = themeRequest(handler, nil).Body.String()
	require.NotContains(t, page, customThemeCard)
	require.Equal(t, http.StatusBadRequest, themeRequest(handler, url.Values{"theme": {"custom"}}).Code)
	require.Contains(t, themeRequest(handler, nil).Body.String(), `style="--color-background:#ffffff;--color-primary:#222222;--color-accent:#888888;"`)
}

// With login enabled the custom color form is a second form on the page and
// carries its own copy of the session-bound token.
func TestCustomThemeColorsRequireSessionAndToken(t *testing.T) {
	customThemeDirectory(t, "Theme: blackboard\nTitle: original\n")
	store, err := security.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	require.NoError(t, store.Initialize("admin", "legacy", "local"))
	app, err := NewFiberApp(&model.Flags{Port: 5005, CookieName: "settings", Visibility: "DEFAULT", StartupStatus: "ready"}, store)
	require.NoError(t, err)
	token, err := store.Authenticate("admin", "legacy")
	require.NoError(t, err)
	session := &http.Cookie{Name: auth.RequestHandleSessionName("settings", 5005) + "_" + store.Info().InstanceID, Value: token}
	request := func(method string, values url.Values, signedIn bool, origin string) (int, string) {
		var body io.Reader
		if values != nil {
			body = strings.NewReader(values.Encode())
		}
		req := httptest.NewRequest(method, "http://flare.test/settings/theme", body)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if signedIn {
			req.AddCookie(session)
		}
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		res, e := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		require.NoError(t, e)
		raw, e := io.ReadAll(res.Body)
		res.Body.Close()
		require.NoError(t, e)
		return res.StatusCode, string(raw)
	}

	status, page := request(http.MethodGet, nil, true, "")
	require.Equal(t, http.StatusOK, status)
	form := regexp.MustCompile(`(?s)class="?theme-custom-form"?>.*?</form>`).FindString(page)
	match := regexp.MustCompile(`name="?csrf-token"? value="?([A-Za-z0-9_.-]+)`).FindStringSubmatch(form)
	require.Len(t, match, 2, "the custom color form must carry the token")

	colors := customColorForm("#10241b", "#e8f1e4", "#f2b84b")
	before, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	status, _ = request(http.MethodPost, colors, false, "")
	require.Equal(t, http.StatusUnauthorized, status, "no session")
	status, _ = request(http.MethodPost, colors, true, "")
	require.Equal(t, http.StatusForbidden, status, "no token")
	colors.Set("csrf-token", match[1])
	status, _ = request(http.MethodPost, colors, true, "https://evil.example")
	require.Equal(t, http.StatusForbidden, status, "foreign origin")
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, before, after, "a rejected request changed config.yml")

	status, page = request(http.MethodPost, colors, true, "")
	require.Equal(t, http.StatusOK, status, page)
	require.Contains(t, page, customThemeStyle)
	after, err = os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Contains(t, string(after), "Theme: custom")
	require.Contains(t, string(after), "Background: '#10241b'")
}
