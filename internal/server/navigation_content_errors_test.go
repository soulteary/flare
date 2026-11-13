package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/cmd"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/stretchr/testify/require"
)

const contentErrorSettings = `ShowApps: true
ShowBookmarks: true
ShowSearchComponent: true
ShowWeather: false
Locale: en
Theme: blackboard
Search:
  Mode: hybrid
  DefaultTarget: example
  OpenNewTab: false
  Engines:
    - ID: example
      Name: Example
      URLTemplate: https://search.example/?q={query}
      Enabled: true
`

// Templates are minified in release builds, so attribute quotes are optional.
var navigationAlert = regexp.MustCompile(`class="?navigation-data-error"? role="?alert"?`)

// contentErrorInstance runs the real startup gate and HTTP stack in a new data
// directory. Navigation files are written after the process-wide caches left by
// earlier tests are invalidated, so every request parses the fixture.
func contentErrorInstance(t *testing.T, inputs map[string]string, apps, bookmarks string) (*fiber.App, cmd.SecurityPreparation, *model.Flags) {
	t.Helper()
	original := define.AppFlags
	t.Cleanup(func() {
		define.AppFlags = original
		data.RequireLoginForPrivateContent(false)
	})
	t.Setenv("FLARE_BASELINE", "1")
	require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{}))
	require.True(t, data.SaveNormalBookmarks(model.Bookmarks{}))
	require.NoError(t, os.WriteFile("apps.yml", []byte(apps), 0600))
	require.NoError(t, os.WriteFile("bookmarks.yml", []byte(bookmarks), 0600))
	require.NoError(t, os.WriteFile("config.yml", []byte(contentErrorSettings), 0600))
	flags := &model.Flags{Port: 5005, CookieName: "content", User: define.DEFAULT_USER_NAME, Visibility: define.DEFAULT_VISIBILITY, DisableLoginMode: define.DEFAULT_DISABLE_LOGIN, EnableEditor: true, EnableOfflineMode: true, RuntimeInputs: inputs}
	prepared := cmd.PrepareSecurity(flags)
	require.Equal(t, "ready", prepared.Status, prepared.Message)
	t.Cleanup(func() { _ = prepared.Store.Close() })
	app, err := NewFiberApp(flags, prepared.Store)
	require.NoError(t, err)
	return app, prepared, flags
}

func contentErrorRequest(t *testing.T, app *fiber.App, method, path, body string, cookies ...*http.Cookie) (int, http.Header, string) {
	t.Helper()
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, "http://flare.test"+path, reader)
	if body != "" {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	for _, cookie := range cookies {
		req.AddCookie(cookie)
	}
	response, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
	require.NoError(t, err)
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, response.Header, string(raw)
}

