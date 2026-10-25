package formvalue

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestGetMatchesHTTPFormValue(t *testing.T) {
	multipartBody := func(values ...string) (string, string) {
		t.Helper()
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		if err := writer.SetBoundary("MiXeD_Boundary"); err != nil {
			t.Fatal(err)
		}
		for _, value := range values {
			if err := writer.WriteField("value", value); err != nil {
				t.Fatal(err)
			}
		}
		if err := writer.Close(); err != nil {
			t.Fatal(err)
		}
		return body.String(), writer.FormDataContentType()
	}
	multipartValue, multipartType := multipartBody("multipart", "second")
	multipartEmpty, _ := multipartBody("", "second")
	mixedMultipartType := strings.Replace(multipartType, "multipart/form-data; boundary=", "MuLtIpArT/FoRm-DaTa; BoUnDaRy=", 1)
	for _, tc := range []struct {
		name, method, query, contentType, body, want string
	}{
		{"POST body first", "POST", "value=query", fiber.MIMEApplicationForm, "value=body", "body"},
		{"PUT body first", "PUT", "value=query", fiber.MIMEApplicationForm, "value=body", "body"},
		{"PATCH body first", "PATCH", "value=query", fiber.MIMEApplicationForm, "value=body", "body"},
		{"GET ignores urlencoded body", "GET", "value=query", fiber.MIMEApplicationForm, "value=body", "query"},
		{"DELETE ignores urlencoded body", "DELETE", "value=query", fiber.MIMEApplicationForm, "value=body", "query"},
		{"mixed-case urlencoded type", "POST", "value=query", "Application/X-WWW-Form-Urlencoded; Charset=UTF-8", "value=body", "body"},
		{"explicit empty body", "POST", "value=query", fiber.MIMEApplicationForm, "value=", ""},
		{"bare body key", "POST", "value=query", fiber.MIMEApplicationForm, "value", ""},
		{"repeated body first", "POST", "value=query", fiber.MIMEApplicationForm, "value=first&value=second", "first"},
		{"repeated body first empty", "POST", "value=query", fiber.MIMEApplicationForm, "value=&value=second", ""},
		{"body key missing", "POST", "value=query", fiber.MIMEApplicationForm, "other=body", "query"},
		{"body missing", "POST", "value=query", fiber.MIMEApplicationForm, "", "query"},
		{"repeated query first", "POST", "value=first&value=second", fiber.MIMEApplicationForm, "", "first"},
		{"repeated query first empty", "POST", "value=&value=second", fiber.MIMEApplicationForm, "", ""},
		{"query before multipart", "POST", "value=query", multipartType, multipartValue, "query"},
		{"empty query before multipart", "POST", "value=", multipartType, multipartValue, ""},
		{"multipart first value", "POST", "", multipartType, multipartValue, "multipart"},
		{"multipart first empty", "POST", "", multipartType, multipartEmpty, ""},
		{"mixed-case multipart type and boundary name", "POST", "", mixedMultipartType, multipartValue, "multipart"},
		{"JSON ignored", "POST", "value=query", fiber.MIMEApplicationJSON, `{"value":"body"}`, "query"},
		{"missing field", "POST", "", fiber.MIMEApplicationForm, "other=body", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request := func() *http.Request {
				r := httptest.NewRequest(tc.method, "/?"+tc.query, strings.NewReader(tc.body))
				r.Header.Set("Content-Type", tc.contentType)
				return r
			}
			standard := request()
			if got := standard.FormValue("value"); got != tc.want {
				t.Fatalf("net/http oracle returned %q, want %q", got, tc.want)
			}
			if standard.MultipartForm != nil {
				defer standard.MultipartForm.RemoveAll()
			}
			app := fiber.New(fiber.Config{Immutable: true, DisablePreParseMultipartForm: true})
			app.All("/", func(c fiber.Ctx) error { return c.SendString(Get(c, "value")) })
			response, err := app.Test(request())
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != http.StatusOK || string(body) != tc.want {
				t.Fatalf("Fiber returned %d %q, want 200 %q", response.StatusCode, body, tc.want)
			}
		})
	}
}
