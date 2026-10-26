package auth

import (
	"bytes"
	"github.com/stretchr/testify/require"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
)

func TestLoginFormValuePrecedence(t *testing.T) {
	t.Chdir(t.TempDir())
	original := define.AppFlags
	defer func() { define.AppFlags = original }()
	define.AppFlags = model.Flags{
		Port: 5005, CookieName: "flare", CookieSecret: testCookieSecret,
		User: "testuser", Pass: "testpass",
	}
	multipartBody := func(username, password string) (string, string) {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		require.NoError(t, writer.SetBoundary("MiXeD_Login_Boundary"))
		for _, field := range [][2]string{{"username", username}, {"password", password}} {
			require.NoError(t, writer.WriteField(field[0], field[1]))
		}
		require.NoError(t, writer.Close())
		return body.String(), writer.FormDataContentType()
	}
	validMultipart, multipartType := multipartBody("testuser", "testpass")
	invalidMultipart, _ := multipartBody("otheruser", "otherpass")
	const valid = "username=testuser&password=testpass"
	const invalid = "username=otheruser&password=otherpass"
	for _, tc := range []struct {
		name, query, contentType, body string
		status                         int
	}{
		{"urlencoded body beats query", invalid, fiber.MIMEApplicationForm, valid, http.StatusFound},
		{"query cannot bypass wrong body", valid, fiber.MIMEApplicationForm, invalid, http.StatusBadRequest},
		{"empty username cannot fall back", valid, fiber.MIMEApplicationForm, "username=&password=testpass", http.StatusBadRequest},
		{"empty password cannot fall back", valid, fiber.MIMEApplicationForm, "username=testuser&password=", http.StatusBadRequest},
		{"first empty body value wins", valid, fiber.MIMEApplicationForm, "username=&username=testuser&password=testpass", http.StatusBadRequest},
		{"first duplicate body value wins", invalid, fiber.MIMEApplicationForm, valid + "&username=otheruser&password=otherpass", http.StatusFound},
		{"missing body key falls back", "username=testuser", fiber.MIMEApplicationForm, "password=testpass", http.StatusFound},
		{"missing body falls back", valid, fiber.MIMEApplicationForm, "", http.StatusFound},
		{"first empty query value wins", "username=&username=testuser&password=testpass", fiber.MIMEApplicationForm, "", http.StatusBadRequest},
		{"mixed-case urlencoded body", invalid, "Application/X-WWW-Form-Urlencoded; Charset=UTF-8", valid, http.StatusFound},
		{"multipart query beats body", invalid, multipartType, validMultipart, http.StatusBadRequest},
		{"multipart valid query wins", valid, multipartType, invalidMultipart, http.StatusFound},
		{"empty multipart query cannot fall back", "password=", multipartType, validMultipart, http.StatusBadRequest},
		{"mixed-case multipart boundary", "", strings.Replace(multipartType, "multipart/form-data; boundary=", "MuLtIpArT/FoRm-DaTa; BoUnDaRy=", 1), validMultipart, http.StatusFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := fiber.New(fiber.Config{Immutable: true, DisablePreParseMultipartForm: true})
			require.NoError(t, RequestHandle(app))
			app.Get("/protected", AuthRequired, func(c fiber.Ctx) error { return c.SendString(GetUserName(c)) })
			req := httptest.NewRequest(http.MethodPost, define.MiscPages.Login.Path+"?"+tc.query, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", tc.contentType)
			response, err := app.Test(req)
			require.NoError(t, err)
			body, err := io.ReadAll(response.Body)
			response.Body.Close()
			require.NoError(t, err)
			if response.StatusCode != tc.status {
				t.Fatalf("login returned %d %s, want %d", response.StatusCode, body, tc.status)
			}
			cookies := response.Cookies()
			if tc.status != http.StatusFound {
				if len(cookies) != 0 {
					t.Fatal("failed login created a session cookie")
				}
				return
			}
			if len(cookies) != 1 {
				t.Fatalf("successful login returned %d cookies", len(cookies))
			}
			protected := httptest.NewRequest(http.MethodGet, "/protected", nil)
			protected.AddCookie(cookies[0])
			verified, err := app.Test(protected)
			require.NoError(t, err)
			defer verified.Body.Close()
			username, err := io.ReadAll(verified.Body)
			require.NoError(t, err)
			if verified.StatusCode != http.StatusOK || string(username) != "testuser" {
				t.Fatalf("session verification returned %d %q", verified.StatusCode, username)
			}
		})
	}
}
