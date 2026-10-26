package weather

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"strings"
	"time"

	"github.com/gofiber/fiber/v3"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/auth"
	"github.com/soulteary/flare/internal/pool"
	weather "github.com/soulteary/funny-china-weather"
)

type RemoteWeatherResponse struct {
	Data struct {
		Observe struct {
			Degree        string `json:"degree"`
			Humidity      string `json:"humidity"`
			Precipitation string `json:"precipitation"`
			Pressure      string `json:"pressure"`
			UpdateTime    string `json:"update_time"`
			Weather       string `json:"weather"`
			WeatherCode   string `json:"weather_code"`
			WeatherShort  string `json:"weather_short"`
			WindDirection string `json:"wind_direction"`
			WindPower     string `json:"wind_power"`
		} `json:"observe"`
	} `json:"data"`
	Message string `json:"message"`
	Status  int    `json:"status"`
}

type WeatherQueryParams struct {
	Location string `form:"location"`
}

// IconHTML renders only exact keys from the dependency's embedded SVG assets.
// Missing keys stay empty, including the homepage's not-yet-loaded snapshot.
func IconHTML(key string) template.HTML {
	svg, ok := weather.GetIconSVG(key)
	if !ok {
		return ""
	}
	// #nosec G203 -- GetIconSVG emits packaged SVGs, never caller-provided markup.
	return template.HTML(svg)
}

func GetWeatherInfo(location string) (response model.Weather, desc string, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var observation weather.Observation
	var fetchRemoteErr error
	parts := strings.Split(location, ",")
	if len(parts) == 2 || len(parts) == 3 {
		selected := weather.Location{Province: strings.TrimSpace(parts[0]), City: strings.TrimSpace(parts[1])}
		if len(parts) == 3 {
			selected.County = strings.TrimSpace(parts[2])
		}
		observation, fetchRemoteErr = weather.GetObservationAt(ctx, selected)
	} else {
		observation, fetchRemoteErr = weather.GetObservation(ctx, location)
	}
	if fetchRemoteErr != nil {
		return response, "获取远程数据失败", fmt.Errorf("weather fetch: %w", fetchRemoteErr)
	}
	code, degree, humidity, lastUpdate := observation.Code, observation.TemperatureC, observation.HumidityPercent, observation.UpdateTime

	hour, _, _ := time.Now().Clock()
	isDay := hour >= 5 && hour <= 18
	conditionCode, conditionText := weather.GetWeatherIconByCode(code)
	const _WEATHER_DATA_CACHE_TIME = 60 * 10 // 10 minutes
	response.ExternalLastUpdate = lastUpdate
	response.Degree = degree
	response.IsDay = isDay
	response.ConditionCode = conditionCode
	response.ConditionText = conditionText
	response.Humidity = humidity
	response.Expires = time.Now().Unix() + _WEATHER_DATA_CACHE_TIME
	return response, "接口正常", nil
}

func RegisterRouting(e *fiber.App) {
	e.Get(define.SettingPages.Weather.Path, auth.AuthRequired, pageHome)
	if !define.AppFlags.EnableOfflineMode {
		e.Get("/settings/weather/detect", auth.AuthRequired, detectLocation)
		e.Get("/settings/weather/locations", auth.AuthRequired, searchLocations)
		e.Get("/settings/weather/picker.js", auth.AuthRequired, pickerScript)
		e.Post(define.SettingPages.Weather.Path, auth.AuthRequired, updateWeatherOptions)
		e.Post(define.SettingPagesAPI.WeatherTest.Path, auth.AuthRequired, testWeatherFetch)
	}
}

func pageHome(c fiber.Ctx) error {
	return render(c, "")
}

type weatherForm struct {
	Location    string `form:"location"`
	Selected    string `form:"selected"`
	Keyword     string `form:"keyword"`
	ShowWeather bool   `form:"show"`
}

