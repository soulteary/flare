package data

import (
	"github.com/soulteary/flare/config/model"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestWeatherSelectionPersistsAndClearsPreviousCounty(t *testing.T) {
	t.Chdir(t.TempDir())
	district := &model.WeatherLocation{ID: "1", Province: "北京", City: "北京", County: "朝阳"}
	require.True(t, UpdateSelectedWeather(true, district))
	city := &model.WeatherLocation{ID: "2", Province: "北京", City: "北京"}
	require.True(t, UpdateSelectedWeather(true, city))
	options, err := GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, city, options.WeatherLocation)
	require.True(t, UpdateLocation("上海"))
	options, err = GetAllSettingsOptions()
	require.NoError(t, err)
	require.Nil(t, options.WeatherLocation)
	require.False(t, MigrateWeatherLocation("北京", district))
	options, err = GetAllSettingsOptions()
	require.NoError(t, err)
	require.Equal(t, "上海", options.Location)
}
