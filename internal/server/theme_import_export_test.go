package server

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/html"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/security"
)

// importBody is the upload of the import form, with the token if there is one.
func importBody(t *testing.T, filename, content, token string, replace bool) (*bytes.Buffer, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	require.NoError(t, writer.WriteField("action", "import"))
	if token != "" {
		require.NoError(t, writer.WriteField("csrf-token", token))
	}
	if replace {
		require.NoError(t, writer.WriteField("replace", "1"))
	}
	if filename != "" {
		part, err := writer.CreateFormFile("theme-file", filename)
		require.NoError(t, err)
		_, err = part.Write([]byte(content))
		require.NoError(t, err)
	}
	require.NoError(t, writer.Close())
	return &body, writer.FormDataContentType()
}

func importFile(t *testing.T, handler http.Handler, filename, content string, replace bool) *httptest.ResponseRecorder {
	t.Helper()
	body, contentType := importBody(t, filename, content, "", replace)
	req := httptest.NewRequest(http.MethodPost, "/settings/theme", body)
	req.Header.Set("Content-Type", contentType)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	return rec
}

// The parts of the theme page about importing and exporting.
type importPage struct {
	// form holds the fields of the import form by name, with their types.
	form     map[string]string
	encoding string
	alerts   []string
	status   []string
	// downloads holds the addresses of the download links, in order.
	downloads []string
}

func readImportPage(t *testing.T, page string) importPage {
	t.Helper()
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	read := importPage{form: map[string]string{}}
	walkNodes(document, func(node *html.Node) {
		if node.Type != html.ElementNode {
			return
		}
		switch {
		case nodeHasClass(node, "theme-import-form"):
			require.Equal(t, "POST", nodeAttribute(node, "method"))
			read.encoding = nodeAttribute(node, "enctype")
			walkNodes(node, func(part *html.Node) {
				switch {
				case part.Type == html.ElementNode && part.Data == "input":
					read.form[nodeAttribute(part, "name")] = nodeAttribute(part, "type")
				case nodeHasClass(part, "theme-notice") && nodeAttribute(part, "role") == "alert":
					read.alerts = append(read.alerts, nodeText(part))
				case nodeHasClass(part, "theme-notice") && nodeAttribute(part, "role") == "status":
					read.status = append(read.status, nodeText(part))
				}
			})
		case node.Data == "a" && strings.Contains(nodeAttribute(node, "href"), "/export?"):
			require.Equal(t, i18n.T("en", "theme_export"), nodeText(node))
			read.downloads = append(read.downloads, nodeAttribute(node, "href"))
		}
	})
	return read
}

