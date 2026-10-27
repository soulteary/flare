package home

import (
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

func TestNavigationLoadsSettingsWhenOptionsOmitted(t *testing.T) {
	t.Chdir(t.TempDir())
	flags := define.AppFlags
	t.Cleanup(func() { define.AppFlags = flags })
	define.AppFlags.EnableOfflineMode = true
	require.True(t, data.SaveFavoriteBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "App sentinel", URL: "https://app.example"}}}))
	require.True(t, data.SaveNormalBookmarks(model.Bookmarks{Items: []model.Bookmark{{Name: "Bookmark sentinel", URL: "https://bookmark.example"}}}))
	for _, existing := range []bool{false, true} {
		markup, message := generateApplicationsForRead("", nil, false, existing)
		require.Empty(t, message)
		require.Contains(t, string(markup), "App sentinel")
		markup, message = generateBookmarkNavigationForRead("", "", "/", nil, false, existing)
		require.Empty(t, message)
		require.Contains(t, string(markup), "Bookmark sentinel")
	}
	require.NoError(t, os.WriteFile("config.yml", []byte("ShowApps: [invalid"), 0600))
	for _, existing := range []bool{false, true} {
		markup, message := generateApplicationsForRead("", nil, false, existing)
		require.Empty(t, markup)
		require.NotEmpty(t, message)
		markup, message = generateBookmarkNavigationForRead("", "", "/", nil, false, existing)
		require.Empty(t, markup)
		require.NotEmpty(t, message)
	}
}

func TestWeatherPublicAccessorUsesSettingsAndOfflineMode(t *testing.T) {
	t.Chdir(t.TempDir())
	flags := define.AppFlags
	cache := homepageWeather
	t.Cleanup(func() { define.AppFlags = flags; homepageWeather = cache })
	define.AppFlags.EnableOfflineMode = true
	require.Empty(t, GetWeatherData())
	define.AppFlags.EnableOfflineMode = false
	require.NoError(t, os.WriteFile("config.yml", []byte("ShowWeather: false\n"), 0600))
	require.Empty(t, GetWeatherData())
	require.NoError(t, os.WriteFile("config.yml", []byte("ShowWeather: [invalid"), 0600))
	require.Empty(t, GetWeatherData())
	clock := testWeatherClock()
	homepageWeather = newWeatherCache(clock.now, func(location string) (model.Weather, error) {
		return model.Weather{Location: location, Degree: 24, Expires: clock.now().Add(time.Minute).Unix()}, nil
	})
	require.NoError(t, os.WriteFile("config.yml", []byte("ShowWeather: true\nLocation: Beijing\n"), 0600))
	_ = GetWeatherData()
	waitWeatherIdle(t, homepageWeather)
	require.Equal(t, 24, GetWeatherData().Degree)
}

func TestHomepageRoutesFailClosedOnInvalidConfiguration(t *testing.T) {
	t.Chdir(t.TempDir())
	flags := define.AppFlags
	t.Cleanup(func() { define.AppFlags = flags })
	define.AppFlags.DisableLoginMode = true
	define.AppFlags.Visibility = "DEFAULT"
	define.AppFlags.EnableOfflineMode = true
	require.NoError(t, os.WriteFile("config.yml", []byte("ShowApps: [invalid"), 0600))
	app := fiber.New()
	RegisterRouting(app)
	for _, path := range []string{"/", "/applications", "/bookmarks", "/help"} {
		r, err := app.Test(httptest.NewRequest("GET", path, nil))
		require.NoError(t, err)
		require.Equal(t, 500, r.StatusCode, path)
		require.NoError(t, r.Body.Close())
	}
	r, err := app.Test(httptest.NewRequest("POST", "/", strings.NewReader("search=test")))
	require.NoError(t, err)
	defer r.Body.Close()
	require.Equal(t, 500, r.StatusCode)
	// A failed read must not silently overwrite the original invalid document.
	raw, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, "ShowApps: [invalid", string(raw))
	_, err = data.GetAllSettingsOptions()
	require.Error(t, err)
}
