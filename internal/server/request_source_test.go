package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestWriteSourceProtection(t *testing.T) {
	for _, tc := range []struct {
		name, origin, referer, site string
		want                        int
	}{
		{name: "headerless API", want: 200},
		{name: "same origin", origin: "http://flare.example:8080", site: "same-origin", want: 200},
		{name: "TLS proxy original host", origin: "https://flare.example:8080", referer: "https://flare.example:8080/settings", site: "same-origin", want: 200},
		{name: "cross site", site: "cross-site", want: 403},
		{name: "same site sibling", origin: "https://sibling.example:8080", site: "same-site", want: 403},
		{name: "foreign origin without metadata", origin: "https://evil.example", want: 403},
		{name: "foreign referer", referer: "https://evil.example/", want: 403},
		{name: "opaque origin", origin: "null", want: 403},
		{name: "origin userinfo", origin: "http://user@flare.example:8080", want: 403},
		{name: "origin path", origin: "http://flare.example:8080/path", want: 403},
		{name: "wrong public port", origin: "http://flare.example", want: 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writes := 0
			app := fiber.New()
			app.Use(protectWriteSource)
			app.Post("/write", func(c fiber.Ctx) error { writes++; return c.SendStatus(200) })
			req := httptest.NewRequest("POST", "http://flare.example:8080/write", strings.NewReader("value=x"))
			if tc.origin != "" {
				req.Header.Set("Origin", tc.origin)
			}
			if tc.referer != "" {
				req.Header.Set("Referer", tc.referer)
			}
			if tc.site != "" {
				req.Header.Set("Sec-Fetch-Site", tc.site)
			}
			req.Header.Set("X-Forwarded-Host", "evil.example")
			res, err := app.Test(req)
			if err != nil {
				t.Fatal(err)
			}
			defer res.Body.Close()
			if res.StatusCode != tc.want || (tc.want == 403 && writes != 0) {
				t.Fatalf("status=%d writes=%d", res.StatusCode, writes)
			}
		})
	}
}
func TestWriteSourceProtectionRejectsDuplicateMetadata(t *testing.T) {
	app := fiber.New()
	app.Use(protectWriteSource)
	app.Post("/", func(c fiber.Ctx) error { return c.SendStatus(200) })
	for _, name := range []string{"Origin", "Referer", "Sec-Fetch-Site"} {
		req := httptest.NewRequest(http.MethodPost, "http://flare.example/", nil)
		value := "http://flare.example"
		if name == "Sec-Fetch-Site" {
			value = "same-origin"
		}
		req.Header.Add(name, value)
		req.Header.Add(name, value)
		res, err := app.Test(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
		if res.StatusCode != 403 {
			t.Fatalf("duplicate %s status=%d", name, res.StatusCode)
		}
	}
}
