package weather_test

import (
	"fmt"
	"html/template"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	consumer "github.com/soulteary/flare/internal/settings/weather"
	weather "github.com/soulteary/funny-china-weather"
	"github.com/stretchr/testify/require"
)

type weatherTransport func(*http.Request) (*http.Response, error)

func (transport weatherTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return transport(request)
}

// Exercise the actual consumer's parsing and trusted inline-SVG contract without
// depending on external weather services or the current day/night boundary.
func TestWeatherDependencyConsumerCompatibility(t *testing.T) {
	original := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = original })
	code := 0
	http.DefaultTransport = weatherTransport(func(request *http.Request) (*http.Response, error) {
		if request.URL.Host != "wis.qq.com" {
			return nil, fmt.Errorf("unexpected weather provider: %s", request.URL.Host)
		}
		if request.URL.Query().Get("province") != "北京" || request.URL.Query().Get("city") != "北京" {
			return nil, fmt.Errorf("wrong selected location: %s", request.URL)
		}
		body := fmt.Sprintf(`{"status":200,"data":{"observe":{"weather_code":"%d","degree":"20","humidity":"50","update_time":"202610260800"}}}`, code)
		return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	view := template.Must(template.New("weather").Parse(`<div>{{.Icon}}</div><span>{{.Degree}}°C {{.Humidity}}%</span>`))
	codes := []int{-1, 49, 53, 54, 55, 56, 57, 58, 301, 302, 999}
	for value := 0; value <= 32; value++ {
		codes = append(codes, value)
	}
	for _, value := range codes {
		t.Run(fmt.Sprint(value), func(t *testing.T) {
			code = value
			result, _, err := consumer.GetWeatherInfo("北京, 北京")
			require.NoError(t, err)
			require.Equal(t, 20, result.Degree)
			require.Equal(t, 50, result.Humidity)
			require.Equal(t, "202610260800", result.ExternalLastUpdate)
			icon := consumer.IconHTML(result.ConditionCode)
			require.NotEmpty(t, icon, "missing icon %q", result.ConditionCode)
			var output strings.Builder
			require.NoError(t, view.Execute(&output, map[string]any{"Icon": icon, "Degree": result.Degree, "Humidity": result.Humidity}))
			require.Contains(t, output.String(), "<svg")
			require.NotContains(t, output.String(), "&lt;svg")
			require.Contains(t, output.String(), "20°C 50%")
			if value < 0 || value == 999 {
				require.Equal(t, "unknown", result.ConditionCode)
				require.Equal(t, "未知", result.ConditionText)
			}
			for _, hour := range []int{0, 12} {
				key, description := weather.GetWeatherIconAt(value, time.Date(2026, 10, 26, hour, 0, 0, 0, time.UTC))
				require.NotEmpty(t, description)
				require.NotEmpty(t, consumer.IconHTML(key), "missing %02d:00 icon %q", hour, key)
				if value < 0 || value == 999 {
					require.Equal(t, "unknown", key)
				}
			}
		})
	}
}

func TestWeatherIconRejectsUntrustedAndRemovedKeys(t *testing.T) {
	for _, key := range []string{"", "<svg onload=alert(1)>", "../icons/clear-day.svg", "day", "icon-sleet"} {
		require.Empty(t, consumer.IconHTML(key), "unexpected markup for %q", key)
	}
}