func updateWeatherOptions(c fiber.Ctx) error {
	var body weatherForm
	if err := c.Bind().Body(&body); err != nil {
		return c.Status(http.StatusForbidden).JSON("提交数据缺失")
	}
	options, readErr := data.GetAllSettingsOptions()
	if readErr != nil {
		return renderForm(c, "", http.StatusInternalServerError, &body)
	}
	var selected *model.WeatherLocation
	if !body.ShowWeather {
		selected = options.WeatherLocation
	} else if body.Selected != "" {
		candidates, err := lookup(c, body.Keyword)
		if err != nil {
			return renderForm(c, message(c, "weather_search_failed"), http.StatusBadGateway, &body)
		}
		for _, candidate := range candidates {
			if candidate.ID == body.Selected {
				value := candidate
				selected = &value
				break
			}
		}
		if selected == nil {
			return renderForm(c, message(c, "weather_choose"), http.StatusBadRequest, &body)
		}
	} else if body.Location == options.Location {
		selected = options.WeatherLocation
	} else {
		return renderForm(c, message(c, "weather_choose"), http.StatusBadRequest, &body)
	}
	if body.ShowWeather && selected == nil {
		return renderForm(c, message(c, "weather_choose"), http.StatusBadRequest, &body)
	}
	if !data.UpdateSelectedWeather(body.ShowWeather, selected) {
		return renderForm(c, "", http.StatusInternalServerError, &body)
	}

	return render(c, "")
}

type weatherTestKey struct{}

func testWeatherFetch(c fiber.Ctx) error {
	location, _ := data.GetLocationAndWeatherShow()
	value, _, err := GetWeatherInfo(location)
	desc := message(c, "weather_test_failed")
	if err == nil {
		c.Locals(weatherTestKey{}, value)
		desc = fmt.Sprintf("%s: %d°C, %d%%, %s, %s", location, value.Degree, value.Humidity, value.ConditionText, value.ExternalLastUpdate)
	}

	return render(c, desc)
}

func render(c fiber.Ctx, testResult string) error {
	return renderForm(c, testResult, http.StatusOK, nil)
}

func renderForm(c fiber.Ctx, testResult string, status int, submitted *weatherForm) error {
	location, showWeather := data.GetLocationAndWeatherShow()
	options, err := data.GetAllSettingsOptions()
	if err != nil && submitted == nil {
		return c.Status(http.StatusInternalServerError).SendString("config error")
	}
	locale := options.Locale
	if locale == "" {
		locale = "zh"
	}
	m := pool.GetTemplateMap()
	defer pool.PutTemplateMap(m)
	m["Locale"] = locale
	m["DebugMode"] = define.AppFlags.DebugMode
	m["PageInlineStyle"] = define.GetPageInlineStyle()
	m["ShowWeatherModule"] = !define.AppFlags.EnableOfflineMode && showWeather
	m["ShowWeather"] = showWeather
	m["PageName"] = "Weather"
	m["PageAppearance"] = define.GetAppBodyStyle()
	m["SettingPages"] = define.SettingPages
	m["SettingsURI"] = define.RegularPages.Settings.Path
	m["OptionTitle"] = options.Title
	m["SettingPagesAPI"] = define.SettingPagesAPI
	m["Location"] = options.Location
	m["Offline"] = define.AppFlags.EnableOfflineMode
	m["NeedsSelection"] = options.WeatherLocation == nil
	m["LocationKey"] = location
	m["TestResult"] = testResult
	if value, ok := c.Locals(weatherTestKey{}).(model.Weather); ok {
		m["WeatherTest"] = value
		m["WeatherTestTime"] = value.ExternalLastUpdate
		if updated, parseErr := weather.ParseUpdateTime(value.ExternalLastUpdate); parseErr == nil {
			m["WeatherTestTime"] = updated.Format("2006-01-02 15:04")
		}
	}
	if submitted != nil {
		m["Location"] = submitted.Location
		m["Selected"] = submitted.Selected
		m["Keyword"] = submitted.Keyword
		m["ShowWeather"] = submitted.ShowWeather
		m["ShowWeatherModule"] = !define.AppFlags.EnableOfflineMode && submitted.ShowWeather
		m["SaveFailed"] = true
	}
	return c.Status(status).Render("settings.html", m)
}
