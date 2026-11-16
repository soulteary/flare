package server

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"html"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/soulteary/flare/internal/themefetch"
)

// themeHost serves files over HTTPS on this machine, and makes the
// application fetch from it: the fetcher trusts its certificate and reaches
// this machine until the test ends. It counts the requests it gets.
type themeHost struct {
	server   *httptest.Server
	files    map[string]string
	requests atomic.Int32
}

func serveThemes(t *testing.T, files map[string]string) *themeHost {
	t.Helper()
	host := &themeHost{files: files}
	host.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		host.requests.Add(1)
		content, known := host.files[r.URL.Path]
		if r.URL.Path == "/around.yml" {
			http.Redirect(w, r, "/around.yml", http.StatusFound)
			return
		}
		if !known {
			http.NotFound(w, r)
			return
		}
		_, err := w.Write([]byte(content))
		require.NoError(t, err)
	}))
	t.Cleanup(host.server.Close)
	pool := x509.NewCertPool()
	pool.AddCert(host.server.Certificate())
	previous, offline := themefetch.Default, define.AppFlags.EnableOfflineMode
	themefetch.Default = &themefetch.Fetcher{Allow: func(netip.Addr) bool { return true }, RootCAs: pool, AnyHost: true}
	define.AppFlags.EnableOfflineMode = false
	t.Cleanup(func() { themefetch.Default, define.AppFlags.EnableOfflineMode = previous, offline })
	return host
}

func sha256Of(content string) string {
	digest := sha256.Sum256([]byte(content))
	return hex.EncodeToString(digest[:])
}

func installLines(t *testing.T, page string) []string {
	t.Helper()
	var lines []string
	rest := page
	start := strings.Index(rest, `<ul class="theme-file-lines theme-install-lines">`)
	if start < 0 {
		return nil
	}
	rest = rest[start:]
	rest = rest[:strings.Index(rest, "</ul>")]
	for _, item := range strings.Split(rest, "<li>")[1:] {
		lines = append(lines, strings.TrimSpace(strings.Split(item, "</li>")[0]))
	}
	return lines
}

func alertOf(page string) string {
	_, rest, found := strings.Cut(page, `<p class=theme-notice role=alert>`)
	if !found {
		return ""
	}
	return html.UnescapeString(strings.Split(rest, "</p>")[0])
}

