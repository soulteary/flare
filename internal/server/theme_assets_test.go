package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
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

// Files that start as a PNG image and as a WOFF2 font do.
const (
	themeAssetPNG   = "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR-PICTURE-OF-THE-THEME"
	themeAssetWOFF2 = "wOF2\x00\x01\x00\x00-FONT-OF-THE-THEME"
)

const themeAssetStylesheet = "@font-face { font-family: Brand; src: url(assets/brand.woff2) format(\"woff2\") }\n" +
	".app-item { font-family: Brand, sans-serif; background: url(\"assets/paper.png\") }\n"

// themeStylesheetOf returns the address of the stylesheet a page links, and
// the hash in it.
func themeStylesheetOf(t *testing.T, page string) (string, string) {
	t.Helper()
	styles := pageStyles(t, page)
	require.Len(t, styles, 2)
	require.Regexp(t, themeStylesheetAddress, styles[1])
	return styles[1], strings.Split(styles[1], "/")[3]
}

// A theme that is a directory can bring images and fonts in its assets. The
// stylesheet refers to them, and they are served beside it.
func TestThemeAssetsHTTP(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: styled\nShowApps: true\n", map[string]string{
		"styled/theme.yml":          themeFileColors,
		"styled/theme.css":          themeAssetStylesheet,
		"styled/assets/paper.png":   themeAssetPNG,
		"styled/assets/brand.woff2": themeAssetWOFF2,
		// Files of the directory that are not assets.
		"styled/assets/notes.txt":       "NOTES-OF-THE-AUTHOR",
		"styled/assets/logo.svg":        `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`,
		"styled/assets/.hidden.png":     themeAssetPNG,
		"styled/assets/more/nested.png": themeAssetPNG,
		"styled/preview.png":            themeAssetPNG,
		// A theme with assets and no stylesheet.
		"other/theme.yml":        themeFileColors,
		"other/assets/b.png":     themeAssetPNG + "-OTHER",
		"plain.yml":              themeFileColors,
		"markup/theme.yml":       themeFileColors,
		"markup/assets/<b>x.png": themeAssetPNG,
		"wrong/theme.yml":        themeFileColors,
		"wrong/assets/a.png":     "<!doctype html><script>alert(1)</script>",
		"missing/theme.yml":      themeFileColors,
		"missing/theme.css":      ".app-item { background: url(assets/a.png) }\n",
		"missing/assets/b.png":   themeAssetPNG,
	})
	require.NoError(t, os.WriteFile("secret.png", []byte(themeAssetPNG+"-SECRET"), 0600))

	// The pages link the stylesheet, and nothing else of the theme: a
	// browser asks for the files the stylesheet refers to.
	home := getPage(t, handler, "/")
	address, hash := themeStylesheetOf(t, home)
	require.Equal(t, themeAssetStylesheet, getStylesheet(handler, http.MethodGet, address).Body.String())
	require.NotContains(t, home, "/assets/paper.png")
	base := "/theme/styled/" + hash + "/assets/"

	// A file of the assets is served beside the stylesheet as it is, under
	// the type its name stands for and nothing else, and for good: another
	// state of the theme has another address.
	for name, want := range map[string][2]string{
		"paper.png":   {"image/png", themeAssetPNG},
		"brand.woff2": {"font/woff2", themeAssetWOFF2},
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			served := getStylesheet(handler, method, base+name)
			require.Equal(t, http.StatusOK, served.Code, name)
			require.Equal(t, want[0], served.Header().Get("Content-Type"), name)
			require.Equal(t, "nosniff", served.Header().Get("X-Content-Type-Options"), name)
			require.Equal(t, "public, max-age=31536000, immutable", served.Header().Get("Cache-Control"), name)
			require.Equal(t, strconv.Itoa(len(want[1])), served.Header().Get("Content-Length"), name)
			require.Empty(t, served.Header().Get("Set-Cookie"), name)
			if method == http.MethodGet {
				require.Equal(t, want[1], served.Body.String(), name)
			} else {
				require.Empty(t, served.Body.String(), name)
			}
		}
		require.Equal(t, http.StatusMethodNotAllowed, getStylesheet(handler, http.MethodPost, base+name).Code, name)
	}
	// A copy that is still held is confirmed without sending the content,
	// and a part of a file is sent on request.
	tag := getStylesheet(handler, http.MethodGet, base+"paper.png").Header().Get("ETag")
	require.NotEmpty(t, tag)
	conditional := httptest.NewRequest(http.MethodGet, base+"paper.png", nil)
	conditional.Header.Set("If-None-Match", tag)
	confirmed := httptest.NewRecorder()
	handler.ServeHTTP(confirmed, conditional)
	require.Equal(t, http.StatusNotModified, confirmed.Code)
	require.Empty(t, confirmed.Body.String())
	ranged := httptest.NewRequest(http.MethodGet, base+"paper.png", nil)
	ranged.Header.Set("Range", "bytes=1-3")
	part := httptest.NewRecorder()
	handler.ServeHTTP(part, ranged)
	require.Equal(t, http.StatusPartialContent, part.Code)
	require.Equal(t, "PNG", part.Body.String())

	// A name can have hyphens, underscores and more than one dot.
	writeThemeFile(t, "other/assets/Brand-Regular_2.v1.woff2", themeAssetWOFF2)
	define.ReloadUserThemes()

	// The assets of a theme without a stylesheet are served as well, under
	// the hash of that theme.
	themes, _, _ := define.UserThemes()
	otherHash := ""
	for _, theme := range themes {
		if theme.Name == "other" {
			otherHash = theme.Hash
		}
	}
	require.NotEmpty(t, otherHash)
	require.Equal(t, themeAssetPNG+"-OTHER", getStylesheet(handler, http.MethodGet, "/theme/other/"+otherHash+"/assets/b.png").Body.String())
	named := getStylesheet(handler, http.MethodGet, "/theme/other/"+otherHash+"/assets/Brand-Regular_2.v1.woff2")
	require.Equal(t, http.StatusOK, named.Code)
	require.Equal(t, "font/woff2", named.Header().Get("Content-Type"))
	require.Equal(t, themeAssetWOFF2, named.Body.String())

	// Nothing else is served there: no other hash, no file of the directory
	// that is not an asset, no file of another theme or of a theme that is
	// not in use, and no file from anywhere else.
	for _, wrong := range []string{
		"/theme/styled/" + strings.Repeat("0", 16) + "/assets/paper.png",
		"/theme/styled/" + strings.ToUpper(hash) + "/assets/paper.png",
		"/theme/styled/" + otherHash + "/assets/paper.png",
		"/theme/styled/assets/paper.png",
		"/theme/other/" + hash + "/assets/b.png",
		"/theme/other/" + otherHash + "/assets/paper.png",
		"/theme/styled/" + hash + "/assets/b.png",
		base + "notes.txt",
		base + "logo.svg",
		base + ".hidden.png",
		base + "more/nested.png",
		base + "more",
		base + "nested.png",
		base,
		"/theme/styled/" + hash + "/assets",
		"/theme/styled/" + hash + "/preview.png",
		base + "paper.png/",
		base + "paper.png/x",
		base + "PAPER.PNG",
		base + "paper.PNG",
		base + "theme.css",
		base + "..%2Ftheme.yml",
		base + "%2e%2e%2Ftheme.yml",
		base + "..%5Ctheme.yml",
		base + "%2e%2e/theme.yml",
		base + "..%2F..%2F..%2Fsecret.png",
		base + "..%2F..%2Fother%2Fassets%2Fb.png",
		base + "paper.png%00",
		base + "paper.png%20",
		"/theme/..%2Fsecret/" + hash + "/assets/paper.png",
		"/theme/plain/" + hash + "/assets/paper.png",
		"/theme/blackboard/" + hash + "/assets/paper.png",
		"/theme/wrong/" + hash + "/assets/a.png",
		"/theme/missing/" + hash + "/assets/b.png",
		"/theme/markup/" + hash + "/assets/%3Cb%3Ex.png",
	} {
		served := getStylesheet(handler, http.MethodGet, wrong)
		require.Equal(t, http.StatusNotFound, served.Code, wrong)
		for _, kept := range []string{"SECRET", "NOTES", "alert(1)", "Theme:", "PICTURE", "FONT-OF", "contract"} {
			require.NotContains(t, served.Body.String(), kept, wrong)
		}
		require.Empty(t, served.Header().Get("Cache-Control"), wrong)
	}

	// The theme page names a file that is not accepted and the reason, as
	// text, and the theme it belongs to is not offered.
	raw := getPage(t, handler, "/settings/theme")
	page := readThemeFilePage(t, raw)
	offered := make([]string, 0, len(page.cards))
	for _, card := range page.cards {
		offered = append(offered, card.value)
	}
	require.Equal(t, []string{"other", "plain", "styled"}, offered)
	require.Equal(t, []string{
		"markup/assets/<b>x.png " + i18n.T("en", "theme_file_problem_asset_name"),
		"missing/theme.css " + i18n.T("en", "theme_file_problem_style_asset") + " line 1: url(assets/a.png)",
		"wrong/assets/a.png " + i18n.T("en", "theme_file_problem_asset_content"),
	}, page.lineTexts)
	require.NotContains(t, raw, "<b>x.png")
	require.Zero(t, page.elements["b"])

	// A preview of a theme takes its stylesheet with the files it refers to.
	require.NoError(t, os.WriteFile("config.yml", []byte("Locale: en\nTheme: blackboard\nShowApps: true\n"), 0600))
	define.UpdatePagePalettes()
	require.Equal(t, []string{"style"}, pageStyles(t, getPage(t, handler, "/")))
	previewed := pageStyles(t, getPage(t, handler, "/?theme-preview=styled"))
	require.Equal(t, []string{"style", address}, previewed)
	require.Equal(t, themeAssetPNG, getStylesheet(handler, http.MethodGet, base+"paper.png").Body.String())
	require.Equal(t, []string{"style"}, pageStyles(t, getPage(t, handler, "/?theme-preview=other")))
	require.NoError(t, os.WriteFile("config.yml", []byte("Locale: en\nTheme: styled\nShowApps: true\n"), 0600))
	define.UpdatePagePalettes()

	// A file that is edited is taken by the next page: the stylesheet has
	// another address, the file is served under it, and the earlier
	// addresses are gone.
	edited := themeAssetPNG + "-EDITED"
	writeThemeFile(t, "styled/assets/paper.png", edited)
	second, secondHash := themeStylesheetOf(t, getPage(t, handler, "/"))
	require.NotEqual(t, address, second)
	require.Equal(t, edited, getStylesheet(handler, http.MethodGet, "/theme/styled/"+secondHash+"/assets/paper.png").Body.String())
	require.Equal(t, themeAssetWOFF2, getStylesheet(handler, http.MethodGet, "/theme/styled/"+secondHash+"/assets/brand.woff2").Body.String())
	require.Equal(t, http.StatusNotFound, getStylesheet(handler, http.MethodGet, address).Code)
	require.Equal(t, http.StatusNotFound, getStylesheet(handler, http.MethodGet, base+"paper.png").Code)
	require.Equal(t, http.StatusNotFound, getStylesheet(handler, http.MethodGet, base+"brand.woff2").Code)

	// A file that is not accepted leaves the pages without the theme: the
	// colors of the stylesheet of the application, no stylesheet of the
	// theme, and none of its files. The theme page names the file.
	writeThemeFile(t, "styled/assets/paper.png", "<!doctype html><title>not a picture</title>")
	for _, path := range []string{"/", "/applications", "/bookmarks", "/help"} {
		body := getPage(t, handler, path)
		require.Equal(t, []string{"style"}, pageStyles(t, body), path)
		require.Empty(t, bodyStyle(t, body), path)
	}
	require.Equal(t, http.StatusNotFound, getStylesheet(handler, http.MethodGet, second).Code)
	require.Equal(t, http.StatusNotFound, getStylesheet(handler, http.MethodGet, "/theme/styled/"+secondHash+"/assets/paper.png").Code)
	require.Equal(t, http.StatusNotFound, getStylesheet(handler, http.MethodGet, "/theme/styled/"+secondHash+"/assets/brand.woff2").Code)
	page = readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Equal(t, []string{i18n.Format("en", "theme_unavailable", map[string]any{"name": "styled"})}, page.alerts)
	require.Contains(t, page.lineTexts, "styled/assets/paper.png "+i18n.T("en", "theme_file_problem_asset_content"))

	// A file the stylesheet refers to that is removed does the same, and
	// putting it back brings the theme back.
	require.NoError(t, os.Remove(filepath.Join(define.UserThemeDirectory, "styled", "assets", "paper.png")))
	require.Equal(t, []string{"style"}, pageStyles(t, getPage(t, handler, "/")))
	page = readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Contains(t, page.lineTexts, "styled/theme.css "+i18n.T("en", "theme_file_problem_style_asset")+" line 2: url(assets/paper.png)")
	writeThemeFile(t, "styled/assets/paper.png", themeAssetPNG)
	body := getPage(t, handler, "/")
	third, thirdHash := themeStylesheetOf(t, body)
	require.NotEqual(t, second, third)
	require.Equal(t, themeFileStyle, bodyStyle(t, body))
	require.Equal(t, themeAssetPNG, getStylesheet(handler, http.MethodGet, "/theme/styled/"+thirdHash+"/assets/paper.png").Body.String())
	require.Empty(t, readThemeFilePage(t, getPage(t, handler, "/settings/theme")).alerts)

	// The reasons are given in the interface language.
	require.NoError(t, os.WriteFile("config.yml", []byte("Locale: zh\nTheme: styled\nShowApps: true\n"), 0600))
	page = readThemeFilePage(t, getPage(t, handler, "/settings/theme"))
	require.Equal(t, []string{
		"markup/assets/<b>x.png " + i18n.T("zh", "theme_file_problem_asset_name"),
		"missing/theme.css " + i18n.T("zh", "theme_file_problem_style_asset") + " line 1: url(assets/a.png)",
		"wrong/assets/a.png " + i18n.T("zh", "theme_file_problem_asset_content"),
	}, page.lineTexts)
}

