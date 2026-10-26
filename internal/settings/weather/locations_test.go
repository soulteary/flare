package weather

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/i18n"
	provider "github.com/soulteary/funny-china-weather"
	"github.com/stretchr/testify/require"
)

func TestMessageLocale(t *testing.T) {
	for _, tc := range []struct {
		name   string
		config string
		locale string
	}{
		{"configured", "Locale: en\n", "en"},
		{"default", "Locale: ''\n", "zh"},
		{"invalid config", "Locale: [\n", "zh"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Chdir(t.TempDir())
			require.NoError(t, os.WriteFile("config.yml", []byte(tc.config), 0600))
			require.Equal(t, i18n.T(tc.locale, "weather_search_failed"), message(nil, "weather_search_failed"))
		})
	}
}

func fakeLocations(t *testing.T, values []provider.Location, err error) {
	t.Helper()
	original := searchProvider
	t.Cleanup(func() { searchProvider = original })
	searchProvider = func(ctx context.Context, keyword string) ([]provider.Location, error) {
		require.NotEmpty(t, keyword)
		_, deadline := ctx.Deadline()
		require.True(t, deadline)
		return values, err
	}
}
func weatherApp(t *testing.T) (*fiber.App, *capturedWeatherView) {
	t.Helper()
	t.Chdir(t.TempDir())
	flags := define.AppFlags
	t.Cleanup(func() { define.AppFlags = flags })
	define.AppFlags.DisableLoginMode = true
	define.AppFlags.EnableOfflineMode = false
	require.True(t, data.UpdateWeatherAndLocation(true, "朝阳"))
	view := &capturedWeatherView{}
	app := fiber.New(fiber.Config{Views: view})
	RegisterRouting(app)
	return app, view
}
func postWeather(t *testing.T, app *fiber.App, values url.Values) int {
	t.Helper()
	req := httptest.NewRequest("POST", "/settings/weather", strings.NewReader(values.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := app.Test(req)
	require.NoError(t, err)
	defer response.Body.Close()
	return response.StatusCode
}
func TestSelectCountyAndRejectForgedOrUnselectedLocation(t *testing.T) {
	app, _ := weatherApp(t)
	fakeLocations(t, []provider.Location{{ID: "101", Province: "湖南", City: "常德", County: "津市"}, {ID: "102", Province: "辽宁", City: "朝阳"}}, nil)
	values := url.Values{"show": {"1"}, "keyword": {"津市"}, "selected": {"forged"}, "location": {"伪造地址"}}
	require.Equal(t, 400, postWeather(t, app, values))
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "朝阳", options.Location)
	values.Set("selected", "101")
	require.Equal(t, 200, postWeather(t, app, values))
	options, err = data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, &model.WeatherLocation{ID: "101", Province: "湖南", City: "常德", County: "津市"}, options.WeatherLocation)
	require.Equal(t, "湖南, 常德, 津市", options.Location)
	location, enabled := data.GetLocationAndWeatherShow()
	require.True(t, enabled)
	require.Equal(t, options.Location, location)
	values.Set("selected", "")
	values.Set("location", "北京")
	require.Equal(t, 400, postWeather(t, app, values))
}
func TestSearchFailuresAndEmptyResults(t *testing.T) {
	app, _ := weatherApp(t)
	fakeLocations(t, nil, nil)
	response, err := app.Test(httptest.NewRequest("GET", "/settings/weather/locations?q=朝阳", nil))
	require.NoError(t, err)
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	require.Equal(t, 200, response.StatusCode)
	require.Equal(t, "[]", string(body))
	fakeLocations(t, nil, errors.New("timeout"))
	response, err = app.Test(httptest.NewRequest("GET", "/settings/weather/locations?q=北京", nil))
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, 502, response.StatusCode)
	values := url.Values{"show": {"1"}, "keyword": {"北京"}, "selected": {"1"}, "location": {"北京, 北京"}}
	require.Equal(t, 502, postWeather(t, app, values))
	options, _ := data.GetAllSettingsOptions()
	require.Equal(t, "朝阳", options.Location)
}
func TestLegacyMigrationRequiresUniqueCandidate(t *testing.T) {
	for _, count := range []int{0, 1, 2} {
		t.Run(string(rune('0'+count)), func(t *testing.T) {
			_, _ = weatherApp(t)
			values := make([]provider.Location, count)
			for i := range values {
				values[i] = provider.Location{ID: "1", Province: "北京", City: "北京"}
			}
			fakeLocations(t, values, nil)
			before, err := os.ReadFile("config.yml")
			require.NoError(t, err)
			MigrateLegacyLocation("朝阳")
			options, err := data.GetAllSettingsOptions()
			require.NoError(t, err)
			if count == 1 {
				require.NotNil(t, options.WeatherLocation)
				require.Equal(t, "北京, 北京", options.Location)
			} else {
				after, err := os.ReadFile("config.yml")
				require.NoError(t, err)
				require.Equal(t, before, after)
				require.Nil(t, options.WeatherLocation)
			}
		})
	}
}
func TestOfflineRoutesAndAuthentication(t *testing.T) {
	app, _ := weatherApp(t)
	define.AppFlags.DisableLoginMode = false
	response, err := app.Test(httptest.NewRequest(http.MethodGet, "/settings/weather/locations?q=北京", nil))
	require.NoError(t, err)
	response.Body.Close()
	require.NotEqual(t, 200, response.StatusCode)
	define.AppFlags.EnableOfflineMode = true
	app = fiber.New()
	RegisterRouting(app)
	response, err = app.Test(httptest.NewRequest(http.MethodGet, "/settings/weather/locations?q=北京", nil))
	require.NoError(t, err)
	response.Body.Close()
	require.Equal(t, 404, response.StatusCode)
}