// A theme is installed from an address in two steps: it is fetched and
// shown with its SHA-256, and stored when that is confirmed, as an upload
// of the same content would be.
func TestInstallThemeFromAnAddressHTTP(t *testing.T) {
	file := themeFileColors + "name: Fetched\n"
	archive := packThemeArchive(t, "theme.yml", themeFileColors+"name: Packed\n", "theme.css", ".app-item { color: red }\n",
		"templates/home.html", themeShellTemplate, "templates/notes.txt", "notes", "LICENSE.txt", "free")
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\nThemeTemplates: true\nThemeSettings:\n    packed:\n        IgnoreLayout: []\n        Templates: true\n", nil)
	host := serveThemes(t, map[string]string{"/themes/fetched.yml": file, "/packed.zip": archive})
	post := func(fields url.Values) (int, string) {
		rec := postThemeForm(t, handler, fields)
		return rec.Code, rec.Body.String()
	}
	stored := func() []string { return themesDirectoryNames(t) }

	// The page has the form, with the address and the box, and no theme
	// that is offered.
	page := getPage(t, handler, "/settings/theme")
	fields, shown := templateForm(t, page, "theme-install-form")
	require.True(t, shown)
	require.Equal(t, map[string]string{"action": "fetch-theme", "theme-address": "", "replace": "", "submit": i18n.T("en", "theme_install_fetch")}, fields)
	_, shown = templateForm(t, page, "theme-install-confirm-form")
	require.False(t, shown)
	require.Zero(t, host.requests.Load())

	// Fetching shows the theme and stores nothing.
	address := host.server.URL + "/themes/fetched.yml"
	status, page := post(url.Values{"action": {"fetch-theme"}, "theme-address": {address}})
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, []string{"Theme: fetched", "Size in bytes: " + strconv.Itoa(len(file)), "SHA-256: " + sha256Of(file), i18n.T("en", "theme_install_offer_file")}, installLines(t, page))
	require.Contains(t, page, i18n.T("en", "theme_install_offer"))
	fields, shown = templateForm(t, page, "theme-install-confirm-form")
	require.True(t, shown)
	require.Equal(t, map[string]string{"action": "install-theme", "theme-address": address, "theme-hash": sha256Of(file), "submit": i18n.T("en", "theme_install_submit")}, fields)
	fields, _ = templateForm(t, page, "theme-install-form")
	require.Equal(t, address, fields["theme-address"], "the form keeps the address")
	require.Empty(t, stored())
	require.EqualValues(t, 1, host.requests.Load())

	// Installing fetches again and stores what was shown.
	confirm := url.Values{"action": {"install-theme"}, "theme-address": {address}, "theme-hash": {sha256Of(file)}}
	status, page = post(confirm)
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, page, strings.ReplaceAll(i18n.T("en", "theme_import_done"), "{name}", "Fetched"))
	require.Equal(t, []string{"fetched.yml"}, stored())
	content, err := os.ReadFile(filepath.Join(define.UserThemeDirectory, "fetched.yml"))
	require.NoError(t, err)
	require.Equal(t, file, string(content))
	require.EqualValues(t, 2, host.requests.Load())

	// A theme of the name is replaced only with the box, as with an upload.
	status, page = post(confirm)
	require.Equal(t, http.StatusConflict, status)
	require.Equal(t, i18n.T("en", "theme_import_exists"), alertOf(page))
	confirm.Set("replace", "1")
	status, _ = post(confirm)
	require.Equal(t, http.StatusOK, status)

	// What the address answers with has changed since it was shown: it is
	// not what was confirmed, and nothing is stored.
	host.files["/themes/fetched.yml"] = file + "version: 2.0.0\n"
	status, page = post(confirm)
	require.Equal(t, http.StatusConflict, status)
	require.Equal(t, i18n.T("en", "theme_install_changed"), alertOf(page))
	content, err = os.ReadFile(filepath.Join(define.UserThemeDirectory, "fetched.yml"))
	require.NoError(t, err)
	require.Equal(t, file, string(content))
	// So is a request that states no hash, or another.
	for _, hash := range []string{"", strings.Repeat("0", 64), sha256Of(file)} {
		status, _ = post(url.Values{"action": {"install-theme"}, "theme-address": {address}, "theme-hash": {hash}, "replace": {"1"}})
		require.Equal(t, http.StatusConflict, status, hash)
	}

	// An archive is shown with what it holds. Its templates are named, and
	// so are the entries that will be left out.
	address = host.server.URL + "/packed.zip"
	status, page = post(url.Values{"action": {"fetch-theme"}, "theme-address": {address}, "replace": {"1"}})
	require.Equal(t, http.StatusOK, status)
	require.Equal(t, []string{
		"Theme: packed", "Size in bytes: " + strconv.Itoa(len(archive)), "SHA-256: " + sha256Of(archive),
		"An archive. Stylesheet: yes. Images and fonts: 0. Text documents: 1. Templates: home.html.",
		i18n.T("en", "theme_install_templates_note"),
		strings.ReplaceAll(i18n.T("en", "theme_install_left_out"), "{files}", "templates/notes.txt"),
	}, installLines(t, page))
	fields, _ = templateForm(t, page, "theme-install-confirm-form")
	require.Equal(t, "1", fields["replace"], "the box is kept for the second step")
	require.NotContains(t, stored(), "packed")
	status, page = post(url.Values{"action": {"install-theme"}, "theme-address": {address}, "theme-hash": {fields["theme-hash"]}, "replace": {"1"}})
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, page, strings.ReplaceAll(i18n.T("en", "theme_import_done"), "{name}", "Packed"))
	require.Contains(t, page, strings.ReplaceAll(i18n.T("en", "theme_import_left_out"), "{files}", "templates/notes.txt"))
	// A confirmation that was stored for the name is withdrawn, as with
	// any import.
	require.Contains(t, page, strings.ReplaceAll(i18n.T("en", "theme_import_withdrawn"), "{themes}", "Packed"))
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.False(t, options.ThemeSettings["packed"].Templates)
	require.FileExists(t, filepath.Join(define.UserThemeDirectory, "packed", "templates", "home.html"))
	require.NoFileExists(t, filepath.Join(define.UserThemeDirectory, "packed", "templates", "notes.txt"))
	// An archive is replaced only with the box, too.
	status, page = post(url.Values{"action": {"install-theme"}, "theme-address": {address}, "theme-hash": {fields["theme-hash"]}})
	require.Equal(t, http.StatusConflict, status)
	require.Equal(t, i18n.T("en", "theme_import_exists"), alertOf(page))
}

