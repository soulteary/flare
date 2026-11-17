package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/security"
)

const (
	safeModeConfig = "Locale: en\nTitle: Site\nTheme: styled\nShowApps: true\nShowBookmarks: true\nHomepage:\n  DesktopColumns: \"4\"\n"
	safeModeTheme  = themeFileColors + "layout:\n  DesktopColumns: 6\n"
)

var safeModeFiles = map[string]string{
	"styled/theme.yml": safeModeTheme,
	"styled/theme.css": themeStylesheetText,
	"other.yml":        themeFileColors + "  radius: 12px\n",
}

// One response can be shown without the themes of the themes directory: the
// page as it is when no such theme has a say, with a notice and the way back
// to the settings. Nothing is changed by it.
func TestThemeSafeModeForOnePageHTTP(t *testing.T) {
	handler := themeFilesRouter(t, safeModeConfig, safeModeFiles)
	get := func(path string) (string, http.Header) {
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		require.Equal(t, http.StatusOK, rec.Code, path)
		return rec.Body.String(), rec.Header()
	}
	themed := func(path string) {
		t.Helper()
		page, header := get(path)
		require.Equal(t, themeFileStyle, bodyStyle(t, page), path)
		require.Len(t, pageStyles(t, page), 2, path)
		require.Contains(t, bodyClasses(t, page), "desktop-columns-6", path)
		require.Nil(t, readPreviewNotice(t, page), path)
		require.NotEqual(t, "private, no-store", header.Get("Cache-Control"), path)
	}
	stored, err := os.ReadFile("config.yml")
	require.NoError(t, err)

	for _, path := range []string{"/", "/applications", "/bookmarks"} {
		themed(path)
		for _, query := range []string{"?theme-safe-mode", "?theme-safe-mode=1", "?theme-safe-mode=0", "?theme-safe-mode=&other=1", "?other=1&theme-safe-mode"} {
			page, header := get(path + query)
			// No colors, no stylesheet and no layout of the theme.
			require.Empty(t, bodyStyle(t, page), path+query)
			require.Equal(t, []string{"style"}, pageStyles(t, page), path+query)
			require.NotContains(t, bodyClasses(t, page), "desktop-columns-6", path+query)
			require.Contains(t, bodyClasses(t, page), "desktop-columns-4", path+query)
			require.NotContains(t, page, "/theme/styled/", path+query)
			// The notice says what is shown and leads back to the settings.
			// It has nothing to submit.
			notice := readPreviewNotice(t, page)
			require.NotNil(t, notice, path+query)
			require.Equal(t, previewNotice{
				text: i18n.T("en", "theme_safe_mode_page"),
				back: i18n.T("en", "theme_preview_back"),
				href: "/settings/theme",
			}, *notice, path+query)
			require.Equal(t, "private, no-store", header.Get("Cache-Control"), path+query)
		}
		// The next response is shown with the theme again.
		themed(path)
	}
	page, _ := get("/help?theme-safe-mode")
	require.Empty(t, bodyStyle(t, page))
	require.Equal(t, []string{"style"}, pageStyles(t, page))
	require.NotNil(t, readPreviewNotice(t, page))

	// Safe mode comes before a preview.
	page, _ = get("/?theme-preview=other&theme-safe-mode")
	require.Empty(t, bodyStyle(t, page))
	require.Equal(t, i18n.T("en", "theme_safe_mode_page"), readPreviewNotice(t, page).text)
	page, _ = get("/?theme-preview=other")
	require.Equal(t, themeFileStyle+"--flare-radius:12px;", bodyStyle(t, page))

	// The settings are not what safe mode is about.
	for _, path := range []string{"/settings/theme?theme-safe-mode", "/settings/layout?theme-safe-mode"} {
		page, _ = get(path)
		require.Equal(t, themeFileStyle, bodyStyle(t, page), path)
		require.Nil(t, readPreviewNotice(t, page), path)
	}

	// Nothing was written, and the theme is still in use.
	unchanged, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, string(stored), string(unchanged))
	themed("/")

	// A bundled theme and the custom colors are not themes of the
	// directory: the page keeps them.
	require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"cloud"}}).Code)
	page, _ = get("/?theme-safe-mode")
	require.Equal(t, "--color-background:#f1f2f0;--color-primary:#35342f;--color-accent:#37bbe4;", bodyStyle(t, page))
	require.NotNil(t, readPreviewNotice(t, page))
}

// Safe mode for a page is for the administrator, like a preview: a visitor
// gets the page the site shows.
func TestThemeSafeModeForOnePageIsForTheAdministrator(t *testing.T) {
	customThemeDirectory(t, safeModeConfig)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(define.UserThemeDirectory)) })
	for file, content := range safeModeFiles {
		writeThemeFile(t, file, content)
	}
	store, err := security.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	require.NoError(t, store.Initialize("admin", "legacy", "local"))
	app, err := NewFiberApp(&model.Flags{Port: 5005, CookieName: "settings", Visibility: "DEFAULT", StartupStatus: "ready"}, store)
	require.NoError(t, err)
	token, err := store.Authenticate("admin", "legacy")
	require.NoError(t, err)
	session := &http.Cookie{Name: auth.RequestHandleSessionName("settings", 5005) + "_" + store.Info().InstanceID, Value: token}
	get := func(path string, signedIn bool) string {
		req := httptest.NewRequest(http.MethodGet, "http://flare.test"+path, nil)
		if signedIn {
			req.AddCookie(session)
		}
		res, e := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		require.NoError(t, e)
		raw, e := io.ReadAll(res.Body)
		res.Body.Close()
		require.NoError(t, e)
		require.Equal(t, http.StatusOK, res.StatusCode, path)
		return string(raw)
	}
	for _, path := range []string{"/?theme-safe-mode", "/bookmarks?theme-safe-mode=1"} {
		visitor := get(path, false)
		require.Equal(t, themeFileStyle, bodyStyle(t, visitor), path)
		require.Len(t, pageStyles(t, visitor), 2, path)
		require.Nil(t, readPreviewNotice(t, visitor), path)

		administrator := get(path, true)
		require.Empty(t, bodyStyle(t, administrator), path)
		require.Equal(t, []string{"style"}, pageStyles(t, administrator), path)
		require.NotNil(t, readPreviewNotice(t, administrator), path)
	}
}