func TestIPDetectionRequiresExactProviderFieldsAndDoesNotSave(t *testing.T) {
	app, _ := weatherApp(t)
	original := locateProvider
	t.Cleanup(func() { locateProvider = original })
	locateProvider = func(context.Context) (string, error) { return "北京, 北京", nil }
	fakeLocations(t, []provider.Location{{ID: "1", Province: "北京", City: "北京"}, {ID: "2", Province: "北京", City: "北京", County: "朝阳"}}, nil)
	response, err := app.Test(httptest.NewRequest("GET", "/settings/weather/detect", nil))
	require.NoError(t, err)
	defer response.Body.Close()
	require.Equal(t, 200, response.StatusCode)
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "朝阳", options.Location)
	require.Nil(t, options.WeatherLocation)
}

func TestSelectedSaveFailureRetainsCandidateAndKeyword(t *testing.T) {
	app, view := weatherApp(t)
	original := searchProvider
	t.Cleanup(func() { searchProvider = original })
	searchProvider = func(context.Context, string) ([]provider.Location, error) {
		require.NoError(t, os.Rename("config.yml", "config-before.yml"))
		require.NoError(t, os.Mkdir("config.yml", 0700))
		return []provider.Location{{ID: "1", Province: "湖南", City: "常德", County: "津市"}}, nil
	}
	values := url.Values{"show": {"1"}, "keyword": {"津市"}, "selected": {"1"}, "location": {"湖南, 常德, 津市"}}
	require.Equal(t, 500, postWeather(t, app, values))
	require.Equal(t, "1", view.values["Selected"])
	require.Equal(t, "津市", view.values["Keyword"])
	require.Equal(t, "湖南, 常德, 津市", view.values["Location"])
	require.NoError(t, os.Remove("config.yml"))
	require.NoError(t, os.Rename("config-before.yml", "config.yml"))
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "朝阳", options.Location)
}
func TestLegacyMigrationProviderFailurePreservesBytes(t *testing.T) {
	weatherApp(t)
	fakeLocations(t, nil, context.DeadlineExceeded)
	before, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	MigrateLegacyLocation("朝阳")
	after, err := os.ReadFile("config.yml")
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestDisableWeatherPreservesSelectedLocation(t *testing.T) {
	app, _ := weatherApp(t)
	selected := &model.WeatherLocation{ID: "1", Province: "北京", City: "北京", County: "朝阳"}
	require.True(t, data.UpdateSelectedWeather(true, selected))
	// An unfinished search must not prevent switching weather off or replace its location.
	require.Equal(t, 200, postWeather(t, app, url.Values{"show": {"0"}, "keyword": {"unfinished"}, "location": {""}}))
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.False(t, options.ShowWeather)
	require.Equal(t, selected, options.WeatherLocation)
}

func TestLegacyMunicipalityAliasesAreProviderVerified(t *testing.T) {
	for legacy, name := range map[string]string{"北京市": "北京", "天津市": "天津", "上海市": "上海", "重庆市": "重庆"} {
		t.Run(legacy, func(t *testing.T) {
			weatherApp(t)
			require.True(t, data.UpdateWeatherAndLocation(true, legacy))
			original := searchProvider
			t.Cleanup(func() { searchProvider = original })
			searchProvider = func(_ context.Context, keyword string) ([]provider.Location, error) {
				require.Equal(t, name, keyword)
				return []provider.Location{{ID: "correct", Province: name, City: name}, {ID: "district", Province: name, City: name, County: "朝阳"}, {ID: "wrong", Province: "其他", City: name}}, nil
			}
			MigrateLegacyLocation(legacy)
			options, err := data.GetAllSettingsOptions()
			require.NoError(t, err)
			require.NotNil(t, options.WeatherLocation)
			require.Equal(t, "correct", options.WeatherLocation.ID)
		})
	}
}
func TestLegacyCountySearchesLeafThenMatchesFullAddress(t *testing.T) {
	weatherApp(t)
	require.True(t, data.UpdateWeatherAndLocation(true, "北京, 北京, 朝阳"))
	original := searchProvider
	t.Cleanup(func() { searchProvider = original })
	searchProvider = func(_ context.Context, keyword string) ([]provider.Location, error) {
		require.Equal(t, "朝阳", keyword)
		return []provider.Location{{ID: "beijing", Province: "北京", City: "北京", County: "朝阳"}, {ID: "changchun", Province: "吉林", City: "长春", County: "朝阳"}, {ID: "liaoning", Province: "辽宁", City: "朝阳"}}, nil
	}
	MigrateLegacyLocation("北京, 北京, 朝阳")
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	require.NotNil(t, options.WeatherLocation)
	require.Equal(t, "beijing", options.WeatherLocation.ID)
}

func TestMunicipalitySearchAcceptsSuffixAndPreservesNetworkError(t *testing.T) {
	app, _ := weatherApp(t)
	original := searchProvider
	t.Cleanup(func() { searchProvider = original })
	searchProvider = func(_ context.Context, keyword string) ([]provider.Location, error) {
		require.Equal(t, "北京", keyword)
		return []provider.Location{{ID: "101010100", Province: "北京", City: "北京"}}, nil
	}
	response, err := app.Test(httptest.NewRequest("GET", "/settings/weather/locations?q=北京市", nil))
	require.NoError(t, err)
	raw, err := io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, 200, response.StatusCode)
	require.Contains(t, string(raw), "101010100")
	searchProvider = func(context.Context, string) ([]provider.Location, error) {
		return nil, &net.OpError{Op: "dial", Net: "tcp", Err: syscall.EBADF}
	}
	response, err = app.Test(httptest.NewRequest("GET", "/settings/weather/locations?q=北京", nil))
	require.NoError(t, err)
	raw, err = io.ReadAll(response.Body)
	response.Body.Close()
	require.NoError(t, err)
	require.Equal(t, 502, response.StatusCode)
	require.Contains(t, string(raw), `"code":"network"`)
	require.Contains(t, string(raw), "服务端无法连接")
}