func TestThemeImportAndExportHTTP(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\n", nil)
	require.NoError(t, os.Remove(define.UserThemeDirectory))
	stored, err := os.ReadFile("config.yml")
	require.NoError(t, err)

	// The page has the form and a download link for every theme.
	page := readImportPage(t, getPage(t, handler, "/settings/theme"))
	require.Equal(t, "multipart/form-data", page.encoding)
	require.Equal(t, map[string]string{"action": "hidden", "theme-file": "file", "replace": "checkbox", "": "submit"}, page.form)
	require.Empty(t, page.alerts)
	require.Empty(t, page.status)
	downloads := make([]string, 0, len(define.ThemePalettes))
	for _, theme := range define.ThemePalettes {
		downloads = append(downloads, "/settings/theme/export?name="+theme.Name)
	}
	require.Equal(t, downloads, page.downloads)

	// A bundled theme is downloaded as a theme file and imported under the
	// name the download has. The page then lists it; nothing is selected.
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/theme/export?name=tron", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, `attachment; filename="tron-copy.yml"`, rec.Header().Get("Content-Disposition"))
	require.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"))
	exported := rec.Body.String()

	imported := importFile(t, handler, "tron-copy.yml", exported, false)
	require.Equal(t, http.StatusOK, imported.Code)
	page = readImportPage(t, imported.Body.String())
	require.Equal(t, []string{i18n.Format("en", "theme_import_done", map[string]any{"name": "tron-copy"})}, page.status)
	require.Empty(t, page.alerts)
	require.Equal(t, append(downloads, "/settings/theme/export?name=tron-copy"), page.downloads)
	cards := readThemeFilePage(t, imported.Body.String()).cards
	require.Len(t, cards, 1)
	require.Equal(t, "tron-copy", cards[0].value)
	require.Contains(t, imported.Body.String(), blackboardThemeStyle, "importing selected the theme")
	raw, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, string(stored), string(raw))
	onDisk, err := os.ReadFile(filepath.Join(define.UserThemeDirectory, "tron-copy.yml"))
	require.NoError(t, err)
	require.Equal(t, exported, string(onDisk))

	// The imported theme is downloaded as the file that was stored, and can
	// be previewed and selected like any theme file.
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/theme/export?name=tron-copy", nil))
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, `attachment; filename="tron-copy.yml"`, rec.Header().Get("Content-Disposition"))
	require.Equal(t, exported, rec.Body.String())
	require.Equal(t, "--color-background:#242B33;--color-primary:#EFFBFF;--color-accent:#6EE2FF;", bodyStyle(t, getPage(t, handler, "/?theme-preview=tron-copy")))

	// Every refusal says why and that nothing was imported, changes nothing,
	// and is shown as text. A file that is not a usable theme is refused with
	// the reason the list of unused files gives for such a file.
	rejected := func(reason string) string {
		return i18n.Format("en", "theme_import_rejected", map[string]any{"reason": i18n.T("en", reason)})
	}
	for name, tc := range map[string]struct {
		filename, content string
		replace           bool
		status            int
		text              string
	}{
		"no file":         {"", "", false, http.StatusBadRequest, i18n.T("en", "theme_import_missing")},
		"extension":       {"forest.yaml", themeFileColors, false, http.StatusBadRequest, i18n.T("en", "theme_import_name")},
		"bundled name":    {"tron.yml", themeFileColors, false, http.StatusBadRequest, i18n.T("en", "theme_import_name")},
		"existing":        {"tron-copy.yml", themeFileColors, false, http.StatusConflict, i18n.T("en", "theme_import_exists")},
		"too large":       {"forest.yml", themeFileColors + "#" + strings.Repeat("x", define.MaximumThemeFileBytes), false, http.StatusBadRequest, i18n.T("en", "theme_import_size")},
		"value":           {"forest.yml", themeFileColors + "  radius: 4em\n", false, http.StatusBadRequest, rejected("theme_file_problem_value") + " tokens.radius"},
		"value in a mode": {"forest.yml", themeFileColors + "modes:\n  light:\n    icon: red\n", false, http.StatusBadRequest, rejected("theme_file_problem_value") + " modes.light.icon"},
		"contrast":        {"forest.yml", "contract: 1\ntokens:\n  background: \"#202020\"\n  primary: \"#2a2a2a\"\n  accent: \"#f2b84b\"\n", false, http.StatusBadRequest, rejected("theme_file_problem_contrast")},
		"no contract":     {"forest.yml", strings.TrimPrefix(themeFileColors, "contract: 1\n"), false, http.StatusBadRequest, rejected("theme_file_problem_contract")},
	} {
		refused := importFile(t, handler, tc.filename, tc.content, tc.replace)
		require.Equal(t, tc.status, refused.Code, name)
		page = readImportPage(t, refused.Body.String())
		require.Equal(t, []string{tc.text}, page.alerts, name)
		require.Contains(t, page.alerts[0], "Nothing was imported.", name)
		require.Empty(t, page.status, name)
		entries, readErr := os.ReadDir(define.UserThemeDirectory)
		require.NoError(t, readErr)
		require.Len(t, entries, 1, "%s: the directory changed", name)
		onDisk, err = os.ReadFile(filepath.Join(define.UserThemeDirectory, "tron-copy.yml"))
		require.NoError(t, err)
		require.Equal(t, exported, string(onDisk), name)
	}

	// The message of the parser is part of a refusal, and it is text.
	refused := importFile(t, handler, "forest.yml", "contract: \"<script>alert(1)</script>\"\ntokens: {}\n", false)
	require.Equal(t, http.StatusBadRequest, refused.Code)
	page = readImportPage(t, refused.Body.String())
	require.Len(t, page.alerts, 1)
	require.True(t, strings.HasPrefix(page.alerts[0], "Nothing was imported. "+i18n.T("en", "theme_file_problem_syntax")+" "), page.alerts[0])
	require.Contains(t, page.alerts[0], "`<script")
	require.NotContains(t, refused.Body.String(), "`<script")
	require.Contains(t, refused.Body.String(), "`&lt;script")

	// With the box ticked, a theme file of the same name is replaced.
	replaced := importFile(t, handler, "tron-copy.yml", themeFileColors+"name: Replaced\n", true)
	require.Equal(t, http.StatusOK, replaced.Code)
	page = readImportPage(t, replaced.Body.String())
	require.Equal(t, []string{i18n.Format("en", "theme_import_done", map[string]any{"name": "Replaced"})}, page.status)
	require.Equal(t, themeFileStyle, bodyStyle(t, getPage(t, handler, "/?theme-preview=tron-copy")))

	// What is not a theme in use cannot be downloaded.
	require.NoError(t, os.WriteFile("secret.yml", []byte("Secret: value\n"), 0600))
	for _, name := range []string{"", "missing", "../secret", "../config", "tron-copy.yml"} {
		rec = httptest.NewRecorder()
		handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/settings/theme/export?name="+name, nil))
		require.Equal(t, http.StatusNotFound, rec.Code, "%q", name)
		require.NotContains(t, rec.Body.String(), "Secret", "%q", name)
	}
}

