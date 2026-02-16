package weather

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

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

func GetWeatherInfo(location string) (response model.Weather, desc string, err error) {
	code, degree, humidity, lastUpdate, fetchRemoteErr := weather.GetWeatherByLocation(location)
	if fetchRemoteErr != nil {
		return response, "获取远程数据失败", errors.New("获取远程数据失败")
	}
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

func RegisterRouting(e *echo.Echo) {
	e.GET(define.SettingPages.Weather.Path, pageHome, auth.AuthRequired)
	if !define.AppFlags.EnableOfflineMode {
		e.POST(define.SettingPages.Weather.Path, updateWeatherOptions, auth.AuthRequired)
		e.POST(define.SettingPagesAPI.WeatherTest.Path, testWeatherFetch, auth.AuthRequired)
	}
}

func pageHome(c *echo.Context) error {
	return render(c, "")
}

func updateWeatherOptions(c *echo.Context) error {
	var body struct {
		Location    string `form:"location"`
		ShowWeather bool   `form:"show"`
	}
	if err := c.Bind(&body); err != nil {
		return c.JSON(http.StatusForbidden, "提交数据缺失")
	}
	data.UpdateWeatherAndLocation(body.ShowWeather, body.Location)
	return render(c, "")
}

func testWeatherFetch(c *echo.Context) error {
	location, _ := data.GetLocationAndWeatherShow()
	_, desc, err := GetWeatherInfo(location)
	if err != nil {
		desc = ""
	}
	return render(c, desc)
}

func render(c *echo.Context, testResult string) error {
	location, showWeather := data.GetLocationAndWeatherShow()
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return c.String(http.StatusInternalServerError, "config error")
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
	m["Location"] = location
	m["TestResult"] = testResult
	return c.Render(http.StatusOK, "settings.html", m)
}
