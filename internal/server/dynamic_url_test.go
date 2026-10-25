package server

import (
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func dynamicURLRouter(t *testing.T, encoded bool, target string) http.Handler {
	t.Helper()
	original := define.AppFlags
	t.Cleanup(func() { define.AppFlags = original })
	t.Chdir(t.TempDir())
	t.Setenv("FLARE_BASELINE", "1")
	handler, err := NewRouter(&model.Flags{Port: 5005, DisableLoginMode: true, EnableOfflineMode: true})
	require.NoError(t, err)
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	options.EnableEncryptedLink = encoded
	options.ShowApps, options.ShowBookmarks = true, true
	require.True(t, data.UpdateAppearance(options))
	items := model.Bookmarks{Items: []model.Bookmark{{Name: "Dynamic", URL: target}}}
	require.True(t, data.SaveFavoriteBookmarks(items))
	require.True(t, data.SaveNormalBookmarks(items))
	return handler
}

func TestConcurrentNavigationUsesRequestScopedDynamicURL(t *testing.T) {
	handler := dynamicURLRouter(t, false, "{origin}/target")
	var wg sync.WaitGroup
	errors := make(chan string, 96)
	for client := 0; client < 8; client++ {
		wg.Add(1)
		go func(client int) {
			defer wg.Done()
			for repeat := 0; repeat < 4; repeat++ {
				for _, path := range []string{"/", "/applications", "/bookmarks"} {
					host := fmt.Sprintf("client-%d.example:5005", client)
					request := httptest.NewRequest(http.MethodGet, "http://"+host+path, nil)
					response := httptest.NewRecorder()
					handler.ServeHTTP(response, request)
					if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `href="http://`+host+`/target"`) {
						errors <- fmt.Sprintf("%s%s returned cross-request/missing target", host, path)
					}
				}
			}
		}(client)
	}
	wg.Wait()
	close(errors)
	for err := range errors {
		t.Error(err)
	}
}

func TestEncodedDynamicNavigationPreservesSourceAndAllowlist(t *testing.T) {
	cases := []struct{ target, expected string }{
		{"{origin}/target?q=one%20two#fragment", "http://source.example:5005/target?q=one%20two#fragment"},
		{"http://{host}/target", "http://source.example:5005/target"},
		{"https://example.com/?q=~~~", "https://example.com/?q=~~~"},
		{"https://target.example{pathname}?fixed=1", "https://target.example/applications?fixed=1"},
		{"{href}#saved", "http://source.example:5005/applications?source=1#saved"},
	}
	for _, tc := range cases {
		t.Run(tc.target, func(t *testing.T) {
			handler := dynamicURLRouter(t, true, tc.target)
			page := httptest.NewRecorder()
			handler.ServeHTTP(page, httptest.NewRequest(http.MethodGet, "http://source.example:5005/applications?source=1", nil))
			matches := regexp.MustCompile(`href="([^"]+)" class="app-item"`).FindStringSubmatch(page.Body.String())
			require.Len(t, matches, 2)
			link := html.UnescapeString(matches[1])
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://source.example:5005"+link, nil))
			require.Equal(t, http.StatusFound, response.Code)
			require.Equal(t, tc.expected, response.Header().Get("Location"))
			parsed, err := url.Parse(link)
			require.NoError(t, err)
			values := parsed.Query()
			values.Set("go", data.Base64EncodeUrl("https://unlisted.example/"))
			parsed.RawQuery = values.Encode()
			response = httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "http://source.example:5005"+parsed.String(), nil))
			require.Empty(t, response.Header().Get("Location"), "arbitrary URLs must never bypass the bookmark allowlist")
		})
	}
}

func TestEncodedNavigationRejectsForeignSourceAndUnsafeTarget(t *testing.T) {
	handler := dynamicURLRouter(t, true, "javascript:alert(1)")
	for _, from := range []string{"https://foreign.example/", "//foreign.example/", "/bad\\path"} {
		response := httptest.NewRecorder()
		endpoint := "http://source.example/redir/url?go=" + data.Base64EncodeUrl("javascript:alert(1)") + "&from=" + url.QueryEscape(from)
		handler.ServeHTTP(response, httptest.NewRequest(http.MethodGet, endpoint, nil))
		require.Equal(t, http.StatusBadRequest, response.Code)
		require.Empty(t, response.Header().Get("Location"))
	}
}
