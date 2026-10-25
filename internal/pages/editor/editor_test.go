package editor

import (
	"github.com/gofiber/fiber/v3"
	"github.com/stretchr/testify/require"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLegacyEditorCannotOverwriteWithoutVersion(t *testing.T) {
	t.Chdir(t.TempDir())
	app := fiber.New()
	app.Post("/editor", updateData)
	req := httptest.NewRequest(http.MethodPost, "/editor", strings.NewReader("categories=&bookmarks="))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rec := httptest.NewRecorder()
	serveTransferHTTP(t, app, rec, req)
	require.Equal(t, http.StatusConflict, rec.Code)
	require.Contains(t, rec.Body.String(), `"saved":false`)
}
