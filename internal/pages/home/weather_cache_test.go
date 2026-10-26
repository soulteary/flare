package home

import (
	"errors"
	"github.com/gofiber/fiber/v3"
	"github.com/soulteary/flare/config/data"
	"github.com/soulteary/flare/config/define"
	"github.com/soulteary/flare/internal/resources/mdi"
	"github.com/soulteary/flare/internal/resources/templates"
	"github.com/stretchr/testify/require"
	"io"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soulteary/flare/config/model"
)

type weatherTestClock struct{ timestamp atomic.Int64 }

func (clock *weatherTestClock) now() time.Time          { return time.Unix(0, clock.timestamp.Load()) }
func (clock *weatherTestClock) advance(d time.Duration) { clock.timestamp.Add(int64(d)) }
func testWeatherClock() *weatherTestClock {
	clock := &weatherTestClock{}
	clock.timestamp.Store(time.Date(2026, 10, 26, 12, 0, 0, 0, time.UTC).UnixNano())
	return clock
}
func waitWeatherIdle(t *testing.T, cache *weatherCache) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		cache.mu.Lock()
		idle := !cache.refreshing
		cache.mu.Unlock()
		if idle {
			return
		}
		runtime.Gosched()
	}
	t.Fatal("background weather refresh did not complete")
}

func TestWeatherColdStartDoesNotWaitAndCoalescesRequests(t *testing.T) {
	clock := testWeatherClock()
	release := make(chan struct{})
	started := make(chan struct{})
	var calls atomic.Int64
	cache := newWeatherCache(clock.now, func(location string) (model.Weather, error) {
		calls.Add(1)
		close(started)
		<-release
		return model.Weather{Degree: 23, Humidity: 40, ConditionText: "sunny", Expires: clock.now().Add(10 * time.Minute).Unix()}, nil
	})
	returned := make(chan model.Weather, 1)
	go func() { returned <- cache.get("Beijing") }()
	select {
	case value := <-returned:
		if value.Location != "" {
			t.Fatal("cold cache unexpectedly had weather")
		}
	case <-time.After(time.Second):
		t.Fatal("cold read blocked on upstream")
	}
	<-started
	var readers sync.WaitGroup
	for i := 0; i < 100; i++ {
		readers.Add(1)
		go func() { defer readers.Done(); cache.get("Beijing") }()
	}
	readers.Wait()
	if calls.Load() != 1 {
		t.Fatalf("concurrent reads started %d fetches", calls.Load())
	}
	close(release)
	waitWeatherIdle(t, cache)
	value := cache.get("Beijing")
	if value.Degree != 23 || value.Humidity != 40 || value.ConditionText != "sunny" || value.Location != "Beijing" {
		t.Fatalf("incomplete published snapshot: %+v", value)
	}
	value.Degree = 99
	if cache.get("Beijing").Degree != 23 {
		t.Fatal("caller mutated cached snapshot")
	}
}

func TestWeatherStaleSnapshotSurvivesFailureWithBoundedBackoff(t *testing.T) {
	clock := testWeatherClock()
	var calls atomic.Int64
	cache := newWeatherCache(clock.now, func(string) (model.Weather, error) { calls.Add(1); return model.Weather{}, errors.New("offline") })
	cache.location = "Beijing"
	cache.snapshot = model.Weather{Location: "Beijing", Degree: 12, Humidity: 50, Expires: clock.now().Add(-time.Second).Unix()}
	for _, expected := range []time.Duration{15 * time.Second, 30 * time.Second, time.Minute, 2 * time.Minute, 4 * time.Minute, 5 * time.Minute, 5 * time.Minute} {
		before := calls.Load()
		value := cache.get("Beijing")
		if value.Degree != 12 || value.Humidity != 50 {
			t.Fatal("stale snapshot lost")
		}
		waitWeatherIdle(t, cache)
		if calls.Load() != before+1 {
			t.Fatal("missing retry")
		}
		cache.mu.Lock()
		retry := cache.retryAt.Sub(clock.now())
		cache.mu.Unlock()
		if retry != expected {
			t.Fatalf("retry=%s, want %s", retry, expected)
		}
		for i := 0; i < 20; i++ {
			cache.get("Beijing")
		}
		if calls.Load() != before+1 {
			t.Fatal("failure caused retry storm")
		}
		clock.advance(expected)
	}
}

