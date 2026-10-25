package editor

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestEditorSaveReportsResult(t *testing.T) {
	t.Chdir(t.TempDir())
	e := fiber.New()
	e.Post("/editor", updateData)
	for _, tc := range []struct {
		name, categories, bookmarks string
		status                      int
	}{
		{"save", "1,测试分类", "1,测试书签,https://example.com,测试分类,,描述", http.StatusOK},
		{"malformed CSV", "\"unterminated", "", http.StatusInternalServerError},
		{"empty collections", "", "", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			form := url.Values{"categories": {tc.categories}, "bookmarks": {tc.bookmarks}}
			req := httptest.NewRequest(http.MethodPost, "/editor", strings.NewReader(form.Encode()))
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			req.Header.Set("Accept", "application/json")
			rec := httptest.NewRecorder()
			serveTransferHTTP(t, e, rec, req)
			if rec.Code != tc.status {
				t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
			}
			want := `"saved":true`
			if tc.status != http.StatusOK {
				want = `"saved":false`
			}
			if !strings.Contains(rec.Body.String(), want) {
				t.Fatalf("unexpected response: %s", rec.Body.String())
			}
		})
	}
	t.Run("write failure", func(t *testing.T) {
		if err := os.Remove("apps.yml"); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir("apps.yml", 0700); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, "/editor", strings.NewReader(url.Values{"categories": {""}, "bookmarks": {""}}.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")
		rec := httptest.NewRecorder()
		serveTransferHTTP(t, e, rec, req)
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), `"saved":false`) {
			t.Fatalf("write failure reported as success: %d %s", rec.Code, rec.Body.String())
		}
	})

}
