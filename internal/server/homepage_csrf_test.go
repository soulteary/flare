package server

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/cmd"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

var homepageCSRFField = regexp.MustCompile(`name="?csrf-token"? value="([^"]+)"`)

func homepageCSRFApp(t *testing.T, trusted bool, visibility string) (*fiber.App, *security.Store) {
	t.Helper()
	t.Chdir(t.TempDir())
	original := define.AppFlags
	t.Cleanup(func() { define.AppFlags = original })
	require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{}))
	require.True(t, data.SaveNormalBookmarks(model.Bookmarks{}))
	flags := &model.Flags{Port: 5005, CookieName: "home-csrf-test", DisableLoginMode: trusted,
		Visibility: visibility, EnableOfflineMode: true, StartupStatus: "ready", RuntimeInputs: map[string]string{}}
	var store *security.Store
	if trusted {
		prepared := cmd.PrepareSecurity(flags)
		require.Equal(t, "ready", prepared.Status)
		store = prepared.Store
	} else {
		var err error
		store, err = security.Open(t.TempDir())
		require.NoError(t, err)
		require.NoError(t, store.Initialize("admin", "homepage test password", "local"))
	}
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	app, err := NewFiberApp(flags, store)
	require.NoError(t, err)
	return app, store
}

func TestHomepageCSRFOnlyForProtectedVisibleSearch(t *testing.T) {
	for _, role := range []string{"trusted", "anonymous", "authenticated"} {
		t.Run(role, func(t *testing.T) {
			app, store := homepageCSRFApp(t, role == "trusted", "DEFAULT")
			client, base := nativeFiberClient(t, app, false)
			if role == "authenticated" {
				token, err := store.CreateSession()
				require.NoError(t, err)
				origin, err := url.Parse(base)
				require.NoError(t, err)
				name := auth.RequestHandleSessionName("home-csrf-test", 5005) + "_" + store.Info().InstanceID
				client.Jar.SetCookies(origin, []*http.Cookie{{Name: name, Value: token, Path: "/"}})
			}
			// Returning to false also verifies that neither an existing browser
			// binding nor a previously populated template map leaks a token.
			for index, search := range []bool{false, true, false} {
				require.NoError(t, os.WriteFile("config.yml", []byte(fmt.Sprintf(
					"Title: CSRF test\nShowSearchComponent: %t\nShowApps: true\nShowBookmarks: true\nShowDateTime: false\nShowWeather: false\nTheme: blackboard\n", search)), 0600))
				for _, path := range []string{"/", "/?search=missing", "/bookmarks"} {
					response, err := client.Get(base + path)
					require.NoError(t, err)
					body, err := io.ReadAll(response.Body)
					response.Body.Close()
					require.NoError(t, err)
					require.Equal(t, http.StatusOK, response.StatusCode, path)
					wantToken := search && role != "trusted" && path != "/bookmarks"
					require.Equal(t, wantToken, homepageCSRFField.Match(body), "step %d %s", index, path)
					wantCookie := wantToken && role != "authenticated"
					require.Equal(t, wantCookie, len(response.Header.Values("Set-Cookie")) > 0, "step %d %s", index, path)
				}
			}
		})
	}
}

func TestTrustedReadonlyPagesKeepManagementCSRF(t *testing.T) {
	for _, visible := range []bool{false, true} {
		t.Run(fmt.Sprintf("search-visible-%t", visible), func(t *testing.T) {
			testTrustedReadonlyPagesKeepManagementCSRF(t, visible)
		})
	}
}