func TestWeatherLocationChangeDiscardsInflightOldCity(t *testing.T) {
	clock := testWeatherClock()
	release := make(chan struct{})
	started := make(chan struct{})
	var calls atomic.Int64
	cache := newWeatherCache(clock.now, func(location string) (model.Weather, error) {
		calls.Add(1)
		if location == "Beijing" {
			close(started)
			<-release
		}
		return model.Weather{Degree: 28, Expires: clock.now().Add(time.Minute).Unix()}, nil
	})
	cache.get("Beijing")
	<-started
	if value := cache.get("Shanghai"); value.Location != "" {
		t.Fatal("old city displayed as the new city")
	}
	if calls.Load() != 1 {
		t.Fatal("location change started overlapping fetch")
	}
	close(release)
	waitWeatherIdle(t, cache)
	cache.mu.Lock()
	stored := cache.snapshot
	cache.mu.Unlock()
	if stored.Location != "" {
		t.Fatal("old inflight result was published")
	}
	cache.get("Shanghai")
	waitWeatherIdle(t, cache)
	if value := cache.get("Shanghai"); value.Location != "Shanghai" || value.Degree != 28 {
		t.Fatalf("new city not refreshed: %+v", value)
	}
}

func TestWeatherSuccessfulRefreshClearsFailureBackoff(t *testing.T) {
	clock := testWeatherClock()
	cache := newWeatherCache(clock.now, func(string) (model.Weather, error) {
		return model.Weather{Degree: 21, Expires: clock.now().Add(time.Minute).Unix()}, nil
	})
	cache.location = "Beijing"
	cache.failures = 5
	cache.get("Beijing")
	waitWeatherIdle(t, cache)
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if cache.failures != 0 || !cache.retryAt.IsZero() {
		t.Fatal("success did not clear retry state")
	}
}

func TestWeatherWidgetWaitsForSnapshotOnHomeAndHelp(t *testing.T) {
	t.Chdir(t.TempDir())
	originalFlags := define.AppFlags
	originalCache := homepageWeather
	defer func() { define.AppFlags = originalFlags; homepageWeather = originalCache }()
	define.AppFlags.EnableOfflineMode = false
	options, err := data.GetAllSettingsOptions()
	require.NoError(t, err)
	options.ShowTitle = true
	options.ShowWeather = true
	options.Location = "Beijing"
	if !data.UpdateAppearance(options) || !data.UpdateWeatherAndLocation(true, "Beijing") {
		t.Fatal("prepare settings")
	}
	if !data.SaveFavoriteBookmarks(model.Bookmarks{}) || !data.SaveNormalBookmarks(model.Bookmarks{}) {
		t.Fatal("prepare bookmarks")
	}
	clock := testWeatherClock()
	release := make(chan struct{})
	homepageWeather = newWeatherCache(clock.now, func(string) (model.Weather, error) {
		<-release
		return model.Weather{Degree: 24, Humidity: 48, Expires: clock.now().Add(time.Minute).Unix()}, nil
	})
	cache := homepageWeather
	defer func() { close(release); waitWeatherIdle(t, cache) }()
	require.NoError(t, mdi.Init())
	views, err := templates.NewRenderer()
	require.NoError(t, err)
	app := fiber.New(fiber.Config{Views: views})
	app.Get("/", pageHome)
	app.Get("/help", renderHelp)
	request := func(path string) string {
		response, requestErr := app.Test(httptest.NewRequest("GET", path, nil))
		require.NoError(t, requestErr)
		defer response.Body.Close()
		body, readErr := io.ReadAll(response.Body)
		require.NoError(t, readErr)
		if response.StatusCode != 200 {
			t.Fatalf("%s returned %d: %s", path, response.StatusCode, body)
		}
		return string(body)
	}
	for _, path := range []string{"/", "/help"} {
		if body := request(path); strings.Contains(body, `id=plugin-weather`) || strings.Contains(body, `id="plugin-weather"`) {
			t.Fatalf("%s rendered fabricated cold weather", path)
		}
	}
	// Publish a complete same-city snapshot; both pages should render real values.
	cache.mu.Lock()
	cache.snapshot = model.Weather{Location: "Beijing", Degree: 24, Humidity: 48, Expires: clock.now().Add(time.Minute).Unix()}
	cache.mu.Unlock()
	for _, path := range []string{"/", "/help"} {
		body := request(path)
		require.True(t, strings.Contains(body, `id=plugin-weather`) || strings.Contains(body, `id="plugin-weather"`), "%s omitted weather widget", path)
		require.Contains(t, body, "24°C", path)
		require.Contains(t, body, "48%", path)
	}
	cache.mu.Lock()
	cache.location = "Shanghai"
	cache.snapshot = model.Weather{}
	cache.mu.Unlock()
	if !data.UpdateWeatherAndLocation(true, "Shanghai") {
		t.Fatal("change city")
	}
	for _, path := range []string{"/", "/help"} {
		if body := request(path); strings.Contains(body, `id=plugin-weather`) || strings.Contains(body, `id="plugin-weather"`) {
			t.Fatalf("%s rendered fabricated changed-city weather", path)
		}
	}
}
