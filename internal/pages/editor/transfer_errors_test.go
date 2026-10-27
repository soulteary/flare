package editor

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/bookmarks"
	"github.com/stretchr/testify/require"
)

func TestTransferRejectsMalformedUploadsAndTickets(t *testing.T) {
	t.Chdir(t.TempDir())
	before := define.AppFlags
	t.Cleanup(func() { define.AppFlags = before })
	define.AppFlags.DisableLoginMode = true
	app := fiber.New(fiber.Config{BodyLimit: 32 << 20, DisablePreParseMultipartForm: true})
	registerTransfers(app)
	request := func(path string, raw []byte, contentType string, cross bool) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, transferPath+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", contentType)
		req.Header.Set("X-Flare-Transfer", "1")
		if cross {
			req.Header.Set("Sec-Fetch-Site", "cross-site")
		}
		rec := httptest.NewRecorder()
		serveTransferHTTP(t, app, rec, req)
		return rec
	}
	require.Equal(t, 403, request("/commit", []byte(`{}`), "application/json", true).Code)
	require.Equal(t, 400, request("/commit", []byte(`{`), "application/json", false).Code)
	require.Equal(t, 400, request("/preview", []byte("broken"), "multipart/form-data; boundary=missing", false).Code)
	for _, tc := range []struct {
		name, raw, collection, strategy string
		file                            bool
		want                            int
	}{
		{name: "missing file", collection: "bookmarks", want: 400},
		{name: "invalid format", raw: "invalid", collection: "bookmarks", file: true, want: 400},
		{name: "invalid collection", raw: "links: []\n", collection: "unknown", file: true, want: 400},
		{name: "private without login", raw: "links:\n- name: Private\n  link: https://private.example\n  private: true\n", collection: "bookmarks", file: true, want: 409},
		{name: "invalid strategy", raw: "links: []\n", collection: "bookmarks", strategy: "bad", file: true, want: 400},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var body bytes.Buffer
			writer := multipart.NewWriter(&body)
			if tc.file {
				part, err := writer.CreateFormFile("file", "bookmarks.yml")
				require.NoError(t, err)
				_, err = part.Write([]byte(tc.raw))
				require.NoError(t, err)
			}
			for k, v := range map[string]string{"format": "yaml", "collection": tc.collection, "strategy": tc.strategy} {
				require.NoError(t, writer.WriteField(k, v))
			}
			require.NoError(t, writer.Close())
			rec := request("/preview", body.Bytes(), writer.FormDataContentType(), false)
			require.Equal(t, tc.want, rec.Code, rec.Body.String())
		})
	}
	_, version, err := data.BookmarkSnapshot("bookmarks")
	require.NoError(t, err)
	for _, tc := range []struct {
		name   string
		ticket transferTicket
		want   int
	}{
		{"invalid source", transferTicket{Raw: []byte("["), Format: "yaml", Collection: "bookmarks"}, 400},
		{"invalid collection", transferTicket{Raw: []byte("links: []\n"), Format: "yaml", Collection: "invalid"}, 500},
		{"stale preview", transferTicket{Raw: []byte("links: []\n"), Format: "yaml", Collection: "bookmarks", Version: "stale"}, 409},
		{"invalid merge strategy", transferTicket{Raw: []byte("links: []\n"), Format: "yaml", Collection: "bookmarks", Version: version, Options: bookmarks.Options{Strategy: "invalid"}}, 400},
		{"private merge", transferTicket{Raw: []byte("links:\n- name: Private\n  link: https://private.example\n  private: true\n"), Format: "yaml", Collection: "bookmarks", Version: version, Options: bookmarks.Options{Strategy: "skip"}}, 409},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.ticket.Expires = time.Now().Add(time.Minute).Unix()
			token, signErr := signTicket(tc.ticket)
			require.NoError(t, signErr)
			raw, marshalErr := json.Marshal(map[string]string{"token": token})
			require.NoError(t, marshalErr)
			rec := request("/commit", raw, "application/json", false)
			require.Equal(t, tc.want, rec.Code, rec.Body.String())
		})
	}
	for _, token := range []string{"missing-separator", "%.00", base64.RawURLEncoding.EncodeToString([]byte("{}")) + ".zz"} {
		_, ticketErr := readTicket(token, "")
		require.Error(t, ticketErr)
	}
	// A valid MAC must not make malformed JSON into a valid import capability.
	raw := []byte("{")
	mac := hmac.New(sha256.New, transferKey[:])
	_, err = mac.Write(raw)
	require.NoError(t, err)
	_, err = readTicket(base64.RawURLEncoding.EncodeToString(raw)+"."+hex.EncodeToString(mac.Sum(nil)), "")
	require.Error(t, err)
}

func TestTransferBackupDownloadsAndErrors(t *testing.T) {
	t.Chdir(t.TempDir())
	before := define.AppFlags
	t.Cleanup(func() { define.AppFlags = before })
	define.AppFlags.DisableLoginMode = true
	app := fiber.New()
	registerTransfers(app)
	get := func(path string) *httptest.ResponseRecorder {
		rec := httptest.NewRecorder()
		serveTransferHTTP(t, app, rec, httptest.NewRequest("GET", transferPath+path, nil))
		return rec
	}
	require.Equal(t, 404, get("/backup?collection=bookmarks").Code)
	require.Equal(t, 500, get("/backup?collection=invalid").Code)
	require.Equal(t, 400, get("/export?collection=invalid&format=yaml").Code)
	require.Equal(t, 400, get("/export?collection=bookmarks&format=invalid").Code)
	current, version, err := data.BookmarkSnapshot("bookmarks")
	require.NoError(t, err)
	original, err := os.ReadFile("bookmarks.yml")
	require.NoError(t, err)
	require.NoError(t, data.CommitBookmarkImport("bookmarks", version, current))
	rec := get("/backup?collection=bookmarks")
	require.Equal(t, 200, rec.Code, rec.Body.String())
	require.Equal(t, string(original), rec.Body.String())
	require.Equal(t, "no-store", rec.Header().Get("Cache-Control"))
	require.Contains(t, rec.Header().Get("Content-Disposition"), "before-import-")
}
