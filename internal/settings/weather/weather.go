package weather

import (
	"errors"
	"net/http"
	"time"

	"github.com/labstack/echo/v5"

	FlareData "github.com/soulteary/flare/config/data"
	FlareDefine "github.com/soulteary/flare/config/define"
	FlareModel "github.com/soulteary/flare/config/model"
	FlareAuth "github.com/soulteary/flare/internal/auth"
	FlarePool "github.com/soulteary/flare/internal/pool"
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

func GetWeatherInfo(location string) (response FlareModel.Weather, desc string, err error) {
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
	e.GET(FlareDefine.SettingPages.Weather.Path, pageHome, FlareAuth.AuthRequired)
	if !FlareDefine.AppFlags.EnableOfflineMode {
		e.POST(FlareDefine.SettingPages.Weather.Path, updateWeatherOptions, FlareAuth.AuthRequired)
		e.POST(FlareDefine.SettingPagesAPI.WeatherTest.Path, testWeatherFetch, FlareAuth.AuthRequired)
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
	FlareData.UpdateWeatherAndLocation(body.ShowWeather, body.Location)
	return render(c, "")
}

func testWeatherFetch(c *echo.Context) error {
	location, _ := FlareData.GetLocationAndWeatherShow()
	_, desc, _ := GetWeatherInfo(location)
	return render(c, desc)
}

func render(c *echo.Context, testResult string) error {
	location, showWeather := FlareData.GetLocationAndWeatherShow()
	options := FlareData.GetAllSettingsOptions()
	m := FlarePool.GetTemplateMap()
	defer FlarePool.PutTemplateMap(m)
	m["DebugMode"] = FlareDefine.AppFlags.DebugMode
	m["PageInlineStyle"] = FlareDefine.GetPageInlineStyle()
	m["ShowWeatherModule"] = !FlareDefine.AppFlags.EnableOfflineMode && showWeather
	m["ShowWeather"] = showWeather
	m["PageName"] = "Weather"
	m["PageAppearance"] = FlareDefine.GetAppBodyStyle()
	m["SettingPages"] = FlareDefine.SettingPages
	m["SettingsURI"] = FlareDefine.RegularPages.Settings.Path
	m["OptionTitle"] = options.Title
	m["SettingPagesAPI"] = FlareDefine.SettingPagesAPI
	m["Location"] = location
	m["TestResult"] = testResult
	return c.Render(http.StatusOK, "settings.html", m)
}
