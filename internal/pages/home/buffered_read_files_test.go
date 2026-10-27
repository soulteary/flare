package home

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/resources/templates"
	"github.com/soulteary/flare/internal/security"
	"github.com/stretchr/testify/require"
)

func TestBufferedViewsDoNotInitializeMissingFiles(t *testing.T) {
	for _, test := range []struct {
		name, path, missing string
		status              int
	}{
		{"missing settings", "/", "config.yml", 500},
		{"missing home apps", "/", "apps.yml", 500},
		{"missing home bookmarks", "/?search=sentinel", "bookmarks.yml", 500},
		{"missing subpage apps", "/applications", "apps.yml", 500},
		{"missing subpage bookmarks", "/bookmarks", "bookmarks.yml", 500},
		{"missing category collection", "/bookmarks?category=public", "bookmarks.yml", 404},
		{"complete successful read", "/", "", 200},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			original := define.AppFlags
			t.Cleanup(func() { define.AppFlags = original })
			define.AppFlags = model.Flags{Port: 5005, CookieName: "flare", Visibility: "DEFAULT", EnableOfflineMode: true}
			require.NoError(t, os.WriteFile("config.yml", []byte("Title: read test\nShowApps: true\nShowBookmarks: true\nLocale: en\n"), 0600))
			require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "app sentinel", URL: "https://app.example", Private: true}}}))
			require.True(t, data.SaveNormalBookmarks(model.Bookmarks{Categories: []model.Category{{ID: "public", Name: "Public"}}, Items: []model.Bookmark{{Name: "bookmark sentinel", URL: "https://bookmark.example", Category: "public"}}}))
			// Warm both byte and parsed caches before an external removal. No
			// editor cache invalidation accompanies an operator deleting YAML.
			_, settingsErr := data.ReadExistingSettingsOptions()
			require.NoError(t, settingsErr)
			_, _, navigationErr := data.ReadExistingVisibleNavigationPair(true, true, true)
			require.NoError(t, navigationErr)
			if test.missing != "" {
				require.NoError(t, os.Remove(test.missing))
			}
			store, storeErr := security.Open(t.TempDir())
			require.NoError(t, storeErr)
			defer store.Close()
			require.NoError(t, store.Initialize("admin", "short", "local"))
			token, tokenErr := store.CreateSession()
			require.NoError(t, tokenErr)
			before, beforeErr := store.Session(token, false)
			require.NoError(t, beforeErr)
			views, viewsErr := templates.NewRenderer()
			require.NoError(t, viewsErr)
			app := fiber.New(fiber.Config{Views: views})
			require.NoError(t, auth.RequestHandle(app, store))
			RegisterRouting(app)
			request := httptest.NewRequest("GET", "http://flare.test"+test.path, nil)
			request.AddCookie(&http.Cookie{Name: auth.RequestHandleSessionName("flare", 5005) + "_" + store.Info().InstanceID, Value: token})
			response, responseErr := app.Test(request)
			require.NoError(t, responseErr)
			defer response.Body.Close()
			body, bodyErr := io.ReadAll(response.Body)
			require.NoError(t, bodyErr)
			require.Equal(t, test.status, response.StatusCode, string(body))
			after, afterErr := store.Session(token, false)
			require.NoError(t, afterErr)
			if test.missing != "" {
				_, statErr := os.Stat(test.missing)
				require.True(t, os.IsNotExist(statErr), "tentative read recreated "+test.missing)
				require.Equal(t, before.LastActive, after.LastActive, "failed read must not extend idle")
			} else {
				require.Contains(t, string(body), "app sentinel")
				require.Contains(t, string(body), "bookmark sentinel")
			}
		})
	}
}
