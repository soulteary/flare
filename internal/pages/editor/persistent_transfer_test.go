package editor

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/bookmarks"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

func TestPersistentTransferRequiresConfirmationAndCSRF(t *testing.T) {
	t.Chdir(t.TempDir())
	before := define.AppFlags
	t.Cleanup(func() { define.AppFlags = before })
	define.AppFlags.DisableLoginMode = false
	store, err := security.Open(t.TempDir())
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.Initialize("admin", "a sufficiently long password", "local"))
	app := fiber.New()
	require.NoError(t, auth.RequestHandle(app, store))
	registerTransfers(app)
	registerEditorDocument(app)
	app.Get("/test-csrf", func(c fiber.Ctx) error { return c.SendString(auth.CSRFToken(c)) })
	cookies := map[string]*http.Cookie{}
	request := func(method, path string, body []byte, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://flare.test"+path, bytes.NewReader(body))
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		serveTransferHTTP(t, app, rec, req)
		for _, cookie := range rec.Result().Cookies() {
			cookies[cookie.Name] = cookie
		}
		return rec
	}
	csrf := request("GET", "/test-csrf", nil, nil).Body.String()
	login := url.Values{"username": {"admin"}, "password": {"a sufficiently long password"}, "csrf-token": {csrf}}
	require.Equal(t, 303, request("POST", "/login", []byte(login.Encode()), map[string]string{"Content-Type": "application/x-www-form-urlencoded"}).Code)
	csrf = request("GET", "/test-csrf", nil, nil).Body.String()
	for _, path := range []string{"preview", "commit"} {
		require.Equal(t, 403, request("POST", transferPath+"/"+path, []byte(`{}`), map[string]string{"X-Flare-Transfer": "1"}).Code)
	}
	require.Equal(t, 403, request("POST", "/editor/document", []byte(`{}`), map[string]string{"X-Flare-Editor": "1"}).Code)
	for _, path := range []string{"export?collection=bookmarks&format=yaml", "backup?collection=bookmarks"} {
		require.Equal(t, 409, request("GET", transferPath+"/"+path, nil, nil).Code)
	}
	rec := request("GET", transferPath+"/export?collection=bookmarks&format=yaml&confirm=1", nil, nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "links:")
	require.Equal(t, 404, request("GET", transferPath+"/backup?collection=bookmarks&confirm=1", nil, nil).Code)
	snapshot, err := data.EditorSnapshot()
	require.NoError(t, err)
	snapshot.Apps[0].Private = true
	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)
	headers := map[string]string{"Content-Type": "application/json", "X-Flare-Editor": "1", "X-CSRF-Token": csrf}
	rec = request("POST", "/editor/document", raw, headers)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	snapshot, err = data.EditorSnapshot()
	require.NoError(t, err)
	snapshot.Apps[0].Private = false
	raw, err = json.Marshal(snapshot)
	require.NoError(t, err)
	rec = request("POST", "/editor/document", raw, headers)
	require.Equal(t, 409, rec.Code)
	require.Contains(t, rec.Body.String(), "PUBLIC_CONFIRMATION_REQUIRED")
	headers["X-Flare-Confirm-Public"] = "1"
	rec = request("POST", "/editor/document", raw, headers)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	// Public imports use a session-bound preview and recheck authorization in the commit.
	app.Get("/test-binding", func(c fiber.Ctx) error { return c.SendString(auth.SessionBinding(c)) })
	binding := request("GET", "/test-binding", nil, nil).Body.String()
	_, version, err := data.BookmarkSnapshot("bookmarks")
	require.NoError(t, err)
	token, err := signTicket(transferTicket{Raw: []byte("links:\n- name: Public\n  link: https://public.example\n"), Format: "yaml", Collection: "bookmarks", Version: version, Options: bookmarks.Options{Strategy: "skip", Public: true}, User: binding, Expires: time.Now().Add(15 * time.Minute).Unix()})
	require.NoError(t, err)
	raw, err = json.Marshal(map[string]string{"token": token})
	require.NoError(t, err)
	rec = request("POST", transferPath+"/commit", raw, map[string]string{"Content-Type": "application/json", "X-Flare-Transfer": "1", "X-CSRF-Token": csrf})
	require.Equal(t, 200, rec.Code, rec.Body.String())
	rec = request("GET", transferPath+"/backup?collection=bookmarks&confirm=1", nil, nil)
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Contains(t, rec.Header().Get("Content-Disposition"), "before-import-")
	// Revoked cookies must never release an export, even with explicit confirmation.
	require.NoError(t, store.RevokeAll())
	rec = request("GET", transferPath+"/export?collection=bookmarks&format=yaml&confirm=1", nil, nil)
	require.NotEqual(t, 200, rec.Code)
}

func TestEditorDocumentRejectsInvalidRequests(t *testing.T) {
	t.Chdir(t.TempDir())
	before := define.AppFlags
	t.Cleanup(func() { define.AppFlags = before })
	define.AppFlags.DisableLoginMode = true
	app := fiber.New()
	registerEditorDocument(app)
	for _, raw := range []string{`{`, `{"unknown":true}`, `{} {}`} {
		req := httptest.NewRequest("POST", "/editor/document", strings.NewReader(raw))
		req.Header.Set("X-Flare-Editor", "1")
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		serveTransferHTTP(t, app, rec, req)
		require.Equal(t, 400, rec.Code)
	}
	res, err := app.Test(httptest.NewRequest("GET", "/editor/csrf", nil))
	require.NoError(t, err)
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.JSONEq(t, `{"csrfToken":""}`, string(raw))
}
