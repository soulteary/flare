package server

import (
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/soulteary/flare/internal/i18n"
	"github.com/stretchr/testify/require"
)

// An edit made outside the process is served by the next request: no restart,
// and no stale homepage fragment for the previous content.
func TestExternalNavigationEditsAreServedWithoutRestart(t *testing.T) {
	t.Chdir(t.TempDir())
	app, _, _ := contentErrorInstance(t, map[string]string{},
		"links:\n  - name: FIRST-APP-SENTINEL\n    link: https://first-app.example.com/\n",
		"links:\n  - name: FIRST-BOOKMARK-SENTINEL\n    link: https://first-bookmark.example.com/\n")
	settle := func(name string, age time.Duration) {
		t.Helper()
		settled := time.Now().Add(-age).Truncate(time.Second)
		require.NoError(t, os.Chtimes(name, settled, settled))
	}
	settle("apps.yml", time.Hour)
	settle("bookmarks.yml", time.Hour)
	page := func(path string) string {
		t.Helper()
		status, _, body := contentErrorRequest(t, app, http.MethodGet, path, "")
		require.Equal(t, http.StatusOK, status, path)
		return body
	}
	for i := 0; i < 2; i++ {
		body := page("/")
		require.Contains(t, body, "FIRST-APP-SENTINEL")
		require.Contains(t, body, "FIRST-BOOKMARK-SENTINEL")
	}

	require.NoError(t, os.WriteFile("bookmarks.yml", []byte("links:\n  - name: SECOND-BOOKMARK-SENTINEL\n    link: https://second-bookmark.example.com/\n"), 0600))
	settle("bookmarks.yml", time.Minute)
	for _, path := range []string{"/", "/bookmarks"} {
		body := page(path)
		require.Contains(t, body, "SECOND-BOOKMARK-SENTINEL", path)
		require.NotContains(t, body, "FIRST-BOOKMARK-SENTINEL", path)
	}
	require.Contains(t, page("/"), "FIRST-APP-SENTINEL", "the untouched collection is unaffected")

	// A broken edit shows the module error; correcting the file clears it.
	require.NoError(t, os.WriteFile("apps.yml", []byte("links: [broken"), 0600))
	body := page("/")
	require.Contains(t, body, i18n.T("en", "navigation.applications_error"))
	require.Contains(t, body, "SECOND-BOOKMARK-SENTINEL")
	require.NoError(t, os.WriteFile("apps.yml", []byte("links:\n  - name: SECOND-APP-SENTINEL\n    link: https://second-app.example.com/\n"), 0600))
	body = page("/")
	require.Contains(t, body, "SECOND-APP-SENTINEL")
	require.Empty(t, navigationAlert.FindAllString(body, -1))

	// With login disabled, a private entry added by hand is withheld at once.
	require.NoError(t, os.WriteFile("bookmarks.yml", []byte("links:\n  - name: HAND-ADDED-PRIVATE-SENTINEL\n    link: https://hand-added.example.com/\n    private: true\n"), 0600))
	body = page("/")
	require.Contains(t, body, i18n.T("en", "navigation.private_requires_login"))
	require.NotContains(t, body, "HAND-ADDED-PRIVATE-SENTINEL")
	require.NotContains(t, body, "hand-added.example.com")
}
