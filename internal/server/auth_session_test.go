package server

import (
	"crypto/tls"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// authSessionTransport preserves the client's cookie policy while exercising
// the real router without a listening socket. Backend TLS is independent of the
// client URL so HTTPS termination at a reverse proxy can be represented.
type authSessionTransport struct {
	handler    http.Handler
	backendTLS bool
	headers    http.Header
}

// RoundTrip supplies an origin-form server request, as net/http would after
// accepting a connection, and leaves the client's original URL for its jar.
func (transport authSessionTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	serverRequest := request.Clone(request.Context())
	serverRequest.RequestURI = request.URL.RequestURI()
	serverRequest.Host = request.URL.Host
	serverRequest.URL.Scheme = ""
	serverRequest.URL.Host = ""
	serverRequest.TLS = nil
	if transport.backendTLS {
		serverRequest.TLS = &tls.ConnectionState{}
	}
	for name, values := range transport.headers {
		serverRequest.Header[name] = append([]string(nil), values...)
	}
	recorder := httptest.NewRecorder()
	transport.handler.ServeHTTP(recorder, serverRequest)
	response := recorder.Result()
	response.Request = request
	return response, nil
}

// TestNewRouterSessionCookiePolicy covers the browser-visible login lifecycle.
// A real cookie jar is essential: manually copying Set-Cookie into Cookie would
// hide issue #121, where a Secure cookie was never returned over ordinary HTTP.
func TestNewRouterSessionCookiePolicy(t *testing.T) {
	tests := []struct {
		name        string
		scheme      string
		backendTLS  bool
		forceSecure bool
		headers     http.Header
		wantSecure  bool
		wantLogin   bool
	}{
		{name: "HTTP direct", scheme: "http", wantLogin: true},
		{name: "HTTPS direct", scheme: "https", backendTLS: true, wantSecure: true, wantLogin: true},
		{name: "HTTPS proxy with explicit secure cookie", scheme: "https", forceSecure: true, wantSecure: true, wantLogin: true},
		{
			name: "HTTP ignores forged forwarding headers", scheme: "http", wantLogin: true,
			headers: http.Header{
				"X-Forwarded-Proto": {"https"},
				"Forwarded":         {"for=192.0.2.1;proto=https"},
				"X-Forwarded-Ssl":   {"on"},
			},
		},
		{
			name: "HTTPS cannot be downgraded by forwarding headers", scheme: "https", backendTLS: true,
			wantSecure: true, wantLogin: true,
			headers: http.Header{"X-Forwarded-Proto": {"http"}, "Forwarded": {"proto=http"}},
		},
		{name: "explicit secure cookie is withheld over HTTP", scheme: "http", forceSecure: true, wantSecure: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			handler := newAuthSessionRouter(t, test.forceSecure)
			jar, err := cookiejar.New(nil)
			require.NoError(t, err)
			client := &http.Client{
				Jar: jar,
				Transport: authSessionTransport{
					handler: handler, backendTLS: test.backendTLS, headers: test.headers,
				},
				CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse },
			}
			base := test.scheme + "://flare.example.test:5005"
			baseURL, err := url.Parse(base)
			require.NoError(t, err)

			badLogin, err := client.PostForm(base+"/login", url.Values{"username": {"session-user"}, "password": {"wrong"}})
			require.NoError(t, err)
			closeAuthSessionResponse(t, badLogin)
			assert.Equal(t, http.StatusBadRequest, badLogin.StatusCode)
			assert.Empty(t, jar.Cookies(baseURL), "invalid credentials must not establish a session")
			assertAuthSessionPages(t, client, base, false)

			login, err := client.PostForm(base+"/login", url.Values{"username": {"session-user"}, "password": {"session-pass"}})
			require.NoError(t, err)
			closeAuthSessionResponse(t, login)
			require.Equal(t, http.StatusFound, login.StatusCode)
			assert.Equal(t, "/settings/application", login.Header.Get("Location"))
			assertAuthSessionCookie(t, login, test.wantSecure)
			if test.wantLogin {
				require.Len(t, jar.Cookies(baseURL), 1, "the next request must actually send the login cookie")
			} else {
				require.Empty(t, jar.Cookies(baseURL), "explicit Secure policy must remain effective over HTTP")
			}
			assertAuthSessionPages(t, client, base, test.wantLogin)
			if !test.wantLogin {
				return
			}

			logout, err := client.PostForm(base+"/logout", nil)
			require.NoError(t, err)
			closeAuthSessionResponse(t, logout)
			assert.Equal(t, http.StatusFound, logout.StatusCode)
			assert.Equal(t, "/settings/application", logout.Header.Get("Location"))
			assertAuthSessionCookie(t, logout, test.wantSecure)
			assertAuthSessionPages(t, client, base, false)
		})
	}
}