// What is not fetched and what is not offered, each with its reason.
func TestInstallThemeFromAnAddressRefusalsHTTP(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\n", nil)
	host := serveThemes(t, map[string]string{
		"/broken.yml": "contract: 1\ntokens: [", "/other.yml": "contract: 2\n", "/large.yml": strings.Repeat("a", define.MaximumThemeFileBytes+1),
		"/notzip.zip": "contract: 1\n", "/empty.zip": packThemeArchive(t, "theme.css", "a{}"), "/tron.yml": themeFileColors, "/Upper.yml": themeFileColors,
	})
	fetch := func(address string) (int, string) {
		rec := postThemeForm(t, handler, url.Values{"action": {"fetch-theme"}, "theme-address": {address}})
		require.NotContains(t, rec.Body.String(), "theme-install-confirm-form", address)
		return rec.Code, alertOf(rec.Body.String())
	}
	base := host.server.URL
	for address, want := range map[string]struct {
		status int
		key    string
	}{
		"": {http.StatusBadRequest, "theme_install_refused_address"},
		"http://" + base[len("https://"):] + "/a.yml":                {http.StatusBadRequest, "theme_install_refused_address"},
		"https://user:secret@example.test/a.yml":                     {http.StatusBadRequest, "theme_install_refused_address"},
		"https://example.test/" + strings.Repeat("a", 2048) + ".yml": {http.StatusBadRequest, "theme_install_refused_address"},
		base + "/theme.tar.gz":                                       {http.StatusBadRequest, "theme_install_refused_name"},
		base + "/tron.yml":                                           {http.StatusBadRequest, "theme_install_refused_name"},
		base + "/Upper.yml":                                          {http.StatusBadRequest, "theme_install_refused_name"},
		base + "/absent.yml":                                         {http.StatusBadGateway, "theme_install_refused_status"},
		base + "/around.yml":                                         {http.StatusBadGateway, "theme_install_refused_redirect"},
		base + "/large.yml":                                          {http.StatusBadRequest, "theme_install_refused_size"},
		base + "/notzip.zip":                                         {http.StatusBadRequest, "theme_import_archive_read"},
		base + "/empty.zip":                                          {http.StatusBadRequest, "theme_import_archive_empty"},
	} {
		status, alert := fetch(address)
		require.Equal(t, want.status, status, address)
		require.Equal(t, i18n.T("en", want.key), alert, address)
	}
	// A file that is no theme file is refused with what a reading of it
	// says, as an upload is.
	status, alert := fetch(base + "/broken.yml")
	require.Equal(t, http.StatusBadRequest, status)
	require.True(t, strings.HasPrefix(alert, "Nothing was imported. "), alert)
	status, alert = fetch(base + "/other.yml")
	require.Equal(t, http.StatusBadRequest, status)
	require.True(t, strings.HasPrefix(alert, "Nothing was imported. "), alert)
	require.Empty(t, themesDirectoryNames(t))

	// The names that are refused are refused before anything is asked for.
	before := host.requests.Load()
	fetch(base + "/tron.yml")
	fetch("https://example.test/theme.txt")
	require.Equal(t, before, host.requests.Load())

	// Without the fetcher of this test, this machine is not reached at all.
	strict := themefetch.Default
	themefetch.Default = &themefetch.Fetcher{RootCAs: strict.RootCAs, AnyHost: true}
	status, alert = fetch(base + "/tron2.yml")
	themefetch.Default = strict
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, i18n.T("en", "theme_install_refused_destination"), alert)
	require.Equal(t, before, host.requests.Load())

	// A host whose certificate is not trusted is not fetched from.
	themefetch.Default = &themefetch.Fetcher{Allow: strict.Allow, AnyHost: true}

	// The fetcher of the application takes a host by its name only: the
	// server of this test, which has a number for a host, is not asked.
	previous := themefetch.Default
	themefetch.Default = &themefetch.Fetcher{RootCAs: strict.RootCAs}
	status, alert = fetch(base + "/tron2.yml")
	themefetch.Default = previous
	require.Equal(t, http.StatusBadRequest, status)
	require.Equal(t, i18n.T("en", "theme_install_refused_address"), alert)
	require.Equal(t, before, host.requests.Load())
	status, alert = fetch(base + "/tron2.yml")
	themefetch.Default = strict
	require.Equal(t, http.StatusBadGateway, status)
	require.Equal(t, i18n.T("en", "theme_install_refused_failed"), alert)
}

// An instance in offline mode fetches nothing, and one in safe mode stores
// no theme: the page says so in place of the form, and a request that is
// sent all the same is refused before anything is asked for.
func TestInstallThemeFromAnAddressIsUnavailableHTTP(t *testing.T) {
	handler := themeFilesRouter(t, "Locale: en\nTheme: blackboard\n", nil)
	host := serveThemes(t, map[string]string{"/fetched.yml": themeFileColors})
	address := host.server.URL + "/fetched.yml"
	for key, set := range map[string]func(on bool){
		"theme_install_offline":  func(on bool) { define.AppFlags.EnableOfflineMode = on },
		"theme_import_safe_mode": func(on bool) { define.AppFlags.ThemeSafeMode = on },
	} {
		set(true)
		page := getPage(t, handler, "/settings/theme")
		require.Contains(t, page, `<p class="theme-help theme-install-unavailable">`+i18n.T("en", key)+`</p>`, key)
		fields, shown := templateForm(t, page, "theme-install-form")
		require.True(t, shown, key)
		require.Equal(t, map[string]string{"action": "fetch-theme"}, fields, key)
		for _, action := range []string{"fetch-theme", "install-theme"} {
			rec := postThemeForm(t, handler, url.Values{"action": {action}, "theme-address": {address}, "theme-hash": {sha256Of(themeFileColors)}})
			require.Equal(t, http.StatusConflict, rec.Code, key+" "+action)
			require.Equal(t, i18n.T("en", key), alertOf(rec.Body.String()), key+" "+action)
			require.NotContains(t, rec.Body.String(), "theme-install-confirm-form")
		}
		set(false)
	}
	require.Zero(t, host.requests.Load())
	require.Empty(t, themesDirectoryNames(t))
	rec := postThemeForm(t, handler, url.Values{"action": {"fetch-theme"}, "theme-address": {address}})
	require.Equal(t, http.StatusOK, rec.Code)
	require.EqualValues(t, 1, host.requests.Load())
}
