package home

import (
	"log/slog"
	"sync"
	"time"

	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/config/model"
	"github.com/soulteary/flare/internal/settings/weather"
)

const weatherRetryInitial = 15 * time.Second
const weatherRetryMaximum = 5 * time.Minute

// weatherCache has at most one upstream request in flight. Readers get complete
// value snapshots and never wait for network I/O. A changed city must not display
// the previous city's weather; an expired snapshot for the same city remains useful.
type weatherCache struct {
	mu         sync.Mutex
	snapshot   model.Weather
	location   string
	refreshing bool
	retryAt    time.Time
	failures   int
	now        func() time.Time
	fetch      func(string) (model.Weather, error)
}

func newWeatherCache(now func() time.Time, fetch func(string) (model.Weather, error)) *weatherCache {
	return &weatherCache{now: now, fetch: fetch}
}

var homepageWeather = newWeatherCache(time.Now, func(location string) (model.Weather, error) {
	value, _, err := weather.GetWeatherInfo(location)
	return value, err
})

// GetWeatherData returns immediately, including on cold starts and upstream
// failures. The next page load shows a completed background refresh.
func GetWeatherData() model.Weather {
	if define.AppFlags.EnableOfflineMode {
		return model.Weather{}
	}
	options, err := data.GetAllSettingsOptions()
	if err != nil {
		return model.Weather{}
	}
	return weatherForSettings(options)
}

func weatherForSettings(options model.Application) model.Weather {
	if define.AppFlags.EnableOfflineMode {
		return model.Weather{}
	}
	location := options.Location
	if options.WeatherLocation != nil {
		location = options.WeatherLocation.String()
	}
	if !options.ShowWeather || location == "" {
		return model.Weather{}
	}
	return homepageWeather.get(location)
}

func (cache *weatherCache) get(location string) model.Weather {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	now := cache.now()
	if cache.location != location {
		cache.location = location
		cache.failures = 0
		cache.retryAt = time.Time{}
	}
	value := cache.snapshot
	if value.Location != location {
		value = model.Weather{}
	}
	if !cache.refreshing && !now.Before(cache.retryAt) && (value.Location == "" || value.Expires <= now.Unix()) {
		cache.refreshing = true
		go cache.refresh(location)
	}
	return value
}

func (cache *weatherCache) refresh(location string) {
	value, err := cache.fetch(location)
	cache.mu.Lock()
	defer cache.mu.Unlock()
	cache.refreshing = false
	// A settings change while the request was running invalidates its result.
	if cache.location != location {
		return
	}
	if err != nil {
		cache.failures++
		delay := weatherRetryInitial
		for i := 1; i < cache.failures && delay < weatherRetryMaximum; i++ {
			delay *= 2
		}
		if delay > weatherRetryMaximum {
			delay = weatherRetryMaximum
		}
		cache.retryAt = cache.now().Add(delay)
		slog.Warn("weather refresh failed; retaining the previous snapshot", "location", location, "retry_after", delay, "error", err)
		return
	}
	value.Location = location
	// Guard against an already-expired upstream timestamp causing a retry storm.
	if value.Expires <= cache.now().Unix() {
		value.Expires = cache.now().Add(weatherRetryInitial).Unix()
	}
	cache.snapshot = value
	cache.failures = 0
	cache.retryAt = time.Time{}
}
