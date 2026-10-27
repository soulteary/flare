package editor

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
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

func TestEditorSaveFailureRecoveryMetadata(t *testing.T) {
	for _, recoveryRequired := range []bool{false, true} {
		t.Run(map[bool]string{false: "restored", true: "manual recovery"}[recoveryRequired], func(t *testing.T) {
			app := fiber.New()
			cause := errors.New("replacement failed")
			app.Post("/failure", func(c fiber.Ctx) error {
				return editorSaveFailure(c, &data.EditorSaveError{RecoveryPath: ".flare-editor-recovery/save-test", RecoveryRequired: recoveryRequired, Cause: cause})
			})
			rec := httptest.NewRecorder()
			serveTransferHTTP(t, app, rec, httptest.NewRequest(http.MethodPost, "/failure", nil))
			require.Equal(t, http.StatusInternalServerError, rec.Code)
			var result struct {
				Saved            bool   `json:"saved"`
				Code             string `json:"code"`
				RecoveryRequired bool   `json:"recoveryRequired"`
				RecoveryPath     string `json:"recoveryPath"`
				Error            string `json:"error"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &result))
			require.False(t, result.Saved)
			require.Equal(t, recoveryRequired, result.RecoveryRequired)
			require.Equal(t, ".flare-editor-recovery/save-test", result.RecoveryPath)
			require.Contains(t, result.Error, cause.Error())
			require.Equal(t, map[bool]string{false: "save_failed", true: "recovery_required"}[recoveryRequired], result.Code)
		})
	}
}

func TestEditorTrustedRejectsPrivateAndPublicChangesDetected(t *testing.T) {
	t.Chdir(t.TempDir())
	before := define.AppFlags
	defer func() { define.AppFlags = before }()
	define.AppFlags.DisableLoginMode = true
	app := fiber.New()
	registerEditorDocument(app)
	snapshot, err := data.EditorSnapshot()
	require.NoError(t, err)
	snapshot.Apps[0].Private = true
	raw, err := json.Marshal(snapshot)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, "/editor/document", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Flare-Editor", "1")
	rec := httptest.NewRecorder()
	serveTransferHTTP(t, app, rec, req)
	require.Equal(t, 409, rec.Code)
	original, err := data.EditorSnapshot()
	require.NoError(t, err)
	require.False(t, original.Apps[0].Private)
	private := original
	private.Apps = append([]model.Bookmark(nil), original.Apps...)
	private.Apps[0].Private = true
	require.True(t, documentRelaxed(private, original))
	newPublic := original
	newPublic.Apps = append(append([]model.Bookmark(nil), original.Apps...), model.Bookmark{Name: "New", URL: "https://new.example"})
	require.True(t, documentRelaxed(original, newPublic))
	require.False(t, documentRelaxed(original, private))
}

func TestEditorExtensionsRetainedWhenURLChanges(t *testing.T) {
	t.Chdir(t.TempDir())
	require.NoError(t, os.WriteFile("apps.yml", []byte("links:\n- name: App\n  link: https://old.example\n  extension: {nested: retained}\n"), 0600))
	require.NoError(t, os.WriteFile("bookmarks.yml", []byte("links: []\n"), 0600))
	doc, err := data.EditorSnapshot()
	require.NoError(t, err)
	// HTTP JSON is the editor's actual round trip; extension metadata is optional
	// and carries no new required user-facing YAML fields.
	raw, err := json.Marshal(doc)
	require.NoError(t, err)
	var submitted data.EditorDocument
	require.NoError(t, json.Unmarshal(raw, &submitted))
	submitted.Apps[0].URL = "https://changed.example"
	_, err = data.CommitEditorDocument(submitted)
	require.NoError(t, err)
	saved, err := os.ReadFile("apps.yml")
	require.NoError(t, err)
	require.Contains(t, string(saved), "nested: retained")
	require.Contains(t, string(saved), "https://changed.example")
}