// newAuthSessionRouter isolates settings and account flags for the real router.
// These tests must remain serial because the application configuration is global.
func newAuthSessionRouter(t *testing.T, forceSecure bool) http.Handler {
	t.Helper()
	originalFlags := define.AppFlags
	t.Cleanup(func() { define.AppFlags = originalFlags })
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	flags := model.Flags{
		Port: 5005, EnableOfflineMode: true, Visibility: "DEFAULT",
		CookieName: "flare", CookieSecret: "session-test-signing-key-at-least-32-bytes",
		CookieSecure: forceSecure, User: "session-user", Pass: "session-pass",
	}
	handler, err := NewRouter(&flags)
	require.NoError(t, err)
	return handler
}

// assertAuthSessionCookie checks attributes the standard cookie jar does not
// enforce, notably HttpOnly and SameSite, as well as the transport policy.
func assertAuthSessionCookie(t *testing.T, response *http.Response, secure bool) {
	t.Helper()
	cookies := response.Cookies()
	require.Len(t, cookies, 1)
	cookie := cookies[0]
	assert.Equal(t, "flare_5005", cookie.Name)
	assert.Equal(t, "/", cookie.Path)
	assert.Empty(t, cookie.Domain, "session cookies must remain host-only")
	assert.Equal(t, secure, cookie.Secure)
	assert.True(t, cookie.HttpOnly)
	assert.Equal(t, http.SameSiteLaxMode, cookie.SameSite)
	assert.Equal(t, 30*24*60*60, cookie.MaxAge)
}

// assertAuthSessionPages checks all settings links reported in issue #121 and
// the application page, which returns a login form rather than another redirect.
func assertAuthSessionPages(t *testing.T, client *http.Client, base string, loggedIn bool) {
	t.Helper()
	for _, path := range []string{"/settings/appearance", "/settings/search", "/settings/weather", "/settings/theme"} {
		response, err := client.Get(base + path)
		require.NoError(t, err)
		closeAuthSessionResponse(t, response)
		if loggedIn {
			assert.Equal(t, http.StatusOK, response.StatusCode, "GET %s", path)
		} else {
			assert.Equal(t, http.StatusFound, response.StatusCode, "GET %s", path)
			assert.Equal(t, "/settings/application", response.Header.Get("Location"), "GET %s", path)
		}
	}
	response, err := client.Get(base + "/settings/application")
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
	assert.Equal(t, http.StatusOK, response.StatusCode)
	loginForm := strings.Contains(string(body), "name=username") || strings.Contains(string(body), `name="username"`)
	assert.Equal(t, !loggedIn, loginForm, "the application page must reflect the session returned by the jar")
}

// closeAuthSessionResponse consumes each response as an HTTP client would before
// reusing its connection, keeping the transport interchangeable with net/http.
func closeAuthSessionResponse(t *testing.T, response *http.Response) {
	t.Helper()
	_, err := io.Copy(io.Discard, response.Body)
	require.NoError(t, err)
	require.NoError(t, response.Body.Close())
}
