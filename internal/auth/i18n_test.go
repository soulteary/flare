package auth

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

func TestAuthErrorUsesSiteLanguage(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.WriteFile("config.yml", []byte("Locale: en-US\n"), 0600); err != nil {
		t.Fatal(err)
	}
	app := fiber.New()
	app.Get("/error", func(c fiber.Ctx) error { return authError(c, "auth.input_error") })
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/error", nil))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusBadRequest || !strings.Contains(string(body), `lang="en"`) || !strings.Contains(string(body), "Please enter the correct username and password.") {
		t.Fatalf("unexpected auth response: %d %s", response.StatusCode, body)
	}
}
