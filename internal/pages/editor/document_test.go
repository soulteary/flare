package editor

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/stretchr/testify/require"
)

func TestEditorDocumentHTTP(t *testing.T) {
	t.Chdir(t.TempDir())
	before := define.AppFlags
	defer func() { define.AppFlags = before }()
	define.AppFlags.DisableLoginMode = true
	app := fiber.New()
	registerEditorDocument(app)
	snapshot, err := data.EditorSnapshot()
	require.NoError(t, err)
	post := func(doc any, header bool) *httptest.ResponseRecorder {
		b, marshalErr := json.Marshal(doc)
		require.NoError(t, marshalErr)
		req := httptest.NewRequest(http.MethodPost, "/editor/document", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		if header {
			req.Header.Set("X-Flare-Editor", "1")
		}
		rec := httptest.NewRecorder()
		serveTransferHTTP(t, app, rec, req)
		return rec
	}
	require.Equal(t, 403, post(snapshot, false).Code)
	snapshot.Categories[0].Name = "Renamed"
	require.Equal(t, 200, post(snapshot, true).Code)
	require.Equal(t, 409, post(snapshot, true).Code)
	fresh, err := data.EditorSnapshot()
	require.NoError(t, err)
	fresh.Bookmarks[0].Category = "missing"
	rec := post(fresh, true)
	require.Equal(t, 422, rec.Code)
	require.Contains(t, rec.Body.String(), "bookmarks.0.Category")
	define.AppFlags.DisableLoginMode = false
	req := httptest.NewRequest(http.MethodGet, "/editor/document", nil)
	rec = httptest.NewRecorder()
	serveTransferHTTP(t, app, rec, req)
	require.NotEqual(t, 200, rec.Code)
}
