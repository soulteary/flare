package auth

import (
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

const thirtyDaysInSeconds = 30 * 24 * 60 * 60

// issuedSessionCookie returns the single session cookie set by a response,
// ignoring the separate pre-login CSRF cookie.
func issuedSessionCookie(t *testing.T, response *http.Response) *http.Cookie {
	t.Helper()
	var found *http.Cookie
	for _, cookie := range response.Cookies() {
		if strings.HasSuffix(cookie.Name, "_preauth") {
			continue
		}
		require.Nil(t, found, "response set more than one session cookie")
		found = cookie
	}
	require.NotNil(t, found, "response did not set a session cookie")
	return found
}

func TestSessionCookieMaxAgeMatchesSessionLifetime(t *testing.T) {
	require.Equal(t, thirtyDaysInSeconds, sessionCookieMaxAge)
	require.Equal(t, security.AbsoluteTimeout, time.Duration(sessionCookieMaxAge)*time.Second)
}

func TestPersistentSessionCookieLastsThirtyDays(t *testing.T) {
	_, b := persistentFixture(t, "local")
	token := b.csrf()
	// The pre-login CSRF cookie keeps its own 30-minute lifetime.
	require.Len(t, b.cookies, 1)
	for name, cookie := range b.cookies {
		require.True(t, strings.HasSuffix(name, "_preauth"))
		require.Equal(t, 30*60, cookie.MaxAge)
	}

	response := b.request("POST", "/login", url.Values{"username": {"admin"}, "password": {"legacy short"}, "csrf-token": {token}}, nil)
	require.Equal(t, 303, response.StatusCode)
	login := issuedSessionCookie(t, response)
	require.NotEmpty(t, login.Value)
	require.Equal(t, thirtyDaysInSeconds, login.MaxAge)
	require.True(t, login.HttpOnly)
	require.Equal(t, http.SameSiteLaxMode, login.SameSite)
	require.Equal(t, "/", login.Path)
	require.Empty(t, login.Domain)
	require.False(t, login.Secure)

	// Reauthentication rotates the token; the replacement cookie is persistent too.
	token = b.csrf()
	response = b.request("POST", "/account/reauthenticate", url.Values{"csrf-token": {token}, "password": {"legacy short"}}, nil)
	require.Equal(t, 303, response.StatusCode)
	rotated := issuedSessionCookie(t, response)
	require.NotEmpty(t, rotated.Value)
	require.NotEqual(t, login.Value, rotated.Value)
	require.Equal(t, thirtyDaysInSeconds, rotated.MaxAge)

	// The account page reports the cookie lifetime with the other attributes.
	response = b.request("GET", define.SettingPages.Others.Path, nil, nil)
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), "Max-Age="+strconv.Itoa(thirtyDaysInSeconds))

	token = b.csrf()
	response = b.request("POST", "/account/password", url.Values{"csrf-token": {token}, "password": {"legacy short"}, "new_password": {"A long unique new phrase 42!"}, "confirm_password": {"A long unique new phrase 42!"}}, nil)
	require.Equal(t, 303, response.StatusCode)
	changed := issuedSessionCookie(t, response)
	require.NotEqual(t, rotated.Value, changed.Value)
	require.Equal(t, thirtyDaysInSeconds, changed.MaxAge)
	require.Equal(t, 200, b.request("GET", "/protected", nil, nil).StatusCode)

	// Signing out still deletes the cookie immediately.
	token = b.csrf()
	response = b.request("POST", "/logout", url.Values{"csrf-token": {token}}, nil)
	require.Equal(t, 303, response.StatusCode)
	cleared := issuedSessionCookie(t, response)
	require.Empty(t, cleared.Value)
	require.Negative(t, cleared.MaxAge)
	require.Equal(t, 401, b.request("GET", "/protected", nil, nil).StatusCode)
}

func TestPersistentSessionCookieClearedBySignOutAll(t *testing.T) {
	_, b := persistentFixture(t, "local")
	b.login()
	token := b.csrf()
	response := b.request("POST", "/logout-all", url.Values{"csrf-token": {token}}, nil)
	require.Equal(t, 303, response.StatusCode)
	cleared := issuedSessionCookie(t, response)
	require.Empty(t, cleared.Value)
	require.Negative(t, cleared.MaxAge)
	require.Equal(t, 401, b.request("GET", "/protected", nil, nil).StatusCode)
}

func TestPersistentTakeLocalControlIssuesThirtyDayCookie(t *testing.T) {
	_, b := persistentFixture(t, "deployment")
	b.login()
	token := b.csrf()
	response := b.request("POST", "/account/local", url.Values{"csrf-token": {token}, "password": {"legacy short"}, "confirm": {"local"}}, nil)
	require.Equal(t, 303, response.StatusCode)
	cookie := issuedSessionCookie(t, response)
	require.NotEmpty(t, cookie.Value)
	require.Equal(t, thirtyDaysInSeconds, cookie.MaxAge)
	require.Equal(t, 200, b.request("GET", "/protected", nil, nil).StatusCode)
}