func testTrustedReadonlyPagesKeepManagementCSRF(t *testing.T, visible bool) {
	t.Helper()
	app, _ := homepageCSRFApp(t, true, "DEFAULT")
	require.NoError(t, os.WriteFile("config.yml", []byte(fmt.Sprintf("Title: unchanged\nShowSearchComponent: %t\nShowWeather: false\nTheme: blackboard\n", visible)), 0600))
	client, base := nativeFiberClient(t, app, false)
	response, err := client.Get(base + "/")
	require.NoError(t, err)
	response.Body.Close()
	require.Empty(t, response.Header.Values("Set-Cookie"))
	response, err = client.PostForm(base+"/", url.Values{"search": {"missing"}})
	require.NoError(t, err)
	initialSearchBody, readErr := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, readErr)
	require.Equal(t, http.StatusOK, response.StatusCode)
	require.Empty(t, response.Header.Values("Set-Cookie"))
	require.False(t, homepageCSRFField.Match(initialSearchBody))
	response, err = client.Get(base + "/settings/appearance")
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	match := homepageCSRFField.FindSubmatch(body)
	require.Len(t, match, 2, "management form must still issue a token")
	token := string(match[1])
	require.NotEmpty(t, response.Header.Values("Set-Cookie"))
	before, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	write := func(client *http.Client, value, origin string) int {
		form := url.Values{"title": {"updated"}, "csrf-token": {value}}
		request, requestErr := http.NewRequest(http.MethodPost, base+"/settings/appearance", strings.NewReader(form.Encode()))
		require.NoError(t, requestErr)
		request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		if origin != "" {
			request.Header.Set("Origin", origin)
		}
		result, sendErr := client.Do(request)
		require.NoError(t, sendErr)
		defer result.Body.Close()
		_, drainErr := io.Copy(io.Discard, result.Body)
		require.NoError(t, drainErr)
		return result.StatusCode
	}
	// Reading and searching must neither issue nor renew the binding acquired
	// from the management form, and must not need its token themselves.
	for _, path := range []string{"/", "/?search=missing", "/bookmarks"} {
		readResponse, readErr := client.Get(base + path)
		require.NoError(t, readErr)
		readBody, readErr := io.ReadAll(readResponse.Body)
		readResponse.Body.Close()
		require.NoError(t, readErr)
		require.Equal(t, http.StatusOK, readResponse.StatusCode)
		require.Empty(t, readResponse.Header.Values("Set-Cookie"))
		require.False(t, homepageCSRFField.Match(readBody))
	}
	searchResponse, searchErr := client.PostForm(base+"/", url.Values{"search": {"missing"}})
	require.NoError(t, searchErr)
	searchBody, searchErr := io.ReadAll(searchResponse.Body)
	searchResponse.Body.Close()
	require.NoError(t, searchErr)
	require.Equal(t, http.StatusOK, searchResponse.StatusCode)
	require.Empty(t, searchResponse.Header.Values("Set-Cookie"))
	require.False(t, homepageCSRFField.Match(searchBody))
	unbound := &http.Client{Transport: client.Transport, Timeout: client.Timeout}
	for _, status := range []int{write(client, "", base), write(client, "invalid", base),
		write(client, token, "https://other.example"), write(unbound, token, base)} {
		require.Equal(t, http.StatusForbidden, status)
	}
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, before, after, "rejected writes must not change settings")
	require.Equal(t, http.StatusOK, write(client, token, base))
	after, err = os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Contains(t, string(after), "updated")
}

// Private search still uses AuthRequired for POSTs and needs the visible form's
// session-bound token. Trusted-mode optimization must not remove that proof.
func TestPrivateSearchRetainsCSRF(t *testing.T) {
	app, store := homepageCSRFApp(t, false, "PRIVATE")
	require.NoError(t, os.WriteFile("config.yml", []byte("ShowSearchComponent: true\nShowWeather: false\nTheme: blackboard\n"), 0600))
	client, base := nativeFiberClient(t, app, false)
	session, err := store.CreateSession()
	require.NoError(t, err)
	origin, err := url.Parse(base)
	require.NoError(t, err)
	name := auth.RequestHandleSessionName("home-csrf-test", 5005) + "_" + store.Info().InstanceID
	client.Jar.SetCookies(origin, []*http.Cookie{{Name: name, Value: session, Path: "/"}})
	response, err := client.Get(base + "/")
	require.NoError(t, err)
	body, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, response.StatusCode)
	match := homepageCSRFField.FindSubmatch(body)
	require.Len(t, match, 2)
	for _, token := range []string{"", "invalid", string(match[1])} {
		response, err = client.PostForm(base+"/", url.Values{"search": {"missing"}, "csrf-token": {token}})
		require.NoError(t, err)
		_, err = io.Copy(io.Discard, response.Body)
		response.Body.Close()
		require.NoError(t, err)
		want := http.StatusForbidden
		if token == string(match[1]) {
			want = http.StatusOK
		}
		require.Equal(t, want, response.StatusCode)
	}
}