// The files of a theme are for whoever the pages that use them are for:
// anyone on an instance that is open to visitors, and the administrator alone
// on a private one.
func TestThemeAssetsFollowTheVisibilityOfThePages(t *testing.T) {
	for visibility, open := range map[string]bool{"DEFAULT": true, "PRIVATE": false} {
		t.Run(visibility, func(t *testing.T) {
			customThemeDirectory(t, "Locale: en\nTheme: styled\n")
			t.Cleanup(func() { require.NoError(t, os.RemoveAll(define.UserThemeDirectory)) })
			writeThemeFile(t, "styled/theme.yml", themeFileColors)
			writeThemeFile(t, "styled/theme.css", themeAssetStylesheet)
			writeThemeFile(t, "styled/assets/paper.png", themeAssetPNG)
			writeThemeFile(t, "styled/assets/brand.woff2", themeAssetWOFF2)
			store, err := security.Open(t.TempDir())
			require.NoError(t, err)
			t.Cleanup(func() { store.Close() })
			require.NoError(t, store.Initialize("admin", "legacy", "local"))
			app, err := NewFiberApp(&model.Flags{Port: 5005, CookieName: "settings", Visibility: visibility, StartupStatus: "ready"}, store)
			require.NoError(t, err)
			token, err := store.Authenticate("admin", "legacy")
			require.NoError(t, err)
			session := &http.Cookie{Name: auth.RequestHandleSessionName("settings", 5005) + "_" + store.Info().InstanceID, Value: token}
			send := func(method, path string, signedIn bool) (*http.Response, string) {
				req := httptest.NewRequest(method, "http://flare.test"+path, nil)
				if signedIn {
					req.AddCookie(session)
				}
				res, e := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
				require.NoError(t, e)
				raw, e := io.ReadAll(res.Body)
				res.Body.Close()
				require.NoError(t, e)
				return res, string(raw)
			}

			res, home := send(http.MethodGet, "/", true)
			require.Equal(t, http.StatusOK, res.StatusCode)
			_, hash := themeStylesheetOf(t, home)
			for name, content := range map[string]string{"paper.png": themeAssetPNG, "brand.woff2": themeAssetWOFF2} {
				address := "/theme/styled/" + hash + "/assets/" + name
				res, body := send(http.MethodGet, address, true)
				require.Equal(t, http.StatusOK, res.StatusCode, name)
				require.Equal(t, content, body, name)
				for _, method := range []string{http.MethodGet, http.MethodHead} {
					res, body = send(method, address, false)
					if open {
						require.Equal(t, http.StatusOK, res.StatusCode, name)
						require.Equal(t, "public, max-age=31536000, immutable", res.Header.Get("Cache-Control"), name)
						continue
					}
					require.NotEqual(t, http.StatusOK, res.StatusCode, name)
					require.NotContains(t, body, "OF-THE-THEME", name)
					require.NotContains(t, res.Header.Get("Cache-Control"), "immutable", name)
				}
				if !open {
					res, _ = send(http.MethodGet, address, true)
					require.Equal(t, "private, max-age=31536000, immutable", res.Header.Get("Cache-Control"), name)
				}
			}
		})
	}
}
