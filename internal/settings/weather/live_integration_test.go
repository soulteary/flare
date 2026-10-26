//go:build integration

package weather

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
)

// Explicit opt-in: these tests use the real Tencent directory and weather APIs.
// All configuration writes stay in t.TempDir; normal tests remain offline.
func TestLiveWeatherSelectionAndMigration(t *testing.T) {
	app, view := weatherApp(t)
	request := func(method, path string, form url.Values) *http.Response {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(form.Encode()))
		if method == http.MethodPost {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		response, err := app.Test(req, fiber.TestConfig{Timeout: 10 * time.Second, FailOnTimeout: true})
		require.NoError(t, err)
		t.Cleanup(func() { response.Body.Close() })
		return response
	}
	selectedLocations := []model.WeatherLocation{}
	for _, keyword := range []string{"北京", "北京市", "朝阳", "津市"} {
		response := request("GET", "/settings/weather/locations?q="+url.QueryEscape(keyword), nil)
		require.Equal(t, 200, response.StatusCode)
		var candidates []model.WeatherLocation
		require.NoError(t, json.NewDecoder(response.Body).Decode(&candidates))
		require.NotEmpty(t, candidates)
		t.Logf("live search %s: %+v", keyword, candidates)
		if keyword == "朝阳" {
			require.GreaterOrEqual(t, len(candidates), 3)
		}
		for _, candidate := range candidates {
			t.Run(keyword+"/"+candidate.String(), func(t *testing.T) {
				response := request("POST", "/settings/weather", url.Values{"show": {"1"}, "keyword": {keyword}, "selected": {candidate.ID}, "location": {candidate.String()}})
				require.Equal(t, 200, response.StatusCode)
				options, err := data.GetAllSettingsOptions()
				require.NoError(t, err)
				require.Equal(t, &candidate, options.WeatherLocation)
				require.Equal(t, candidate.String(), options.Location)
				raw, err := os.ReadFile("config.yml")
				require.NoError(t, err)
				require.Contains(t, string(raw), candidate.ID)
				// The settings weather test queries the saved provider fields, not a keyword.
				response = request("POST", "/settings/weather/test", nil)
				require.Equal(t, 200, response.StatusCode)
				result, ok := view.values["TestResult"].(string)
				require.True(t, ok)
				require.Contains(t, result, candidate.String())
				require.Contains(t, result, "°C")
				require.NotContains(t, result, "失败")
				t.Logf("saved and queried real weather: %s (ID %s)", result, candidate.ID)
				selectedLocations = append(selectedLocations, candidate)
			})
		}
	}
	// Every selected serialized address must migrate without choosing a different
	// district; the bare ambiguous keyword must preserve the original bytes.
	cases := []struct {
		value string
		want  *model.WeatherLocation
	}{{"北京", nil}, {"北京市", nil}, {"朝阳", nil}, {"山西省太原市", nil}, {"湖南省常德市津市市", nil}}
	for _, candidate := range selectedLocations {
		value := candidate
		cases = append(cases, struct {
			value string
			want  *model.WeatherLocation
		}{candidate.String(), &value})
	}
	for _, test := range cases {
		t.Run("migration/"+test.value, func(t *testing.T) {
			require.True(t, data.UpdateWeatherAndLocation(true, test.value))
			before, err := os.ReadFile("config.yml")
			require.NoError(t, err)
			MigrateLegacyLocation(test.value)
			options, err := data.GetAllSettingsOptions()
			require.NoError(t, err)
			switch test.value {
			case "朝阳", "山西省太原市", "湖南省常德市津市市":
				after, err := os.ReadFile("config.yml")
				require.NoError(t, err)
				require.Equal(t, before, after)
				require.Nil(t, options.WeatherLocation)
				t.Logf("legacy %q safely retained without automatic selection", test.value)
			default:
				require.NotNil(t, options.WeatherLocation, "legacy %s did not migrate", test.value)
				if test.want != nil {
					require.Equal(t, test.want, options.WeatherLocation)
				} else {
					require.Equal(t, "北京, 北京", options.Location)
				}
				location, enabled := data.GetLocationAndWeatherShow()
				require.True(t, enabled)
				weather, _, err := GetWeatherInfo(location)
				require.NoError(t, err)
				require.NotEmpty(t, weather.ExternalLastUpdate)
				t.Logf("legacy %q -> %s: %d°C, %s, updated %s", test.value, location, weather.Degree, weather.ConditionText, weather.ExternalLastUpdate)
			}
		})
	}
	// A reload still renders the persisted selection through the real settings route.
	response := request("GET", "/settings/weather", nil)
	require.Equal(t, 200, response.StatusCode)
	_, err := io.ReadAll(response.Body)
	require.NoError(t, err)
}

func TestLiveWeatherBrowser(t *testing.T) {
	if os.Getenv("FLARE_WEATHER_LIVE_BROWSER") != "1" {
		t.Skip("set FLARE_WEATHER_LIVE_BROWSER=1 to run real-provider browser verification")
	}
	runWeatherBrowser(t, true)
}
