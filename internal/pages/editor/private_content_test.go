package editor

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
	"github.com/stretchr/testify/require"
)

// With login disabled, no editor or transfer endpoint may return a collection
// that contains private entries, and each one says why.
func TestEditorAndTransferRefusePrivateContentWithoutLogin(t *testing.T) {
	t.Chdir(t.TempDir())
	before := define.AppFlags
	t.Cleanup(func() {
		define.AppFlags = before
		data.RequireLoginForPrivateContent(false)
	})
	define.AppFlags.DisableLoginMode = true
	require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "PUBLIC-APP-SENTINEL", URL: "https://app.example"}}}))
	require.True(t, data.SaveNormalBookmarks(model.Bookmarks{Items: []model.Bookmark{
		{Name: "PUBLIC-BOOKMARK-SENTINEL", URL: "https://public.example"},
		{Name: "PRIVATE-BOOKMARK-SENTINEL", URL: "https://private.example", Private: true},
	}}))
	// An import made while login was enabled left a backup of this state.
	require.NoError(t, os.Mkdir(".flare-bookmark-backups", 0700))
	live, err := os.ReadFile("bookmarks.yml")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(".flare-bookmark-backups", "bookmarks-20260101T000000.000000000-fixture.yml"), live, 0600))
	data.RequireLoginForPrivateContent(true)

	app := fiber.New(fiber.Config{BodyLimit: 32 << 20, DisablePreParseMultipartForm: true})
	registerEditorDocument(app)
	registerTransfers(app)
	serve := func(req *http.Request) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		serveTransferHTTP(t, app, rec, req)
		return rec
	}
	refused := func(rec *httptest.ResponseRecorder, where string) {
		t.Helper()
		require.Equal(t, http.StatusConflict, rec.Code, where+": "+rec.Body.String())
		var payload struct{ Code, Error string }
		require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &payload), where)
		require.Equal(t, privateContentCode, payload.Code, where)
		require.Equal(t, i18n.T(editorLocale(), "navigation.private_requires_login"), payload.Error, where)
		for _, sentinel := range []string{"PRIVATE-BOOKMARK-SENTINEL", "PUBLIC-BOOKMARK-SENTINEL", "private.example", "public.example"} {
			require.NotContains(t, rec.Body.String(), sentinel, where)
		}
	}

	refused(serve(httptest.NewRequest(http.MethodGet, "/editor/document", nil)), "document")

	save := httptest.NewRequest(http.MethodPost, "/editor/document", bytes.NewReader([]byte(`{"version":"any","categories":[],"apps":[],"bookmarks":[]}`)))
	save.Header.Set("Content-Type", "application/json")
	save.Header.Set("X-Flare-Editor", "1")
	refused(serve(save), "save")

	for _, format := range []string{"yaml", "html"} {
		refused(serve(httptest.NewRequest(http.MethodGet, transferPath+"/export?collection=bookmarks&format="+format, nil)), "export "+format)
	}
	refused(serve(httptest.NewRequest(http.MethodGet, transferPath+"/backup?collection=bookmarks", nil)), "backup")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "import.yml")
	require.NoError(t, err)
	_, err = part.Write([]byte("links:\n- name: Imported\n  link: https://imported.example\n"))
	require.NoError(t, err)
	for key, value := range map[string]string{"format": "yaml", "collection": "bookmarks", "strategy": "skip"} {
		require.NoError(t, writer.WriteField(key, value))
	}
	require.NoError(t, writer.Close())
	preview := httptest.NewRequest(http.MethodPost, transferPath+"/preview", &body)
	preview.Header.Set("Content-Type", writer.FormDataContentType())
	preview.Header.Set("X-Flare-Transfer", "1")
	refused(serve(preview), "import preview")

	// The collection without private entries remains exportable.
	rec := serve(httptest.NewRequest(http.MethodGet, transferPath+"/export?collection=apps&format=yaml", nil))
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	require.Contains(t, rec.Body.String(), "PUBLIC-APP-SENTINEL")

	after, err := os.ReadFile("bookmarks.yml")
	require.NoError(t, err)
	require.Equal(t, live, after)
}