func TestUndecodableApplicationsKeepInstanceAndBookmarksAvailable(t *testing.T) {
	t.Chdir(t.TempDir())
	app, _, _ := contentErrorInstance(t, map[string]string{},
		"links: [broken-apps-sentinel",
		"links:\n  - name: HEALTHY-BOOKMARK-SENTINEL\n    link: https://healthy.example.com/\n")
	status, _, body := contentErrorRequest(t, app, http.MethodGet, "/ping", "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, "pong")

	applicationsError := i18n.T("en", "navigation.applications_error")
	for _, path := range []string{"/", "/?search=healthy"} {
		status, _, body = contentErrorRequest(t, app, http.MethodGet, path, "")
		require.Equal(t, http.StatusOK, status, path)
		require.Contains(t, body, applicationsError, path)
		require.Len(t, navigationAlert.FindAllString(body, -1), 1, "only the applications module reports an error: %s", path)
		require.Contains(t, body, "HEALTHY-BOOKMARK-SENTINEL", path)
		require.NotContains(t, body, "broken-apps-sentinel", path)
		require.NotContains(t, body, "yaml:", path)
	}
	status, _, body = contentErrorRequest(t, app, http.MethodPost, "/", "search=healthy&target=local")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, "HEALTHY-BOOKMARK-SENTINEL")
	require.Contains(t, body, applicationsError)

	status, _, body = contentErrorRequest(t, app, http.MethodGet, "/applications", "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, applicationsError)
	status, _, body = contentErrorRequest(t, app, http.MethodGet, "/bookmarks", "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, "HEALTHY-BOOKMARK-SENTINEL")
	require.Empty(t, navigationAlert.FindAllString(body, -1))

	// Redirects to entries of the readable collection keep working.
	status, headers, _ := contentErrorRequest(t, app, http.MethodGet, "/redir/url?go="+data.Base64EncodeUrl("https://healthy.example.com/"), "")
	require.Equal(t, http.StatusFound, status)
	require.Equal(t, "https://healthy.example.com/", headers.Get("Location"))

	// Suggestions and the editor report the failure without exposing file content.
	status, _, body = contentErrorRequest(t, app, http.MethodPost, "/search/suggest", "search=healthy")
	require.Equal(t, http.StatusInternalServerError, status)
	require.NotContains(t, body, "broken-apps-sentinel")
	status, _, body = contentErrorRequest(t, app, http.MethodGet, "/editor/document", "")
	require.Equal(t, http.StatusInternalServerError, status)
	require.NotContains(t, body, "broken-apps-sentinel")
	require.NotContains(t, body, "HEALTHY-BOOKMARK-SENTINEL")
}

const privateWithoutLoginBookmarks = `categories:
  - id: open
    title: OPEN-CATEGORY-SENTINEL
  - id: secret
    title: PRIVATE-CATEGORY-SENTINEL
    private: true
links:
  - name: PUBLIC-BOOKMARK-SENTINEL
    link: https://public-bookmark.example.com/
    category: open
  - name: PRIVATE-ITEM-SENTINEL
    link: https://private-item.example.com/
    category: open
    private: true
  - name: PRIVATE-INHERITED-SENTINEL
    link: https://private-inherited.example.com/
    category: secret
`

var privateWithoutLoginSecrets = []string{
	"PRIVATE-ITEM-SENTINEL", "PRIVATE-INHERITED-SENTINEL", "PRIVATE-CATEGORY-SENTINEL",
	"private-item.example.com", "private-inherited.example.com",
	// The collection is withheld as a whole, including its public entries.
	"PUBLIC-BOOKMARK-SENTINEL", "public-bookmark.example.com", "OPEN-CATEGORY-SENTINEL",
}

func TestPrivateEntriesAreWithheldWhileLoginIsDisabled(t *testing.T) {
	t.Chdir(t.TempDir())
	apps := "links:\n  - name: PUBLIC-APP-SENTINEL\n    link: https://public-app.example.com/\n"
	app, prepared, _ := contentErrorInstance(t, map[string]string{}, apps, privateWithoutLoginBookmarks)
	require.True(t, prepared.Store.Info().Initialized)
	withheld := i18n.T("en", "navigation.private_requires_login")
	absent := func(body, where string) {
		t.Helper()
		for _, secret := range privateWithoutLoginSecrets {
			require.NotContains(t, body, secret, where)
		}
	}

	status, _, body := contentErrorRequest(t, app, http.MethodGet, "/ping", "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, "pong")

	for _, path := range []string{"/", "/?search=SENTINEL", "/?category=open", "/?category=secret"} {
		status, _, body = contentErrorRequest(t, app, http.MethodGet, path, "")
		absent(body, path)
		if strings.Contains(path, "category=") {
			// A category of a withheld collection is indistinguishable from a missing one.
			require.Equal(t, http.StatusNotFound, status, path)
			continue
		}
		require.Equal(t, http.StatusOK, status, path)
		require.Contains(t, body, withheld, path)
		require.Contains(t, body, "PUBLIC-APP-SENTINEL", path)
	}
	status, _, body = contentErrorRequest(t, app, http.MethodPost, "/", "search=SENTINEL&target=local")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, withheld)
	absent(body, "POST /")

	status, _, body = contentErrorRequest(t, app, http.MethodGet, "/bookmarks", "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, withheld)
	absent(body, "/bookmarks")
	status, _, body = contentErrorRequest(t, app, http.MethodGet, "/applications", "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, "PUBLIC-APP-SENTINEL")
	require.NotContains(t, body, withheld)

	status, _, body = contentErrorRequest(t, app, http.MethodPost, "/search/suggest", "search=SENTINEL")
	require.Equal(t, http.StatusInternalServerError, status)
	absent(body, "/search/suggest")

	for _, link := range []string{"https://private-item.example.com/", "https://private-inherited.example.com/", "https://public-bookmark.example.com/"} {
		redirectStatus, redirectHeaders, redirectBody := contentErrorRequest(t, app, http.MethodGet, "/redir/url?go="+data.Base64EncodeUrl(link), "")
		require.Equal(t, http.StatusNotFound, redirectStatus, link)
		require.Empty(t, redirectHeaders.Get("Location"), link)
		absent(redirectBody, link)
	}
	status, headers, _ := contentErrorRequest(t, app, http.MethodGet, "/redir/url?go="+data.Base64EncodeUrl("https://public-app.example.com/"), "")
	require.Equal(t, http.StatusFound, status)
	require.Equal(t, "https://public-app.example.com/", headers.Get("Location"))

	status, _, body = contentErrorRequest(t, app, http.MethodGet, "/editor/document", "")
	require.Equal(t, http.StatusConflict, status)
	require.Contains(t, body, `"code":"PRIVATE_CONTENT_REQUIRES_LOGIN"`)
	absent(body, "/editor/document")
	require.NotContains(t, body, "PUBLIC-APP-SENTINEL", "the editor document is refused as a whole")

	for _, format := range []string{"yaml", "html"} {
		path := "/editor/transfer/export?collection=bookmarks&format=" + format
		status, _, body = contentErrorRequest(t, app, http.MethodGet, path, "")
		require.Equal(t, http.StatusConflict, status, path)
		require.Contains(t, body, `"code":"PRIVATE_CONTENT_REQUIRES_LOGIN"`, path)
		absent(body, path)
	}
	status, _, body = contentErrorRequest(t, app, http.MethodGet, "/editor/transfer/export?collection=apps&format=yaml", "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, "PUBLIC-APP-SENTINEL")

	raw, err := os.ReadFile("bookmarks.yml")
	require.NoError(t, err)
	require.Equal(t, privateWithoutLoginBookmarks, string(raw), "withholding never rewrites the file")
}

func TestPrivateEntriesReturnOnceLoginIsEnabled(t *testing.T) {
	t.Chdir(t.TempDir())
	apps := "links:\n  - name: PUBLIC-APP-SENTINEL\n    link: https://public-app.example.com/\n"
	app, prepared, _ := contentErrorInstance(t, map[string]string{}, apps, privateWithoutLoginBookmarks)
	status, _, body := contentErrorRequest(t, app, http.MethodGet, "/", "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, i18n.T("en", "navigation.private_requires_login"))
	require.NoError(t, prepared.Store.Close())

	// Restart the same data directory with login enabled.
	password := "an administrator phrase for enabling login"
	app, prepared, flags := contentErrorInstance(t, map[string]string{"FLARE_DISABLE_LOGIN": "false", "FLARE_PASS": password}, apps, privateWithoutLoginBookmarks)
	require.False(t, flags.DisableLoginMode)

	status, _, body = contentErrorRequest(t, app, http.MethodGet, "/", "")
	require.Equal(t, http.StatusOK, status)
	require.Contains(t, body, "PUBLIC-BOOKMARK-SENTINEL")
	require.Contains(t, body, "PUBLIC-APP-SENTINEL")
	require.Empty(t, navigationAlert.FindAllString(body, -1))
	for _, secret := range []string{"PRIVATE-ITEM-SENTINEL", "PRIVATE-INHERITED-SENTINEL", "PRIVATE-CATEGORY-SENTINEL"} {
		require.NotContains(t, body, secret, "anonymous visitors see the public view")
	}

	token, err := prepared.Store.Authenticate(define.DEFAULT_USER_NAME, password)
	require.NoError(t, err)
	cookie := &http.Cookie{Name: auth.RequestHandleSessionName(flags.CookieName, flags.Port) + "_" + prepared.Store.Info().InstanceID, Value: token}
	status, _, body = contentErrorRequest(t, app, http.MethodGet, "/", "", cookie)
	require.Equal(t, http.StatusOK, status)
	for _, expected := range []string{"PUBLIC-BOOKMARK-SENTINEL", "PRIVATE-ITEM-SENTINEL", "PRIVATE-INHERITED-SENTINEL", "PRIVATE-CATEGORY-SENTINEL"} {
		require.Contains(t, body, expected)
	}
}
