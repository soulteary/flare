//go:build fiber

package editor

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/bookmarks"
)

func TestTransferHTTP(t *testing.T) {
	t.Chdir(t.TempDir())
	before := define.AppFlags
	defer func() { define.AppFlags = before }()
	define.AppFlags.DisableLoginMode = true
	e := fiber.New(fiber.Config{BodyLimit: 32 << 20})
	registerTransfers(e)
	preview := func(header bool) *httptest.ResponseRecorder {
		var buf bytes.Buffer
		w := multipart.NewWriter(&buf)
		part, err := w.CreateFormFile("file", "bookmarks.html")
		if err != nil {
			t.Fatal(err)
		}
		_, err = part.Write([]byte(`<DL><p><DT><A HREF="https://nas:8443/a?x=1&amp;y=2#x">中文 &amp; &lt;script&gt;</A></DL>`))
		if err != nil {
			t.Fatal(err)
		}
		for k, v := range map[string]string{"format": "html", "collection": "bookmarks", "strategy": "skip"} {
			if err = w.WriteField(k, v); err != nil {
				t.Fatal(err)
			}
		}
		if err = w.Close(); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest(http.MethodPost, transferPath+"/preview", &buf)
		req.Header.Set("Content-Type", w.FormDataContentType())
		if header {
			req.Header.Set("X-Flare-Transfer", "1")
		}
		rec := httptest.NewRecorder()
		serveTransferHTTP(t, e, rec, req)
		return rec
	}
	if rec := preview(false); rec.Code != http.StatusForbidden {
		t.Fatalf("CSRF accepted: %d", rec.Code)
	}
	rec := preview(true)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview: %d %s", rec.Code, rec.Body)
	}
	var response struct {
		Token string         `json:"token"`
		Plan  bookmarks.Plan `json:"plan"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Plan.Added != 1 {
		t.Fatalf("unexpected preview: %+v", response.Plan)
	}
	commit := func(token string) *httptest.ResponseRecorder {
		body, _ := json.Marshal(map[string]string{"token": token})
		req := httptest.NewRequest(http.MethodPost, transferPath+"/commit", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Flare-Transfer", "1")
		rec := httptest.NewRecorder()
		serveTransferHTTP(t, e, rec, req)
		return rec
	}
	if bad := commit(response.Token + "bad"); bad.Code != 400 {
		t.Fatal("tampered ticket accepted")
	}
	original, err := os.ReadFile("bookmarks.yml")
	if err != nil {
		t.Fatal(err)
	}
	if result := commit(response.Token); result.Code != 200 {
		t.Fatalf("commit: %s", result.Body)
	}
	if result := commit(response.Token); result.Code != 409 {
		t.Fatalf("replay: %d %s", result.Code, result.Body)
	}
	backup, _, err := data.LatestBookmarkBackup("bookmarks")
	if err != nil || string(backup) != string(original) {
		t.Fatalf("backup: %v", err)
	}
	for _, format := range []string{"html", "yaml"} {
		req := httptest.NewRequest(http.MethodGet, transferPath+"/export?collection=bookmarks&format="+format, nil)
		rec := httptest.NewRecorder()
		serveTransferHTTP(t, e, rec, req)
		if rec.Code != 200 || !strings.Contains(rec.Header().Get("Content-Disposition"), "attachment") {
			t.Fatalf("export %s: %d", format, rec.Code)
		}
		if _, err = bookmarks.Parse(rec.Body.Bytes(), format); err != nil {
			t.Fatalf("export parse: %v", err)
		}
	}
}

func TestTransferAuthAndTicketExpiry(t *testing.T) {
	t.Chdir(t.TempDir())
	before := define.AppFlags
	defer func() { define.AppFlags = before }()
	define.AppFlags.DisableLoginMode = false
	define.AppFlags.CookieSecret = strings.Repeat("s", 32)
	e := fiber.New(fiber.Config{BodyLimit: 32 << 20})
	if err := auth.RequestHandle(e); err != nil {
		t.Fatal(err)
	}
	registerTransfers(e)
	for _, path := range []string{"", "/export?collection=apps&format=yaml", "/backup?collection=apps"} {
		req := httptest.NewRequest("GET", transferPath+path, nil)
		rec := httptest.NewRecorder()
		serveTransferHTTP(t, e, rec, req)
		if rec.Code != 302 {
			t.Fatalf("unauthenticated route %s: %d", path, rec.Code)
		}
	}
	for _, path := range []string{"preview", "commit"} {
		req := httptest.NewRequest("POST", transferPath+"/"+path, nil)
		req.Header.Set("X-Flare-Transfer", "1")
		rec := httptest.NewRecorder()
		serveTransferHTTP(t, e, rec, req)
		if rec.Code != 302 {
			t.Fatalf("unauthenticated write %s: %d", path, rec.Code)
		}
	}
	token, err := signTicket(transferTicket{User: "alice", Expires: time.Now().Add(time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = readTicket(token, "bob"); err == nil {
		t.Fatal("ticket not bound to user")
	}
	token, err = signTicket(transferTicket{Expires: time.Now().Add(-time.Minute).Unix()})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = readTicket(token, ""); err == nil {
		t.Fatal("expired ticket accepted")
	}
}

func serveTransferHTTP(t *testing.T, app *fiber.App, rec *httptest.ResponseRecorder, req *http.Request) {
	t.Helper()
	res, err := app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	for key, values := range res.Header {
		rec.Header()[key] = values
	}
	rec.WriteHeader(res.StatusCode)
	if _, err := io.Copy(rec, res.Body); err != nil {
		t.Fatal(err)
	}
}