// An instance that is started in safe mode does not read the themes
// directory. Its themes are not listed and cannot be selected, previewed or
// imported, and the theme page says why.
func TestThemeSafeModeForTheInstanceHTTP(t *testing.T) {
	customThemeDirectory(t, safeModeConfig)
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(define.UserThemeDirectory)) })
	for file, content := range safeModeFiles {
		writeThemeFile(t, file, content)
	}
	writeThemeFile(t, "rejected.yml", themeFileColors+"  radius: 4em\n")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true, ThemeSafeMode: true})
	require.NoError(t, err)
	stored, err := os.ReadFile("config.yml")
	require.NoError(t, err)

	// The pages have the colors of the stylesheet of the application and
	// the layout of the site.
	for _, path := range []string{"/", "/applications", "/bookmarks", "/help", "/settings/layout"} {
		page := getPage(t, handler, path)
		require.Empty(t, bodyStyle(t, page), path)
		require.Equal(t, []string{"style"}, pageStyles(t, page), path)
	}
	require.Contains(t, bodyClasses(t, getPage(t, handler, "/")), "desktop-columns-4")

	// The theme page lists no theme of the directory and no remark about
	// it, and says that safe mode is on and what that means for the
	// selected theme.
	raw := getPage(t, handler, "/settings/theme")
	page := readThemeFilePage(t, raw)
	require.Empty(t, page.cards)
	require.Empty(t, page.lines)
	require.Equal(t, []string{i18n.Format("en", "theme_safe_mode_selected", map[string]any{"name": "styled"})}, page.alerts)
	require.Contains(t, raw, i18n.T("en", "theme_safe_mode_notice"))
	require.Contains(t, raw, "FLARE_THEME_SAFE_MODE")

	// A theme of the directory cannot be selected or previewed, and its
	// stylesheet is not served.
	for _, name := range []string{"styled", "other", "rejected"} {
		require.Equal(t, http.StatusBadRequest, themeRequest(handler, url.Values{"theme": {name}}).Code, name)
		require.Empty(t, bodyStyle(t, getPage(t, handler, "/?theme-preview="+name)), name)
	}
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/theme/export?name=other", nil))
	require.Equal(t, http.StatusNotFound, rec.Code)

	// No file is imported, with the box for replacing or without it.
	for _, replace := range []bool{false, true} {
		refused := importFile(t, handler, "other.yml", themeFileColors, replace)
		require.Equal(t, http.StatusConflict, refused.Code)
		require.Equal(t, []string{i18n.T("en", "theme_import_safe_mode")}, readImportPage(t, refused.Body.String()).alerts)
	}
	refused := importFile(t, handler, "added.yml", themeFileColors, false)
	require.Equal(t, http.StatusConflict, refused.Code)
	require.NoFileExists(t, filepath.Join(define.UserThemeDirectory, "added.yml"))
	unchanged, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, string(stored), string(unchanged))

	// The notices are given in the interface language.
	require.NoError(t, os.WriteFile("config.yml", []byte(strings.Replace(safeModeConfig, "Locale: en", "Locale: zh", 1)), 0600))
	raw = getPage(t, handler, "/settings/theme")
	require.Contains(t, raw, i18n.T("zh", "theme_safe_mode_notice"))
	require.Equal(t, []string{i18n.Format("zh", "theme_safe_mode_selected", map[string]any{"name": "styled"})}, readThemeFilePage(t, raw).alerts)
	require.NoError(t, os.WriteFile("config.yml", stored, 0600))

	// Bundled themes and custom colors work as always, and with one of them
	// selected the page has nothing to warn about.
	require.Equal(t, http.StatusOK, themeRequest(handler, url.Values{"theme": {"cloud"}}).Code)
	require.Equal(t, "--color-background:#f1f2f0;--color-primary:#35342f;--color-accent:#37bbe4;", bodyStyle(t, getPage(t, handler, "/")))
	page = readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Empty(t, page.alerts)
	require.Contains(t, getPage(t, handler, "/settings/theme"), i18n.T("en", "theme_safe_mode_notice"))
}

// An instance that is not in safe mode does not mention it.
func TestThemePageWithoutSafeMode(t *testing.T) {
	handler := themeFilesRouter(t, safeModeConfig, safeModeFiles)
	raw := getPage(t, handler, "/settings/theme")
	require.NotContains(t, raw, "theme-safe-mode")
	require.NotContains(t, raw, "FLARE_THEME_SAFE_MODE")
	require.Len(t, readThemeFilePage(t, raw).cards, 2)
}
