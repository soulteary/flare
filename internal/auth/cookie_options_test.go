//go:build !fiber

package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/labstack/echo/v5"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
)

// TestSessionCookieOptionsAreRequestScoped checks that TLS cannot change the
// shared store's policy for later or concurrent HTTP requests.
func TestSessionCookieOptionsAreRequestScoped(t *testing.T) {
	originalFlags, originalSessionName := define.AppFlags, sessionName
	t.Cleanup(func() {
		define.AppFlags, sessionName = originalFlags, originalSessionName
	})
	define.AppFlags = model.Flags{
		Port:         5005,
		CookieName:   "flare",
		CookieSecret: testCookieSecret,
		User:         "testuser",
		Pass:         "testpass",
	}
	e := echo.New()
	if err := RequestHandle(e); err != nil {
		t.Fatal(err)
	}

	loginCookie := func(scheme string) error {
		request := httptest.NewRequest(http.MethodPost, scheme+"://flare.example.test/login",
			strings.NewReader("username=testuser&password=testpass"))
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		response := httptest.NewRecorder()
		e.ServeHTTP(response, request)
		if response.Code != http.StatusFound {
			return fmt.Errorf("%s login returned %d", scheme, response.Code)
		}
		cookies := response.Result().Cookies()
		if len(cookies) != 1 {
			return fmt.Errorf("%s login returned %d cookies", scheme, len(cookies))
		}
		if cookies[0].Secure != (scheme == "https") {
			return fmt.Errorf("%s login cookie Secure=%v", scheme, cookies[0].Secure)
		}
		return nil
	}

	// A TLS request followed by HTTP deterministically catches a shared option
	// being set to true and accidentally retained after the TLS request ends.
	for _, scheme := range []string{"https", "http", "https", "http"} {
		if err := loginCookie(scheme); err != nil {
			t.Fatal(err)
		}
	}

	const requests = 32
	errors := make(chan error, requests)
	var workers sync.WaitGroup
	for i := range requests {
		workers.Add(1)
		go func() {
			defer workers.Done()
			scheme := "http"
			if i%2 == 0 {
				scheme = "https"
			}
			if err := loginCookie(scheme); err != nil {
				errors <- err
			}
		}()
	}
	workers.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}
