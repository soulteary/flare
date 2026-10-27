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

func homepageCSRFApp(t *testing.T, trusted bool) (*fiber.App, *security.Store) {
	t.Helper()
	t.Chdir(t.TempDir())
	original := define.AppFlags
	t.Cleanup(func() { define.AppFlags = original })
	require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{}))
	require.True(t, data.SaveNormalBookmarks(model.Bookmarks{}))
	flags := &model.Flags{Port: 5005, CookieName: "home-csrf-test", DisableLoginMode: trusted,
		Visibility: "DEFAULT", EnableOfflineMode: true, StartupStatus: "ready", RuntimeInputs: map[string]string{}}
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

func TestHomepageCSRFOnlyForVisibleSearch(t *testing.T) {
	for _, role := range []string{"trusted", "anonymous", "authenticated"} {
		t.Run(role, func(t *testing.T) {
			app, store := homepageCSRFApp(t, role == "trusted")
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
					wantToken := search && path != "/bookmarks"
					require.Equal(t, wantToken, homepageCSRFField.Match(body), "step %d %s", index, path)
					wantCookie := wantToken && role != "authenticated"
					require.Equal(t, wantCookie, len(response.Header.Values("Set-Cookie")) > 0, "step %d %s", index, path)
				}
			}
		})
	}
}

func TestHiddenHomepageSearchKeepsTrustedWriteCSRF(t *testing.T) {
	app, _ := homepageCSRFApp(t, true)
	require.NoError(t, os.WriteFile("config.yml", []byte("Title: unchanged\nShowSearchComponent: false\nShowWeather: false\nTheme: blackboard\n"), 0600))
	client, base := nativeFiberClient(t, app, false)
	response, err := client.Get(base + "/")
	require.NoError(t, err)
	response.Body.Close()
	require.Empty(t, response.Header.Values("Set-Cookie"))
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