// Importing writes into the data directory and exporting reads from it:
// both are for the administrator, and an import needs the token of the
// session like every other change of the settings.
func TestThemeImportAndExportRequireTheAdministrator(t *testing.T) {
	customThemeDirectory(t, "Locale: en\nTheme: blackboard\n")
	t.Cleanup(func() { require.NoError(t, os.RemoveAll(define.UserThemeDirectory)) })
	store, err := security.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { store.Close() })
	require.NoError(t, store.Initialize("admin", "legacy", "local"))
	app, err := NewFiberApp(&model.Flags{Port: 5005, CookieName: "settings", Visibility: "DEFAULT", StartupStatus: "ready"}, store)
	require.NoError(t, err)
	token, err := store.Authenticate("admin", "legacy")
	require.NoError(t, err)
	session := &http.Cookie{Name: auth.RequestHandleSessionName("settings", 5005) + "_" + store.Info().InstanceID, Value: token}
	send := func(req *http.Request, signedIn bool) (int, string) {
		if signedIn {
			req.AddCookie(session)
		}
		res, e := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		require.NoError(t, e)
		raw, e := io.ReadAll(res.Body)
		res.Body.Close()
		require.NoError(t, e)
		return res.StatusCode, string(raw)
	}
	upload := func(csrf string, signedIn bool, origin string) int {
		body, contentType := importBody(t, "forest.yml", themeFileColors, csrf, false)
		req := httptest.NewRequest(http.MethodPost, "http://flare.test/settings/theme", body)
		req.Header.Set("Content-Type", contentType)
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		status, _ := send(req, signedIn)
		return status
	}

	// The token of the import form.
	status, page := send(httptest.NewRequest(http.MethodGet, "http://flare.test/settings/theme", nil), true)
	require.Equal(t, http.StatusOK, status)
	document, err := html.Parse(strings.NewReader(page))
	require.NoError(t, err)
	csrf := ""
	walkNodes(document, func(node *html.Node) {
		if nodeHasClass(node, "theme-import-form") {
			walkNodes(node, func(part *html.Node) {
				if part.Type == html.ElementNode && part.Data == "input" && nodeAttribute(part, "name") == "csrf-token" {
					csrf = nodeAttribute(part, "value")
				}
			})
		}
	})
	require.NotEmpty(t, csrf, "the import form carries no token")

	require.Equal(t, http.StatusUnauthorized, upload(csrf, false, ""), "no session")
	require.Equal(t, http.StatusForbidden, upload("", true, ""), "no token")
	require.Equal(t, http.StatusForbidden, upload(csrf, true, "https://evil.example"), "foreign origin")
	_, statErr := os.Stat(define.UserThemeDirectory)
	require.ErrorIs(t, statErr, os.ErrNotExist, "a refused request wrote into the data directory")
	require.Equal(t, http.StatusOK, upload(csrf, true, ""))
	require.FileExists(t, filepath.Join(define.UserThemeDirectory, "forest.yml"))

	// A download needs the session as well, for a bundled theme too.
	for _, name := range []string{"forest", "tron"} {
		status, body := send(httptest.NewRequest(http.MethodGet, "http://flare.test/settings/theme/export?name="+name, nil), false)
		require.NotEqual(t, http.StatusOK, status, name)
		require.NotContains(t, body, "contract: 1", name)
		status, body = send(httptest.NewRequest(http.MethodGet, "http://flare.test/settings/theme/export?name="+name, nil), true)
		require.Equal(t, http.StatusOK, status, name)
		require.Contains(t, body, "contract: 1", name)
	}
}
